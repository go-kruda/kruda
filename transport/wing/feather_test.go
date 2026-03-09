package wing

import (
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
		{"Plaintext", Plaintext, Bolt},
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

func TestDispatchOption(t *testing.T) {
	f := Feather{}
	f = f.With(Dispatch(Takeover))
	if f.Dispatch != Takeover {
		t.Errorf("Dispatch = %v, want Takeover", f.Dispatch)
	}
}
