package main

import (
	"strings"
	"testing"
)

func TestSuggestPresetStreamingRouteUsesStreamPreset(t *testing.T) {
	preset, reason := suggestPreset(routeInfo{Method: "GET", Path: "/events"})
	if preset != "kruda.Stream" {
		t.Fatalf("preset = %q, want kruda.Stream", preset)
	}
	// The streaming preset is kruda.Stream (v1.5.0+), never the fabricated
	// "WingStream" name an earlier version could have invented.
	if strings.Contains(preset, "WingStream") || strings.Contains(reason, "WingStream") {
		t.Fatalf("streaming suggestion mentions WingStream: preset=%q reason=%q", preset, reason)
	}
}

func TestSuggestPresetWebSocketRouteUsesHijackPreset(t *testing.T) {
	preset, _ := suggestPreset(routeInfo{Method: "GET", Path: "/ws"})
	if preset != "kruda.Hijack" {
		t.Fatalf("preset = %q, want kruda.Hijack", preset)
	}
}

func TestMCPDocsHaveWebSocketTopic(t *testing.T) {
	doc, ok := krudaDocs["websocket"]
	if !ok {
		t.Fatal("missing 'websocket' doc topic")
	}
	if !strings.Contains(doc, "ws.HandleFunc") {
		t.Fatalf("websocket topic missing ws.HandleFunc usage")
	}
}

func TestMCPDocsDoNotMentionWingStream(t *testing.T) {
	for _, topic := range []string{"wing", "sse", "websocket"} {
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

// TestMCPDocsDoNotTeachAutomaticValidation guards the guidance AI assistants
// read. Validation is opt-in, but krudaDocs used to state the opposite —
// "Kruda validates struct tags automatically in typed handlers" — and never
// mentioned WithValidator anywhere, so every assistant following it generated
// apps whose validate tags did nothing. The scaffold ships .mcp.json, so this
// is the default path for AI-assisted users.
func TestMCPDocsDoNotTeachAutomaticValidation(t *testing.T) {
	falseClaims := []string{
		"validates struct tags automatically",
		"validation runs automatically",
		"automatically validated",
		"auto-validation",
		"auto-validate",
		"already parsed and validated",
	}
	for topic, body := range krudaDocs {
		for _, claim := range falseClaims {
			if strings.Contains(strings.ToLower(body), claim) {
				t.Errorf("krudaDocs[%q] tells assistants validation is automatic (%q); it is opt-in via WithValidator", topic, claim)
			}
		}
	}

	// Any topic that shows a validate tag has to say how to turn validation on,
	// or it is teaching the trap.
	for topic, body := range krudaDocs {
		if !strings.Contains(body, `validate:"`) {
			continue
		}
		if !strings.Contains(body, "WithValidator") {
			t.Errorf("krudaDocs[%q] shows validate: tags without mentioning WithValidator", topic)
		}
	}
}

// TestMCPDocsUseValidationRulesAsRules guards a silent failure the file-upload
// topic shipped: it wrote `validate:"required" max_size:"5mb" mime:"image/*"`,
// putting two validation rules in struct tags of their own. Only `required`
// compiles from that; max_size and mime are never read, so an upload of any
// size or type is accepted — with a Validator configured and the tags sitting
// right there in the source. Rules belong inside the validate tag, as
// `validate:"required,max_size=5mb,mime=image/*"`.
func TestMCPDocsUseValidationRulesAsRules(t *testing.T) {
	// Rule names that read like plausible standalone struct tags. A rule used as
	// its own tag is silently dropped, which is worse than a compile error.
	rules := []string{"max_size", "mime", "min", "max", "email", "required", "oneof", "uuid"}
	for topic, body := range krudaDocs {
		for _, r := range rules {
			if strings.Contains(body, r+`:"`) {
				t.Errorf("krudaDocs[%q] uses the validation rule %q as its own struct tag; it must go inside validate:\"...\" or it is silently ignored", topic, r)
			}
		}
	}
}

// TestMCPDocsReferenceRealCoreTypes catches samples naming types the core does
// not export — the file-upload topic said *kruda.Upload, which does not exist
// (it is kruda.FileUpload), so the sample never compiled for anyone who copied
// it. The list is the set this test knows to be wrong; MCP samples are not
// compiled anywhere, so this is a floor, not a guarantee.
func TestMCPDocsReferenceRealCoreTypes(t *testing.T) {
	nonexistent := []string{"kruda.Upload{", "*kruda.Upload ", "*kruda.Upload`"}
	for topic, body := range krudaDocs {
		for _, bad := range nonexistent {
			if strings.Contains(body, bad) {
				t.Errorf("krudaDocs[%q] references %q, which the core does not export (did you mean kruda.FileUpload?)", topic, bad)
			}
		}
	}
}
