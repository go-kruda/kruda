package kruda

import (
	"testing"
)

// testApp creates a minimal App with a router for group testing.
func testApp() *App {
	return &App{
		router: newRouter(),
	}
}

// --- joinPath tests (Req 4.7) ---

func TestJoinPath_BothNonEmpty(t *testing.T) {
	tests := []struct {
		prefix, path, want string
	}{
		{"/api", "/users", "/api/users"},
		{"/api/", "/users", "/api/users"},
		{"/api", "users", "/api/users"},
		{"/api/", "users", "/api/users"},
		{"/api//", "//users", "/api/users"},
	}
	for _, tt := range tests {
		got := joinPath(tt.prefix, tt.path)
		if got != tt.want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", tt.prefix, tt.path, got, tt.want)
		}
	}
}

func TestJoinPath_EmptyPrefix(t *testing.T) {
	if got := joinPath("", "/users"); got != "/users" {
		t.Errorf("joinPath(\"\", \"/users\") = %q, want %q", got, "/users")
	}
}

func TestJoinPath_EmptyPath(t *testing.T) {
	if got := joinPath("/api", ""); got != "/api" {
		t.Errorf("joinPath(\"/api\", \"\") = %q, want %q", got, "/api")
	}
}

func TestJoinPath_SlashPath(t *testing.T) {
	if got := joinPath("/api", "/"); got != "/api" {
		t.Errorf("joinPath(\"/api\", \"/\") = %q, want %q", got, "/api")
	}
}

// --- Group creation (Req 4.1, 4.5) ---

func TestGroup_TopLevel(t *testing.T) {
	app := testApp()
	g := app.Group("/api")

	if g.prefix != "/api" {
		t.Errorf("prefix = %q, want %q", g.prefix, "/api")
	}
	if g.app != app {
		t.Error("group should reference parent app")
	}
	if g.parent != nil {
		t.Error("top-level group should have nil parent")
	}
}

func TestGroup_Nested(t *testing.T) {
	app := testApp()
	api := app.Group("/api")
	v1 := api.Group("/v1")

	if v1.prefix != "/api/v1" {
		t.Errorf("nested prefix = %q, want %q", v1.prefix, "/api/v1")
	}
	if v1.parent != api {
		t.Error("nested group should reference parent group")
	}
	if v1.app != app {
		t.Error("nested group should reference root app")
	}
}

func TestGroup_DeeplyNested(t *testing.T) {
	app := testApp()
	g := app.Group("/a").Group("/b").Group("/c")

	if g.prefix != "/a/b/c" {
		t.Errorf("deeply nested prefix = %q, want %q", g.prefix, "/a/b/c")
	}
}

// --- Route registration via Group (Req 4.2) ---

func TestGroup_GetRegistersRoute(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	g.Get("/users", dummyHandler())

	params := make(map[string]string, 4)
	if app.router.find("GET", "/api/users", params) == nil {
		t.Error("GET /api/users should be registered")
	}
}

func TestGroup_AllHTTPMethods(t *testing.T) {
	app := testApp()
	g := app.Group("/api")

	h := dummyHandler()
	g.Get("/r", h)
	g.Post("/r", h)
	g.Put("/r", h)
	g.Delete("/r", h)
	g.Patch("/r", h)

	params := make(map[string]string, 4)
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for _, m := range methods {
		clear(params)
		if app.router.find(m, "/api/r", params) == nil {
			t.Errorf("%s /api/r should be registered", m)
		}
	}
}

func TestGroup_NestedRouteRegistration(t *testing.T) {
	app := testApp()
	v1 := app.Group("/api").Group("/v1")
	v1.Get("/users", dummyHandler())

	params := make(map[string]string, 4)
	if app.router.find("GET", "/api/v1/users", params) == nil {
		t.Error("GET /api/v1/users should be registered via nested group")
	}
}

// --- Scoped middleware (Req 4.3) ---

