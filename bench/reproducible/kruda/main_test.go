package main

import (
	"testing"

	"github.com/go-kruda/kruda"
)

func TestNormalizeBenchDBDispatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: "takeover"},
		{name: "takeover", input: "takeover", want: "takeover"},
		{name: "spear alias", input: "Spear", want: "takeover"},
		{name: "pool", input: "pool", want: "pool"},
		{name: "arrow alias", input: "Arrow", want: "pool"},
		{name: "spawn", input: "spawn", want: "spawn"},
		{name: "inline", input: "inline", want: "inline"},
		{name: "bolt alias", input: "Bolt", want: "inline"},
		{name: "trim", input: " pool ", want: "pool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBenchDBDispatch(tt.input)
			if err != nil {
				t.Fatalf("normalizeBenchDBDispatch(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeBenchDBDispatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeBenchDBDispatchRejectsUnknown(t *testing.T) {
	if _, err := normalizeBenchDBDispatch("weird"); err == nil {
		t.Fatal("expected error for unknown dispatch mode")
	}
}

func TestDBRouteOptions(t *testing.T) {
	if got := dbRouteOptions("takeover", kruda.WingQuery()); len(got) != 1 {
		t.Fatalf("takeover options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("pool", kruda.WingQuery()); len(got) != 1 {
		t.Fatalf("pool options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("spawn", kruda.WingQuery()); len(got) != 1 {
		t.Fatalf("spawn options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("inline", kruda.WingQuery()); len(got) != 1 {
		t.Fatalf("inline options len = %d, want 1", len(got))
	}
}
