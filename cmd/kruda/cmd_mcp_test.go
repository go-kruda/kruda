package main

import (
	"strings"
	"testing"
)

func TestSuggestFeatherStreamingRouteDoesNotInventWingStream(t *testing.T) {
	feather, reason := suggestFeather(routeInfo{Method: "GET", Path: "/events"})
	if feather != "none" {
		t.Fatalf("feather = %q, want none", feather)
	}
	if strings.Contains(feather, "WingStream") || strings.Contains(reason, "WingStream") {
		t.Fatalf("streaming suggestion mentions WingStream: feather=%q reason=%q", feather, reason)
	}
}

func TestMCPDocsDoNotMentionWingStream(t *testing.T) {
	for _, topic := range []string{"wing", "sse"} {
		if strings.Contains(krudaDocs[topic], "WingStream") {
			t.Fatalf("topic %q mentions nonexistent WingStream API", topic)
		}
	}
}

func TestSuggestFeatherDBReadStyleRoute(t *testing.T) {
	feather, reason := suggestFeather(routeInfo{Method: "GET", Path: "/queries"})
	if feather != "WingQuery()" {
		t.Fatalf("feather = %q, want WingQuery()", feather)
	}
	if !strings.Contains(reason, "read-style query") {
		t.Fatalf("reason = %q, want read-style query guidance", reason)
	}
}

func TestSuggestFeatherWriteRouteRequiresBenchmarking(t *testing.T) {
	feather, reason := suggestFeather(routeInfo{Method: "POST", Path: "/products"})
	if feather != "WingQuery()" {
		t.Fatalf("feather = %q, want WingQuery()", feather)
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
