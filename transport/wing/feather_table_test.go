package wing

import (
	"reflect"
	"testing"
)

func TestFeatherTableLookupExact(t *testing.T) {
	ft := NewFeatherTable(map[string]Feather{
		"GET /plaintext": Bolt,
		"GET /json":      Flash,
		"POST /db":       Arrow,
	}, Arrow)

	tests := []struct {
		method, path string
		want         Feather
	}{
		{"GET", "/plaintext", Bolt},
		{"GET", "/json", Flash},
		{"POST", "/db", Arrow},
		{"GET", "/unknown", Arrow}, // default
		{"DELETE", "/db", Arrow},   // wrong method → default
	}
	for _, tt := range tests {
		got := ft.Lookup(tt.method, tt.path)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Lookup(%q, %q) = %+v, want %+v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestFeatherTableDefault(t *testing.T) {
	ft := NewFeatherTable(nil, Hawk)
	got := ft.Lookup("GET", "/anything")
	if !reflect.DeepEqual(got, Hawk) {
		t.Errorf("empty table Lookup = %+v, want Hawk", got)
	}
}

func TestFeatherTableDefaultsApplied(t *testing.T) {
	// Partial feather — defaults() should fill missing axes.
	ft := NewFeatherTable(map[string]Feather{
		"GET /custom": {Dispatch: Inline},
	}, Arrow)
	got := ft.Lookup("GET", "/custom")
	if got.Engine == 0 {
		t.Error("defaults() not applied: Engine is zero")
	}
	if got.Dispatch != Inline {
		t.Errorf("Dispatch = %v, want Inline", got.Dispatch)
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

func BenchmarkFeatherTableLookup(b *testing.B) {
	ft := NewFeatherTable(map[string]Feather{
		"GET /plaintext": Bolt,
		"GET /json":      Flash,
		"POST /db":       Arrow,
		"GET /fortunes":  Hawk,
		"GET /queries":   Arrow,
		"GET /updates":   Arrow,
		"GET /cached":    Flash,
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
