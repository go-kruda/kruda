package kruda

import (
	"bytes"
	"reflect"
	"testing"
)

func featherEqual(a, b Feather) bool { return reflect.DeepEqual(a, b) }

func TestFeatherDefaults(t *testing.T) {
	var f Feather
	f.defaults()
	want := Feather{Dispatch: Inline}
	if !featherEqual(f, want) {
		t.Errorf("defaults() = %+v, want %+v", f, want)
	}
}

func TestFeatherWith(t *testing.T) {
	f := Arrow.With(Dispatch(Spawn))
	if f.Dispatch != Spawn {
		t.Errorf("Dispatch = %v, want Spawn", f.Dispatch)
	}
}

func TestFeatherWithDoesNotMutateOriginal(t *testing.T) {
	original := Arrow
	_ = Arrow.With(Dispatch(Inline))
	if !featherEqual(Arrow, original) {
		t.Errorf("With() mutated original preset: got %+v, want %+v", Arrow, original)
	}
}

func TestPresetValues(t *testing.T) {
	tests := []struct {
		name string
		f    Feather
		want Feather
	}{
		{"Bolt", Bolt, Feather{Dispatch: Inline}},
		{"Arrow", Arrow, Feather{Dispatch: Pool}},
		{"Spear", Spear, Feather{Dispatch: Takeover}},
		{"Plaintext", Plaintext, Feather{Dispatch: Inline, ResponseMode: responsePlaintext}},
		{"JSON", JSON, Bolt},
		{"Query", Query, Spear},
		{"Render", Render, Spear},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !featherEqual(tt.f, tt.want) {
				t.Errorf("%s = %+v, want %+v", tt.name, tt.f, tt.want)
			}
		})
	}
}

func TestPlaintextPresetResponseMode(t *testing.T) {
	if Plaintext.ResponseMode != responsePlaintext {
		t.Fatalf("Plaintext response mode = %v, want responsePlaintext", Plaintext.ResponseMode)
	}
	if Bolt.ResponseMode != responseGeneric {
		t.Fatalf("Bolt response mode = %v, want responseGeneric", Bolt.ResponseMode)
	}
}

func TestStringers(t *testing.T) {
	if s := Inline.String(); s != "Inline" {
		t.Errorf("Inline.String() = %q", s)
	}
	if s := Pool.String(); s != "Pool" {
		t.Errorf("Pool.String() = %q", s)
	}
	if s := Spawn.String(); s != "Spawn" {
		t.Errorf("Spawn.String() = %q", s)
	}
	if s := Takeover.String(); s != "Takeover" {
		t.Errorf("Takeover.String() = %q", s)
	}
}

func TestUnknownStringers(t *testing.T) {
	if s := DispatchMode(99).String(); s != "Unknown" {
		t.Errorf("DispatchMode(99).String() = %q", s)
	}
}

func TestStaticOption(t *testing.T) {
	resp := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK")
	f := Bolt.With(Static(resp))
	if !reflect.DeepEqual(f.StaticResponse, resp) {
		t.Errorf("StaticResponse = %q, want %q", f.StaticResponse, resp)
	}
	// original must not be mutated
	if Bolt.StaticResponse != nil {
		t.Error("Static() mutated Bolt preset")
	}
}

func TestWingStaticTextOption(t *testing.T) {
	var rc routeConfig
	WingStaticText(200, "text/plain", "ready")(&rc)

	if rc.wingFeather == nil {
		t.Fatal("WingStaticText did not set a Wing feather")
	}
	if rc.wingFeather.Dispatch != Inline {
		t.Fatalf("Dispatch = %v, want Inline", rc.wingFeather.Dispatch)
	}
	if !bytes.HasPrefix(rc.wingFeather.StaticResponse, []byte("HTTP/1.1 200 OK\r\n")) {
		t.Fatalf("static response has wrong status line: %q", rc.wingFeather.StaticResponse)
	}
	if !bytes.Contains(rc.wingFeather.StaticResponse, []byte("ready")) {
		t.Fatalf("static response missing body: %q", rc.wingFeather.StaticResponse)
	}
	if Bolt.StaticResponse != nil {
		t.Fatal("WingStaticText mutated Bolt preset")
	}
}

func TestWingStaticJSONOption(t *testing.T) {
	var rc routeConfig
	WingStaticJSON(200, `{"ok":true}`)(&rc)

	if rc.wingFeather == nil {
		t.Fatal("WingStaticJSON did not set a Wing feather")
	}
	if !bytes.Contains(rc.wingFeather.StaticResponse, []byte("Content-Type: application/json; charset=utf-8\r\n")) {
		t.Fatalf("static JSON response missing content type: %q", rc.wingFeather.StaticResponse)
	}
	if !bytes.Contains(rc.wingFeather.StaticResponse, []byte(`{"ok":true}`)) {
		t.Fatalf("static JSON response missing body: %q", rc.wingFeather.StaticResponse)
	}
	if JSON.StaticResponse != nil {
		t.Fatal("WingStaticJSON mutated JSON preset")
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

func TestDispatchOption(t *testing.T) {
	f := Feather{}
	f = f.With(Dispatch(Takeover))
	if f.Dispatch != Takeover {
		t.Errorf("Dispatch = %v, want Takeover", f.Dispatch)
	}
}
