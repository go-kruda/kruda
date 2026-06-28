package kruda

import (
	"bytes"
	"reflect"
	"testing"
)

func presetEqual(a, b Preset) bool { return reflect.DeepEqual(a, b) }

func TestPresetDefaults(t *testing.T) {
	var f Preset
	f.defaults()
	want := Preset{Dispatch: Inline}
	if !presetEqual(f, want) {
		t.Errorf("defaults() = %+v, want %+v", f, want)
	}
}

func TestPresetWith(t *testing.T) {
	f := Arrow.With(Dispatch(Spawn))
	if f.Dispatch != Spawn {
		t.Errorf("Dispatch = %v, want Spawn", f.Dispatch)
	}
}

func TestPresetWithDoesNotMutateOriginal(t *testing.T) {
	original := Arrow
	_ = Arrow.With(Dispatch(Inline))
	if !presetEqual(Arrow, original) {
		t.Errorf("With() mutated original preset: got %+v, want %+v", Arrow, original)
	}
}

func TestPresetValues(t *testing.T) {
	tests := []struct {
		name string
		f    Preset
		want Preset
	}{
		{"Bolt", Bolt, Preset{Dispatch: Inline}},
		{"Arrow", Arrow, Preset{Dispatch: Pool}},
		{"Spear", Spear, Preset{Dispatch: Takeover}},
		{"Plaintext", Plaintext, Preset{Dispatch: Inline}},
		{"JSON", JSON, Bolt},
		{"DB", DB, Spear},
		{"Render", Render, Preset{Dispatch: Takeover, ResponseMode: responseRender}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !presetEqual(tt.f, tt.want) {
				t.Errorf("%s = %+v, want %+v", tt.name, tt.f, tt.want)
			}
		})
	}
}

func TestRenderPresetResponseMode(t *testing.T) {
	if Render.ResponseMode != responseRender {
		t.Fatalf("Render response mode = %v, want responseRender", Render.ResponseMode)
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

func TestStaticTextOption(t *testing.T) {
	var rc routeConfig
	StaticText(200, "text/plain", "ready").applyRoute(&rc)

	if rc.preset == nil {
		t.Fatal("StaticText did not set a Wing preset")
	}
	if rc.preset.Dispatch != Inline {
		t.Fatalf("Dispatch = %v, want Inline", rc.preset.Dispatch)
	}
	if !bytes.HasPrefix(rc.preset.StaticResponse, []byte("HTTP/1.1 200 OK\r\n")) {
		t.Fatalf("static response has wrong status line: %q", rc.preset.StaticResponse)
	}
	if !bytes.Contains(rc.preset.StaticResponse, []byte("ready")) {
		t.Fatalf("static response missing body: %q", rc.preset.StaticResponse)
	}
	if Bolt.StaticResponse != nil {
		t.Fatal("StaticText mutated Bolt preset")
	}
}

func TestWingPresetOption(t *testing.T) {
	var rc routeConfig
	Arrow.applyRoute(&rc)

	if rc.preset == nil {
		t.Fatal("Preset option did not set a Wing preset")
	}
	if rc.preset.Dispatch != Pool {
		t.Fatalf("Dispatch = %v, want Pool", rc.preset.Dispatch)
	}
	if Arrow.StaticResponse != nil {
		t.Fatal("Preset option mutated Arrow preset")
	}
}

func TestStaticJSONOption(t *testing.T) {
	var rc routeConfig
	StaticJSON(200, `{"ok":true}`).applyRoute(&rc)

	if rc.preset == nil {
		t.Fatal("StaticJSON did not set a Wing preset")
	}
	if !bytes.Contains(rc.preset.StaticResponse, []byte("Content-Type: application/json; charset=utf-8\r\n")) {
		t.Fatalf("static JSON response missing content type: %q", rc.preset.StaticResponse)
	}
	if !bytes.Contains(rc.preset.StaticResponse, []byte(`{"ok":true}`)) {
		t.Fatalf("static JSON response missing body: %q", rc.preset.StaticResponse)
	}
	if JSON.StaticResponse != nil {
		t.Fatal("StaticJSON mutated JSON preset")
	}
}

func TestDispatchOption(t *testing.T) {
	f := Preset{}
	f = f.With(Dispatch(Takeover))
	if f.Dispatch != Takeover {
		t.Errorf("Dispatch = %v, want Takeover", f.Dispatch)
	}
}

func TestStreamPreset(t *testing.T) {
	if Stream.Dispatch != Takeover {
		t.Errorf("Stream.Dispatch = %v, want Takeover", Stream.Dispatch)
	}
	if Stream.ResponseMode != responseStream {
		t.Errorf("Stream.ResponseMode = %v, want responseStream", Stream.ResponseMode)
	}
}
