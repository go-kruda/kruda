package main

import (
	"testing"

	"github.com/go-kruda/kruda"
)

func TestNormalizeBenchDispatchDBDefault(t *testing.T) {
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
			got, err := normalizeBenchDispatch(tt.input, "takeover")
			if err != nil {
				t.Fatalf("normalizeBenchDispatch(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeBenchDispatch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeBenchDispatchCPUDefault(t *testing.T) {
	got, err := normalizeBenchDispatch("", "inline")
	if err != nil {
		t.Fatalf("normalizeBenchDispatch CPU default error = %v", err)
	}
	if got != "inline" {
		t.Fatalf("normalizeBenchDispatch CPU default = %q, want inline", got)
	}
}

func TestNormalizeBenchDispatchRejectsUnknown(t *testing.T) {
	if _, err := normalizeBenchDispatch("weird", "inline"); err == nil {
		t.Fatal("expected error for unknown dispatch mode")
	}
}

func TestCPURouteOptions(t *testing.T) {
	if got := cpuRouteOptions("inline", kruda.Plaintext); len(got) != 1 {
		t.Fatalf("inline options len = %d, want 1", len(got))
	}
	if got := cpuRouteOptions("pool", kruda.JSON); len(got) != 1 {
		t.Fatalf("pool options len = %d, want 1", len(got))
	}
	if got := cpuRouteOptions("spawn", kruda.JSON); len(got) != 1 {
		t.Fatalf("spawn options len = %d, want 1", len(got))
	}
	if got := cpuRouteOptions("takeover", kruda.JSON); len(got) != 1 {
		t.Fatalf("takeover options len = %d, want 1", len(got))
	}
}

func TestDBRouteOptions(t *testing.T) {
	if got := dbRouteOptions("takeover", kruda.DB); len(got) != 1 {
		t.Fatalf("takeover options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("pool", kruda.DB); len(got) != 1 {
		t.Fatalf("pool options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("spawn", kruda.DB); len(got) != 1 {
		t.Fatalf("spawn options len = %d, want 1", len(got))
	}
	if got := dbRouteOptions("inline", kruda.DB); len(got) != 1 {
		t.Fatalf("inline options len = %d, want 1", len(got))
	}
}
