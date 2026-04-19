// Package swagger serves a Swagger UI HTML page that points at an existing
// OpenAPI JSON endpoint.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/swagger"
//
//	app := kruda.New()
//	app.Get("/docs", swagger.New())
//
// With custom config:
//
//	app.Get("/docs", swagger.New(swagger.Config{
//	    SpecURL:  "/api/v1/openapi.json",
//	    Title:    "My API",
//	    DarkMode: true,
//	}))
//
// # What it does
//
// [New] returns a request handler (not middleware) that serves a single
// pre-rendered HTML page. The page loads Swagger UI assets from a CDN and
// points the viewer at the configured OpenAPI spec URL — Kruda already
// generates that JSON automatically from typed handlers (see
// [kruda.WithOpenAPIInfo]). User-provided values are HTML-escaped to
// prevent XSS, and the HTML is rendered once at init time so each request
// serves a static byte slice.
//
// # Configuration
//
//   - SpecURL:  URL of the OpenAPI JSON spec (default "/openapi.json")
//   - Title:    HTML page title (default "API Documentation")
//   - DarkMode: enable Swagger UI dark theme (default false)
//
// # See also
//
//   - https://swagger.io/tools/swagger-ui/
//   - kruda.WithOpenAPIInfo for configuring the spec endpoint
package swagger
