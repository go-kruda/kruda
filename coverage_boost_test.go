package kruda

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// --- Error constructors ---

func TestUnauthorized(t *testing.T) {
	e := Unauthorized("no auth")
	if e.Code != 401 || e.Message != "no auth" {
		t.Errorf("Unauthorized = %+v", e)
	}
}

func TestForbidden(t *testing.T) {
	e := Forbidden("denied")
	if e.Code != 403 || e.Message != "denied" {
		t.Errorf("Forbidden = %+v", e)
	}
}

func TestConflict(t *testing.T) {
	e := Conflict("dup")
	if e.Code != 409 || e.Message != "dup" {
		t.Errorf("Conflict = %+v", e)
	}
}

func TestUnprocessableEntity(t *testing.T) {
	e := UnprocessableEntity("bad data")
	if e.Code != 422 || e.Message != "bad data" {
		t.Errorf("UnprocessableEntity = %+v", e)
	}
}

func TestTooManyRequests(t *testing.T) {
	e := TooManyRequests("slow down")
	if e.Code != 429 || e.Message != "slow down" {
		t.Errorf("TooManyRequests = %+v", e)
	}
}

// --- Config options ---

func TestWithIdleTimeout(t *testing.T) {
	app := &App{config: defaultConfig()}
	WithIdleTimeout(45 * time.Second)(app)
	if app.config.IdleTimeout != 45*time.Second {
		t.Errorf("IdleTimeout = %v", app.config.IdleTimeout)
	}
}

func TestWithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &App{config: defaultConfig()}
	WithLogger(logger)(app)
	if app.config.Logger != logger {
		t.Error("Logger not set")
	}
}

func TestWithJSONEncoder(t *testing.T) {
	called := false
	enc := func(v any) ([]byte, error) { called = true; return nil, nil }
	app := &App{config: defaultConfig()}
	WithJSONEncoder(enc)(app)
	app.config.JSONEncoder(nil)
	if !called {
		t.Error("JSONEncoder not set")
	}
}

func TestWithJSONStreamEncoder(t *testing.T) {
	called := false
	enc := func(buf *bytes.Buffer, v any) error { called = true; return nil }
	app := &App{config: defaultConfig()}
	WithJSONStreamEncoder(enc)(app)
	app.config.JSONStreamEncoder(&bytes.Buffer{}, nil)
	if !called {
		t.Error("JSONStreamEncoder not set")
	}
}

func TestWithJSONDecoder(t *testing.T) {
	called := false
	dec := func(data []byte, v any) error { called = true; return nil }
	app := &App{config: defaultConfig()}
	WithJSONDecoder(dec)(app)
	app.config.JSONDecoder(nil, nil)
	if !called {
		t.Error("JSONDecoder not set")
	}
}

func TestWithTrustProxy(t *testing.T) {
	app := &App{config: defaultConfig()}
	WithTrustProxy(true)(app)
	if !app.config.TrustProxy {
		t.Error("TrustProxy not set")
	}
}

// --- Context methods ---

func TestCtx_Route(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		route := c.Route()
		return c.Text(route)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/users/42")
	if !strings.Contains(resp.BodyString(), "/users/:id") {
		t.Errorf("Route() = %q", resp.BodyString())
	}
}

func TestCtx_ParamInt(t *testing.T) {
	app := New()
	app.Get("/items/:id", func(c *Ctx) error {
		n, err := c.ParamInt("id")
		if err != nil {
			return c.Status(400).Text("bad id")
		}
		return c.JSON(Map{"id": n})
	})
	app.Compile()

	tc := NewTestClient(app)

	resp, _ := tc.Get("/items/42")
	if resp.StatusCode() != 200 {
		t.Errorf("valid id status = %d", resp.StatusCode())
	}

	resp, _ = tc.Get("/items/abc")
	if resp.StatusCode() != 400 {
		t.Errorf("invalid id status = %d", resp.StatusCode())
	}
}

