package wing_test

import (
	"testing"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport/wing"
)

func TestAliasReExportsAllSymbols(t *testing.T) {
	// Compilation alone proves the type aliases re-export the symbols.
	var _ wing.Config
	var _ *wing.Transport
	var _ wing.Feather
	var _ wing.FeatherOption
	var _ wing.FeatherTable
	var _ wing.DispatchMode
	var _ wing.RawRequest

	// Constants are constants in both packages.
	if wing.Inline != kruda.Inline {
		t.Errorf("wing.Inline %v != kruda.Inline %v", wing.Inline, kruda.Inline)
	}
	if wing.Pool != kruda.Pool {
		t.Errorf("wing.Pool %v != kruda.Pool %v", wing.Pool, kruda.Pool)
	}
	if wing.Spawn != kruda.Spawn {
		t.Errorf("wing.Spawn %v != kruda.Spawn %v", wing.Spawn, kruda.Spawn)
	}
	if wing.Takeover != kruda.Takeover {
		t.Errorf("wing.Takeover %v != kruda.Takeover %v", wing.Takeover, kruda.Takeover)
	}
}

func TestAliasFeatherPresetsMatchCore(t *testing.T) {
	cases := []struct {
		name string
		got  wing.Feather
		want kruda.Feather
	}{
		{"Bolt", wing.Bolt, kruda.Bolt},
		{"Arrow", wing.Arrow, kruda.Arrow},
		{"Spear", wing.Spear, kruda.Spear},
		{"Plaintext", wing.Plaintext, kruda.Plaintext},
		{"JSON", wing.JSON, kruda.JSON},
		{"Query", wing.Query, kruda.Query},
		{"Render", wing.Render, kruda.Render},
	}
	for _, tc := range cases {
		if tc.got.Dispatch != tc.want.Dispatch {
			t.Errorf("%s dispatch mismatch: wing=%v kruda=%v", tc.name, tc.got.Dispatch, tc.want.Dispatch)
		}
	}
}

func TestAliasNewReturnsCoreTransport(t *testing.T) {
	tr := wing.New(wing.Config{Workers: 1})
	if tr == nil {
		t.Fatal("wing.New returned nil")
	}
	// Type aliases mean *wing.Transport IS *kruda.Transport, no conversion needed.
	var _ *kruda.Transport = tr
}

func TestAliasOptionConstructors(t *testing.T) {
	if opt := wing.Dispatch(wing.Pool); opt == nil {
		t.Fatal("wing.Dispatch returned nil FeatherOption")
	}
	if opt := wing.Static([]byte("OK")); opt == nil {
		t.Fatal("wing.Static returned nil FeatherOption")
	}
	tbl := wing.NewFeatherTable(map[string]wing.Feather{
		"GET /ping": wing.Bolt,
	}, wing.Bolt)
	_ = tbl
}
