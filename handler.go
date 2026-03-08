package kruda

import "reflect"

// C is the generic typed context that embeds *Ctx with a parsed input field.
// c.In contains the parsed + validated input from all sources (param, query, body).
type C[T any] struct {
	*Ctx
	In T // parsed input from request
}

// RouteOption configures per-route metadata (for future OpenAPI integration).
type RouteOption func(*routeConfig)

type routeConfig struct {
	description  string
	tags         []string
	inType       reflect.Type // input type T (for OpenAPI schema generation)
	outType      reflect.Type // output type Out (for OpenAPI schema generation)
	wingFeather  any          // wing.Feather (any to avoid import cycle)
}

// WithDescription sets a route description (used by OpenAPI in Phase 2B).
func WithDescription(desc string) RouteOption {
	return func(rc *routeConfig) { rc.description = desc }
}

// WithTags sets route tags (used by OpenAPI in Phase 2B).
func WithTags(tags ...string) RouteOption {
	return func(rc *routeConfig) { rc.tags = tags }
}

// buildTypedHandler creates the handler closure with pre-compiled parser and validators.
// Called once at route registration time.
//
// NOTE: Validators are compiled at registration time. Custom validation rules
// (via app.Validator().Register()) must be configured BEFORE registering typed routes.
// Rules added after route registration will not take effect for those routes.
func buildTypedHandler[In any, Out any](
	app *App,
	method, path string,
	handler func(*C[In]) (*Out, error),
	opts []RouteOption,
) HandlerFunc {
	// Pre-compile at registration time
	parser := buildInputParser[In]()
	var validators []fieldValidator
	if app.config.Validator != nil {
		validators = buildValidators[In](app.config.Validator)
	}

	// Apply route options
	rc := routeConfig{
		inType:  reflect.TypeOf((*In)(nil)).Elem(),
		outType: reflect.TypeOf((*Out)(nil)).Elem(),
	}
	for _, opt := range opts {
		opt(&rc)
	}

	// Store route info for OpenAPI generation
	app.routeInfos = append(app.routeInfos, routeInfo{
		method:      method,
		path:        path,
		config:      rc,
		hasBody:     parser.hasBody,
		hasForm:     parser.hasForm,
		hasValidate: len(validators) > 0,
	})

	return func(c *Ctx) error {
		val, err := parser.parse(c)
		if err != nil {
			return err
		}

		if len(app.hooks.OnParse) > 0 {
			ptr := val.Addr().Interface()
			for _, hook := range app.hooks.OnParse {
				if err := hook(c, ptr); err != nil {
					return err
				}
			}
		}

		if len(validators) > 0 {
			if ve := validate(validators, val, app.config.Validator.messages); ve != nil {
				return ve
			}
		}

		tc := &C[In]{Ctx: c, In: val.Interface().(In)}
		result, err := handler(tc)
		if err != nil {
			return err
		}

		if result != nil {
			return c.JSON(result)
		}
		return c.NoContent()
	}
}

// Get registers a typed GET handler with pre-compiled binding and validation.
// Binds from param/query only (no body for GET).
func Get[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	h := buildTypedHandler[In, Out](app, "GET", path, handler, opts)
	app.Get(path, h)
}

// Post registers a typed POST handler with pre-compiled binding and validation.
func Post[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	h := buildTypedHandler[In, Out](app, "POST", path, handler, opts)
	app.Post(path, h)
}

// Put registers a typed PUT handler with pre-compiled binding and validation.
func Put[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	h := buildTypedHandler[In, Out](app, "PUT", path, handler, opts)
	app.Put(path, h)
}

// Delete registers a typed DELETE handler with pre-compiled binding and validation.
// Binds from param/query only (no body for DELETE).
func Delete[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	h := buildTypedHandler[In, Out](app, "DELETE", path, handler, opts)
	app.Delete(path, h)
}

// Patch registers a typed PATCH handler with pre-compiled binding and validation.
func Patch[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	h := buildTypedHandler[In, Out](app, "PATCH", path, handler, opts)
	app.Patch(path, h)
}

// GetX registers a short typed GET handler (no error return).
// Panics are caught by Recovery middleware. For prototyping and simple endpoints.
func GetX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption) {
	Get[In, Out](app, path, func(c *C[In]) (*Out, error) {
		return handler(c), nil
	}, opts...)
}

// PostX registers a short typed POST handler (no error return).
func PostX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption) {
	Post[In, Out](app, path, func(c *C[In]) (*Out, error) {
		return handler(c), nil
	}, opts...)
}

// PutX registers a short typed PUT handler (no error return).
func PutX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption) {
	Put[In, Out](app, path, func(c *C[In]) (*Out, error) {
		return handler(c), nil
	}, opts...)
}

// DeleteX registers a short typed DELETE handler (no error return).
func DeleteX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption) {
	Delete[In, Out](app, path, func(c *C[In]) (*Out, error) {
		return handler(c), nil
	}, opts...)
}

// PatchX registers a short typed PATCH handler (no error return).
func PatchX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption) {
	Patch[In, Out](app, path, func(c *C[In]) (*Out, error) {
		return handler(c), nil
	}, opts...)
}

// GroupGet registers a typed GET handler on a Group.
func GroupGet[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	fullPath := joinPath(g.prefix, path)
	h := buildTypedHandler[In, Out](g.app, "GET", fullPath, handler, opts)
	g.Get(path, h)
}

// GroupPost registers a typed POST handler on a Group.
func GroupPost[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	fullPath := joinPath(g.prefix, path)
	h := buildTypedHandler[In, Out](g.app, "POST", fullPath, handler, opts)
	g.Post(path, h)
}

// GroupPut registers a typed PUT handler on a Group.
func GroupPut[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	fullPath := joinPath(g.prefix, path)
	h := buildTypedHandler[In, Out](g.app, "PUT", fullPath, handler, opts)
	g.Put(path, h)
}

// GroupDelete registers a typed DELETE handler on a Group.
func GroupDelete[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	fullPath := joinPath(g.prefix, path)
	h := buildTypedHandler[In, Out](g.app, "DELETE", fullPath, handler, opts)
	g.Delete(path, h)
}

// GroupPatch registers a typed PATCH handler on a Group.
func GroupPatch[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption) {
	fullPath := joinPath(g.prefix, path)
	h := buildTypedHandler[In, Out](g.app, "PATCH", fullPath, handler, opts)
	g.Patch(path, h)
}
