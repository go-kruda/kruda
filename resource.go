package kruda

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// maxResourceListLimit caps the per-page item count an auto-CRUD list endpoint
// will request. A larger ?limit is clamped to this value (not rejected).
const maxResourceListLimit = 100

// ResourceList is the response envelope for an auto-CRUD list endpoint. It is
// used as the OpenAPI response type for list operations and marshals to the
// same JSON the list handler has always produced ({data, total, page, limit}).
type ResourceList[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

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
//
// Create and update validate the decoded body using the same machinery and
// error contract as typed C[T] handlers: validation runs only when a *Validator
// is configured (via WithValidator) and T carries validate tags, emitting a 422
// on failure. Malformed JSON yields a 400 ("invalid request body").
//
// Service errors propagate unchanged: a ResourceService returning a *KrudaError
// (e.g. NotFound) gets that status; a plain error becomes the generic 500. The
// framework never synthesizes a 404, and OpenAPI advertises only the success
// code, a 422 when validation is engaged, and the default error response.
func Resource[T any, ID comparable](app *App, path string, svc ResourceService[T, ID], opts ...ResourceOption) *App {
	registerResource(appRegistrar{app}, app, "", path, svc, opts...)
	return app
}

// GroupResource auto-wires 5 REST endpoints from a ResourceService on a Group.
// It registers: GET (list), GET/:id (get), POST (create), PUT/:id (update), DELETE/:id (delete).
// Behavior matches Resource (validation, error propagation, OpenAPI); see Resource.
func GroupResource[T any, ID comparable](g *Group, path string, svc ResourceService[T, ID], opts ...ResourceOption) *Group {
	registerResource(groupRegistrar{g}, g.app, g.prefix, path, svc, opts...)
	return g
}

func registerResource[T any, ID comparable](r routeRegistrar, app *App, pathPrefix, path string, svc ResourceService[T, ID], opts ...ResourceOption) {
	cfg := defaultResourceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	idType := reflect.TypeOf((*ID)(nil)).Elem()
	if !resourceIDKindSupported(idType.Kind()) {
		panic(fmt.Sprintf("kruda: Resource ID type %s unsupported; use string/int/int64/uint/uint64", idType))
	}
	// Build the path-segment→ID parser once at registration so the per-request
	// handlers do no reflect.Type lookup (the resource path is opt-in, but it
	// still need not pay avoidable per-request reflection).
	parseID := buildResourceIDParser[ID](idType)

	// Precompile validators once, gated exactly like the typed path
	// (handler.go): only when a *Validator is configured. No auto-default.
	var validators []fieldValidator
	var msgs map[string]string
	if app.config.Validator != nil {
		validators = buildValidators[T](app.config.Validator)
		msgs = app.config.Validator.messages
	}
	hasValidate := len(validators) > 0

	bodyType := reflect.TypeOf((*T)(nil)).Elem()
	listRespType := reflect.TypeOf(ResourceList[T]{})

	idPath := path + "/:" + cfg.idParam
	fullPath := func(rel string) string { return joinPath(pathPrefix, rel) }

	if resourceShouldRegister(cfg, "GET") {
		r.Get(path, resourceWrapMW(cfg.middleware, resourceListHandler(svc)))
		app.routeInfos = append(app.routeInfos, routeInfo{
			method: "GET", path: fullPath(path),
			resourceOp: &resourceOp{needsListQuery: true, respType: listRespType, successCode: "200"},
		})
		r.Get(idPath, resourceWrapMW(cfg.middleware, resourceGetHandler[T, ID](svc, cfg, parseID)))
		app.routeInfos = append(app.routeInfos, routeInfo{
			method: "GET", path: fullPath(idPath),
			resourceOp: &resourceOp{idParam: cfg.idParam, idType: idType, respType: bodyType, successCode: "200"},
		})
	} else {
		if resourceShouldRegister(cfg, "LIST") {
			r.Get(path, resourceWrapMW(cfg.middleware, resourceListHandler(svc)))
			app.routeInfos = append(app.routeInfos, routeInfo{
				method: "GET", path: fullPath(path),
				resourceOp: &resourceOp{needsListQuery: true, respType: listRespType, successCode: "200"},
			})
		}
		if resourceShouldRegister(cfg, "GET_BY_ID") {
			r.Get(idPath, resourceWrapMW(cfg.middleware, resourceGetHandler[T, ID](svc, cfg, parseID)))
			app.routeInfos = append(app.routeInfos, routeInfo{
				method: "GET", path: fullPath(idPath),
				resourceOp: &resourceOp{idParam: cfg.idParam, idType: idType, respType: bodyType, successCode: "200"},
			})
		}
	}
	if resourceShouldRegister(cfg, "POST") {
		r.Post(path, resourceWrapMW(cfg.middleware, resourceCreateHandler[T, ID](svc, validators, msgs)))
		app.routeInfos = append(app.routeInfos, routeInfo{
			method: "POST", path: fullPath(path),
			resourceOp: &resourceOp{bodyType: bodyType, respType: bodyType, successCode: "201", hasValidate: hasValidate},
		})
	}
	if resourceShouldRegister(cfg, "PUT") {
		r.Put(idPath, resourceWrapMW(cfg.middleware, resourceUpdateHandler[T, ID](svc, cfg, validators, msgs, parseID)))
		app.routeInfos = append(app.routeInfos, routeInfo{
			method: "PUT", path: fullPath(idPath),
			resourceOp: &resourceOp{idParam: cfg.idParam, idType: idType, bodyType: bodyType, respType: bodyType, successCode: "200", hasValidate: hasValidate},
		})
	}
	if resourceShouldRegister(cfg, "DELETE") {
		r.Delete(idPath, resourceWrapMW(cfg.middleware, resourceDeleteHandler[T, ID](svc, cfg, parseID)))
		app.routeInfos = append(app.routeInfos, routeInfo{
			method: "DELETE", path: fullPath(idPath),
			resourceOp: &resourceOp{idParam: cfg.idParam, idType: idType, respType: nil, successCode: "204"},
		})
	}
}

func resourceListHandler[T any, ID comparable](svc ResourceService[T, ID]) HandlerFunc {
	return func(c *Ctx) error {
		page, err := resourceParsePositiveInt(c.Query("page"), "page", 1)
		if err != nil {
			return err
		}
		limit, err := resourceParsePositiveInt(c.Query("limit"), "limit", 20)
		if err != nil {
			return err
		}
		if limit > maxResourceListLimit {
			limit = maxResourceListLimit
		}
		items, total, err := svc.List(c.Context(), page, limit)
		if err != nil {
			return err
		}
		return c.JSON(ResourceList[T]{Data: items, Total: total, Page: page, Limit: limit})
	}
}

// resourceParsePositiveInt parses a pagination query value. Absent (empty) →
// def. Present must parse as an int >= 1, else a 400 with the typed-path
// message format ("invalid query parameter \"<name>\": expected int").
func resourceParsePositiveInt(raw, name string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, BadRequest(fmt.Sprintf("invalid query parameter %q: expected int", name))
	}
	return n, nil
}