func TestGroup_ScopedMiddleware(t *testing.T) {
	app := testApp()

	var order []string
	mw := func(c *Ctx) error { order = append(order, "mw"); return c.Next() }
	handler := func(c *Ctx) error { order = append(order, "handler"); return nil }

	g := app.Group("/api")
	g.Use(mw)
	g.Get("/test", handler)

	params := make(map[string]string, 4)
	chain := app.router.find("GET", "/api/test", params)
	if chain == nil {
		t.Fatal("GET /api/test should be registered")
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2 (mw + handler)", len(chain))
	}

	// Execute chain and verify order
	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"mw", "handler"})
}

func TestGroup_MiddlewareNotAppliedToOtherGroups(t *testing.T) {
	app := testApp()

	mw := func(c *Ctx) error { return c.Next() }
	h := dummyHandler()

	api := app.Group("/api")
	api.Use(mw)
	api.Get("/guarded", h)

	// Different group — should NOT have the middleware
	other := app.Group("/other")
	other.Get("/open", h)

	params := make(map[string]string, 4)
	guardedChain := app.router.find("GET", "/api/guarded", params)
	openChain := app.router.find("GET", "/other/open", params)

	if len(guardedChain) != 2 {
		t.Errorf("/api/guarded chain length = %d, want 2", len(guardedChain))
	}
	if len(openChain) != 1 {
		t.Errorf("/other/open chain length = %d, want 1 (no group mw)", len(openChain))
	}
}

// --- Guard alias (Req 4.4) ---

func TestGroup_GuardIsAliasForUse(t *testing.T) {
	app := testApp()

	var order []string
	auth := func(c *Ctx) error { order = append(order, "auth"); return c.Next() }
	handler := func(c *Ctx) error { order = append(order, "handler"); return nil }

	g := app.Group("/admin").Guard(auth)
	g.Get("/dashboard", handler)

	params := make(map[string]string, 4)
	chain := app.router.find("GET", "/admin/dashboard", params)
	if chain == nil {
		t.Fatal("GET /admin/dashboard should be registered")
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2", len(chain))
	}

	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"auth", "handler"})
}

// --- Nested group middleware inheritance (Req 4.3, 4.5) ---

func TestGroup_NestedMiddlewareChain(t *testing.T) {
	app := testApp()

	var order []string
	outerMW := func(c *Ctx) error { order = append(order, "outer"); return c.Next() }
	innerMW := func(c *Ctx) error { order = append(order, "inner"); return c.Next() }
	handler := func(c *Ctx) error { order = append(order, "handler"); return nil }

	outer := app.Group("/api")
	outer.Use(outerMW)

	inner := outer.Group("/v1")
	inner.Use(innerMW)
	inner.Get("/users", handler)

	params := make(map[string]string, 4)
	chain := app.router.find("GET", "/api/v1/users", params)
	if chain == nil {
		t.Fatal("GET /api/v1/users should be registered")
	}
	if len(chain) != 3 {
		t.Fatalf("chain length = %d, want 3 (outer + inner + handler)", len(chain))
	}

	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"outer", "inner", "handler"})
}

// --- Global + group middleware ordering ---

func TestGroup_GlobalPlusGroupMiddleware(t *testing.T) {
	app := testApp()
	app.middleware = []HandlerFunc{
		func(c *Ctx) error { return c.Next() }, // global mw placeholder
	}

	groupMW := func(c *Ctx) error { return c.Next() }
	h := dummyHandler()

	g := app.Group("/api")
	g.Use(groupMW)
	g.Get("/test", h)

	params := make(map[string]string, 4)
	chain := app.router.find("GET", "/api/test", params)
	if chain == nil {
		t.Fatal("GET /api/test should be registered")
	}
	// global(1) + group(1) + handler(1) = 3
	if len(chain) != 3 {
		t.Fatalf("chain length = %d, want 3 (global + group + handler)", len(chain))
	}
}

