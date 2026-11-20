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

func TestPresetTableParamRoutesMatchWholePattern(t *testing.T) {
	db := Spear
	plain := Arrow
	static := Preset{Dispatch: Inline, StaticResponse: []byte("HTTP/1.1 200 OK\r\nContent-Length: 1\r\n\r\ns")}
	routes := map[string]Preset{
		"GET /users/:id":         db,
		"GET /users/:id/avatar":  plain,
		"GET /items/:id/summary": static,
		"GET /files/*path":       plain,
		"GET /tags/:name?":       db,
	}
	def := Render
	for range 20 { // map order differs per build; every build must agree
		ft := NewPresetTable(routes, def)
		for _, tc := range []struct {
			path string
			want Preset
		}{
			{"/users/7", db},
			{"/users/7/avatar", plain},
			{"/users/7/other", def},
			{"/items/42/summary", static},
			{"/items/42/missing", def},
			{"/items/42", def},
			{"/files/a/b.txt", plain},
			{"/files/", plain},
			{"/tags", db},
			{"/tags/go", db},
			{"/tags/go/x", def},
		} {
			got := ft.Lookup("GET", tc.path)
			if got.Dispatch != tc.want.Dispatch || got.ResponseMode != tc.want.ResponseMode || string(got.StaticResponse) != string(tc.want.StaticResponse) {
				t.Fatalf("Lookup(%s) = %+v, want %+v", tc.path, got, tc.want)
			}
		}
	}
}

// TestPresetTableRegexParamFollowsRouter gives a regex-constrained param a
// preset next to a plain param route without one. The router skips the regex
// route for a value its regex rejects, so the preset must not apply there.
func TestPresetTableRegexParamFollowsRouter(t *testing.T) {
	routes := map[string]Preset{
		"GET /jobs/:id<[0-9]+>":  Spear,
		"GET /jobs/:slug":        {inherit: true},
		"GET /runs/:id<[0-9]+>?": Arrow,
	}
	def := Render
	for range 20 {
		ft := NewPresetTable(routes, def)
		for _, tc := range []struct {
			path string
			want Preset
		}{
			{"/jobs/42", Spear},
			{"/jobs/latest", def},
			{"/runs/7", Arrow},
			{"/runs", Arrow},
			{"/runs/x", def},
		} {
			if got := ft.Lookup("GET", tc.path); got.Dispatch != tc.want.Dispatch || got.ResponseMode != tc.want.ResponseMode {
				t.Fatalf("Lookup(%s) = %v/%v, want %v/%v", tc.path, got.Dispatch, got.ResponseMode, tc.want.Dispatch, tc.want.ResponseMode)
			}
		}
	}
}

// TestPresetTableCustomMethodsMatchParams gives OPTIONS and a custom method
// param and wildcard presets. They follow the same rules as GET.
func TestPresetTableCustomMethodsMatchParams(t *testing.T) {
	routes := map[string]Preset{
		"OPTIONS /items/:id":    Arrow,
		"OPTIONS /items/static": {inherit: true},
		"PURGE /cache/*key":     Spear,
		"PURGE /exact":          Arrow,
	}
	def := Render
	ft := NewPresetTable(routes, def)
	for _, tc := range []struct {
		method, path string
		want         Preset
	}{
		{"OPTIONS", "/items/7", Arrow},
		{"OPTIONS", "/items/static", def},
		{"OPTIONS", "/items/7/x", def},
		{"PURGE", "/cache/a/b", Spear},
		{"PURGE", "/exact", Arrow},
		{"PURGE", "/other", def},
		{"REPORT", "/cache/a", def},
	} {
		if got := ft.Lookup(tc.method, tc.path); got.Dispatch != tc.want.Dispatch || got.ResponseMode != tc.want.ResponseMode {
			t.Fatalf("Lookup(%s %s) = %v/%v, want %v/%v", tc.method, tc.path, got.Dispatch, got.ResponseMode, tc.want.Dispatch, tc.want.ResponseMode)
		}
	}
	if n := testing.AllocsPerRun(100, func() { ft.Lookup("PURGE", "/exact") }); n != 0 {
		t.Fatalf("custom-method lookup allocates %v", n)
	}
}

// TestPresetUnknownDispatchRunsInline gives a preset a dispatch mode Wing does
// not know. It runs inline instead of reaching a dispatch branch that has no
// Pool behind it.
func TestPresetUnknownDispatchRunsInline(t *testing.T) {
	ft := NewPresetTable(map[string]Preset{"GET /x": {Dispatch: DispatchMode(99)}}, Preset{Dispatch: DispatchMode(42)})
	if got := ft.Lookup("GET", "/x").Dispatch; got != Inline {
		t.Fatalf("route dispatch = %v, want Inline", got)
	}
	if got := ft.Lookup("GET", "/y").Dispatch; got != Inline {
		t.Fatalf("default dispatch = %v, want Inline", got)
	}
}
