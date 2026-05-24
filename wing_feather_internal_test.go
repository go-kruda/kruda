//go:build linux || darwin

package kruda

import (
	"bytes"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

type workerSingleHandlerSpy struct {
	singleCalls   int
	routeCalls    int
	serveCalls    int
	singleHandled bool
}

func (s *workerSingleHandlerSpy) ServeKruda(transport.ResponseWriter, transport.Request) {
	s.serveCalls++
}

func (s *workerSingleHandlerSpy) serveKrudaRoute(transport.ResponseWriter, transport.Request, []HandlerFunc) {
	s.routeCalls++
}

func (s *workerSingleHandlerSpy) serveKrudaSingleHandler(transport.ResponseWriter, transport.Request, HandlerFunc) bool {
	s.singleCalls++
	return s.singleHandled
}

func TestWorkerServeRouteUsesSingleHandlerFastPath(t *testing.T) {
	spy := &workerSingleHandlerSpy{singleHandled: true}
	w := &worker{handler: spy}
	resp := acquireResponse()
	defer releaseResponse(resp)

	w.serveRoute(resp, &wingRequest{method: "GET", path: "/fast", keepAlive: true}, Feather{
		handlers: []HandlerFunc{func(c *Ctx) error { return nil }},
	})

	if spy.singleCalls != 1 || spy.routeCalls != 0 || spy.serveCalls != 0 {
		t.Fatalf("single=%d route=%d serve=%d", spy.singleCalls, spy.routeCalls, spy.serveCalls)
	}
}

func TestWorkerServeRouteFallsBackWhenSingleHandlerDeclines(t *testing.T) {
	spy := &workerSingleHandlerSpy{singleHandled: false}
	w := &worker{handler: spy}
	resp := acquireResponse()
	defer releaseResponse(resp)

	w.serveRoute(resp, &wingRequest{method: "GET", path: "/fast", keepAlive: true}, Feather{
		handlers: []HandlerFunc{func(c *Ctx) error { return nil }},
	})

	if spy.singleCalls != 1 || spy.routeCalls != 1 || spy.serveCalls != 0 {
		t.Fatalf("single=%d route=%d serve=%d", spy.singleCalls, spy.routeCalls, spy.serveCalls)
	}
}

func TestWorkerServeRouteUsesCleanExactRouteFlag(t *testing.T) {
	spy := &workerSingleHandlerSpy{singleHandled: true}
	w := &worker{handler: spy}
	resp := acquireResponse()
	defer releaseResponse(resp)

	w.serveRoute(resp, &wingRequest{method: "GET", path: "/plaintext", keepAlive: true}, Feather{
		handlers:  []HandlerFunc{func(c *Ctx) error { return nil }},
		path:      "/plaintext",
		pathClean: true,
	})

	if spy.singleCalls != 1 || spy.routeCalls != 0 || spy.serveCalls != 0 {
		t.Fatalf("single=%d route=%d serve=%d", spy.singleCalls, spy.routeCalls, spy.serveCalls)
	}
}

func TestWorkerServeRouteSkipsFastPathForDirtyExactRoute(t *testing.T) {
	spy := &workerSingleHandlerSpy{singleHandled: true}
	w := &worker{handler: spy}
	resp := acquireResponse()
	defer releaseResponse(resp)

	w.serveRoute(resp, &wingRequest{method: "GET", path: "/assets/app.js", keepAlive: true}, Feather{
		handlers:  []HandlerFunc{func(c *Ctx) error { return nil }},
		path:      "/assets/app.js",
		pathClean: false,
	})

	if spy.singleCalls != 0 || spy.routeCalls != 0 || spy.serveCalls != 1 {
		t.Fatalf("single=%d route=%d serve=%d", spy.singleCalls, spy.routeCalls, spy.serveCalls)
	}
}

func TestWorkerLookupFeatherCachesExactRoute(t *testing.T) {
	w := &worker{feathers: NewFeatherTable(map[string]Feather{
		"GET /plaintext": Plaintext,
	}, Arrow)}

	got := w.lookupFeather("GET", "/plaintext")
	if got.path != "/plaintext" {
		t.Fatalf("first lookup path = %q, want /plaintext", got.path)
	}
	if w.lastMethod0 != "GET" || w.lastPath0 != "/plaintext" {
		t.Fatalf("cache = %q %q, want GET /plaintext", w.lastMethod0, w.lastPath0)
	}

	got = w.lookupFeather("GET", "/plaintext")
	if got.Dispatch != Plaintext.Dispatch || got.ResponseMode != Plaintext.ResponseMode {
		t.Fatalf("cached lookup = %+v, want Plaintext", got)
	}
}

func TestWorkerLookupFeatherDoesNotCacheParamOrDefaultPath(t *testing.T) {
	w := &worker{feathers: NewFeatherTable(map[string]Feather{
		"GET /plaintext": Plaintext,
		"GET /users/:id": Arrow,
	}, Bolt)}

	_ = w.lookupFeather("GET", "/plaintext")
	if w.lastPath0 != "/plaintext" {
		t.Fatalf("exact route was not cached: %q", w.lastPath0)
	}

	got := w.lookupFeather("GET", "/users/42")
	if got.Dispatch != Arrow.Dispatch {
		t.Fatalf("param lookup dispatch = %v, want Arrow", got.Dispatch)
	}
	if w.lastPath0 != "/plaintext" {
		t.Fatalf("param lookup replaced exact cache with %q", w.lastPath0)
	}

	got = w.lookupFeather("GET", "/missing")
	if got.Dispatch != Bolt.Dispatch {
		t.Fatalf("default lookup dispatch = %v, want Bolt", got.Dispatch)
	}
	if w.lastPath0 != "/plaintext" {
		t.Fatalf("default lookup replaced exact cache with %q", w.lastPath0)
	}
}

func TestWorkerLookupFeatherPromotesSecondExactCacheSlot(t *testing.T) {
	w := &worker{feathers: NewFeatherTable(map[string]Feather{
		"GET /plaintext": Plaintext,
		"GET /json":      JSON,
	}, Arrow)}

	_ = w.lookupFeather("GET", "/plaintext")
	_ = w.lookupFeather("GET", "/json")
	if w.lastPath0 != "/json" || w.lastPath1 != "/plaintext" {
		t.Fatalf("cache = [%q, %q], want [/json, /plaintext]", w.lastPath0, w.lastPath1)
	}

	got := w.lookupFeather("GET", "/plaintext")
	if got.ResponseMode != Plaintext.ResponseMode {
		t.Fatalf("promoted lookup = %+v, want Plaintext", got)
	}
	if w.lastPath0 != "/plaintext" || w.lastPath1 != "/json" {
		t.Fatalf("promoted cache = [%q, %q], want [/plaintext, /json]", w.lastPath0, w.lastPath1)
	}
}

func TestWingResponsePlaintextModeUsesHandlerFastPath(t *testing.T) {
	resp := acquireResponse()
	defer releaseResponse(resp)

	resp.responseMode = responsePlaintext
	resp.SetStaticText(201, "text/plain; charset=utf-8", "created")

	if resp.staticResp != nil {
		t.Fatal("plaintext response mode should not use the shared static response cache")
	}
	if !resp.plaintextFast {
		t.Fatal("plaintext response mode did not enable plaintext fast serialization")
	}

	data := resp.buildZeroCopy()
	for _, want := range [][]byte{
		[]byte("HTTP/1.1 201 Created\r\n"),
		[]byte("Content-Type: text/plain; charset=utf-8\r\n"),
		[]byte("Content-Length: 7\r\n"),
		[]byte("\r\n\r\ncreated"),
	} {
		if !bytes.Contains(data, want) {
			t.Fatalf("plaintext response missing %q in:\n%s", want, data)
		}
	}
}

func TestWingResponseGenericStaticTextKeepsStaticCache(t *testing.T) {
	resp := acquireResponse()
	defer releaseResponse(resp)

	resp.SetStaticText(200, "text/plain; charset=utf-8", "ok")

	if resp.staticResp == nil {
		t.Fatal("generic Wing text path should keep using the shared static response cache")
	}
	if resp.plaintextFast {
		t.Fatal("generic Wing text path should not enable plaintext response mode")
	}
}

func TestWingPlaintextModeStillRunsHandlerMiddlewareLifecycle(t *testing.T) {
	app := New(Wing())
	var middlewareRan, beforeRan, handlerRan, afterRan bool
	app.Use(func(c *Ctx) error {
		middlewareRan = true
		return c.Next()
	})
	app.BeforeHandle(func(c *Ctx) error {
		beforeRan = true
		return nil
	})
	app.AfterHandle(func(c *Ctx) error {
		afterRan = true
		return nil
	})
	app.Get("/plaintext", func(c *Ctx) error {
		handlerRan = true
		return c.Text("ok")
	}, WingPlaintext())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /plaintext"]
	if len(f.handlers) == 0 {
		t.Fatal("WingPlaintext route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)
	resp.responseMode = responsePlaintext

	app.serveKrudaRoute(resp, &wingRequest{method: "GET", path: "/plaintext", keepAlive: true}, f.handlers)

	if !middlewareRan || !beforeRan || !handlerRan || !afterRan {
		t.Fatalf("middleware=%v before=%v handler=%v after=%v", middlewareRan, beforeRan, handlerRan, afterRan)
	}
	if !resp.plaintextFast {
		t.Fatal("simple WingPlaintext handler did not use plaintext response mode")
	}
}

func TestWingPlaintextModeGroupRouteRetainsHandlerChain(t *testing.T) {
	app := New(Wing())
	var groupMiddlewareRan, handlerRan bool
	api := app.Group("/api").Use(func(c *Ctx) error {
		groupMiddlewareRan = true
		return c.Next()
	})
	api.Get("/plaintext", func(c *Ctx) error {
		handlerRan = true
		return c.Text("ok")
	}, WingPlaintext())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /api/plaintext"]
	if len(f.handlers) == 0 {
		t.Fatal("WingPlaintext group route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)
	resp.responseMode = responsePlaintext

	app.serveKrudaRoute(resp, &wingRequest{method: "GET", path: "/api/plaintext", keepAlive: true}, f.handlers)

	if !groupMiddlewareRan || !handlerRan {
		t.Fatalf("groupMiddleware=%v handler=%v", groupMiddlewareRan, handlerRan)
	}
	if !resp.plaintextFast {
		t.Fatal("simple grouped WingPlaintext handler did not use plaintext response mode")
	}
}

func TestWingPlaintextModeCustomHeaderFallsBackToGenericResponse(t *testing.T) {
	app := New(Wing())
	app.Get("/plaintext", func(c *Ctx) error {
		c.SetHeader("X-Test", "yes")
		return c.Text("ok")
	}, WingPlaintext())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /plaintext"]
	if len(f.handlers) == 0 {
		t.Fatal("WingPlaintext route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)
	resp.responseMode = responsePlaintext

	app.serveKrudaRoute(resp, &wingRequest{method: "GET", path: "/plaintext", keepAlive: true}, f.handlers)

	if resp.plaintextFast {
		t.Fatal("custom response headers must fall back to generic serialization")
	}
	data := resp.buildZeroCopy()
	if !bytes.Contains(data, []byte("X-Test: yes\r\n")) {
		t.Fatalf("generic fallback missing custom header:\n%s", data)
	}
}

func TestWingJSONStaticBytesUseJSONResponder(t *testing.T) {
	app := New(Wing())
	var handlerRan bool
	app.Get("/json-static", func(c *Ctx) error {
		handlerRan = true
		return c.SendStaticWithTypeBytes(jsonContentType, []byte(`{"message":"ok"}`))
	}, WingJSON())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /json-static"]
	if len(f.handlers) == 0 {
		t.Fatal("WingJSON route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)

	app.serveKrudaRoute(resp, &wingRequest{method: "GET", path: "/json-static", keepAlive: true}, f.handlers)

	if !handlerRan {
		t.Fatal("handler did not run")
	}
	if !resp.jsonFast {
		t.Fatal("static JSON bytes did not use Wing JSON responder")
	}
	data := resp.buildZeroCopy()
	if !bytes.Contains(data, []byte("Content-Type: application/json; charset=utf-8\r\n")) {
		t.Fatalf("JSON fast response missing content type:\n%s", data)
	}
	if !bytes.Contains(data, []byte("\r\n\r\n{\"message\":\"ok\"}")) {
		t.Fatalf("JSON fast response missing body:\n%s", data)
	}
}

func TestWingSendStaticJSONUsesJSONResponder(t *testing.T) {
	app := New(Wing())
	var handlerRan bool
	app.Get("/json-static", func(c *Ctx) error {
		handlerRan = true
		return c.SendStaticJSON([]byte(`{"message":"ok"}`))
	}, WingJSON())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /json-static"]
	if len(f.handlers) == 0 {
		t.Fatal("WingJSON route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)

	app.serveKrudaRoute(resp, &wingRequest{method: "GET", path: "/json-static", keepAlive: true}, f.handlers)

	if !handlerRan {
		t.Fatal("handler did not run")
	}
	if !resp.jsonFast {
		t.Fatal("SendStaticJSON did not use Wing JSON responder")
	}
	data := resp.buildZeroCopy()
	if !bytes.Contains(data, []byte("Content-Type: application/json; charset=utf-8\r\n")) {
		t.Fatalf("JSON fast response missing content type:\n%s", data)
	}
	if !bytes.Contains(data, []byte("\r\n\r\n{\"message\":\"ok\"}")) {
		t.Fatalf("JSON fast response missing body:\n%s", data)
	}
}

func TestWingResponseJSONAppendMatchesBuildZeroCopy(t *testing.T) {
	body := []byte(`{"message":"ok"}`)

	builtResp := acquireResponse()
	builtResp.SetJSON(201, body)
	built := append([]byte(nil), builtResp.buildZeroCopy()...)
	releaseResponse(builtResp)

	directResp := acquireResponse()
	defer releaseResponse(directResp)
	directResp.SetJSON(201, body)
	direct := directResp.appendJSONTo(nil)

	if !bytes.Equal(direct, built) {
		t.Fatalf("direct JSON response differs from buildZeroCopy:\ndirect:\n%s\nbuilt:\n%s", direct, built)
	}
}

func BenchmarkWorkerLookupFeatherCachedExact(b *testing.B) {
	w := &worker{feathers: NewFeatherTable(map[string]Feather{
		"GET /plaintext": Plaintext,
		"GET /json":      JSON,
	}, Arrow)}
	_ = w.lookupFeather("GET", "/plaintext")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.lookupFeather("GET", "/plaintext")
	}
}

func BenchmarkWorkerLookupFeatherAlternatingExact(b *testing.B) {
	w := &worker{feathers: NewFeatherTable(map[string]Feather{
		"GET /plaintext": Plaintext,
		"GET /json":      JSON,
	}, Arrow)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.lookupFeather("GET", "/plaintext")
		_ = w.lookupFeather("GET", "/json")
	}
}

func TestWingJSONModeStillRunsHandlerMiddlewareLifecycle(t *testing.T) {
	app := New(Wing())
	var middlewareRan, beforeRan, handlerRan, afterRan bool
	app.Use(func(c *Ctx) error {
		middlewareRan = true
		return c.Next()
	})
	app.BeforeHandle(func(c *Ctx) error {
		beforeRan = true
		return nil
	})
	app.AfterHandle(func(c *Ctx) error {
		afterRan = true
		return nil
	})
	app.Get("/json-static", func(c *Ctx) error {
		handlerRan = true
		return c.SendStaticWithTypeBytes(jsonContentType, []byte(`{"message":"ok"}`))
	}, WingJSON())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /json-static"]
	if len(f.handlers) == 0 {
		t.Fatal("WingJSON route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)
	w := &worker{handler: app}
	w.serveRoute(resp, &wingRequest{method: "GET", path: "/json-static", keepAlive: true}, f)

	if !middlewareRan || !beforeRan || !handlerRan || !afterRan {
		t.Fatalf("middleware=%v before=%v handler=%v after=%v", middlewareRan, beforeRan, handlerRan, afterRan)
	}
	if !resp.jsonFast {
		t.Fatal("WingJSON static bytes did not use JSON responder")
	}
}

func TestWingJSONStaticBytesCustomHeaderFallsBackToGenericResponse(t *testing.T) {
	app := New(Wing())
	app.Get("/json-static", func(c *Ctx) error {
		c.SetHeader("X-Test", "yes")
		return c.SendStaticWithTypeBytes(jsonContentType, []byte(`{"message":"ok"}`))
	}, WingJSON())
	app.Compile()

	tr, ok := app.transport.(*Transport)
	if !ok {
		t.Skip("Wing transport unavailable on this platform")
	}
	f := tr.config.Feathers["GET /json-static"]
	if len(f.handlers) == 0 {
		t.Fatal("WingJSON route did not retain its handler chain")
	}

	resp := acquireResponse()
	defer releaseResponse(resp)
	w := &worker{handler: app}
	w.serveRoute(resp, &wingRequest{method: "GET", path: "/json-static", keepAlive: true}, f)

	if resp.jsonFast {
		t.Fatal("custom response headers must fall back to generic JSON serialization")
	}
	data := resp.buildZeroCopy()
	if !bytes.Contains(data, []byte("X-Test: yes\r\n")) {
		t.Fatalf("generic fallback missing custom header:\n%s", data)
	}
}
