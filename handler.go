package kruda

import (
	"log/slog"
	"reflect"
	"sync"
)

// C is the generic typed context that embeds *Ctx with a parsed input field.
// c.In holds the input parsed from every source (param, query, body). It is
// validated against the validate tags only when the App has a Validator; see
// WithValidator. Without one the tags are inert and c.In is unchecked.
type C[T any] struct {
	*Ctx
	In T // parsed input from request
}

// RouteOption configures per-route behavior at registration time: OpenAPI
// metadata (WithDescription, WithTags) and Wing per-route presets (Bolt,
// kruda.DB, …), which implement RouteOption directly.
type RouteOption interface{ applyRoute(*routeConfig) }

// routeOptionFunc adapts a plain function to RouteOption.
type routeOptionFunc func(*routeConfig)

func (f routeOptionFunc) applyRoute(rc *routeConfig) { f(rc) }

type routeConfig struct {
	description     string
	tags            []string
	inType          reflect.Type // input type T (for OpenAPI schema generation)
	outType         reflect.Type // output type Out (for OpenAPI schema generation)
	requestExample  any
	responseExample any
	security        []map[string][]string
	preset          *Preset // per-route Wing dispatch hint (nil = use defaults)
}

// WithDescription sets a route description (used by OpenAPI in Phase 2B).
func WithDescription(desc string) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) { rc.description = desc })
}

// WithTags sets route tags (used by OpenAPI in Phase 2B).
func WithTags(tags ...string) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) { rc.tags = tags })
}

// WithRequestExample sets the OpenAPI request body example for a typed route.
func WithRequestExample(example any) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) { rc.requestExample = example })
}

// WithResponseExample sets the OpenAPI 200 response example for a typed route.
func WithResponseExample(example any) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) { rc.responseExample = example })
}

// WithOpenAPISecurity adds an OpenAPI security requirement to a typed route.
func WithOpenAPISecurity(name string, scopes ...string) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) {
		// OpenAPI requires the scopes value to be an array, never null — use an
		// empty slice when no scopes are given (e.g. bearer auth) so it
		// serializes as [] not null.
		if scopes == nil {
			scopes = []string{}
		}
		rc.security = append(rc.security, map[string][]string{name: scopes})
	})
}

// generatedPlanMarker identifies plan options of any input type, so a plan
// wired to a route for a different input can warn instead of passing
// silently: the type assertion for the route's own input simply misses it.
type generatedPlanMarker interface {
	RouteOption
	isGeneratedPlan()
}

type generatedPlanOption[In any] struct {
	plan GeneratedPlan[In]
}

func (o generatedPlanOption[In]) applyRoute(*routeConfig) {}
func (o generatedPlanOption[In]) isGeneratedPlan()        {}

// warnedPlanMismatch keeps the crossed-plan warning to one line per route.
var warnedPlanMismatch sync.Map

// WithGeneratedPlan selects a generated binder (and its compiled validator,
// when the generator emitted one) for a typed route. The plan's Shape is
// attested against the route input at registration: a stale plan drops its
// binder silently and the route keeps the generic parser, exactly as if no
// plan had been passed. A plan generated for a different input type cannot
// match the route and is ignored with a startup warning.
//
// Experimental API for generated binders; its shape may change before any
// release.
func WithGeneratedPlan[In any](plan GeneratedPlan[In]) RouteOption {
	return generatedPlanOption[In]{plan: plan}
}

// selectGeneratedPlan resolves the effective binder and validator factory
// from the route options. Later plans win, matching the other RouteOptions.
// Every mismatch fails closed to the generic parser and validation.
func selectGeneratedPlan[In any](method, path string, opts []RouteOption) (
	binder func(*Ctx) (reflect.Value, error),
	validatorFactory func([]ValidatorDescriptor) func(*In) bool,
) {
	var plan *GeneratedPlan[In]
	sawPlan := false
	for _, opt := range opts {
		if _, ok := opt.(generatedPlanMarker); ok {
			sawPlan = true
		}
		if p, ok := opt.(generatedPlanOption[In]); ok {
			cp := p.plan
			plan = &cp
		}
	}
	if plan == nil {
		if sawPlan {
			key := method + "\x00" + path
			if _, loaded := warnedPlanMismatch.LoadOrStore(key, struct{}{}); !loaded {
				slog.Warn("kruda: generated plan for a different input type — the route keeps the generic parser",
					"method", method, "path", path)
			}
		}
		return nil, nil
	}
	if plan.Binder == nil || !AttestBinderShape[In](plan.Shape) {
		// Stale or declined binder: silent, like the white-box path. The
		// validator factory attests independently against the descriptors.
		return nil, plan.ValidatorFactory
	}
	return plan.Binder, plan.ValidatorFactory
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
	binder, validatorFactory := selectGeneratedPlan[In](method, path, opts)
	return buildTypedHandlerWithBinder(app, method, path, handler, opts, binder, validatorFactory)
}

func buildTypedHandlerWithBinder[In any, Out any](
	app *App,
	method, path string,
	handler func(*C[In]) (*Out, error),
	opts []RouteOption,
	binder func(*Ctx) (reflect.Value, error),
	validatorFactory func([]ValidatorDescriptor) func(*In) bool,
) HandlerFunc {
	return buildTypedHandlerWithValidation(app, method, path, handler, opts, binder, validatorFactory)
}

func buildTypedHandlerWithValidation[In any, Out any](
	app *App,
	method, path string,
	handler func(*C[In]) (*Out, error),
	opts []RouteOption,
	binder func(*Ctx) (reflect.Value, error),
	validatorFactory func([]ValidatorDescriptor) func(*In) bool,
) HandlerFunc {
	// Pre-compile at registration time
	parser := buildInputParser[In]()
	var validators []fieldValidator
	if app.config.Validator != nil {
		validators = buildValidators[In](app.config.Validator)
	}
	var validInput func(*In) bool
	if validatorFactory != nil && len(validators) > 0 {
		validInput = validatorFactory(describeValidators(validators))
	}

	// Apply route options
	rc := routeConfig{
		inType:  reflect.TypeOf((*In)(nil)).Elem(),
		outType: reflect.TypeOf((*Out)(nil)).Elem(),
	}
	for _, opt := range opts {
		opt.applyRoute(&rc)
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

	if binder == nil {
		return func(c *Ctx) error {
			val, err := parser.parse(c)
			if err != nil {
				return err
			}
			return finishTypedHandler(app, validators, handler, c, val, validInput)
		}
	}
	return func(c *Ctx) error {
		val, err := binder(c)
		if err != nil {
			return err
		}
		return finishTypedHandler(app, validators, handler, c, val, validInput)
	}
}

func finishTypedHandler[In any, Out any](app *App, validators []fieldValidator, handler func(*C[In]) (*Out, error), c *Ctx, val reflect.Value, validInput func(*In) bool) error {
	if len(app.hooks.OnParse) > 0 {
		ptr := val.Addr().Interface()
		for _, hook := range app.hooks.OnParse {
			if err := hook(c, ptr); err != nil {
				return err
			}
		}
	}

	// Generated predicates skip only successful checks; failures retain the original snapshots.
	if len(validators) > 0 && (validInput == nil || !validInput(val.Addr().Interface().(*In))) {
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
