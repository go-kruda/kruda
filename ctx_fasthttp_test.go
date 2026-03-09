//go:build !windows

package kruda

import (
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
)

// newFastHTTPCtx creates a Ctx wired to a real fasthttp.RequestCtx for testing
// the fasthttp fast-path methods in ctx_fasthttp.go and serve_fast.go.
func newFastHTTPCtx(app *App) (*Ctx, *fasthttp.RequestCtx) {
	c := newCtx(app)
	fctx := &fasthttp.RequestCtx{}
	c.embeddedFHResp.ctx = fctx
	c.embeddedFHReq.ctx = fctx
	return c, fctx
}

func TestWriteHeadersFastHTTP_ContentType(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.contentType = "application/json"

	c.writeHeadersFastHTTP(fctx)

	got := string(fctx.Response.Header.ContentType())
	if got != "application/json" {
		t.Errorf("ContentType = %q, want application/json", got)
	}
}

func TestWriteHeadersFastHTTP_ContentLength(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.contentLength = 42

	c.writeHeadersFastHTTP(fctx)

	got := fctx.Response.Header.ContentLength()
	if got != 42 {
		t.Errorf("ContentLength = %d, want 42", got)
	}
}

func TestWriteHeadersFastHTTP_CacheControl(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.cacheControl = "no-cache"

	c.writeHeadersFastHTTP(fctx)

	got := string(fctx.Response.Header.Peek("Cache-Control"))
	if got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestWriteHeadersFastHTTP_Location(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.location = "/new-path"

	c.writeHeadersFastHTTP(fctx)

	got := string(fctx.Response.Header.Peek("Location"))
	if got != "/new-path" {
		t.Errorf("Location = %q, want /new-path", got)
	}
}

func TestWriteHeadersFastHTTP_RespHeaders(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.respHeaders["X-Custom"] = []string{"val1", "val2"}

	c.writeHeadersFastHTTP(fctx)

	got := string(fctx.Response.Header.Peek("X-Custom"))
	if got != "val1" {
		t.Errorf("X-Custom first = %q, want val1", got)
	}
}

func TestWriteHeadersFastHTTP_Cookies(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.cookies = append(c.cookies, &Cookie{
		Name:  "session",
		Value: "abc123",
		Path:  "/",
	})

	c.writeHeadersFastHTTP(fctx)

	got := string(fctx.Response.Header.Peek("Set-Cookie"))
	if got == "" {
		t.Error("Set-Cookie header not set")
	}
}

func TestWriteHeadersFastHTTP_NoHeaders(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	// All defaults — nothing should be set by writeHeadersFastHTTP
	c.contentLength = -1

	c.writeHeadersFastHTTP(fctx)

	// fasthttp has its own default content type, so we just verify writeHeadersFastHTTP
	// does not set cache-control or location when they are empty.
	if cc := string(fctx.Response.Header.Peek("Cache-Control")); cc != "" {
		t.Errorf("unexpected Cache-Control = %q", cc)
	}
	if loc := string(fctx.Response.Header.Peek("Location")); loc != "" {
		t.Errorf("unexpected Location = %q", loc)
	}
}

func TestTrySendBytesFastHTTP_Success(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 201

	ok := c.trySendBytesFastHTTP([]byte("hello"))
	if !ok {
		t.Fatal("expected true")
	}
	if fctx.Response.StatusCode() != 201 {
		t.Errorf("status = %d, want 201", fctx.Response.StatusCode())
	}
	if string(fctx.Response.Body()) != "hello" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestTrySendBytesFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	// No fasthttp context set
	ok := c.trySendBytesFastHTTP([]byte("hello"))
	if ok {
		t.Fatal("expected false when no fasthttp ctx")
	}
}

func TestTrySendFastHTTP_WithBody(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200
	c.body = []byte("body content")

	ok := c.trySendFastHTTP()
	if !ok {
		t.Fatal("expected true")
	}
	if string(fctx.Response.Body()) != "body content" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
	if c.body != nil {
		t.Error("body should be nil after send")
	}
}

func TestTrySendFastHTTP_NoBody(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 204
	c.body = nil

	ok := c.trySendFastHTTP()
	if !ok {
		t.Fatal("expected true")
	}
	if fctx.Response.StatusCode() != 204 {
		t.Errorf("status = %d, want 204", fctx.Response.StatusCode())
	}
}

func TestTrySendFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySendFastHTTP()
	if ok {
		t.Fatal("expected false when no fasthttp ctx")
	}
}

func TestTrySetHeaderFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)

	ok := c.trySetHeaderFastHTTP("X-Test", "value")
	if !ok {
		t.Fatal("expected true")
	}
	got := string(fctx.Response.Header.Peek("X-Test"))
	if got != "value" {
		t.Errorf("X-Test = %q, want value", got)
	}
}

func TestTrySetHeaderFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySetHeaderFastHTTP("X-Test", "value")
	if ok {
		t.Fatal("expected false when no fasthttp ctx")
	}
}

func TestTrySetHeaderBytesFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)

	ok := c.trySetHeaderBytesFastHTTP("X-Bytes", []byte("byteval"))
	if !ok {
		t.Fatal("expected true")
	}
	got := string(fctx.Response.Header.Peek("X-Bytes"))
	if got != "byteval" {
		t.Errorf("X-Bytes = %q, want byteval", got)
	}
}

func TestTrySetHeaderBytesFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySetHeaderBytesFastHTTP("X-Bytes", []byte("byteval"))
	if ok {
		t.Fatal("expected false when no fasthttp ctx")
	}
}

func TestTrySendBytesWithTypeFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200

	ok := c.trySendBytesWithTypeFastHTTP("text/plain", []byte("hi"))
	if !ok {
		t.Fatal("expected true")
	}
	if string(fctx.Response.Header.ContentType()) != "text/plain" {
		t.Errorf("ct = %q", fctx.Response.Header.ContentType())
	}
	if string(fctx.Response.Body()) != "hi" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestTrySendBytesWithTypeFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySendBytesWithTypeFastHTTP("text/plain", []byte("hi"))
	if ok {
		t.Fatal("expected false")
	}
}

func TestTrySendBytesWithTypeBytesFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200

	ok := c.trySendBytesWithTypeBytesFastHTTP([]byte("text/html"), []byte("<h1>Hi</h1>"))
	if !ok {
		t.Fatal("expected true")
	}
	if string(fctx.Response.Header.ContentType()) != "text/html" {
		t.Errorf("ct = %q", fctx.Response.Header.ContentType())
	}
	if string(fctx.Response.Body()) != "<h1>Hi</h1>" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestTrySendBytesWithTypeBytesFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySendBytesWithTypeBytesFastHTTP([]byte("text/html"), []byte("<h1>Hi</h1>"))
	if ok {
		t.Fatal("expected false")
	}
}

func TestTrySendStaticWithTypeBytesFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200

	staticBody := []byte("static content") // immutable
	ok := c.trySendStaticWithTypeBytesFastHTTP([]byte("text/plain"), staticBody)
	if !ok {
		t.Fatal("expected true")
	}
	if string(fctx.Response.Body()) != "static content" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestTrySendStaticWithTypeBytesFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.trySendStaticWithTypeBytesFastHTTP([]byte("text/plain"), []byte("static"))
	if ok {
		t.Fatal("expected false")
	}
}

func TestTryQueryFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)

	// Set up query string on the fasthttp request
	fctx.Request.SetRequestURI("/test?name=tiger&lang=go")

	got := c.tryQueryFastHTTP("name")
	if got != "tiger" {
		t.Errorf("name = %q, want tiger", got)
	}
	got = c.tryQueryFastHTTP("lang")
	if got != "go" {
		t.Errorf("lang = %q, want go", got)
	}
	got = c.tryQueryFastHTTP("missing")
	if got != "" {
		t.Errorf("missing = %q, want empty", got)
	}
}

func TestTryQueryFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	got := c.tryQueryFastHTTP("name")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestTryBodyBytesFastHTTP(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)

	fctx.Request.SetBody([]byte("request body"))

	body, ok := c.tryBodyBytesFastHTTP()
	if !ok {
		t.Fatal("expected true")
	}
	if string(body) != "request body" {
		t.Errorf("body = %q", body)
	}
}

func TestTryBodyBytesFastHTTP_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	body, ok := c.tryBodyBytesFastHTTP()
	if ok {
		t.Fatal("expected false")
	}
	if body != nil {
		t.Errorf("expected nil body, got %v", body)
	}
}

