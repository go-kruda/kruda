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

func TestWingFeatherOption(t *testing.T) {
	var rc routeConfig
	WingFeather(Arrow)(&rc)

	if rc.wingFeather == nil {
		t.Fatal("WingFeather did not set a Wing feather")
	}
	if rc.wingFeather.Dispatch != Pool {
		t.Fatalf("Dispatch = %v, want Pool", rc.wingFeather.Dispatch)
	}
	if Arrow.StaticResponse != nil {
		t.Fatal("WingFeather mutated Arrow preset")
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

func TestDispatchOption(t *testing.T) {
	f := Feather{}
	f = f.With(Dispatch(Takeover))
	if f.Dispatch != Takeover {
		t.Errorf("Dispatch = %v, want Takeover", f.Dispatch)
	}
}
