package kruda

import "github.com/go-kruda/kruda/transport"

// Wing Preset presets — select optimal dispatch per route type.
// Use as the last argument to app.Get(), app.Post(), etc.
//
//	app.Get("/health", handler, kruda.WingJSON())
//	app.Get("/db", handler, kruda.WingQuery())
//	app.Get("/fortunes", handler, kruda.WingRender())

func presetOpt(f Preset) RouteOption {
	return routeOptionFunc(func(rc *routeConfig) { rc.preset = &f })
}

// WingPreset applies a low-level Wing Preset to a route. It preserves the
// normal handler path unless the Preset includes a StaticResponse.
func WingPreset(f Preset) RouteOption { return presetOpt(f) }

// WingPlaintext — static text, health checks. Inline in ioLoop.
func WingPlaintext() RouteOption { return presetOpt(Plaintext) }

// WingJSON — JSON response, no external I/O. Inline in ioLoop.
func WingJSON() RouteOption { return presetOpt(JSON) }

// WingQuery — DB/Redis short I/O. Blocking goroutine per connection.
func WingQuery() RouteOption { return presetOpt(Query) }

// WingRender — DB + template/HTML rendering. Blocking goroutine per connection.
func WingRender() RouteOption { return presetOpt(Render) }

// WingStaticText configures an opt-in prebuilt Wing response for public static
// hot paths. It bypasses the handler, middleware, lifecycle hooks, cookies,
// CORS, and secure-header injection on Wing transports.
func WingStaticText(status int, contentType string, body string) RouteOption {
	return presetOpt(Bolt.With(Static(
		transport.GetStaticResponseString(status, contentType, body),
	)))
}

// WingStaticJSON configures an opt-in prebuilt Wing JSON response for public
// static hot paths. It has the same bypass semantics as WingStaticText.
func WingStaticJSON(status int, body string) RouteOption {
	return WingStaticText(status, "application/json; charset=utf-8", body)
}
