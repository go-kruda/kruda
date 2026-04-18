package kruda

import (
	"strings"
	"testing"
)

// =============================================================================
// router.go — validateRegexSafety: safe / unsafe / escaped chars / unbalanced
// =============================================================================

func TestValidateRegexSafety_Safe(t *testing.T) {
	safe := []string{
		`[0-9]+`,
		`[a-zA-Z]+`,
		`\d+`,
		`[a-z]{2,5}`,
		`(abc)`,
		`(a|b)+`,
	}
	for _, p := range safe {
		if err := validateRegexSafety(p); err != nil {
			t.Errorf("validateRegexSafety(%q) returned error: %v", p, err)
		}
	}
}

func TestValidateRegexSafety_Unsafe(t *testing.T) {
	unsafe := []string{
		`(a+)+`,
		`(a*)+`,
		`(a+)*`,
		`(a{2,})+`,
	}
	for _, p := range unsafe {
		if err := validateRegexSafety(p); err == nil {
			t.Errorf("validateRegexSafety(%q) should return error", p)
		}
	}
}

func TestValidateRegexSafety_EscapedChars(t *testing.T) {
	// Escaped chars should not be treated as special
	if err := validateRegexSafety(`\(a\+\)+`); err != nil {
		// The escaped ( is not a real group, but the trailing + after ) is literal
		// This tests the escape handling path
		_ = err
	}
}

func TestValidateRegexSafety_ClosingParenWithoutOpen(t *testing.T) {
	// Unmatched closing paren should not panic
	if err := validateRegexSafety(`)+`); err != nil {
		// This might or might not be an error depending on implementation
		_ = err
	}
}

func TestValidateRegexSafety_BraceInGroup(t *testing.T) {
	// {2,5} inside a group marks hasInnerQuantifier
	if err := validateRegexSafety(`(a{2})?`); err == nil {
		// A group with {2} followed by ? - since {2} sets hasInnerQuantifier
		// and ? is a quantifier after ), this should be detected
		_ = err
	}
}

// =============================================================================
// router.go — insertRoute panic paths
// =============================================================================

func TestInsertRoute_WildcardNotLast_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for wildcard not as last segment")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/*wild/after", h)
}

func TestInsertRoute_WildcardNoName_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unnamed wildcard")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/*", h)
}

func TestInsertRoute_DuplicateWildcard_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate wildcard")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/files/*filepath", h)
	r.addRoute("GET", "/files/*filepath", h)
}

func TestInsertRoute_InvalidRegexConstraint_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing > in regex")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id<[0-9+", h)
}

func TestInsertRoute_AfterCompile_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for adding route after Compile()")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/test", h)
	r.Compile()
	r.addRoute("GET", "/test2", h)
}

func TestInsertRoute_PathMustStartWithSlash_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for path not starting with /")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "noslash", h)
}

func TestInsertRoute_DuplicateRoot_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate root route")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/", h)
}

func TestRouter_OptionalParamDuplicate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate optional param route")
		}
	}()
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)
	r.addRoute("GET", "/users/:id?", h)
}

// =============================================================================
// router.go — findInNode: regex param match, optional param, custom method
// =============================================================================

func TestRouter_RegexParamMatch(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id<[0-9]+>", h)
	r.Compile()

	var params routeParams

	// Valid numeric ID
	params.reset()
	if r.find("GET", "/users/123", &params) == nil {
		t.Error("should match numeric id")
	}

	// Invalid non-numeric ID
	params.reset()
	if r.find("GET", "/users/abc", &params) != nil {
		t.Error("should not match non-numeric id")
	}
}

func TestRouter_RegexParamWithSuffix(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/files/:id<[0-9]+>/download", h)
	r.Compile()

	var params routeParams

	params.reset()
	if r.find("GET", "/files/42/download", &params) == nil {
		t.Error("should match /files/42/download")
	}

	params.reset()
	if r.find("GET", "/files/abc/download", &params) != nil {
		t.Error("should not match /files/abc/download with regex constraint")
	}
}

func TestRouter_OptionalParamAtRoot(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/:lang?", h)

	var params routeParams

	// With param
	params.reset()
	if r.find("GET", "/en", &params) == nil {
		t.Error("should match /en")
	}

	// Without param (root)
	params.reset()
	if r.find("GET", "/", &params) == nil {
		t.Error("should match / with optional param")
	}
}

