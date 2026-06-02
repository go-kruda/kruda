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
