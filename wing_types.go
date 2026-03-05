package kruda

import "github.com/go-kruda/kruda/transport/wing"

// Wing Feather presets — select optimal dispatch per route type.
// Use as the last argument to app.Get(), app.Post(), etc.
//
//	app.Get("/health", handler, kruda.WingJSON())
//	app.Get("/db", handler, kruda.WingQuery())
//	app.Get("/fortunes", handler, kruda.WingRender())

func wingFeatherOpt(f wing.Feather) RouteOption {
	return func(rc *routeConfig) { rc.wingFeather = f }
}

// WingPlaintext — static text, health checks. Inline in ioLoop.
func WingPlaintext() RouteOption { return wingFeatherOpt(wing.Plaintext) }

// WingJSON — JSON response, no external I/O. Inline in ioLoop.
func WingJSON() RouteOption { return wingFeatherOpt(wing.JSON) }

// WingQuery — DB/Redis short I/O. Blocking goroutine per connection.
func WingQuery() RouteOption { return wingFeatherOpt(wing.Query) }

// WingRender — DB + template/HTML rendering. Blocking goroutine per connection.
func WingRender() RouteOption { return wingFeatherOpt(wing.Render) }
