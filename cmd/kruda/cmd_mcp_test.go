package main

import (
	"strings"
	"testing"
)

func TestSuggestPresetStreamingRouteDoesNotInventWingStream(t *testing.T) {
	preset, reason := suggestPreset(routeInfo{Method: "GET", Path: "/events"})
	if preset != "none" {
		t.Fatalf("preset = %q, want none", preset)
	}
	if strings.Contains(preset, "WingStream") || strings.Contains(reason, "WingStream") {
		t.Fatalf("streaming suggestion mentions WingStream: preset=%q reason=%q", preset, reason)
	}
}

func TestMCPDocsDoNotMentionWingStream(t *testing.T) {
	for _, topic := range []string{"wing", "sse"} {
		if strings.Contains(krudaDocs[topic], "WingStream") {
			t.Fatalf("topic %q mentions nonexistent WingStream API", topic)
		}
	}
}

func TestSuggestPresetDBReadStyleRoute(t *testing.T) {
	preset, reason := suggestPreset(routeInfo{Method: "GET", Path: "/queries"})
	if preset != "kruda.DB" {
		t.Fatalf("preset = %q, want WingQuery()", preset)
	}
	if !strings.Contains(reason, "read-style query") {
		t.Fatalf("reason = %q, want read-style query guidance", reason)
	}
}

func TestSuggestPresetWriteRouteRequiresBenchmarking(t *testing.T) {
	preset, reason := suggestPreset(routeInfo{Method: "POST", Path: "/products"})
	if preset != "kruda.DB" {
		t.Fatalf("preset = %q, want WingQuery()", preset)
	}
	if !strings.Contains(reason, "benchmark") || !strings.Contains(reason, "p99") {
		t.Fatalf("reason = %q, want benchmark and p99 guidance", reason)
	}
}

func TestMCPWingDocsKeepQueryAndWriteGuidanceSeparate(t *testing.T) {
	doc := krudaDocs["wing"]
	if strings.Contains(doc, "DB read/write") || strings.Contains(doc, "database read/write") {
		t.Fatalf("wing docs use broad read/write guidance: %q", doc)
	}
	if !strings.Contains(doc, "DB/Redis read-style queries") {
		t.Fatalf("wing docs missing read-style query guidance")
	}
	if !strings.Contains(doc, "Benchmark write-heavy routes") {
		t.Fatalf("wing docs missing write-heavy benchmark guidance")
	}
}
