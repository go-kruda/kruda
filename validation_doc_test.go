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
		body := string(b)

		// A document that registers a custom rule may then use it, and that
		// documentation is correct. Register is a supported feature — RuleNames
		// reports registered rules alongside the built-in ones — so a guard that
		// only knew builtinRules would reject a valid page and blame the package
		// for not implementing a rule the page itself supplies.
		local := registeredRuleNames(body)

		for _, m := range docValidateTagRe.FindAllStringSubmatch(body, -1) {
			for _, rule := range strings.Split(m[1], ",") {
				name, _, _ := strings.Cut(strings.TrimSpace(rule), "=")
				if name == "" {
					continue
				}
				checked++
				if known[name] || local[name] {
					continue
				}
				t.Errorf("%s documents validate rule %q, which this package does not implement and "+
					"the file does not Register — a reader copying that tag gets a field that is "+
					"never checked", f, name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("matched files but extracted no rule names; the tag pattern is stale")
	}
	t.Logf("verified %d documented rule use(s)", checked)
}

// registerCallRe matches a custom rule being registered, in either the direct
// form (v.Register("adult", ...)) or through the app (app.Validator().Register).
var registerCallRe = regexp.MustCompile(`\.Register\(\s*"([a-zA-Z_][a-zA-Z0-9_]*)"`)

// registeredRuleNames returns the rule names a document registers for itself.
func registeredRuleNames(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range registerCallRe.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
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
