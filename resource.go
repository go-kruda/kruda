package kruda

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ResourceService is the generic interface for auto-wired CRUD endpoints.
// Implement this interface to get 5 REST endpoints (List, Create, Get, Update, Delete)
// automatically registered via the Resource() or GroupResource() functions.
type ResourceService[T any, ID comparable] interface {
	List(ctx context.Context, page int, limit int) ([]T, int, error)
	Create(ctx context.Context, item T) (T, error)
	Get(ctx context.Context, id ID) (T, error)
	Update(ctx context.Context, id ID, item T) (T, error)
	Delete(ctx context.Context, id ID) error
}

// ResourceOption configures auto-wired CRUD endpoints.
type ResourceOption func(*resourceConfig)

type resourceConfig struct {
	middleware []HandlerFunc
	only       map[string]bool
	except     map[string]bool
	idParam    string
}

func defaultResourceConfig() resourceConfig {
	return resourceConfig{idParam: "id"}
}

// WithResourceMiddleware adds middleware that applies only to the auto-wired CRUD endpoints.
func WithResourceMiddleware(mw ...HandlerFunc) ResourceOption {
	return func(cfg *resourceConfig) { cfg.middleware = append(cfg.middleware, mw...) }
}

// WithResourceOnly restricts which CRUD methods are registered.
// Pass HTTP method names like "GET", "POST", "PUT", "DELETE".
// Note: "GET" registers both list (GET /path) and get-by-id (GET /path/:id).
// Use "LIST" for list-only or "GET_BY_ID" for get-by-id-only.
func WithResourceOnly(methods ...string) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.only = make(map[string]bool)
		for _, m := range methods {
			cfg.only[strings.ToUpper(m)] = true
		}
	}
}

// WithResourceExcept excludes specific CRUD methods from registration.
// Pass HTTP method names like "GET", "POST", "PUT", "DELETE".
func WithResourceExcept(methods ...string) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.except = make(map[string]bool)
		for _, m := range methods {
			cfg.except[strings.ToUpper(m)] = true
		}
	}
}

// WithResourceIDParam overrides the default "id" path parameter name.
func WithResourceIDParam(param string) ResourceOption {
	return func(cfg *resourceConfig) { cfg.idParam = param }
}

// routeRegistrar abstracts route registration for both App and Group.
type routeRegistrar interface {
	Get(path string, handler HandlerFunc) routeRegistrar
	Post(path string, handler HandlerFunc) routeRegistrar
	Put(path string, handler HandlerFunc) routeRegistrar
	Delete(path string, handler HandlerFunc) routeRegistrar
}

// appRegistrar wraps *App to implement routeRegistrar.
type appRegistrar struct{ app *App }

func (a appRegistrar) Get(p string, h HandlerFunc) routeRegistrar    { a.app.Get(p, h); return a }
func (a appRegistrar) Post(p string, h HandlerFunc) routeRegistrar   { a.app.Post(p, h); return a }
func (a appRegistrar) Put(p string, h HandlerFunc) routeRegistrar    { a.app.Put(p, h); return a }
func (a appRegistrar) Delete(p string, h HandlerFunc) routeRegistrar { a.app.Delete(p, h); return a }

// groupRegistrar wraps *Group to implement routeRegistrar.
type groupRegistrar struct{ group *Group }

func (g groupRegistrar) Get(p string, h HandlerFunc) routeRegistrar  { g.group.Get(p, h); return g }
func (g groupRegistrar) Post(p string, h HandlerFunc) routeRegistrar { g.group.Post(p, h); return g }
func (g groupRegistrar) Put(p string, h HandlerFunc) routeRegistrar  { g.group.Put(p, h); return g }
func (g groupRegistrar) Delete(p string, h HandlerFunc) routeRegistrar {
	g.group.Delete(p, h)
	return g
}

// Resource auto-wires 5 REST endpoints from a ResourceService on the App.
// It registers: GET (list), GET/:id (get), POST (create), PUT/:id (update), DELETE/:id (delete).
func Resource[T any, ID comparable](app *App, path string, svc ResourceService[T, ID], opts ...ResourceOption) *App {
	registerResource(appRegistrar{app}, path, svc, opts...)
	return app
}

// GroupResource auto-wires 5 REST endpoints from a ResourceService on a Group.
// It registers: GET (list), GET/:id (get), POST (create), PUT/:id (update), DELETE/:id (delete).
func GroupResource[T any, ID comparable](g *Group, path string, svc ResourceService[T, ID], opts ...ResourceOption) *Group {
	registerResource(groupRegistrar{g}, path, svc, opts...)
	return g
}