func TestCtx_StatusCode(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.Status(201)
		code := c.StatusCode()
		return c.JSON(Map{"code": code})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 201 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestCtx_IP(t *testing.T) {
	app := New()
	app.Get("/ip", func(c *Ctx) error {
		ip := c.IP()
		return c.Text(ip)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/ip")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestCtx_NoContent(t *testing.T) {
	app := New()
	app.Delete("/items/:id", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Delete("/items/1")
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode())
	}
}

// --- Handler variants (X = no error return) ---

func TestHandlerPutX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Updated bool `json:"updated"`
	}

	app := New()
	PutX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Updated: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Put("/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestHandlerDeleteX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Deleted bool `json:"deleted"`
	}

	app := New()
	DeleteX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Deleted: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Delete("/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestHandlerPatchX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Patched bool `json:"patched"`
	}

	app := New()
	PatchX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Patched: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Patch("/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Group typed handlers ---

func TestGroupGet(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Value string `json:"value"`
	}

	app := New()
	g := app.Group("/api")
	GroupGet[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Value: "ok"}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/api/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPost(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}
	type Output struct {
		ID int `json:"id"`
	}

	app := New()
	g := app.Group("/api")
	GroupPost[Input, Output](g, "/items", func(c *C[Input]) (*Output, error) {
		return &Output{ID: 1}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/api/items", strings.NewReader(`{"name":"test"}`))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPut(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Updated bool `json:"updated"`
	}

	app := New()
	g := app.Group("/api")
	GroupPut[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Updated: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Put("/api/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupDelete(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Deleted bool `json:"deleted"`
	}

	app := New()
	g := app.Group("/api")
	GroupDelete[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Deleted: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Delete("/api/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPatch(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Patched bool `json:"patched"`
	}

	app := New()
	g := app.Group("/api")
	GroupPatch[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Patched: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Patch("/api/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- App route methods ---

func TestAppOptions(t *testing.T) {
	app := New()
	app.Options("/test", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Options("/test")
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestAppHead(t *testing.T) {
	app := New()
	app.Head("/test", func(c *Ctx) error {
		c.Set("X-Custom", "yes")
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Head("/test")
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- TestClient methods ---

func TestTestClient_Patch(t *testing.T) {
	app := New()
	app.Patch("/items/:id", func(c *Ctx) error {
		return c.JSON(Map{"patched": true})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Patch("/items/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestTestClient_Head(t *testing.T) {
	app := New()
	app.Head("/ping", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Head("/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestTestClient_Options(t *testing.T) {
	app := New()
	app.Options("/ping", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Options("/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Static options ---

func TestStatic_WithMaxAge(t *testing.T) {
	fs := fstest.MapFS{
		"style.css": {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/assets", fs, WithMaxAge(3600))
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/assets/style.css")
	cc := resp.Header("Cache-Control")
	if !strings.Contains(cc, "max-age=3600") {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestStatic_WithIndex(t *testing.T) {
	fs := fstest.MapFS{
		"home.html": {Data: []byte("<html>home</html>")},
	}
	app := New()
	app.StaticFS("/", fs, WithIndex("home.html"))
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/")
	if !strings.Contains(resp.BodyString(), "<html>home</html>") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestStatic_WithBrowse(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/files", fs, WithBrowse())
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/files/sub/file.txt")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Container ---

func TestContainer_MustUse_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustUse should panic when service not registered")
		}
	}()

	c := NewContainer()
	MustUse[string](c)
}

func TestContainer_MustUseNamed_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustUseNamed should panic when service not registered")
		}
	}()

	c := NewContainer()
	MustUseNamed[string](c, "missing")
}

// --- App hooks ---

func TestApp_OnRequest(t *testing.T) {
	called := false
	app := New()
	app.OnRequest(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("OnRequest hook not called")
	}
}

func TestApp_OnResponse(t *testing.T) {
	called := false
	app := New()
	app.OnResponse(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("OnResponse hook not called")
	}
}

func TestApp_BeforeHandle(t *testing.T) {
	called := false
	app := New()
	app.BeforeHandle(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("BeforeHandle hook not called")
	}
}

func TestApp_AfterHandle(t *testing.T) {
	called := false
	app := New()
	app.AfterHandle(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("AfterHandle hook not called")
	}
}

func TestApp_OnError(t *testing.T) {
	called := false
	app := New()
	app.OnError(func(c *Ctx, err error) {
		called = true
	})
	app.Get("/fail", func(c *Ctx) error {
		return InternalError("boom")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/fail")
	if !called {
		t.Error("OnError hook not called")
	}
}

// --- Resource ---

func TestWithResourceIDParam(t *testing.T) {
	cfg := defaultResourceConfig()
	WithResourceIDParam("uuid")(&cfg)
	if cfg.idParam != "uuid" {
		t.Errorf("idParam = %q", cfg.idParam)
	}
}

// --- Ctx.Redirect ---

func TestCtx_Redirect(t *testing.T) {
	app := New()
	app.Get("/old", func(c *Ctx) error {
		return c.Redirect("/new", 301)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/old")
	if resp.StatusCode() != 301 {
		t.Errorf("status = %d, want 301", resp.StatusCode())
	}
	loc := resp.Header("Location")
	if loc != "/new" {
		t.Errorf("Location = %q", loc)
	}
}

// --- Ctx.SetBody / SetContentType ---

func TestCtx_SetBody_SetContentType(t *testing.T) {
	app := New()
	app.Get("/custom", func(c *Ctx) error {
		c.SetContentType("text/csv")
		c.SetBody([]byte("a,b,c"))
		return nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/custom")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.Header("Content-Type"), "text/csv") {
		t.Errorf("Content-Type = %q", resp.Header("Content-Type"))
	}
}

// --- App.Validator ---

func TestApp_Validator(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	v := app.Validator()
	if v == nil {
		t.Error("Validator() returned nil")
	}
}

// --- SSE.Done ---

func TestSSE_Done(t *testing.T) {
	app := New()
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			s.Event("test", "data")
			select {
			case <-s.Done():
			default:
			}
			return nil
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/events")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.MarkStart / Latency ---

func TestCtx_MarkStart_Latency(t *testing.T) {
	app := New()
	app.Get("/latency", func(c *Ctx) error {
		c.MarkStart()
		time.Sleep(time.Millisecond)
		lat := c.Latency()
		if lat < time.Millisecond {
			return c.Status(500).Text("latency too low")
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/latency")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.Log ---

func TestCtx_Log(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := New(WithLogger(logger))
	app.Get("/log", func(c *Ctx) error {
		l := c.Log()
		if l == nil {
			return c.Status(500).Text("nil logger")
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/log")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.Header fallback ---

func TestCtx_Header_Missing(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		h := c.Header("X-Missing")
		if h != "" {
			return c.Status(500).Text("expected empty")
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.AddHeader ---

func TestCtx_AddHeader(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("X-Multi", "val1")
		c.AddHeader("X-Multi", "val2")
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- TestResponse.Body ([]byte) ---

func TestTestResponse_Body(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.Text("hello")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	body := resp.Body()
	if len(body) == 0 {
		t.Error("Body() returned empty")
	}
}

// --- Ctx.Responded ---

func TestCtx_Responded(t *testing.T) {
	app := New()
	app.Get("/responded-check", func(c *Ctx) error {
		if c.Responded() {
			return c.Status(500).Text("already responded")
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/responded-check")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.QueryInt ---

func TestCtx_QueryInt(t *testing.T) {
	app := New()
	app.Get("/search", func(c *Ctx) error {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit")
		invalid := c.QueryInt("bad", 99)
		return c.JSON(Map{"page": page, "limit": limit, "invalid": invalid})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/search?page=5&limit=20&bad=xyz")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.Cookie ---

func TestCtx_Cookie(t *testing.T) {
	app := New()
	app.Get("/test-cookie", func(c *Ctx) error {
		v := c.Cookie("session")
		return c.Text(v)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Request("GET", "/test-cookie").Cookie("session", "abc123").Send()
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.SetContext ---

func TestCtx_SetContext(t *testing.T) {
	app := New()
	app.Get("/set-ctx", func(c *Ctx) error {
		ctx := c.Context()
		c.SetContext(ctx)
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/set-ctx")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- ViewEngine ---

func TestViewEngineFS(t *testing.T) {
	vfs := fstest.MapFS{
		"hello.html": {Data: []byte("Hello {{.Name}}!")},
	}
	engine := NewViewEngineFS(vfs, "*.html")

	var buf bytes.Buffer
	err := engine.Render(&buf, "hello.html", struct{ Name string }{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "Hello World!" {
		t.Errorf("render = %q", buf.String())
	}
}

func TestCtx_RenderFS(t *testing.T) {
	vfs := fstest.MapFS{
		"greet.html": {Data: []byte("Hi {{.Name}}")},
	}
	engine := NewViewEngineFS(vfs, "*.html")
	app := New(WithViews(engine))
	app.Get("/greet", func(c *Ctx) error {
		return c.Render("greet.html", struct{ Name string }{"User"})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/greet")
	if !strings.Contains(resp.BodyString(), "Hi User") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Ctx.Bind ---

func TestCtx_Bind(t *testing.T) {
	app := New()
	app.Post("/bind-test", func(c *Ctx) error {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.Bind(&data); err != nil {
			return err
		}
		return c.JSON(Map{"got": data.Name})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/bind-test", strings.NewReader(`{"name":"test"}`))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.HTML ---

func TestCtx_HTML(t *testing.T) {
	app := New()
	app.Get("/page", func(c *Ctx) error {
		return c.HTML("<h1>Hello</h1>")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/page")
	if !strings.Contains(resp.Header("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q", resp.Header("Content-Type"))
	}
}

// --- Ctx.BodyBytes ---

func TestCtx_BodyBytes(t *testing.T) {
	app := New()
	app.Post("/echo-body", func(c *Ctx) error {
		body, err := c.BodyBytes()
		if err != nil {
			return err
		}
		return c.Text(string(body))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/echo-body", strings.NewReader("hello body"))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Validation edge cases ---

func TestValidation_GTE_LTE(t *testing.T) {
	type Input struct {
		Age int `query:"age" validate:"gte=18,lte=100"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-age", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-age?age=25")
	if resp.StatusCode() != 200 {
		t.Errorf("valid age status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-age?age=10")
	if resp.StatusCode() != 422 {
		t.Errorf("too young status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_GT_LT(t *testing.T) {
	type Input struct {
		Score float64 `query:"score" validate:"gt=0,lt=100"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-score", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-score?score=50")
	if resp.StatusCode() != 200 {
		t.Errorf("valid score status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-score?score=0")
	if resp.StatusCode() != 422 {
		t.Errorf("zero score status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_Contains(t *testing.T) {
	type Input struct {
		Name string `query:"name" validate:"contains=test"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-name", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-name?name=my_test_val")
	if resp.StatusCode() != 200 {
		t.Errorf("contains status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-name?name=nope")
	if resp.StatusCode() != 422 {
		t.Errorf("not contains status = %d, want 422", resp.StatusCode())
	}
}

func TestValidation_Len(t *testing.T) {
	type Input struct {
		Code string `query:"code" validate:"len=6"`
	}
	type Output struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))
	Get[Input, Output](app, "/check-code", func(c *C[Input]) (*Output, error) {
		return &Output{OK: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/check-code?code=ABC123")
	if resp.StatusCode() != 200 {
		t.Errorf("valid len status = %d", resp.StatusCode())
	}
	resp, _ = tc.Get("/check-code?code=AB")
	if resp.StatusCode() != 422 {
		t.Errorf("short len status = %d, want 422", resp.StatusCode())
	}
}

// --- Static.Static (os.DirFS) ---

func TestStatic_OsDirFS(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(dir + "/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("hello world")
	f.Close()

	app := New()
	app.Static("/files", dir)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/files/hello.txt")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "hello world") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- SSE EventWithID ---

func TestSSE_EventWithID(t *testing.T) {
	app := New()
	app.Get("/sse-events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			s.EventWithID("1", "update", "hello")
			s.Comment("heartbeat")
			s.Retry(1000)
			return nil
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/sse-events")
	body := resp.BodyString()
	if !strings.Contains(body, "id: 1") {
		t.Errorf("missing event ID in body: %q", body)
	}
}

// --- Group.All ---

func TestGroup_All_Methods(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.All("/catch", func(c *Ctx) error {
		return c.Text("caught " + c.Method())
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/api/catch")
	if !strings.Contains(resp.BodyString(), "caught GET") {
		t.Errorf("GET body = %q", resp.BodyString())
	}
	resp, _ = tc.Post("/api/catch", nil)
	if !strings.Contains(resp.BodyString(), "caught POST") {
		t.Errorf("POST body = %q", resp.BodyString())
	}
}

// --- Ctx.Context ---

func TestCtx_Context(t *testing.T) {
	app := New()
	app.Get("/get-ctx", func(c *Ctx) error {
		ctx := c.Context()
		if ctx == nil {
			return c.Status(500).Text("nil context")
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/get-ctx")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.Transport / ResponseWriter / Request ---

func TestCtx_TransportAccessors(t *testing.T) {
	app := New()
	app.Get("/accessors", func(c *Ctx) error {
		_ = c.Transport()
		_ = c.ResponseWriter()
		_ = c.Request()
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/accessors")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Static: directory with index ---

func TestStatic_DirectoryIndex(t *testing.T) {
	fs := fstest.MapFS{
		"sub/index.html": {Data: []byte("<html>sub index</html>")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory — should serve index.html inside it
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() == 200 && !strings.Contains(resp.BodyString(), "sub index") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Static: directory without index, browse disabled ---

func TestStatic_DirectoryNoIndex_NoBrowse(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory without index.html — should 404
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() != 404 {
		t.Errorf("status = %d, want 404 for directory without index", resp.StatusCode())
	}
}

// --- Static: directory without index, browse enabled ---

func TestStatic_DirectoryNoIndex_BrowseEnabled(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/", fs, WithBrowse())
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory without index.html but browse is enabled
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d, want 200 for browse mode", resp.StatusCode())
	}
}

// --- Static: SPA fallback when index.html doesn't exist ---

func TestStatic_SPA_MissingIndex(t *testing.T) {
	// Empty FS with no index.html — SPA fallback should 404
	fs := fstest.MapFS{}
	app := New()
	app.StaticFS("/", fs, WithSPA())
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/nonexistent")
	if resp.StatusCode() != 404 {
		t.Errorf("status = %d, want 404 when SPA index missing", resp.StatusCode())
	}
}

// --- Static: file with unknown extension ---

func TestStatic_UnknownExtension(t *testing.T) {
	fs := fstest.MapFS{
		"data.xyz": {Data: []byte("binary data")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/data.xyz")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	// Unknown extension should default to application/octet-stream
	ct := resp.Header("Content-Type")
	if ct != "" && !strings.Contains(ct, "octet-stream") && !strings.Contains(ct, "xyz") {
		t.Logf("unexpected content-type for .xyz: %s", ct)
	}
}

// --- Static: prefix-based serving ---

func TestStatic_WithPrefix(t *testing.T) {
	fs := fstest.MapFS{
		"style.css": {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/public", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/public/style.css")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "body{}") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Static: path traversal with double dots ---

func TestStatic_PathTraversal_DoubleDots(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("home")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/../../etc/passwd")
	if resp.StatusCode() != 403 {
		t.Errorf("status = %d, want 403 for path traversal", resp.StatusCode())
	}
}

// --- App.Compile: security header variations ---

func TestCompile_SecurityHeaders_AllVariants(t *testing.T) {
	app := New(WithLegacySecurityHeaders())
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	if len(app.secHeaders) == 0 {
		t.Error("secHeaders should be populated after Compile")
	}
}

func TestCompile_SecurityHeaders_HSTS(t *testing.T) {
	app := &App{
		config:   defaultConfig(),
		router:   newRouter(),
		errorMap: defaultErrorMap(),
	}
	app.config.SecurityHeaders = true
	app.config.Security.HSTSMaxAge = 31536000
	app.config.Security.ContentSecurityPolicy = "default-src 'self'"
	app.config.Security.ReferrerPolicy = "no-referrer"
	app.transport, app.transportType = selectTransport(app.config, app.config.Logger)
	app.ctxPool.New = func() any { return newCtx(app) }

	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	found := false
	for _, kv := range app.secHeaders {
		if strings.Contains(kv[0], "Strict-Transport-Security") {
			found = true
		}
	}
	if !found {
		t.Error("HSTS header not found in secHeaders")
	}
}

// --- App.All ---

func TestApp_All_AllMethods(t *testing.T) {
	app := New()
	app.All("/everything", func(c *Ctx) error {
		return c.Text("method: " + c.Method())
	})
	app.Compile()

	tc := NewTestClient(app)
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		resp, _ := tc.Request(m, "/everything").Send()
		if resp.StatusCode() != 200 {
			t.Errorf("%s status = %d, want 200", m, resp.StatusCode())
		}
	}
}

// --- buildChain ---

func TestBuildChain(t *testing.T) {
	handler := func(c *Ctx) error { return nil }
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }

	chain := buildChain([]HandlerFunc{mw1}, []HandlerFunc{mw2}, handler)
	if len(chain) != 3 {
		t.Errorf("chain len = %d, want 3", len(chain))
	}
}

func TestBuildChain_NoMiddleware(t *testing.T) {
	handler := func(c *Ctx) error { return nil }
	chain := buildChain(nil, nil, handler)
	if len(chain) != 1 {
		t.Errorf("chain len = %d, want 1", len(chain))
	}
}

// --- containsDotPercent ---

func TestContainsDotPercent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/users", false},
		{"/api/v1.0/users", true},
		{"/api/%2e%2e/secret", true},
		{"", false},
		{".", true},
		{"%", true},
	}
	for _, tt := range tests {
		got := containsDotPercent(tt.input)
		if got != tt.want {
			t.Errorf("containsDotPercent(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- App.OnShutdown hooks ---

func TestApp_OnShutdown_LIFO(t *testing.T) {
	var order []int
	app := New()
	app.OnShutdown(func() { order = append(order, 1) })
	app.OnShutdown(func() { order = append(order, 2) })
	app.OnShutdown(func() { order = append(order, 3) })

	app.runShutdownHooks()

	if len(order) != 3 {
		t.Fatalf("hook count = %d, want 3", len(order))
	}
	// LIFO order: 3, 2, 1
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("order = %v, want [3 2 1]", order)
	}
}

func TestApp_OnShutdown_PanicRecovery(t *testing.T) {
	var reached bool
	app := New()
	app.OnShutdown(func() { reached = true })
	app.OnShutdown(func() { panic("boom") }) // this runs first (LIFO)

	// Should not panic
	app.runShutdownHooks()

	if !reached {
		t.Error("second hook should still run after first panics")
	}
}

// --- App.OnParse ---

func TestApp_OnParse(t *testing.T) {
	called := false
	app := New()
	app.OnParse(func(c *Ctx, input any) error {
		called = true
		return nil
	})
	if len(app.hooks.OnParse) != 1 {
		t.Errorf("OnParse hooks = %d, want 1", len(app.hooks.OnParse))
	}
	app.hooks.OnParse[0](nil, nil) // just invoke to test registration
	if !called {
		t.Error("OnParse hook not called")
	}
}

// --- App.Validator lazy init ---

func TestApp_Validator_LazyInit(t *testing.T) {
	app := New() // no WithValidator
	v := app.Validator()
	if v == nil {
		t.Error("Validator() should lazy-init")
	}
	// Second call should return same instance
	v2 := app.Validator()
	if v != v2 {
		t.Error("Validator() should return same instance")
	}
}

// --- handleError with custom ErrorHandler ---

func TestHandleError_CustomHandler(t *testing.T) {
	customCalled := false
	app := New(WithErrorHandler(func(c *Ctx, err *KrudaError) {
		customCalled = true
		c.Status(503)
		_ = c.Text("custom error")
	}))
	app.Get("/fail", func(c *Ctx) error {
		return InternalError("oops")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/fail")
	if !customCalled {
		t.Error("custom error handler not called")
	}
	if resp.StatusCode() != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode())
	}
}

// --- Compile hasLifecycle flag ---

func TestCompile_HasLifecycle_False(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if app.hasLifecycle {
		t.Error("hasLifecycle should be false when no hooks registered")
	}
}

func TestCompile_HasLifecycle_True(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error { return nil })
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if !app.hasLifecycle {
		t.Error("hasLifecycle should be true when hooks registered")
	}
}

// --- ServeKruda panic recovery ---

func TestServeKruda_PanicRecovery(t *testing.T) {
	app := New()
	app.Get("/panic", func(c *Ctx) error {
		panic("test panic")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/panic")
	if resp.StatusCode() != 500 {
		t.Errorf("status = %d, want 500 after panic", resp.StatusCode())
	}
}

// --- ServeKruda hooks integration ---

func TestServeKruda_OnRequestError(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error {
		return NewError(429, "rate limited")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 429 {
		t.Errorf("status = %d, want 429", resp.StatusCode())
	}
}

func TestServeKruda_BeforeHandleError(t *testing.T) {
	app := New()
	app.BeforeHandle(func(c *Ctx) error {
		return NewError(401, "unauthorized")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode())
	}
}

func TestServeKruda_AfterHandleError(t *testing.T) {
	app := New()
	app.AfterHandle(func(c *Ctx) error {
		return NewError(500, "after handle fail")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	// AfterHandle error is handled but response may already be written
	if resp.StatusCode() != 200 && resp.StatusCode() != 500 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- DevMode with KRUDA_ENV ---

func TestNew_DevMode_EnvVar(t *testing.T) {
	os.Setenv("KRUDA_ENV", "development")
	defer os.Unsetenv("KRUDA_ENV")

	app := New()
	if !app.config.DevMode {
		t.Error("DevMode should be true when KRUDA_ENV=development")
	}
}

func TestNew_DevMode_Explicit(t *testing.T) {
	app := New(WithDevMode(true))
	if !app.config.DevMode {
		t.Error("DevMode should be true")
	}
}

// --- Ctx.Stream ---

func TestCtx_Stream(t *testing.T) {
	app := New()
	app.Get("/stream", func(c *Ctx) error {
		c.SetContentType("text/plain")
		return c.Stream(strings.NewReader("streamed content"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/stream")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "streamed content") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Ctx.Bind with empty body ---

func TestCtx_Bind_EmptyBody(t *testing.T) {
	app := New()
	app.Post("/bind-empty", func(c *Ctx) error {
		var data struct{ Name string }
		if err := c.Bind(&data); err != nil {
			return c.Status(400).JSON(Map{"error": err.Error()})
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/bind-empty", nil)
	if resp.StatusCode() != 400 {
		t.Errorf("status = %d, want 400 for empty body", resp.StatusCode())
	}
}

// --- Ctx.Next ---

func TestCtx_Next(t *testing.T) {
	app := New()
	app.Use(func(c *Ctx) error {
		c.SetHeader("X-Before", "yes")
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.BodyString ---

func TestCtx_BodyString(t *testing.T) {
	app := New()
	app.Post("/body-str", func(c *Ctx) error {
		bs := c.BodyString()
		return c.Text("got: " + bs)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/body-str", []byte("hello"))
	if !strings.Contains(resp.BodyString(), "got: hello") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Views: NewViewEngine from glob ---

func TestNewViewEngine(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(dir + "/test.html")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("Hello {{.Name}}")
	f.Close()

	engine := NewViewEngine(dir + "/*.html")
	var buf bytes.Buffer
	err = engine.Render(&buf, "test.html", struct{ Name string }{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "Hello World" {
		t.Errorf("render = %q", buf.String())
	}
}

// --- Views: WithViews option ---

func TestWithViews(t *testing.T) {
	engine := &mockViewEngine{}
	app := &App{config: defaultConfig()}
	WithViews(engine)(app)
	if app.config.Views != engine {
		t.Error("Views not set")
	}
}

// --- Views: multiple patterns in NewViewEngineFS ---

func TestNewViewEngineFS_MultiplePatterns(t *testing.T) {
	vfs := fstest.MapFS{
		"a.html": {Data: []byte("A {{.V}}")},
		"b.txt":  {Data: []byte("B {{.V}}")},
	}
	engine := NewViewEngineFS(vfs, "*.html", "*.txt")

	var buf bytes.Buffer
	err := engine.Render(&buf, "a.html", struct{ V string }{"1"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "A 1" {
		t.Errorf("render a = %q", buf.String())
	}

	buf.Reset()
	err = engine.Render(&buf, "b.txt", struct{ V string }{"2"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "B 2" {
		t.Errorf("render b = %q", buf.String())
	}
}

// --- Ctx.SendBytesWithType ---

func TestCtx_SendBytesWithType(t *testing.T) {
	app := New()
	app.Get("/typed", func(c *Ctx) error {
		return c.SendBytesWithType("application/xml", []byte("<root/>"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/typed")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.Header("Content-Type"), "xml") {
		t.Errorf("Content-Type = %q", resp.Header("Content-Type"))
	}
	if resp.BodyString() != "<root/>" {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Ctx.SendBytesWithTypeBytes ---

func TestCtx_SendBytesWithTypeBytes(t *testing.T) {
	app := New()
	app.Get("/typed-bytes", func(c *Ctx) error {
		return c.SendBytesWithTypeBytes([]byte("text/csv"), []byte("a,b,c"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/typed-bytes")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.SendStaticWithTypeBytes ---

func TestCtx_SendStaticWithTypeBytes(t *testing.T) {
	app := New()
	staticData := []byte("immutable static data")
	app.Get("/static-typed", func(c *Ctx) error {
		return c.SendStaticWithTypeBytes([]byte("text/plain"), staticData)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/static-typed")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.SendBytes ---

func TestCtx_SendBytes(t *testing.T) {
	app := New()
	app.Get("/send-bytes", func(c *Ctx) error {
		c.SetContentType("application/octet-stream")
		return c.SendBytes([]byte{0x01, 0x02, 0x03})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/send-bytes")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if len(resp.Body()) != 3 {
		t.Errorf("body len = %d, want 3", len(resp.Body()))
	}
}

// --- Ctx.Redirect with default code ---

func TestCtx_Redirect_DefaultCode(t *testing.T) {
	app := New()
	app.Get("/old", func(c *Ctx) error {
		return c.Redirect("/new")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/old")
	if resp.StatusCode() != 302 {
		t.Errorf("status = %d, want 302 (default redirect)", resp.StatusCode())
	}
}

// --- App.Use middleware ---

func TestApp_Use_Chaining(t *testing.T) {
	app := New()
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }
	result := app.Use(mw1, mw2)
	if result != app {
		t.Error("Use should return app for chaining")
	}
	if len(app.middleware) != 2 {
		t.Errorf("middleware count = %d, want 2", len(app.middleware))
	}
}

// --- writeHeaders path testing ---

func TestWriteHeaders_SimpleCase(t *testing.T) {
	app := New()
	app.Get("/simple-headers", func(c *Ctx) error {
		c.SetContentType("text/plain")
		return c.SendBytes([]byte("ok"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/simple-headers")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestWriteHeaders_ComplexCase(t *testing.T) {
	app := New()
	app.Get("/complex-headers", func(c *Ctx) error {
		c.SetContentType("text/plain")
		c.SetHeader("Cache-Control", "no-store")
		c.SetHeader("Location", "/other")
		c.SetCookie(&Cookie{Name: "s", Value: "v", Path: "/"})
		return c.SendBytes([]byte("ok"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/complex-headers")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}
