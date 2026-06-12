package kruda

import "github.com/go-kruda/kruda/transport"

// Presets are RouteOptions — pass them directly at route registration:
//
//	app.Get("/health", handler, kruda.Plaintext)
//	app.Get("/db", handler, kruda.DB)
//	app.Get("/fortunes", handler, kruda.Render)

// StaticText configures an opt-in prebuilt Wing response for public static
// hot paths. It bypasses the handler, middleware, lifecycle hooks, cookies,
// CORS, and secure-header injection on Wing transports.
func StaticText(status int, contentType string, body string) RouteOption {
	return Bolt.With(Static(
		transport.GetStaticResponseString(status, contentType, body),
	))
}

// StaticJSON configures an opt-in prebuilt Wing JSON response for public
// static hot paths. It has the same bypass semantics as StaticText.
func StaticJSON(status int, body string) RouteOption {
	return StaticText(status, "application/json; charset=utf-8", body)
}
