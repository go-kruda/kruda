package kruda

import "github.com/go-kruda/kruda/transport/wing"

// Wing type RouteOptions — select optimal Feather composition per request type.
// Use as the last argument to app.Get(), app.Post(), etc.
//
//	app.Get("/", handler, kruda.WingPlaintext())
//	app.Get("/users/:id", handler, kruda.WingParamJSON())
//	app.Post("/json", handler, kruda.WingPostJSON())
//	app.Get("/db", handler, kruda.WingQuery())

func wingFeatherOpt(f wing.Feather) RouteOption {
	return func(rc *routeConfig) { rc.wingFeather = f }
}

// WingPlaintext selects the Plaintext wing — static text, health checks.
func WingPlaintext() RouteOption { return wingFeatherOpt(wing.Plaintext) }

// WingJSON selects the JSON wing — JSON response without route params.
func WingJSON() RouteOption { return wingFeatherOpt(wing.JSON) }

// WingParamJSON selects the ParamJSON wing — JSON + route param extraction.
func WingParamJSON() RouteOption { return wingFeatherOpt(wing.ParamJSON) }

// WingPostJSON selects the PostJSON wing — POST body parse + JSON response.
func WingPostJSON() RouteOption { return wingFeatherOpt(wing.PostJSON) }

// WingQuery selects the Query wing — DB/Redis short I/O.
func WingQuery() RouteOption { return wingFeatherOpt(wing.Query) }

// WingRender selects the Render wing — template/HTML with variable-size output.
func WingRender() RouteOption { return wingFeatherOpt(wing.Render) }
