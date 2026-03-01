package kruda

import "strings"

// Group represents a collection of routes that share a common prefix
// and scoped middleware. Groups can be nested to create hierarchical
// route structures with inherited middleware chains.
type Group struct {
	prefix     string
	app        *App
	middleware []HandlerFunc
	parent     *Group // nil for top-level groups
}

// Group creates a new top-level route group with the given prefix.
// The group shares the App's global middleware and can add its own
// scoped middleware via Use() or Guard().
//
//	api := app.Group("/api")
//	api.Get("/users", listUsers)
func (app *App) Group(prefix string) *Group {
	return &Group{
		prefix: prefix,
		app:    app,
	}
}

// Group creates a nested group with the combined prefix of parent and child.
//
//	v1 := api.Group("/v1")
//	v1.Get("/users", listUsers) // matches /api/v1/users
func (g *Group) Group(prefix string) *Group {
	return &Group{
		prefix: joinPath(g.prefix, prefix),
		app:    g.app,
		parent: g,
	}
}

// Use adds scoped middleware to this group. The middleware applies only
// to routes registered within this group and its nested groups.
// Returns the group for method chaining.
func (g *Group) Use(middleware ...HandlerFunc) *Group {
	g.middleware = append(g.middleware, middleware...)
	return g
}

// Guard is a semantic alias for Use. It reads as "protect these routes with"
// and is intended for auth/permission middleware.
//
//	admin := api.Group("/admin").Guard(authMiddleware)
func (g *Group) Guard(middleware ...HandlerFunc) *Group {
	return g.Use(middleware...)
}

// Get registers a GET route on this group.
func (g *Group) Get(path string, handler HandlerFunc) *Group {
	g.addRoute("GET", path, handler)
	return g
}

// Post registers a POST route on this group.
func (g *Group) Post(path string, handler HandlerFunc) *Group {
	g.addRoute("POST", path, handler)
	return g
}

// Put registers a PUT route on this group.
func (g *Group) Put(path string, handler HandlerFunc) *Group {
	g.addRoute("PUT", path, handler)
	return g
}

// Delete registers a DELETE route on this group.
func (g *Group) Delete(path string, handler HandlerFunc) *Group {
	g.addRoute("DELETE", path, handler)
	return g
}

// Patch registers a PATCH route on this group.
func (g *Group) Patch(path string, handler HandlerFunc) *Group {
	g.addRoute("PATCH", path, handler)
	return g
}

// Options registers an OPTIONS route on this group.
func (g *Group) Options(path string, handler HandlerFunc) *Group {
	g.addRoute("OPTIONS", path, handler)
	return g
}

// Head registers a HEAD route on this group.
func (g *Group) Head(path string, handler HandlerFunc) *Group {
	g.addRoute("HEAD", path, handler)
	return g
}

// All registers a route on all standard HTTP methods on this group.
func (g *Group) All(path string, handler HandlerFunc) *Group {
	for _, method := range standardMethods {
		g.addRoute(method, path, handler)
	}
	return g
}

// Done returns the parent App for method chaining back to the root.
//
//	app.Group("/api").
//	    Use(authMiddleware).
//	    Get("/users", listUsers).
//	    Done().
//	    Get("/health", healthCheck)
func (g *Group) Done() *App {
	return g.app
}

// addRoute builds the full path and pre-built handler chain, then registers it.
func (g *Group) addRoute(method, path string, handler HandlerFunc) {
	fullPath := joinPath(g.prefix, path)
	chain := buildChain(g.app.middleware, g.collectMiddleware(), handler)
	g.app.router.addRoute(method, fullPath, chain)
}

// collectMiddleware gathers middleware from the entire parent chain,
// outermost group first, innermost last.
func (g *Group) collectMiddleware() []HandlerFunc {
	if g.parent == nil {
		return g.middleware
	}
	parentMW := g.parent.collectMiddleware()
	if len(parentMW) == 0 {
		return g.middleware
	}
	if len(g.middleware) == 0 {
		return parentMW
	}
	mw := make([]HandlerFunc, 0, len(parentMW)+len(g.middleware))
	mw = append(mw, parentMW...)
	mw = append(mw, g.middleware...)
	return mw
}

// joinPath combines a prefix and path, handling leading and trailing slashes
// to avoid double slashes or missing slashes.
func joinPath(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
}
