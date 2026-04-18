package kruda

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// --- Ctx.Route / ParamInt / StatusCode / IP / NoContent ---

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

// --- Ctx.Redirect / SetBody / SetContentType / HTML / SendBytes* ---

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

// --- Ctx.MarkStart / Latency / Log ---

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

func TestCtx_Latency_WithMarkStart(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.MarkStart()
		time.Sleep(2 * time.Millisecond) // Windows timer resolution needs real sleep
		lat := c.Latency()
		if lat == 0 {
			return BadRequest("latency should be > 0 after MarkStart")
		}
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode == 400 {
		t.Error("Latency() returned 0 after MarkStart()")
	}
}

func TestCtx_Latency_WithoutMarkStart(t *testing.T) {
	app := New()
	c := newCtx(app)
	if c.Latency() != 0 {
		t.Errorf("Latency without MarkStart should be 0, got %v", c.Latency())
	}
}

func TestCtx_Latency_ViaMockRequest(t *testing.T) {
	app := New()
	var latency time.Duration
	app.Get("/test", func(c *Ctx) error {
		c.MarkStart()
		time.Sleep(1 * time.Millisecond)
		latency = c.Latency()
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if latency == 0 {
		t.Error("Latency should be > 0 after MarkStart + sleep")
	}
}

// --- Ctx.Header / AddHeader / SetHeaderBytes ---

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

func TestCtx_SetHeaderBytes(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.SetHeaderBytes("X-Custom", []byte("byte-value"))
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("X-Custom")
	if got != "byte-value" {
		t.Errorf("SetHeaderBytes: header = %q, want %q", got, "byte-value")
	}
}

func TestCtx_AddHeader_MultiValue(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Vary", "Accept")
		c.AddHeader("Vary", "Accept-Encoding")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Vary")
	if !strings.Contains(got, "Accept") || !strings.Contains(got, "Accept-Encoding") {
		t.Errorf("AddHeader multi-value: %q", got)
	}
}

func TestCtx_AddHeader_CacheControl(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Cache-Control", "no-cache")
		c.AddHeader("Cache-Control", "no-store")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Cache-Control")
	if !strings.Contains(got, "no-cache") || !strings.Contains(got, "no-store") {
		t.Errorf("AddHeader Cache-Control: %q", got)
	}
}

func TestCtx_AddHeader_ContentType(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Content-Type", "text/plain")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	// Content-Type should be set (may be overridden by Text())
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestCtx_AddHeader_Location(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Location", "/new-path")
		return c.Status(302).Text("")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Location")
	if got != "/new-path" {
		t.Errorf("AddHeader Location: %q", got)
	}
}

func TestCtx_AddHeader_InvalidKey(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		// Invalid header key with space should be skipped
		c.AddHeader("Bad Key", "value")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestCtx_MultipleHeaders(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.SetHeader("X-First", "one")
		c.SetHeader("X-Second", "two")
		c.SetHeader("Cache-Control", "no-cache")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.headers.Get("X-First") != "one" {
		t.Errorf("X-First = %q", resp.headers.Get("X-First"))
	}
	if resp.headers.Get("X-Second") != "two" {
		t.Errorf("X-Second = %q", resp.headers.Get("X-Second"))
	}
}

func TestCtx_Header_FromRequest(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.Text(c.Header("X-Custom"))
	})
	app.Compile()

	req := &mockRequest{
		method:  "GET",
		path:    "/test",
		headers: map[string]string{"X-Custom": "custom-val"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "custom-val") {
		t.Errorf("body = %q", resp.body)
	}
}

func TestCtx_Header_CachedSecondCall(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		// First call fetches from request, second call uses cache
		v1 := c.Header("X-Custom")
		v2 := c.Header("X-Custom")
		if v1 != v2 {
			return BadRequest("header values differ")
		}
		return c.Text(v1)
	})
	app.Compile()

	req := &mockRequest{
		method:  "GET",
		path:    "/test",
		headers: map[string]string{"X-Custom": "cached"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "cached") {
		t.Errorf("body = %q", resp.body)
	}
}

// --- Ctx.Responded / QueryInt / SetContext / Context / Stream ---

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

func TestCtx_Context_Default(t *testing.T) {
	app := New()
	c := newCtx(app)
	ctx := c.Context()
	if ctx == nil {
		t.Error("Context() should not return nil")
	}
}

func TestCtx_Context_WithSetContext(t *testing.T) {
	app := New()
	c := newCtx(app)
	type ctxKey string
	custom := context.WithValue(context.Background(), ctxKey("key"), "val")
	c.SetContext(custom)
	if c.Context() != custom {
		t.Error("Context() should return the custom context set via SetContext")
	}
}

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

// --- Ctx.Transport / ResponseWriter / Request accessors ---

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

// --- Ctx.IP / Route via mockRequest ---

func TestCtx_IP_ReturnsRemoteAddr(t *testing.T) {
	app := New()
	app.Get("/ip", func(c *Ctx) error {
		return c.Text(c.IP())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/ip"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "127.0.0.1") {
		t.Errorf("IP = %q", resp.body)
	}
}

func TestCtx_IP_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	if c.IP() != "" {
		t.Errorf("IP with nil request should return empty, got %q", c.IP())
	}
}

func TestCtx_Route_WithPattern(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return c.Text(c.Route())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/42"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "/users/:id") {
		t.Errorf("Route() = %q, want /users/:id", resp.body)
	}
}

func TestCtx_Route_StaticPath(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.Text(c.Route())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/health"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "/health") {
		t.Errorf("Route() = %q", resp.body)
	}
}

// --- Ctx.File: non-nethttp transport returns error ---

func TestCtx_File_NonNetHTTP(t *testing.T) {
	// File requires net/http transport; mock transport should error
	app := New()
	app.Get("/file", func(c *Ctx) error {
		return c.File("/tmp/nonexistent.txt")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/file"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	// Should return 500 because mock doesn't support File
	if resp.statusCode == 200 {
		t.Error("File with non-nethttp transport should fail")
	}
}

// --- writeHeaders code paths ---

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
