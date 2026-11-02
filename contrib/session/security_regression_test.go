package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

func TestModifiedSessionDoesNotResurrectAfterConcurrentLogout(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	const id = "authenticated"
	if err := store.Save(id, &SessionData{Values: map[string]any{"user": "alice"}}, time.Hour); err != nil {
		t.Fatal(err)
	}

	loaded := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan struct{})
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	app.Post("/update", func(c *kruda.Ctx) error {
		GetSession(c).Set("last_page", "profile")
		close(loaded)
		<-finish
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Post("/logout", func(c *kruda.Ctx) error {
		GetSession(c).Destroy()
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Get("/profile", func(c *kruda.Ctx) error {
		if GetSession(c).GetString("user") == "" {
			return c.Status(http.StatusUnauthorized).JSON(kruda.Map{"error": "not logged in"})
		}
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	cookie := &http.Cookie{Name: "_session", Value: id}
	go func() {
		defer close(done)
		serveSessionRequest(app, http.MethodPost, "/update", cookie)
	}()
	<-loaded
	logout := serveSessionRequest(app, http.MethodPost, "/logout", cookie)
	close(finish)
	<-done
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d", logout.Code)
	}
	deleted := responseSessionCookie(t, logout, "_session")
	if deleted.MaxAge != -1 || deleted.Value != "" {
		t.Fatalf("logout cookie = %#v", deleted)
	}
	data, err := store.Get(id)
	if err != nil || data != nil {
		t.Fatalf("stale request recreated logged-out session: %#v, %v", data, err)
	}
	if got := serveSessionRequest(app, http.MethodGet, "/profile", cookie).Code; got != http.StatusUnauthorized {
		t.Fatalf("old cookie profile status = %d, want 401", got)
	}
}

func TestRegeneratePreventsValidAnonymousSessionFixation(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{
		Store:          store,
		CookiePath:     "/app",
		CookieDomain:   "example.com",
		CookieSecure:   true,
		CookieSameSite: http.SameSiteStrictMode,
	}))
	app.Get("/anonymous", func(c *kruda.Ctx) error {
		GetSession(c).Set("cart", "item")
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Post("/login", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if err := sess.Regenerate(); err != nil {
			return err
		}
		sess.Set("user", "alice")
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Get("/profile", func(c *kruda.Ctx) error {
		if GetSession(c).GetString("user") == "" {
			return c.Status(http.StatusUnauthorized).JSON(kruda.Map{"error": "not logged in"})
		}
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()

	anonymous := responseSessionCookie(t, serveSessionRequest(app, http.MethodGet, "/anonymous", nil), "_session")
	login := serveSessionRequest(app, http.MethodPost, "/login", anonymous)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	authenticated := responseSessionCookie(t, login, "_session")
	if authenticated.Value == "" || authenticated.Value == anonymous.Value {
		t.Fatalf("session ID did not rotate: anonymous=%q authenticated=%q", anonymous.Value, authenticated.Value)
	}
	if authenticated.Path != "/app" || authenticated.Domain != "example.com" || !authenticated.Secure || !authenticated.HttpOnly || authenticated.SameSite != http.SameSiteStrictMode {
		t.Fatalf("replacement cookie attributes = %#v", authenticated)
	}
	old, err := store.Get(anonymous.Value)
	if err != nil || old != nil {
		t.Fatalf("old anonymous ID remains in store: %#v, %v", old, err)
	}
	current, err := store.Get(authenticated.Value)
	if err != nil || current == nil || current.Values["cart"] != "item" || current.Values["user"] != "alice" {
		t.Fatalf("regenerated values = %#v, %v", current, err)
	}
	if got := serveSessionRequest(app, http.MethodGet, "/profile", anonymous).Code; got != http.StatusUnauthorized {
		t.Fatalf("planted anonymous cookie profile status = %d, want 401", got)
	}
	if got := serveSessionRequest(app, http.MethodGet, "/profile", authenticated).Code; got != http.StatusOK {
		t.Fatalf("regenerated cookie profile status = %d, want 200", got)
	}
}

func TestRegenerateNewSessionReplacesQueuedCookie(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	var id string
	app.Post("/login", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if err := sess.Regenerate(); err != nil {
			return err
		}
		id = sess.ID()
		sess.Set("user", "alice")
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/login", nil)
	cookie := responseSessionCookie(t, resp, "_session")
	if cookie.Value != id {
		t.Fatalf("cookie ID = %q, want %q", cookie.Value, id)
	}
	if cookies := resp.Result().Cookies(); len(cookies) != 1 {
		t.Fatalf("response cookies = %#v, want one replacement cookie", cookies)
	}
}

func TestRegenerateAfterResponseDoesNotChangeSession(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{"cart": "item"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	var regenerateErr error
	app.Get("/late", func(c *kruda.Ctx) error {
		if err := c.JSON(kruda.Map{"ok": true}); err != nil {
			return err
		}
		regenerateErr = GetSession(c).Regenerate()
		return nil
	})
	app.Compile()
	serveSessionRequest(app, http.MethodGet, "/late", &http.Cookie{Name: "_session", Value: "id"})
	if !errors.Is(regenerateErr, kruda.ErrAlreadyResponded) {
		t.Fatalf("Regenerate error = %v, want ErrAlreadyResponded", regenerateErr)
	}
	if data, err := store.Get("id"); err != nil || data == nil {
		t.Fatalf("late regeneration deleted current session: %#v, %v", data, err)
	}
}

type legacySessionStore struct {
	memory    *MemoryStore
	gets      int
	saves     int
	deletes   int
	deleteErr error
}

func (s *legacySessionStore) Get(id string) (*SessionData, error) {
	s.gets++
	return s.memory.Get(id)
}
func (s *legacySessionStore) Save(id string, data *SessionData, ttl time.Duration) error {
	s.saves++
	return s.memory.Save(id, data, ttl)
}
func (s *legacySessionStore) Delete(id string) error {
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.memory.Delete(id)
}

func TestLegacyStoreExistingSaveFailsClosed(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	if err := memory.Save("id", &SessionData{Values: map[string]any{"cart": "item"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	store := &legacySessionStore{memory: memory}
	data, err := store.Get("id")
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{id: "id", data: data, store: store, config: &Config{IdleTimeout: time.Hour}}
	sess.Set("user", "alice")
	if sess.ID() != "id" {
		t.Fatalf("Set rotated session ID to %q", sess.ID())
	}
	if err := sess.save(); !errors.Is(err, ErrAtomicStoreRequired) {
		t.Fatalf("save error = %v, want ErrAtomicStoreRequired", err)
	}
	if store.saves != 0 {
		t.Fatalf("unsafe legacy saves = %d", store.saves)
	}
	if data, err := store.Get("id"); err != nil || data == nil || data.Values["user"] != nil || data.Values["cart"] != "item" {
		t.Fatalf("failed closed save changed values = %#v, %v", data, err)
	}
}

func TestLegacyStoreExistingSessionRejectsBeforeEagerHandler(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	if err := memory.Save("id", &SessionData{Values: map[string]any{"cart": "item"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	store := &legacySessionStore{memory: memory}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	handlerCalled := false
	app.Post("/login", func(c *kruda.Ctx) error {
		handlerCalled = true
		GetSession(c).Set("user", "alice")
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/login", &http.Cookie{Name: "_session", Value: "id"})
	if handlerCalled || resp.Code != http.StatusInternalServerError {
		t.Fatalf("handler called = %v, response status = %d", handlerCalled, resp.Code)
	}
	if cookies := resp.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("failed regeneration replaced cookie: %#v", cookies)
	}
	if store.gets != 0 || store.saves != 0 || store.deletes != 0 {
		t.Fatalf("unsupported store accessed: gets=%d saves=%d deletes=%d", store.gets, store.saves, store.deletes)
	}
	if data, err := store.Get("id"); err != nil || data == nil || data.Values["cart"] != "item" {
		t.Fatalf("failed regeneration changed session: %#v, %v", data, err)
	}
	if store.saves != 0 || store.deletes != 0 {
		t.Fatalf("unsafe legacy operations: saves=%d deletes=%d", store.saves, store.deletes)
	}
}

func TestLegacyStoreNewSessionRejectsBeforeEagerHandler(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	store := &legacySessionStore{memory: memory}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	handlerCalled := false
	app.Post("/login", func(c *kruda.Ctx) error {
		handlerCalled = true
		GetSession(c).Set("user", "alice")
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/login", nil)
	if handlerCalled || resp.Code != http.StatusInternalServerError {
		t.Fatalf("handler called = %v, response status = %d", handlerCalled, resp.Code)
	}
	if store.gets != 0 || store.saves != 0 || store.deletes != 0 {
		t.Fatalf("unsupported store accessed: gets=%d saves=%d deletes=%d", store.gets, store.saves, store.deletes)
	}
	if cookies := resp.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("unsupported store issued cookies: %#v", cookies)
	}
}

func TestLegacyStoreSkippedRequestStillRunsHandler(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	store := &legacySessionStore{memory: memory}
	app := kruda.New()
	app.Use(New(Config{Store: store, Skip: func(string, string) bool { return true }}))
	handlerCalled := false
	app.Get("/health", func(c *kruda.Ctx) error {
		handlerCalled = true
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodGet, "/health", nil)
	if !handlerCalled || resp.Code != http.StatusOK {
		t.Fatalf("handler called = %v, response status = %d", handlerCalled, resp.Code)
	}
	if store.gets != 0 || store.saves != 0 || store.deletes != 0 || len(resp.Result().Cookies()) != 0 {
		t.Fatal("skipped request used session management")
	}
}

type updatingSessionStore struct{ *legacySessionStore }

func (s *updatingSessionStore) SaveExisting(id string, data *SessionData, ttl time.Duration) (bool, error) {
	return s.memory.SaveExisting(id, data, ttl)
}

func TestRegenerateNewSessionDoesNotDeleteMissingID(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	store := &updatingSessionStore{&legacySessionStore{memory: memory, deleteErr: errors.New("session not found")}}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	app.Post("/anonymous", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("cart", "item")
		if err := sess.Regenerate(); err != nil {
			return err
		}
		if data, err := store.Get(sess.ID()); err != nil || data == nil || data.Values["cart"] != "item" {
			t.Fatalf("regenerated session not persisted before response: %#v, %v", data, err)
		}
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/anonymous", nil)
	if resp.Code != http.StatusOK || store.deletes != 0 {
		t.Fatalf("regeneration status=%d deletes=%d", resp.Code, store.deletes)
	}
	responseSessionCookie(t, resp, "_session")
}

func TestRegenerateAfterOverlappingLogoutDoesNotReviveSession(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{"user": "alice"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	var regenerateErr error
	app.Post("/rotate", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if sess.GetString("user") != "alice" {
			t.Fatal("rotating request did not load authenticated session")
		}
		logout := serveSessionRequest(app, http.MethodPost, "/logout", &http.Cookie{Name: "_session", Value: sess.ID()})
		if logout.Code != http.StatusOK {
			t.Fatalf("overlapping logout status = %d", logout.Code)
		}
		regenerateErr = sess.Regenerate()
		if sess.ID() != "id" {
			t.Fatalf("failed regeneration changed ID to %q", sess.ID())
		}
		return regenerateErr
	})
	app.Post("/logout", func(c *kruda.Ctx) error {
		GetSession(c).Destroy()
		return c.JSON(kruda.Map{"ok": true})
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/rotate", &http.Cookie{Name: "_session", Value: "id"})
	if !errors.Is(regenerateErr, ErrSessionNotFound) {
		t.Fatalf("Regenerate error = %v, want ErrSessionNotFound", regenerateErr)
	}
	if store.Len() != 0 {
		t.Fatal("regeneration revived logged-out session")
	}
	if cookies := resp.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("failed regeneration issued cookies: %#v", cookies)
	}
}

func TestRegeneratedSessionSaveDoesNotUndoOverlappingLogout(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			store := NewMemoryStore()
			defer store.Close()
			var cookie *http.Cookie
			if existing {
				if err := store.Save("id", &SessionData{Values: map[string]any{"user": "alice"}}, time.Hour); err != nil {
					t.Fatal(err)
				}
				cookie = &http.Cookie{Name: "_session", Value: "id"}
			}
			app := kruda.New()
			app.Use(New(Config{Store: store}))
			app.Post("/rotate", func(c *kruda.Ctx) error {
				sess := GetSession(c)
				if err := sess.Regenerate(); err != nil {
					return err
				}
				sess.Set("user", "alice")
				logout := serveSessionRequest(app, http.MethodPost, "/logout", &http.Cookie{Name: "_session", Value: sess.ID()})
				if logout.Code != http.StatusOK {
					t.Fatalf("overlapping logout status = %d", logout.Code)
				}
				return c.JSON(kruda.Map{"ok": true})
			})
			app.Post("/logout", func(c *kruda.Ctx) error {
				GetSession(c).Destroy()
				return c.JSON(kruda.Map{"ok": true})
			})
			app.Compile()
			resp := serveSessionRequest(app, http.MethodPost, "/rotate", cookie)
			if resp.Code != http.StatusOK {
				t.Fatalf("rotation status = %d", resp.Code)
			}
			if store.Len() != 0 {
				t.Fatal("rotating request recreated logged-out session")
			}
		})
	}
}

type failingRegenerationStore struct {
	*MemoryStore
	err error
}

func (s *failingRegenerationStore) Regenerate(string, string, *SessionData, time.Duration) (bool, error) {
	return false, s.err
}

func TestRegenerateStoreErrorPreservesCurrentSession(t *testing.T) {
	memory := NewMemoryStore()
	defer memory.Close()
	if err := memory.Save("id", &SessionData{Values: map[string]any{"cart": "item"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("regeneration failed")
	store := &failingRegenerationStore{MemoryStore: memory, err: wantErr}
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	var regenerateErr error
	app.Post("/rotate", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		regenerateErr = sess.Regenerate()
		if sess.ID() != "id" {
			t.Fatalf("failed regeneration changed ID to %q", sess.ID())
		}
		return regenerateErr
	})
	app.Compile()
	resp := serveSessionRequest(app, http.MethodPost, "/rotate", &http.Cookie{Name: "_session", Value: "id"})
	if !errors.Is(regenerateErr, wantErr) {
		t.Fatalf("Regenerate error = %v, want %v", regenerateErr, wantErr)
	}
	if cookies := resp.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("failed regeneration replaced cookie: %#v", cookies)
	}
	if data, err := store.Get("id"); err != nil || data == nil || data.Values["cart"] != "item" {
		t.Fatalf("failed regeneration changed session: %#v, %v", data, err)
	}
}

func serveSessionRequest(app *kruda.App, method, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)
	return resp
}

func responseSessionCookie(t *testing.T, resp *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range resp.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie in response", name)
	return nil
}
