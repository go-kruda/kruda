package session

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

var errStoreDown = errors.New("store down")

type downStore struct{ *MemoryStore }

func (s downStore) SaveExisting(string, *SessionData, time.Duration) (bool, error) {
	return false, errStoreDown
}

func (s downStore) Save(string, *SessionData, time.Duration) error { return errStoreDown }

func TestModifiedSaveAfterDestroyReportsNotFound(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	s := Session{id: "id", data: &SessionData{Values: map[string]any{}}, store: store, config: &Config{IdleTimeout: time.Minute}}
	s.Set("theme", "dark")
	if err := store.Delete("id"); err != nil { // another request logged out
		t.Fatal(err)
	}
	if err := s.save(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("save after destroy = %v, want ErrSessionNotFound", err)
	}
	if store.Len() != 0 {
		t.Fatal("save recreated a destroyed session")
	}
}

func TestSaveLetsTheHandlerReportAFailedWrite(t *testing.T) {
	store := downStore{NewMemoryStore()}
	defer store.Close()

	app := kruda.New()
	app.Use(New(Config{Store: store}))
	app.Post("/prefs", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("theme", "dark")
		if err := sess.Save(); err != nil {
			return err
		}
		return c.Text("saved")
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("POST", "/prefs", nil))
	if resp.statusCode != 500 || string(resp.body) == "saved" {
		t.Fatalf("got %d %q, want 500 and no success body", resp.statusCode, resp.body)
	}
}

func TestSaveThenMiddlewareDoesNotWriteTwice(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	s := Session{id: "id", data: &SessionData{Values: map[string]any{}}, store: store, config: &Config{IdleTimeout: time.Minute}}
	s.Set("theme", "dark")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if s.modified {
		t.Fatal("Save left the session marked modified")
	}
}