func TestRawResponseHeader_FastHTTP(t *testing.T) {
	app := New()
	c, _ := newFastHTTPCtx(app)

	rh := c.RawResponseHeader()
	if rh == nil {
		t.Fatal("expected non-nil RawResponseHeader")
	}
	rh.SetBytesV("X-Raw", []byte("rawval"))
}

func TestRawResponseHeader_NoFastHTTP(t *testing.T) {
	app := New()
	c := newCtx(app)
	rh := c.RawResponseHeader()
	if rh != nil {
		t.Fatal("expected nil when no fasthttp ctx")
	}
}

// --- serve_fast.go tests ---

func TestInternMethod(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte("GET"), "GET"},
		{[]byte("PUT"), "PUT"},
		{[]byte("POST"), "POST"},
		{[]byte("HEAD"), "HEAD"},
		{[]byte("PATCH"), "PATCH"},
		{[]byte("DELETE"), "DELETE"},
		{[]byte("OPTIONS"), "OPTIONS"},
		{[]byte("XX"), "XX"},                  // 2 bytes, no match in switch
		{[]byte("ABCDEFGHI"), "ABCDEFGHI"},    // 9 bytes, no match in switch
	}

	for _, tt := range tests {
		got := internMethod(tt.input)
		if got != tt.want {
			t.Errorf("internMethod(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServeFastHTTP_Interface(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *Ctx) error {
		return c.Text("world")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/hello")

	app.ServeFastHTTP(fctx)

	if fctx.Response.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", fctx.Response.StatusCode())
	}
	if string(fctx.Response.Body()) != "world" {
		t.Errorf("body = %q, want world", fctx.Response.Body())
	}
}

func TestServeFast_NotFound(t *testing.T) {
	app := New()
	app.Get("/exists", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/nope")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 404 {
		t.Errorf("status = %d, want 404", fctx.Response.StatusCode())
	}
}

func TestServeFast_MethodNotAllowed(t *testing.T) {
	app := New()
	app.Get("/only-get", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("POST")
	fctx.Request.SetRequestURI("/only-get")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 405 {
		t.Errorf("status = %d, want 405", fctx.Response.StatusCode())
	}
	allow := string(fctx.Response.Header.Peek("Allow"))
	if allow == "" {
		t.Error("Allow header should be set")
	}
}

func TestServeFast_WithSetBody(t *testing.T) {
	app := New()
	app.Get("/lazy", func(c *Ctx) error {
		c.SetContentType("text/csv")
		c.SetBody([]byte("a,b,c"))
		return nil
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/lazy")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", fctx.Response.StatusCode())
	}
	if string(fctx.Response.Body()) != "a,b,c" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
	if string(fctx.Response.Header.ContentType()) != "text/csv" {
		t.Errorf("ct = %q", fctx.Response.Header.ContentType())
	}
}

func TestServeFast_SecurityHeaders(t *testing.T) {
	app := New(WithSecurity())
	app.Get("/sec", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/sec")

	app.ServeFast(fctx)

	// WithSecurity() enables default security headers.
	// Check X-Content-Type-Options which is "nosniff" by default.
	cto := string(fctx.Response.Header.Peek(http.CanonicalHeaderKey("X-Content-Type-Options")))
	if cto != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", cto)
	}
}

func TestServeFast_Lifecycle_Hooks(t *testing.T) {
	var order []string
	app := New()
	app.OnRequest(func(c *Ctx) error {
		order = append(order, "onRequest")
		return nil
	})
	app.BeforeHandle(func(c *Ctx) error {
		order = append(order, "beforeHandle")
		return nil
	})
	app.AfterHandle(func(c *Ctx) error {
		order = append(order, "afterHandle")
		return nil
	})
	app.OnResponse(func(c *Ctx) error {
		order = append(order, "onResponse")
		return nil
	})
	app.Get("/hooks", func(c *Ctx) error {
		order = append(order, "handler")
		return c.Text("ok")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/hooks")

	app.ServeFast(fctx)

	expected := []string{"onRequest", "beforeHandle", "handler", "afterHandle", "onResponse"}
	if len(order) != len(expected) {
		t.Fatalf("hook count = %d, want %d: %v", len(order), len(expected), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want)
		}
	}
}

func TestServeFast_EmptyPath(t *testing.T) {
	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.Text("root")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	// fasthttp normalizes empty to "/" but let's set it explicitly
	fctx.Request.SetRequestURI("/")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", fctx.Response.StatusCode())
	}
}

func TestServeFast_DirtyFlagCleanup(t *testing.T) {
	app := New()
	app.Get("/dirty", func(c *Ctx) error {
		c.SetHeader("X-Custom", "val")
		c.Set("key", "value")
		c.SetCookie(&Cookie{Name: "c1", Value: "v1"})
		return c.Text("ok")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/dirty")

	// Should not panic, dirty flags should be properly cleaned
	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", fctx.Response.StatusCode())
	}
}

func TestServeFast_SetBody_WithRespHeaders(t *testing.T) {
	app := New()
	app.Get("/headers-body", func(c *Ctx) error {
		c.SetContentType("application/xml")
		c.SetHeader("Cache-Control", "no-store")
		c.SetHeader("Location", "/redirect")
		c.SetBody([]byte("<xml/>"))
		return nil
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/headers-body")

	app.ServeFast(fctx)

	if string(fctx.Response.Body()) != "<xml/>" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestServeFast_SetBody_WithCookies(t *testing.T) {
	app := New()
	app.Get("/cookie-body", func(c *Ctx) error {
		c.SetCookie(&Cookie{Name: "sid", Value: "123", Path: "/"})
		c.SetBody([]byte("with cookie"))
		return nil
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/cookie-body")

	app.ServeFast(fctx)

	if string(fctx.Response.Body()) != "with cookie" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
	setCookie := string(fctx.Response.Header.Peek("Set-Cookie"))
	if setCookie == "" {
		t.Error("Set-Cookie should be set")
	}
}

// --- fhReqAdapter tests ---

func TestFhReqAdapter_Methods(t *testing.T) {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("POST")
	fctx.Request.SetRequestURI("/api/test?key=val")
	fctx.Request.Header.Set("X-Custom", "headerval")
	fctx.Request.Header.SetCookie("sid", "cookieval")
	fctx.Request.SetBody([]byte("reqbody"))

	adapter := &fhReqAdapter{ctx: fctx}

	if adapter.Method() != "POST" {
		t.Errorf("Method = %q", adapter.Method())
	}
	if adapter.Path() != "/api/test" {
		t.Errorf("Path = %q", adapter.Path())
	}
	if adapter.Header("X-Custom") != "headerval" {
		t.Errorf("Header = %q", adapter.Header("X-Custom"))
	}
	if adapter.QueryParam("key") != "val" {
		t.Errorf("QueryParam = %q", adapter.QueryParam("key"))
	}
	if adapter.Cookie("sid") != "cookieval" {
		t.Errorf("Cookie = %q", adapter.Cookie("sid"))
	}

	body, err := adapter.Body()
	if err != nil {
		t.Fatalf("Body error: %v", err)
	}
	if string(body) != "reqbody" {
		t.Errorf("Body = %q", body)
	}

	if adapter.RawRequest() == nil {
		t.Error("RawRequest should not be nil")
	}
	if adapter.Context() == nil {
		t.Error("Context should not be nil")
	}
	// RemoteAddr may be empty in test context — just call it for coverage
	_ = adapter.RemoteAddr()
}

// --- fhRespAdapter tests ---

func TestFhRespAdapter_Methods(t *testing.T) {
	fctx := &fasthttp.RequestCtx{}
	adapter := &fhRespAdapter{ctx: fctx}

	adapter.WriteHeader(201)
	if fctx.Response.StatusCode() != 201 {
		t.Errorf("status = %d, want 201", fctx.Response.StatusCode())
	}

	n, err := adapter.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write n = %d, want 5", n)
	}

	hdr := adapter.Header()
	if hdr == nil {
		t.Fatal("Header should not be nil")
	}
	hdr.Set("X-Test", "abc")
}

// --- fhHeaderAdapter tests ---

func TestFhHeaderAdapter(t *testing.T) {
	fctx := &fasthttp.RequestCtx{}
	adapter := &fhHeaderAdapter{ctx: fctx}

	adapter.Set("X-Key", "val1")
	if adapter.Get("X-Key") != "val1" {
		t.Errorf("Get = %q", adapter.Get("X-Key"))
	}

	adapter.Add("X-Key", "val2")
	adapter.Del("X-Key")
	if got := adapter.Get("X-Key"); got != "" {
		t.Errorf("after Del, Get = %q", got)
	}
}

// --- tryFastHTTPText ---

func TestTryFastHTTPText(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200

	ok := c.tryFastHTTPText("hello text")
	if !ok {
		t.Fatal("expected true")
	}
	if !c.responded {
		t.Error("responded should be true")
	}
	if string(fctx.Response.Body()) != "hello text" {
		t.Errorf("body = %q", fctx.Response.Body())
	}
	if string(fctx.Response.Header.ContentType()) != "text/plain; charset=utf-8" {
		t.Errorf("ct = %q", fctx.Response.Header.ContentType())
	}
}

func TestTryFastHTTPText_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.tryFastHTTPText("hello")
	if ok {
		t.Fatal("expected false")
	}
}

// --- tryFastHTTPJSON ---

func TestTryFastHTTPJSON(t *testing.T) {
	app := New()
	c, fctx := newFastHTTPCtx(app)
	c.status = 200

	data := []byte(`{"key":"value"}`)
	ok := c.tryFastHTTPJSON(data)
	if !ok {
		t.Fatal("expected true")
	}
	if !c.responded {
		t.Error("responded should be true")
	}
	if string(fctx.Response.Body()) != `{"key":"value"}` {
		t.Errorf("body = %q", fctx.Response.Body())
	}
}

func TestTryFastHTTPJSON_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.tryFastHTTPJSON([]byte("{}"))
	if ok {
		t.Fatal("expected false")
	}
}

// --- tryFastHTTPJSONDirect ---

func TestTryFastHTTPJSONDirect_NoCtx(t *testing.T) {
	app := New()
	c := newCtx(app)
	ok := c.tryFastHTTPJSONDirect(map[string]string{"a": "b"})
	if ok {
		t.Fatal("expected false when no fasthttp ctx")
	}
}

func TestTryFastHTTPJSONDirect_NoEncoder(t *testing.T) {
	app := New()
	app.config.JSONEncoder = nil
	c, _ := newFastHTTPCtx(app)
	ok := c.tryFastHTTPJSONDirect(map[string]string{"a": "b"})
	if ok {
		t.Fatal("expected false when no encoder")
	}
}

// --- fhReqAdapter MultipartForm ---

func TestFhReqAdapter_MultipartForm_TooLarge(t *testing.T) {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentLength(1000)
	adapter := &fhReqAdapter{ctx: fctx}

	_, err := adapter.MultipartForm(100)
	if err == nil {
		t.Error("expected error for oversized request")
	}
}

func TestServeFast_OnRequest_Error(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error {
		return NewError(403, "blocked")
	})
	app.Get("/blocked", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/blocked")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 403 {
		t.Errorf("status = %d, want 403", fctx.Response.StatusCode())
	}
}

func TestServeFast_BeforeHandle_Error(t *testing.T) {
	app := New()
	app.BeforeHandle(func(c *Ctx) error {
		return NewError(401, "unauthorized")
	})
	app.Get("/guarded", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/guarded")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 401 {
		t.Errorf("status = %d, want 401", fctx.Response.StatusCode())
	}
}

func TestServeFast_HandlerError(t *testing.T) {
	app := New()
	app.Get("/fail", func(c *Ctx) error {
		return NewError(500, "boom")
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/fail")

	app.ServeFast(fctx)

	if fctx.Response.StatusCode() != 500 {
		t.Errorf("status = %d, want 500", fctx.Response.StatusCode())
	}
}

func TestServeFast_SetBody_ContentLength(t *testing.T) {
	app := New()
	app.Get("/cl", func(c *Ctx) error {
		c.contentLength = 99 // explicit content length
		c.SetBody([]byte("data"))
		return nil
	})
	app.Compile()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/cl")

	app.ServeFast(fctx)

	cl := fctx.Response.Header.ContentLength()
	if cl != 99 {
		t.Errorf("ContentLength = %d, want 99", cl)
	}
}