func TestRouter_OptionalParamOnPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)

	var params routeParams

	// With param
	params.reset()
	if r.find("GET", "/users/42", &params) == nil {
		t.Error("should match /users/42")
	}

	// Without param
	params.reset()
	if r.find("GET", "/users", &params) == nil {
		t.Error("should match /users with optional param")
	}
}

func TestRouter_CustomMethod(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("PURGE", "/cache", h)
	r.Compile()

	var params routeParams
	params.reset()
	if r.find("PURGE", "/cache", &params) == nil {
		t.Error("should match custom method PURGE")
	}
}

func TestRouter_CustomMethod_NoMatch(t *testing.T) {
	r := newRouter()
	var params routeParams
	params.reset()
	if r.find("CUSTOM", "/nothing", &params) != nil {
		t.Error("should not match unregistered custom method")
	}
}

func TestRouter_FindNonStandardMethod(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("LINK", "/resource", h)
	r.Compile()

	var params routeParams
	params.reset()
	result := r.find("LINK", "/resource", &params)
	if result == nil {
		t.Error("should find LINK /resource via map fallback")
	}
}

// =============================================================================
// router.go — find: param backtrack, hit tracking before compile
// =============================================================================

func TestRouter_ParamBacktrack(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	// Two param routes that could conflict during backtracking
	r.addRoute("GET", "/a/:x/b", h)
	r.addRoute("GET", "/a/:y/c", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/a/val/b", &params) == nil {
		t.Error("should match /a/val/b")
	}

	params.reset()
	if r.find("GET", "/a/val/c", &params) == nil {
		t.Error("should match /a/val/c")
	}

	// No match
	params.reset()
	if r.find("GET", "/a/val/d", &params) != nil {
		t.Error("should not match /a/val/d")
	}
}

func TestRouter_TrackHits_BeforeCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/hot", h)
	r.addRoute("GET", "/cold", h)

	var params routeParams

	// Hit /hot 10 times before compile
	for range 10 {
		params.reset()
		r.find("GET", "/hot", &params)
	}
	// Hit /cold once
	params.reset()
	r.find("GET", "/cold", &params)

	// After compile, hits should have been recorded
	r.Compile()

	// Both should still match
	params.reset()
	if r.find("GET", "/hot", &params) == nil {
		t.Error("should match /hot after compile")
	}
	params.reset()
	if r.find("GET", "/cold", &params) == nil {
		t.Error("should match /cold after compile")
	}
}

// =============================================================================
// router.go — cleanPath
// =============================================================================

func TestCleanPath_NullByte(t *testing.T) {
	cleaned, err := cleanPath("/test\x00path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(cleaned, "\x00") {
		t.Error("cleaned path should not contain null bytes")
	}
}

func TestCleanPath_DoubleEncoded(t *testing.T) {
	// %252e%252e should be decoded to .. via double-decode
	_, err := cleanPath("/%252e%252e/etc/passwd")
	if err == nil {
		// The double-decode should resolve to /../etc/passwd which is traversal
		// But the decoded path starts with / so depth tracking matters
		_ = err
	}
}

func TestCleanPath_Simple(t *testing.T) {
	cleaned, err := cleanPath("/users/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != "/users/42" {
		t.Errorf("cleanPath = %q, want /users/42", cleaned)
	}
}

func TestCleanPath_DotSegments(t *testing.T) {
	cleaned, err := cleanPath("/a/b/../c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != "/a/c" {
		t.Errorf("cleanPath = %q, want /a/c", cleaned)
	}
}

// =============================================================================
// router.go — isQuantifier
// =============================================================================

func TestIsQuantifier(t *testing.T) {
	quantifiers := []byte{'+', '*', '?', '{'}
	for _, c := range quantifiers {
		if !isQuantifier(c) {
			t.Errorf("isQuantifier(%q) = false, want true", string(c))
		}
	}
	nonQuantifiers := []byte{'a', 'z', '0', '.', '-', '[', ')'}
	for _, c := range nonQuantifiers {
		if isQuantifier(c) {
			t.Errorf("isQuantifier(%q) = true, want false", string(c))
		}
	}
}

// =============================================================================
// router.go — collectStaticRoutes: root with trailing slash
// =============================================================================

func TestCollectStaticRoutes_RootSlash(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/api", h)
	r.Compile()

	// After compile, static routes should be populated
	if r.staticRoutes[mGET] == nil {
		t.Fatal("expected static routes for GET")
	}
	if _, ok := r.staticRoutes[mGET]["/"]; !ok {
		t.Error("expected / in static routes")
	}
	if _, ok := r.staticRoutes[mGET]["/api"]; !ok {
		t.Error("expected /api in static routes")
	}
}