func registerResource[T any, ID comparable](r routeRegistrar, path string, svc ResourceService[T, ID], opts ...ResourceOption) {
	cfg := defaultResourceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	idPath := path + "/:" + cfg.idParam

	if resourceShouldRegister(cfg, "GET") {
		r.Get(path, resourceWrapMW(cfg.middleware, resourceListHandler(svc)))
		r.Get(idPath, resourceWrapMW(cfg.middleware, resourceGetHandler[T, ID](svc, cfg)))
	} else {
		if resourceShouldRegister(cfg, "LIST") {
			r.Get(path, resourceWrapMW(cfg.middleware, resourceListHandler(svc)))
		}
		if resourceShouldRegister(cfg, "GET_BY_ID") {
			r.Get(idPath, resourceWrapMW(cfg.middleware, resourceGetHandler[T, ID](svc, cfg)))
		}
	}
	if resourceShouldRegister(cfg, "POST") {
		r.Post(path, resourceWrapMW(cfg.middleware, resourceCreateHandler[T, ID](svc)))
	}
	if resourceShouldRegister(cfg, "PUT") {
		r.Put(idPath, resourceWrapMW(cfg.middleware, resourceUpdateHandler[T, ID](svc, cfg)))
	}
	if resourceShouldRegister(cfg, "DELETE") {
		r.Delete(idPath, resourceWrapMW(cfg.middleware, resourceDeleteHandler[T, ID](svc, cfg)))
	}
}

func resourceListHandler[T any, ID comparable](svc ResourceService[T, ID]) HandlerFunc {
	return func(c *Ctx) error {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 20)
		items, total, err := svc.List(c.Context(), page, limit)
		if err != nil {
			return err
		}
		return c.JSON(Map{"data": items, "total": total, "page": page, "limit": limit})
	}
}

func resourceGetHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig) HandlerFunc {
	return func(c *Ctx) error {
		id, err := resourceParseID[ID](c.Param(cfg.idParam))
		if err != nil {
			return BadRequest("invalid id")
		}
		item, err := svc.Get(c.Context(), id)
		if err != nil {
			return err
		}
		return c.JSON(item)
	}
}

func resourceCreateHandler[T any, ID comparable](svc ResourceService[T, ID]) HandlerFunc {
	return func(c *Ctx) error {
		var item T
		if err := c.Bind(&item); err != nil {
			return err
		}
		created, err := svc.Create(c.Context(), item)
		if err != nil {
			return err
		}
		return c.Status(201).JSON(created)
	}
}

func resourceUpdateHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig) HandlerFunc {
	return func(c *Ctx) error {
		id, err := resourceParseID[ID](c.Param(cfg.idParam))
		if err != nil {
			return BadRequest("invalid id")
		}
		var item T
		if err := c.Bind(&item); err != nil {
			return err
		}
		updated, err := svc.Update(c.Context(), id, item)
		if err != nil {
			return err
		}
		return c.JSON(updated)
	}
}

func resourceDeleteHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig) HandlerFunc {
	return func(c *Ctx) error {
		id, err := resourceParseID[ID](c.Param(cfg.idParam))
		if err != nil {
			return BadRequest("invalid id")
		}
		if err := svc.Delete(c.Context(), id); err != nil {
			return err
		}
		return c.NoContent()
	}
}

func resourceParseID[ID comparable](raw string) (ID, error) {
	var zero ID
	switch any(zero).(type) {
	case string:
		return any(raw).(ID), nil
	case int:
		v, err := strconv.Atoi(raw)
		return any(v).(ID), err
	case int64:
		v, err := strconv.ParseInt(raw, 10, 64)
		return any(v).(ID), err
	case uint:
		v, err := strconv.ParseUint(raw, 10, 64)
		return any(uint(v)).(ID), err
	case uint64:
		v, err := strconv.ParseUint(raw, 10, 64)
		return any(v).(ID), err
	default:
		return zero, fmt.Errorf("unsupported ID type: %T", zero)
	}
}

func resourceShouldRegister(cfg resourceConfig, method string) bool {
	if cfg.only != nil {
		return cfg.only[method]
	}
	if cfg.except != nil {
		return !cfg.except[method]
	}
	return true
}

// resourceWrapMW wraps a handler with resource-scoped middleware by temporarily
// replacing c.handlers/c.routeIndex. The deferred restore handles cleanup even
// if a middleware panics.
func resourceWrapMW(mw []HandlerFunc, handler HandlerFunc) HandlerFunc {
	if len(mw) == 0 {
		return handler
	}
	chain := make([]HandlerFunc, len(mw)+1)
	copy(chain, mw)
	chain[len(mw)] = handler
	return func(c *Ctx) error {
		origHandlers := c.handlers
		origIndex := c.routeIndex
		defer func() {
			c.handlers = origHandlers
			c.routeIndex = origIndex
		}()
		c.handlers = chain
		c.routeIndex = -1
		return c.Next()
	}
}
