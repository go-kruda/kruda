package kruda

import "testing"

func TestPresetTableLookupExact(t *testing.T) {
	ft := NewPresetTable(map[string]Preset{
		"GET /plaintext": Bolt,
		"GET /json":      Bolt,
		"POST /db":       Arrow,
	}, Arrow)

	tests := []struct {
		method, path, wantPath string
		want                   Preset
	}{
		{"GET", "/plaintext", "/plaintext", Bolt},
		{"GET", "/json", "/json", Bolt},
		{"POST", "/db", "/db", Arrow},
		{"GET", "/unknown", "", Arrow}, // default
		{"DELETE", "/db", "", Arrow},   // wrong method -> default
	}
	for _, tt := range tests {
		got := ft.Lookup(tt.method, tt.path)
		if got.Dispatch != tt.want.Dispatch || got.ResponseMode != tt.want.ResponseMode || got.path != tt.wantPath {
			t.Errorf("Lookup(%q, %q) = %+v, want %+v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestPresetTableDefault(t *testing.T) {
	ft := NewPresetTable(nil, Arrow)
	got := ft.Lookup("GET", "/anything")
	if got.Dispatch != Arrow.Dispatch || got.ResponseMode != Arrow.ResponseMode || got.path != "" {
		t.Errorf("empty table Lookup = %+v, want Arrow", got)
	}
}

func TestPresetTableDefaultsApplied(t *testing.T) {
	ft := NewPresetTable(map[string]Preset{
		"GET /custom": {Dispatch: Inline},
	}, Arrow)
	got := ft.Lookup("GET", "/custom")
	if got.Dispatch != Inline {
		t.Errorf("Dispatch = %v, want Inline", got.Dispatch)
	}
}

func TestPresetTableMarksCleanExactRoutes(t *testing.T) {
	ft := NewPresetTable(map[string]Preset{
		"GET /plaintext":     Bolt,
		"GET /assets/app.js": Bolt,
		"GET /encoded%2F":    Bolt,
		"GET /users/:id":     Arrow,
	}, Arrow)

	if got := ft.Lookup("GET", "/plaintext"); !got.pathClean {
		t.Fatalf("clean exact route pathClean = false")
	}
	if got := ft.Lookup("GET", "/assets/app.js"); got.pathClean {
		t.Fatalf("dotted exact route pathClean = true")
	}
	if got := ft.Lookup("GET", "/encoded%2F"); got.pathClean {
		t.Fatalf("encoded exact route pathClean = true")
	}
	got := ft.Lookup("GET", "/users/42")
	if got.path != "" || got.pathClean {
		t.Fatalf("param route cached path = %q pathClean=%v, want empty/false", got.path, got.pathClean)
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		key          string
		wantM, wantP string
	}{
		{"GET /plaintext", "GET", "/plaintext"},
		{"POST /api/users", "POST", "/api/users"},
		{"DELETE /", "DELETE", "/"},
		{"CUSTOM", "CUSTOM", "/"},
	}
	for _, tt := range tests {
		m, p := splitKey(tt.key)
		if m != tt.wantM || p != tt.wantP {
			t.Errorf("splitKey(%q) = (%q, %q), want (%q, %q)", tt.key, m, p, tt.wantM, tt.wantP)
		}
	}
}

func BenchmarkPresetTableLookup(b *testing.B) {
	ft := NewPresetTable(map[string]Preset{
		"GET /plaintext": Bolt,
		"GET /json":      Bolt,
		"POST /db":       Arrow,
		"GET /fortunes":  Arrow,
		"GET /queries":   Arrow,
		"GET /updates":   Arrow,
	}, Arrow)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ft.Lookup("GET", "/plaintext")
		_ = ft.Lookup("GET", "/json")
		_ = ft.Lookup("POST", "/db")
		_ = ft.Lookup("GET", "/unknown")
	}
}

func BenchmarkPresetTableLookupExactOne(b *testing.B) {
	ft := NewPresetTable(map[string]Preset{
		"GET /plaintext": Bolt,
		"GET /json":      Bolt,
		"POST /db":       Arrow,
		"GET /fortunes":  Arrow,
		"GET /queries":   Arrow,
		"GET /updates":   Arrow,
	}, Arrow)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ft.Lookup("GET", "/plaintext")
	}
}
