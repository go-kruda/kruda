package kruda

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestWithIdleTimeout(t *testing.T) {
	app := New(WithIdleTimeout(60 * time.Second))
	if app.config.IdleTimeout != 60*time.Second {
		t.Fatalf("expected 60s, got %v", app.config.IdleTimeout)
	}
}

func TestWithTransport(t *testing.T) {
	tr := transport.NewNetHTTP(transport.NetHTTPConfig{})
	app := New(WithTransport(tr))
	if app.transport == nil {
		t.Fatal("transport should not be nil")
	}
}

func TestWithLogger(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := New(WithLogger(l))
	if app.config.Logger != l {
		t.Fatal("logger not set")
	}
}

func TestWithJSONEncoderDecoder(t *testing.T) {
	called := false
	enc := func(v any) ([]byte, error) { called = true; return json.Marshal(v) }
	dec := func(data []byte, v any) error { return json.Unmarshal(data, v) }
	app := New(WithJSONEncoder(enc), WithJSONDecoder(dec))
	app.Get("/test", func(c *Ctx) error { return c.JSON(Map{"ok": true}) })
	app.Compile()
	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Fatal("custom encoder not called")
	}
	if app.config.JSONDecoder == nil {
		t.Fatal("decoder not set")
	}
}

func TestWithTrustProxy(t *testing.T) {
	app := New(WithTrustProxy(true))
	if !app.config.TrustProxy {
		t.Fatal("TrustProxy should be true")
	}
}

func TestWithHTTP3(t *testing.T) {
	app := New(WithHTTP3("cert.pem", "key.pem"))
	if !app.config.HTTP3 {
		t.Fatal("HTTP3 should be true")
	}
	if app.config.TLSCertFile != "cert.pem" {
		t.Fatal("cert file not set")
	}
}

func TestCtxParamInt(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		id, err := c.ParamInt("id")
		if err != nil {
			return BadRequest("invalid id")
		}
		return c.JSON(Map{"id": id})
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/users/42")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/users/abc")
	if resp.StatusCode() != 400 {
		t.Fatalf("expected 400 for non-numeric, got %d", resp.StatusCode())
	}
}

