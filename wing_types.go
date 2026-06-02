package kruda

import "github.com/go-kruda/kruda/transport"

// Wing Feather presets — select optimal dispatch per route type.
// Use as the last argument to app.Get(), app.Post(), etc.
//
//	app.Get("/health", handler, kruda.WingJSON())
//	app.Get("/db", handler, kruda.WingQuery())
//	app.Get("/fortunes", handler, kruda.WingRender())

func wingFeatherOpt(f Feather) RouteOption {
	return func(rc *routeConfig) { rc.wingFeather = &f }
}

// WingFeather applies a low-level Wing Feather to a route. It preserves the
// normal handler path unless the Feather includes a StaticResponse.
func WingFeather(f Feather) RouteOption { return wingFeatherOpt(f) }

// WingPlaintext — static text, health checks. Inline in ioLoop.
func WingPlaintext() RouteOption { return wingFeatherOpt(Plaintext) }

// WingJSON — JSON response, no external I/O. Inline in ioLoop.
func WingJSON() RouteOption { return wingFeatherOpt(JSON) }

// WingQuery — DB/Redis short I/O. Blocking goroutine per connection.
func WingQuery() RouteOption { return wingFeatherOpt(Query) }

// WingRender — DB + template/HTML rendering. Blocking goroutine per connection.
func WingRender() RouteOption { return wingFeatherOpt(Render) }

// WingStaticText configures an opt-in prebuilt Wing response for public static
// hot paths. It bypasses the handler, middleware, lifecycle hooks, cookies,
// CORS, and secure-header injection on Wing transports.
func WingStaticText(status int, contentType string, body string) RouteOption {
	return wingFeatherOpt(Bolt.With(Static(
		transport.GetStaticResponseString(status, contentType, body),
	)))
}

// WingStaticJSON configures an opt-in prebuilt Wing JSON response for public
// static hot paths. It has the same bypass semantics as WingStaticText.
func WingStaticJSON(status int, body string) RouteOption {
	return WingStaticText(status, "application/json; charset=utf-8", body)
}