// wrapBindErr normalizes a c.Bind error: an existing *KrudaError (e.g. a 413
// from an oversized body, or the empty-body 400) passes through untouched; a
// raw decoder error becomes a 400 "invalid request body", matching the typed
// path instead of falling through to a 500.
func wrapBindErr(err error) error {
	var ke *KrudaError
	if errors.As(err, &ke) {
		return err
	}
	return BadRequest("invalid request body")
}

func resourceGetHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig, parseID func(string) (ID, error)) HandlerFunc {
	return func(c *Ctx) error {
		id, err := parseID(c.Param(cfg.idParam))
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

func resourceCreateHandler[T any, ID comparable](svc ResourceService[T, ID], validators []fieldValidator, msgs map[string]string) HandlerFunc {
	return func(c *Ctx) error {
		var item T
		if err := c.Bind(&item); err != nil {
			return wrapBindErr(err)
		}
		if len(validators) > 0 {
			if ve := validate(validators, reflect.ValueOf(item), msgs); ve != nil {
				return ve
			}
		}
		created, err := svc.Create(c.Context(), item)
		if err != nil {
			return err
		}
		return c.Status(201).JSON(created)
	}
}

func resourceUpdateHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig, validators []fieldValidator, msgs map[string]string, parseID func(string) (ID, error)) HandlerFunc {
	return func(c *Ctx) error {
		id, err := parseID(c.Param(cfg.idParam))
		if err != nil {
			return BadRequest("invalid id")
		}
		var item T
		if err := c.Bind(&item); err != nil {
			return wrapBindErr(err)
		}
		if len(validators) > 0 {
			if ve := validate(validators, reflect.ValueOf(item), msgs); ve != nil {
				return ve
			}
		}
		updated, err := svc.Update(c.Context(), id, item)
		if err != nil {
			return err
		}
		return c.JSON(updated)
	}
}

func resourceDeleteHandler[T any, ID comparable](svc ResourceService[T, ID], cfg resourceConfig, parseID func(string) (ID, error)) HandlerFunc {
	return func(c *Ctx) error {
		id, err := parseID(c.Param(cfg.idParam))
		if err != nil {
			return BadRequest("invalid id")
		}
		if err := svc.Delete(c.Context(), id); err != nil {
			return err
		}
		return c.NoContent()
	}
}

// resourceIDKindSupported reports whether a reflect.Kind is an ID type the
// resource router can parse from a path segment. The registration gate and the
// parser both consult this single predicate so the two can never drift.
func resourceIDKindSupported(k reflect.Kind) bool {
	switch k {
	case reflect.String, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		return true
	}
	return false
}

// buildResourceIDParser returns a path-segment→ID parser computed once at
// registration. Hoisting the reflect.Type lookup out of the per-request path
// keeps the resource handlers off any per-request reflection beyond the
// unavoidable Convert. It switches on reflect.Kind (not the concrete type) so a
// named type such as `type UserID int64` parses end-to-end, and the parsed value
// is converted back to ID so the caller gets its exact (possibly named) type.
//
// The integer bit width comes from the ID type itself (idType.Bits()), so a
// value that overflows the target width is rejected as a 400 rather than being
// silently truncated by the conversion — this matters for `int`/`uint` IDs on
// 32-bit builds, where a fixed 64-bit parse would wrap.
func buildResourceIDParser[ID comparable](idType reflect.Type) func(string) (ID, error) {
	kind := idType.Kind()
	var bits int
	switch kind {
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		bits = idType.Bits()
	}
	return func(raw string) (ID, error) {
		switch kind {
		case reflect.String:
			return reflect.ValueOf(raw).Convert(idType).Interface().(ID), nil
		case reflect.Int, reflect.Int64:
			v, err := strconv.ParseInt(raw, 10, bits)
			if err != nil {
				var zero ID
				return zero, err
			}
			return reflect.ValueOf(v).Convert(idType).Interface().(ID), nil
		case reflect.Uint, reflect.Uint64:
			v, err := strconv.ParseUint(raw, 10, bits)
			if err != nil {
				var zero ID
				return zero, err
			}
			return reflect.ValueOf(v).Convert(idType).Interface().(ID), nil
		default:
			var zero ID
			return zero, fmt.Errorf("unsupported ID type: %s", idType)
		}
	}
}

// resourceParseID parses a single path segment into the ID type. It is a
// convenience wrapper over buildResourceIDParser for callers that do not hold a
// precomputed parser; the request handlers use the hoisted parser directly.
func resourceParseID[ID comparable](raw string) (ID, error) {
	return buildResourceIDParser[ID](reflect.TypeOf((*ID)(nil)).Elem())(raw)
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
