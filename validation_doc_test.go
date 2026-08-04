package kruda

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var docValidateTagRe = regexp.MustCompile(`validate:\\?"([^"\\]+)`)

// TestDocumentedRulesExist checks every `validate` tag in shipped documentation
// against the rules this package actually implements.
//
// cmd/kruda has the equivalent guard for the MCP server's embedded docs, and it
// caught real defects. Nothing covered the site documentation, README or doc
// comments, which is the larger surface: a reader copies a tag out of the guide,
// the rule does not exist, and the field silently goes unenforced — the failure
// mode validation-by-default was supposed to end.
//
// The check runs against the live rule set rather than a hardcoded list, so
// adding or removing a rule cannot leave the guard asserting yesterday's truth.
func TestDocumentedRulesExist(t *testing.T) {
	known := make(map[string]bool, len(builtinRules()))
	for name := range builtinRules() {
		known[name] = true
	}
	// Modifiers are legal in a tag but carry no ValidatorFunc, so they are
	// absent from the rules map by design. Without them here the guard would
	// reject correct documentation.
	for _, mod := range validationModifiers {
		known[mod] = true
	}

	files := shippedFilesMentioning(t, `validate:"`)
	if len(files) == 0 {
		t.Fatal(`found no shipped file containing validate:" — the guard is scanning the wrong tree`)
	}
	t.Logf("checking %d shipped file(s)", len(files))

	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range docValidateTagRe.FindAllStringSubmatch(string(b), -1) {
			for _, rule := range strings.Split(m[1], ",") {
				name, _, _ := strings.Cut(strings.TrimSpace(rule), "=")
				if name == "" {
					continue
				}
				checked++
				if !known[name] {
					t.Errorf("%s documents validate rule %q, which this package does not implement — "+
						"a reader copying that tag gets a field that is never checked", f, name)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("matched files but extracted no rule names; the tag pattern is stale")
	}
	t.Logf("verified %d documented rule use(s)", checked)
}

// TestValidationModifiersAreHandled pins validationModifiers to the parser. If a
// modifier is dropped from buildValidators, the doc guard above would go on
// accepting it as a legal tag word and documentation would keep teaching a tag
// that no longer does anything.
func TestValidationModifiersAreHandled(t *testing.T) {
	src, err := os.ReadFile("validation.go")
	if err != nil {
		t.Fatalf("read validation.go: %v", err)
	}
	for _, mod := range validationModifiers {
		if !regexp.MustCompile(`(?m)^\s*case "` + mod + `":`).Match(src) {
			t.Errorf("validation.go no longer handles the %q modifier; validationModifiers is stale", mod)
		}
	}
}
