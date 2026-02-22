package kruda

import (
	"testing"
)

// dummyHandler returns a handler that does nothing (for testing route registration).
func dummyHandler() HandlerFunc {
	return func(c *Ctx) error { return nil }
}

func TestRouterStaticRoutes(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/users", h)
	r.addRoute("GET", "/users/settings", h)
	r.addRoute("POST", "/users", h)

	params := make(map[string]string, 4)

	// Test root
	if got := r.find("GET", "/", params); got == nil {
		t.Error("GET / should match")
	}

	// Test static paths
	if got := r.find("GET", "/users", params); got == nil {
		t.Error("GET /users should match")
	}
	if got := r.find("GET", "/users/settings", params); got == nil {
		t.Error("GET /users/settings should match")
	}
	if got := r.find("POST", "/users", params); got == nil {
		t.Error("POST /users should match")
	}

	// Test no match
	if got := r.find("GET", "/notfound", params); got != nil {
		t.Error("GET /notfound should not match")
	}
	if got := r.find("DELETE", "/users", params); got != nil {
		t.Error("DELETE /users should not match (not registered)")
	}
}

func TestRouterParamRoutes(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/:id", h)
	r.addRoute("GET", "/users/:id/posts/:postId", h)

	params := make(map[string]string, 4)

	// Single param
	clear(params)
	if got := r.find("GET", "/users/123", params); got == nil {
		t.Error("GET /users/123 should match")
	} else if params["id"] != "123" {
		t.Errorf("params[id] = %q, want %q", params["id"], "123")
	}

	// Multi param
	clear(params)
	if got := r.find("GET", "/users/1/posts/2", params); got == nil {
		t.Error("GET /users/1/posts/2 should match")
	} else {
		if params["id"] != "1" {
			t.Errorf("params[id] = %q, want %q", params["id"], "1")
		}
		if params["postId"] != "2" {
			t.Errorf("params[postId] = %q, want %q", params["postId"], "2")
		}
	}
}

func TestRouterRegexParam(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/:id<[0-9]+>", h)

	params := make(map[string]string, 4)

	// Matching regex
	clear(params)
	if got := r.find("GET", "/users/123", params); got == nil {
		t.Error("GET /users/123 should match regex [0-9]+")
	}

	// Non-matching regex
	clear(params)
	if got := r.find("GET", "/users/abc", params); got != nil {
		t.Error("GET /users/abc should NOT match regex [0-9]+")
	}
}

func TestRouterOptionalParam(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/:id?", h)

	params := make(map[string]string, 4)

	// With param
	clear(params)
	if got := r.find("GET", "/users/123", params); got == nil {
		t.Error("GET /users/123 should match optional param")
	} else if params["id"] != "123" {
		t.Errorf("params[id] = %q, want %q", params["id"], "123")
	}

	// Without param
	clear(params)
	if got := r.find("GET", "/users", params); got == nil {
		t.Error("GET /users should match optional param (without value)")
	}
}

func TestRouterWildcard(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/files/*filepath", h)

	params := make(map[string]string, 4)

	clear(params)
	if got := r.find("GET", "/files/css/style.css", params); got == nil {
		t.Error("GET /files/css/style.css should match wildcard")
	} else if params["filepath"] != "css/style.css" {
		t.Errorf("params[filepath] = %q, want %q", params["filepath"], "css/style.css")
	}

	clear(params)
	if got := r.find("GET", "/files/readme.txt", params); got == nil {
		t.Error("GET /files/readme.txt should match wildcard")
	} else if params["filepath"] != "readme.txt" {
		t.Errorf("params[filepath] = %q, want %q", params["filepath"], "readme.txt")
	}
}

func TestRouterFindAllowedMethods(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users", h)
	r.addRoute("POST", "/users", h)

	allowed := r.findAllowedMethods("/users")
	if allowed == "" {
		t.Error("findAllowedMethods should return non-empty for /users")
	}
	// Should contain GET and POST
	if !containsMethod(allowed, "GET") || !containsMethod(allowed, "POST") {
		t.Errorf("allowed = %q, want to contain GET and POST", allowed)
	}

	// No match
	allowed = r.findAllowedMethods("/notfound")
	if allowed != "" {
		t.Errorf("findAllowedMethods for /notfound = %q, want empty", allowed)
	}
}

func containsMethod(allowed, method string) bool {
	for _, m := range splitMethods(allowed) {
		if m == method {
			return true
		}
	}
	return false
}

func splitMethods(s string) []string {
	parts := make([]string, 0)
	for _, p := range splitByComma(s) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitByComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func TestRouterCompileFreeze(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/test", h)
	r.Compile()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic after Compile()")
		}
	}()
	r.addRoute("GET", "/another", h)
}

func TestRouterDuplicatePanic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate route")
		}
	}()
	r.addRoute("GET", "/users", h)
}

func TestRouterStaticRadixCompression(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// These share the "us" prefix
	r.addRoute("GET", "/users", h)
	r.addRoute("GET", "/users/settings", h)
	r.addRoute("GET", "/users/profile", h)

	params := make(map[string]string, 4)

	if r.find("GET", "/users", params) == nil {
		t.Error("GET /users should match")
	}
	if r.find("GET", "/users/settings", params) == nil {
		t.Error("GET /users/settings should match")
	}
	if r.find("GET", "/users/profile", params) == nil {
		t.Error("GET /users/profile should match")
	}
	if r.find("GET", "/users/unknown", params) != nil {
		t.Error("GET /users/unknown should not match")
	}
}

func TestRouterMixedStaticAndParam(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/settings", h)
	r.addRoute("GET", "/users/:id", h)

	params := make(map[string]string, 4)

	// Static should take priority (checked first via indices)
	clear(params)
	if r.find("GET", "/users/settings", params) == nil {
		t.Error("GET /users/settings should match static route")
	}

	// Param should match other values
	clear(params)
	if got := r.find("GET", "/users/42", params); got == nil {
		t.Error("GET /users/42 should match param route")
	} else if params["id"] != "42" {
		t.Errorf("params[id] = %q, want %q", params["id"], "42")
	}
}

func TestRouterWildcardDeepPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/static/*filepath", h)

	params := make(map[string]string, 4)

	clear(params)
	if got := r.find("GET", "/static/a/b/c/d.js", params); got == nil {
		t.Error("GET /static/a/b/c/d.js should match wildcard")
	} else if params["filepath"] != "a/b/c/d.js" {
		t.Errorf("params[filepath] = %q, want %q", params["filepath"], "a/b/c/d.js")
	}
}

func TestRouter405Detection(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/api/data", h)
	r.addRoute("POST", "/api/data", h)

	params := make(map[string]string, 4)

	// DELETE /api/data should give 405 (GET and POST exist)
	if r.find("DELETE", "/api/data", params) != nil {
		t.Error("DELETE /api/data should not match")
	}
	allowed := r.findAllowedMethods("/api/data")
	if allowed == "" {
		t.Error("findAllowedMethods should return non-empty for /api/data")
	}
	if !containsMethod(allowed, "GET") || !containsMethod(allowed, "POST") {
		t.Errorf("allowed = %q, want GET and POST", allowed)
	}

	// /totally/unknown should give 404
	allowed = r.findAllowedMethods("/totally/unknown")
	if allowed != "" {
		t.Errorf("findAllowedMethods for /totally/unknown = %q, want empty", allowed)
	}
}

func TestRouterRootRoute(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)

	params := make(map[string]string, 4)
	if r.find("GET", "/", params) == nil {
		t.Error("GET / should match")
	}
}