// --- Done() chaining (Req 4.6) ---

func TestGroup_DoneReturnsApp(t *testing.T) {
	app := testApp()
	g := app.Group("/api")

	if g.Done() != app {
		t.Error("Done() should return the parent App")
	}
}

func TestGroup_DoneChaining(t *testing.T) {
	app := testApp()

	// This pattern should compile and register routes correctly
	app.Group("/api").
		Get("/users", dummyHandler()).
		Post("/users", dummyHandler()).
		Done()

	params := make(map[string]string, 4)
	if app.router.find("GET", "/api/users", params) == nil {
		t.Error("GET /api/users should be registered via chaining")
	}
	if app.router.find("POST", "/api/users", params) == nil {
		t.Error("POST /api/users should be registered via chaining")
	}
}

// --- Method chaining on Group ---

func TestGroup_UseReturnsGroup(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	ret := g.Use(dummyHandler())
	if ret != g {
		t.Error("Use() should return the same group for chaining")
	}
}

func TestGroup_RouteMethodsReturnGroup(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	h := dummyHandler()

	if g.Get("/a", h) != g {
		t.Error("Get() should return group")
	}
	if g.Post("/b", h) != g {
		t.Error("Post() should return group")
	}
	if g.Put("/c", h) != g {
		t.Error("Put() should return group")
	}
	if g.Delete("/d", h) != g {
		t.Error("Delete() should return group")
	}
	if g.Patch("/e", h) != g {
		t.Error("Patch() should return group")
	}
}

// --- collectMiddleware ---

func TestCollectMiddleware_NoParent(t *testing.T) {
	app := testApp()
	mw := dummyHandler()
	g := app.Group("/api")
	g.Use(mw)

	collected := g.collectMiddleware()
	if len(collected) != 1 {
		t.Fatalf("collected length = %d, want 1", len(collected))
	}
}

func TestCollectMiddleware_WithParent(t *testing.T) {
	app := testApp()
	outer := app.Group("/api")
	outer.Use(dummyHandler())
	outer.Use(dummyHandler())

	inner := outer.Group("/v1")
	inner.Use(dummyHandler())

	collected := inner.collectMiddleware()
	if len(collected) != 3 {
		t.Fatalf("collected length = %d, want 3 (2 outer + 1 inner)", len(collected))
	}
}

func TestCollectMiddleware_EmptyGroups(t *testing.T) {
	app := testApp()
	outer := app.Group("/api")
	inner := outer.Group("/v1")

	collected := inner.collectMiddleware()
	if len(collected) != 0 {
		t.Fatalf("collected length = %d, want 0", len(collected))
	}
}

// --- Options, Head, All methods ---

func TestGroup_Options(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	g.Options("/cors", dummyHandler())

	params := make(map[string]string, 4)
	if app.router.find("OPTIONS", "/api/cors", params) == nil {
		t.Error("OPTIONS /api/cors should be registered")
	}
}

func TestGroup_Head(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	g.Head("/ping", dummyHandler())

	params := make(map[string]string, 4)
	if app.router.find("HEAD", "/api/ping", params) == nil {
		t.Error("HEAD /api/ping should be registered")
	}
}

func TestGroup_All(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	g.All("/any", dummyHandler())

	params := make(map[string]string, 4)
	for _, method := range standardMethods {
		clear(params)
		if app.router.find(method, "/api/any", params) == nil {
			t.Errorf("%s /api/any should be registered via All()", method)
		}
	}
}

func TestGroup_OptionsHeadAllReturnGroup(t *testing.T) {
	app := testApp()
	g := app.Group("/api")
	h := dummyHandler()

	if g.Options("/a", h) != g {
		t.Error("Options() should return group")
	}
	if g.Head("/b", h) != g {
		t.Error("Head() should return group")
	}
	if g.All("/c", h) != g {
		t.Error("All() should return group")
	}
}
