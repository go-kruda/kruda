package main

import (
	"os"
	"path/filepath"
	"regexp"
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

// krudaValidationRules extracts the rule names core implements, by reading the
// source rather than duplicating the list here. cmd/kruda does not import the
// core module, so a copied list would silently drift the moment a rule is added
// or removed.
func krudaValidationRules(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "validation.go"))
	if err != nil {
		t.Skipf("cannot read core validation.go: %v", err)
	}
	// The rules map entries look like:  "max_size":   validateMaxSize,
	re := regexp.MustCompile(`(?m)^\s+"([a-z_0-9]+)":\s+validate[A-Za-z]+,`)
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		out[m[1]] = true
	}
	if len(out) < 10 {
		t.Fatalf("extracted only %d rules from core; the pattern is stale", len(out))
	}

	// The modifiers are legal in a tag but are not entries in the rules map —
	// they carry no ValidatorFunc and are handled structurally by buildValidators.
	// Without them here this guard rejects correct documentation, reporting that
	// core does not implement a modifier it does implement.
	for _, mod := range krudaValidationModifiers {
		if !modifierIsHandled(t, string(b), mod) {
			t.Fatalf("core no longer handles the %q modifier; this guard is stale", mod)
		}
		out[mod] = true
	}
	return out
}

// krudaValidationModifiers are the tag words that change how surrounding rules
// apply rather than adding a check of their own.
var krudaValidationModifiers = []string{"omitempty", "dive"}

// modifierIsHandled pins the guard to core rather than to this list: if a
// modifier is ever dropped, the guard must fail instead of quietly permitting a
// tag word that no longer works.
func modifierIsHandled(t *testing.T, validationSrc, mod string) bool {
	t.Helper()
	return regexp.MustCompile(`(?m)^\s*case "` + mod + `":`).MatchString(validationSrc)
}

var validateTagRe = regexp.MustCompile(`validate:\\?"([^"\\]+)`)

// TestMCPDocsOnlyUseRulesThatExist is the check that matters most for AI-facing
// guidance. Kruda's tag syntax looks like go-playground/validator but implements
// about 20 rules against that library's hundreds, and an assistant reproducing
// habits from it will reach for omitempty, dive, eq or required_if. None of those
// exist here, so the tag is skipped and the field goes unenforced — the failure
// being silent is what makes it worth a test rather than a review.
func TestMCPDocsOnlyUseRulesThatExist(t *testing.T) {
	known := krudaValidationRules(t)
	for topic, body := range krudaDocs {
		for _, m := range validateTagRe.FindAllStringSubmatch(body, -1) {
			for _, rule := range strings.Split(m[1], ",") {
				name, _, _ := strings.Cut(strings.TrimSpace(rule), "=")
				if name == "" {
					continue
				}
				if !known[name] {
					t.Errorf("krudaDocs[%q] uses validate rule %q, which core does not implement — it would be skipped and the field left unenforced", topic, name)
				}
			}
		}
	}
}

// TestMCPDocsDoNotTeachValidationIsOptional replaces a guard that asserted the
// opposite. Validation was opt-in and the docs had to say so; since it became the
// default, telling an assistant to add a Validator to switch it on is wrong
// advice, and calling it opt-in is a false statement.
func TestMCPDocsDoNotTeachValidationIsOptional(t *testing.T) {
	stale := []string{
		"validation is opt-in",
		"validation is OPT-IN",
		"opt-in. build the app with a validator",
		"only when a validator",
		"need a validator",
		"without a validator the",
	}
	for topic, body := range krudaDocs {
		low := strings.ToLower(body)
		for _, phrase := range stale {
			if strings.Contains(low, strings.ToLower(phrase)) {
				t.Errorf("krudaDocs[%q] still describes validation as optional (%q); it is the default since v1.7.1", topic, phrase)
			}
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