func TestCtxIP(t *testing.T) {
	app := New()
	app.Get("/ip", func(c *Ctx) error { return c.Text(c.IP()) })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/ip")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

func TestCtxStatusCode(t *testing.T) {
	app := New()
	app.Get("/status", func(c *Ctx) error {
		c.Status(201)
		if c.StatusCode() != 201 {
			return InternalError("status not set")
		}
		return c.JSON(Map{"status": c.StatusCode()})
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/status")
	if resp.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode())
	}
}

func TestCtxRedirect(t *testing.T) {
	app := New()
	app.Get("/old", func(c *Ctx) error { return c.Redirect("/new") })
	app.Get("/perm", func(c *Ctx) error { return c.Redirect("/new", 301) })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/old")
	if resp.StatusCode() != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/perm")
	if resp.StatusCode() != 301 {
		t.Fatalf("expected 301, got %d", resp.StatusCode())
	}
}

func TestCtxStream(t *testing.T) {
	app := New()
	app.Get("/stream", func(c *Ctx) error {
		c.SetHeader("Content-Type", "text/plain")
		return c.Stream(strings.NewReader("streamed data"))
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/stream")
	if resp.BodyString() != "streamed data" {
		t.Fatalf("expected 'streamed data', got %q", resp.BodyString())
	}
}

func TestCtxLatency(t *testing.T) {
	app := New()
	app.Get("/latency", func(c *Ctx) error {
		// Windows timer resolution is ~15ms; latency may be 0 for fast handlers.
		if c.Latency() < 0 {
			return InternalError("latency should be non-negative")
		}
		return c.Text("ok")
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/latency")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

type ctxKey string

func TestCtxSetContext(t *testing.T) {
	app := New()
	app.Get("/ctx", func(c *Ctx) error {
		ctx := context.WithValue(c.Context(), ctxKey("key"), "value")
		c.SetContext(ctx)
		if c.Context().Value(ctxKey("key")) != "value" {
			return InternalError("context value not set")
		}
		return c.Text("ok")
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/ctx")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

func TestCtxLog(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	app := New(WithLogger(l))
	app.Get("/log", func(c *Ctx) error {
		c.Log().Info("test message")
		return c.Text("ok")
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/log")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if !strings.Contains(buf.String(), "test message") {
		t.Fatal("log message not found")
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) *KrudaError
		code int
	}{
		{"Unauthorized", Unauthorized, 401},
		{"Forbidden", Forbidden, 403},
		{"Conflict", Conflict, 409},
		{"UnprocessableEntity", UnprocessableEntity, 422},
		{"TooManyRequests", TooManyRequests, 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("test")
			if err.Code != tt.code {
				t.Fatalf("expected code %d, got %d", tt.code, err.Code)
			}
		})
	}
}

func TestTypedHandlerMethodVariants(t *testing.T) {
	type In struct {
		ID int `param:"id"`
	}
	type Out struct {
		ID int `json:"id"`
	}

	app := New()
	PutX[In, Out](app, "/put/:id", func(c *C[In]) *Out {
		return &Out{ID: c.In.ID}
	})
	DeleteX[In, Out](app, "/delete/:id", func(c *C[In]) *Out {
		return &Out{ID: c.In.ID}
	})
	PatchX[In, Out](app, "/patch/:id", func(c *C[In]) *Out {
		return &Out{ID: c.In.ID}
	})
	app.Compile()
	tc := NewTestClient(app)

	resp, _ := tc.Put("/put/1", nil)
	if resp.StatusCode() != 200 {
		t.Fatalf("PutX: expected 200, got %d", resp.StatusCode())
	}
	resp, _ = tc.Delete("/delete/1")
	if resp.StatusCode() != 200 {
		t.Fatalf("DeleteX: expected 200, got %d", resp.StatusCode())
	}
	resp, _ = tc.Patch("/patch/1", nil)
	if resp.StatusCode() != 200 {
		t.Fatalf("PatchX: expected 200, got %d", resp.StatusCode())
	}
}

func TestGroupTypedHandlers(t *testing.T) {
	type In struct{}
	type Out struct {
		Method string `json:"method"`
	}

	app := New()
	g := app.Group("/api")
	GroupGet[In, Out](g, "/get", func(c *C[In]) (*Out, error) {
		return &Out{Method: "GET"}, nil
	})
	GroupPost[In, Out](g, "/post", func(c *C[In]) (*Out, error) {
		return &Out{Method: "POST"}, nil
	})
	GroupPut[In, Out](g, "/put", func(c *C[In]) (*Out, error) {
		return &Out{Method: "PUT"}, nil
	})
	GroupDelete[In, Out](g, "/delete", func(c *C[In]) (*Out, error) {
		return &Out{Method: "DELETE"}, nil
	})
	GroupPatch[In, Out](g, "/patch", func(c *C[In]) (*Out, error) {
		return &Out{Method: "PATCH"}, nil
	})
	app.Compile()
	tc := NewTestClient(app)

	for _, tt := range []struct{ method, path string }{
		{"GET", "/api/get"},
		{"POST", "/api/post"},
		{"PUT", "/api/put"},
		{"DELETE", "/api/delete"},
		{"PATCH", "/api/patch"},
	} {
		resp, _ := tc.Request(tt.method, tt.path).Send()
		if resp.StatusCode() != 200 {
			t.Fatalf("%s %s: expected 200, got %d", tt.method, tt.path, resp.StatusCode())
		}
	}
}

func TestAppOptionsHead(t *testing.T) {
	app := New()
	app.Options("/opts", func(c *Ctx) error { return c.NoContent() })
	app.Head("/head", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Options("/opts")
	if resp.StatusCode() != 204 {
		t.Fatalf("OPTIONS: expected 204, got %d", resp.StatusCode())
	}
	resp, _ = tc.Head("/head")
	if resp.StatusCode() != 200 {
		t.Fatalf("HEAD: expected 200, got %d", resp.StatusCode())
	}
}

func TestContainerMustUse(t *testing.T) {
	c := NewContainer()
	c.Give(&struct{ Name string }{Name: "test"})
	val := MustUse[*struct{ Name string }](c)
	if val.Name != "test" {
		t.Fatalf("expected 'test', got %q", val.Name)
	}
}

func TestContainerMustUseNamed(t *testing.T) {
	c := NewContainer()
	c.GiveNamed("primary", &struct{ DSN string }{DSN: "primary"})
	val := MustUseNamed[*struct{ DSN string }](c, "primary")
	if val.DSN != "primary" {
		t.Fatalf("expected 'primary', got %q", val.DSN)
	}
}

func TestContainerMustUsePanics(t *testing.T) {
	c := NewContainer()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustUse on missing type")
		}
	}()
	MustUse[*struct{ X int }](c)
}

func TestResolveNamed(t *testing.T) {
	container := NewContainer()
	container.GiveNamed("db", &struct{ DSN string }{DSN: "test"})
	app := New(WithContainer(container))
	app.Get("/resolve", func(c *Ctx) error {
		val, err := ResolveNamed[*struct{ DSN string }](c, "db")
		if err != nil {
			return err
		}
		return c.Text(val.DSN)
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/resolve")
	if resp.BodyString() != "test" {
		t.Fatalf("expected 'test', got %q", resp.BodyString())
	}
}

func TestMustResolveNamed(t *testing.T) {
	container := NewContainer()
	container.GiveNamed("db", &struct{ DSN string }{DSN: "named"})
	app := New(WithContainer(container))
	app.Get("/must", func(c *Ctx) error {
		val := MustResolveNamed[*struct{ DSN string }](c, "db")
		return c.Text(val.DSN)
	})
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/must")
	if resp.BodyString() != "named" {
		t.Fatalf("expected 'named', got %q", resp.BodyString())
	}
}

func TestTestClientPatchHeadOptions(t *testing.T) {
	app := New()
	app.Patch("/p", func(c *Ctx) error { return c.Text("patched") })
	app.Head("/h", func(c *Ctx) error { return c.Text("head") })
	app.Options("/o", func(c *Ctx) error { return c.NoContent() })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Patch("/p", nil)
	if resp.StatusCode() != 200 {
		t.Fatalf("PATCH: expected 200, got %d", resp.StatusCode())
	}
	resp, _ = tc.Head("/h")
	if resp.StatusCode() != 200 {
		t.Fatalf("HEAD: expected 200, got %d", resp.StatusCode())
	}
	resp, _ = tc.Options("/o")
	if resp.StatusCode() != 204 {
		t.Fatalf("OPTIONS: expected 204, got %d", resp.StatusCode())
	}
}

func TestTestResponseBody(t *testing.T) {
	app := New()
	app.Get("/body", func(c *Ctx) error { return c.Text("raw body") })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/body")
	if string(resp.Body()) != "raw body" {
		t.Fatalf("expected 'raw body', got %q", string(resp.Body()))
	}
}

type covUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type covUserService struct {
	users map[string]covUser
}

func (s *covUserService) List(_ context.Context, page, limit int) ([]covUser, int, error) {
	var items []covUser
	for _, u := range s.users {
		items = append(items, u)
	}
	return items, len(items), nil
}

func (s *covUserService) Get(_ context.Context, id string) (covUser, error) {
	u, ok := s.users[id]
	if !ok {
		return covUser{}, NotFound("not found")
	}
	return u, nil
}

func (s *covUserService) Create(_ context.Context, item covUser) (covUser, error) {
	item.ID = "new"
	s.users[item.ID] = item
	return item, nil
}

func (s *covUserService) Update(_ context.Context, id string, item covUser) (covUser, error) {
	item.ID = id
	s.users[id] = item
	return item, nil
}

func (s *covUserService) Delete(_ context.Context, id string) error {
	delete(s.users, id)
	return nil
}

func TestWithResourceIDParam(t *testing.T) {
	app := New()
	svc := &covUserService{users: map[string]covUser{"1": {ID: "1", Name: "Alice"}}}
	Resource(app, "/items", svc, WithResourceIDParam("item_id"))
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/items/1")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

func TestAppValidatorLazyInit(t *testing.T) {
	app := New()
	v := app.Validator()
	if v == nil {
		t.Fatal("Validator() should lazy-init")
	}
	if v2 := app.Validator(); v != v2 {
		t.Fatal("Validator() should return cached instance")
	}
}

func TestAltSvcMiddleware(t *testing.T) {
	mw := altSvcMiddleware(":3000")
	app := New()
	app.Use(mw)
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	tc := NewTestClient(app)
	resp, _ := tc.Get("/")
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}
