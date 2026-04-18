package kruda

import (
	"testing"
)

// FuzzRouterPattern checks that the router never panics in a way that escapes
// the test, regardless of the pattern provided to Get/Compile.
//
// The router is documented to panic on malformed patterns at registration
// time (e.g. routes that don't begin with "/", invalid regex constraints,
// duplicate routes). Those panics are caught by an inner deferred recover —
// the outer recover only fires if the panic happens *outside* of register
// (e.g. during Compile), which would be a real bug.
func FuzzRouterPattern(f *testing.F) {
	seeds := []string{
		"/",
		"/users",
		"/users/:id",
		"/files/*path",
		"/api/:version/users/:id<[0-9]+>",
		"/optional/:id?",
		"//",
		"/a/b/c",
		"/x/:y/z/:w?",
		"/static/*",
		"",
		"no-leading-slash",
		"/dup",
		"/with spaces",
		"/unicode/\xe4\xb8\xad",
		"/escape/\\:id",
		"/regex/:id<>",
		"/regex/:id<(a+)+>",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("router unexpectedly panicked outside register on pattern %q: %v", pattern, r)
			}
		}()

		app := New()

		// Wrap registration in inner func — addRoute is documented to panic on
		// malformed patterns. Those are expected, not bugs.
		func() {
			defer func() { _ = recover() }()
			app.Get(pattern, func(c *Ctx) error { return nil })
		}()

		// Compile must never panic, even on a tree that may have been left in
		// an unusual state by a failed register.
		app.Compile()
	})
}

// FuzzRouterMatch checks that route lookup never panics for arbitrary request
// paths against a fixed routing table containing static, param, and wildcard
// routes plus an optional-param route.
//
// The router's find() has an implicit precondition that path is non-empty
// (production callers in app_serve.go and serve_fast.go normalize "" → "/"
// before invoking find). The fuzz body mirrors that normalization so we
// exercise the same input shape the router actually sees in production.
func FuzzRouterMatch(f *testing.F) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error { return nil })
	app.Get("/files/*path", func(c *Ctx) error { return nil })
	app.Get("/static", func(c *Ctx) error { return nil })
	app.Get("/api/:version/items/:id<[0-9]+>", func(c *Ctx) error { return nil })
	app.Get("/optional/:id?", func(c *Ctx) error { return nil })
	app.Compile()

	seeds := []string{
		"/",
		"/users/123",
		"/files/a/b/c",
		"//",
		"/users/",
		"/static",
		"/static/extra",
		"/users/abc?def",
		"/api/v1/items/42",
		"/api/v1/items/notnum",
		"/optional",
		"/optional/x",
		"/\x00\x01\x02",
		"/" + string(make([]byte, 1024)),
	}
	for _, p := range seeds {
		f.Add(p)
	}

	f.Fuzz(func(t *testing.T, path string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("router lookup panicked on %q: %v", path, r)
			}
		}()

		// Mirror production normalization: empty path → "/" (see app_serve.go:74,
		// serve_fast.go:98). The router's find() requires a non-empty path.
		if path == "" {
			path = "/"
		}

		params := &routeParams{}
		_ = app.router.find("GET", path, params)

		// Also exercise findAllowedMethods which traverses every method tree.
		_ = app.router.findAllowedMethods(path)
	})
}
