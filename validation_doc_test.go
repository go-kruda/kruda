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
		//
		// The exception has to respect the ordering contract, though: validators
		// are compiled at route registration, so a rule registered afterwards
		// never takes effect (see buildTypedHandler). Accepting such a file
		// would bless an example that silently does not validate — the same
		// failure this guard exists to catch, arriving from the other side.
		local := registeredRuleNames(body, strings.HasSuffix(f, ".go"))

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

// routeRegistrationRe matches the calls that compile validators. A custom rule
// added after one of these does not apply to it.
//
// The alternation covers the Group* and *X variants because they reach the same
// place — GetX delegates to Get, GroupResource to registerResource — and an
// example written against a Group would otherwise slip the ordering check
// entirely. The kruda. qualifier is optional because doc comments inside this
// package call these unqualified, and an importer may alias the package.
// TestRouteRegistrationMatcherCoversEveryEntryPoint derives the real list from
// the source so this pattern cannot fall behind a newly added entry point.
var routeRegistrationRe = regexp.MustCompile(`(?:kruda\.)?(?:Group)?(?:Get|Post|Put|Patch|Delete|Resource)X?\[`)

// registeredRuleNames returns the rule names a document registers for itself.
//
// ordered applies the "before route registration" half of the contract, and is
// set only for Go files: there, text order is execution order, so a Register
// call below the first route is provably too late. Markdown presents snippets
// in teaching order rather than execution order — a guide may well show the
// route first and the registration afterwards while the prose sequences them
// correctly — so imposing source order there would fail correct pages.
func registeredRuleNames(body string, ordered bool) map[string]bool {
	limit := len(body)
	if ordered {
		if loc := routeRegistrationRe.FindStringIndex(body); loc != nil {
			limit = loc[0]
		}
	}
	out := map[string]bool{}
	for _, m := range registerCallRe.FindAllStringSubmatchIndex(body, -1) {
		if m[0] >= limit {
			continue
		}
		out[body[m[2]:m[3]]] = true
	}
	return out
}

// TestRegisteredRuleNamesRespectsOrdering pins the half of the exception that is
// easy to lose: a rule registered after route registration never applies, so
// accepting it would let the guard bless an example that silently does not
// validate.
func TestRegisteredRuleNamesRespectsOrdering(t *testing.T) {
	const route = "kruda.Post[In, Out](app, \"/x\", h)"
	const reg = "v.Register(\"adult\", f)"

	cases := []struct {
		name    string
		body    string
		ordered bool
		want    bool
	}{
		{"go, registered before the route", reg + "\n" + route, true, true},
		{"go, registered after the route", route + "\n" + reg, true, false},
		{"go, no route in the file", reg, true, true},
		// Markdown sequences snippets for teaching, not execution.
		{"markdown, order not meaningful", route + "\n" + reg, false, true},
	}
	for _, c := range cases {
		if got := registeredRuleNames(c.body, c.ordered)["adult"]; got != c.want {
			t.Errorf("%s: accepted=%v, want %v", c.name, got, c.want)
		}
	}
}

// TestRouteRegistrationMatcherCoversEveryEntryPoint derives the set of exported
// generic route registrars from handler.go and resource.go and requires the
// ordering matcher to recognise each one.
//
// Hand-written, the pattern had already fallen behind: it listed five names and
// missed eleven, so an example built on a Group or on the *X shorthand escaped
// the ordering check silently. Reading the list from source means adding an
// entry point fails this test instead.
func TestRouteRegistrationMatcherCoversEveryEntryPoint(t *testing.T) {
	declRe := regexp.MustCompile(`(?m)^func ([A-Z][A-Za-z]*)\[`)
	var names []string
	for _, f := range []string{"handler.go", "resource.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range declRe.FindAllStringSubmatch(string(b), -1) {
			names = append(names, m[1])
		}
	}
	if len(names) < 10 {
		t.Fatalf("found only %d generic route registrars; the discovery pattern is stale", len(names))
	}
	for _, n := range names {
		for _, call := range []string{n + "[", "kruda." + n + "["} {
			if !routeRegistrationRe.MatchString(call) {
				t.Errorf("routeRegistrationRe does not match %q — an example registering a custom "+
					"rule after this call would escape the ordering check", call)
			}
		}
	}
	t.Logf("covered %d route registrars", len(names))
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
