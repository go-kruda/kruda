package main

import "testing"

func TestPGOBuildTags(t *testing.T) {
	for _, tc := range []struct{ goflags, want string }{
		{"", "kruda_pprof"},
		{"-tags=custom,kruda_stdjson", "custom,kruda_stdjson,kruda_pprof"},
		{"-tags=kruda_pprof", "kruda_pprof"},
	} {
		t.Setenv("GOFLAGS", tc.goflags)
		got, err := pgoBuildTags(t.TempDir())
		if err != nil || got != tc.want {
			t.Errorf("pgoBuildTags with %q = %q, %v; want %q", tc.goflags, got, err, tc.want)
		}
	}
}
