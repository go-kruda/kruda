// Package swagger serves Swagger UI HTML that points to an existing OpenAPI JSON endpoint.
//
// Usage:
//
//	app := kruda.New()
//	app.Get("/docs", swagger.New())
//	app.Get("/docs", swagger.New(swagger.Config{
//	    SpecURL:  "/api/v1/openapi.json",
//	    Title:    "My API",
//	    DarkMode: true,
//	}))
package swagger

import (
	"html"
	"strings"

	"github.com/go-kruda/kruda"
)

// New creates a Swagger UI handler with optional configuration.
// The handler serves a pre-rendered HTML page that loads Swagger UI from CDN
// and points it to the configured OpenAPI spec URL.
//
// This returns a handler (not middleware) — register it on a specific route:
//
//	app.Get("/docs", swagger.New())
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()

	// Pre-render HTML at init time for zero-alloc serving.
	page := renderHTML(cfg)

	return func(c *kruda.Ctx) error {
		return c.HTML(page)
	}
}

// renderHTML builds the Swagger UI HTML page from configuration.
// Called once at init time; the result is served for every request.
func renderHTML(cfg Config) string {
	// Escape user-provided values to prevent XSS.
	safeTitle := html.EscapeString(cfg.Title)
	safeSpecURL := html.EscapeString(cfg.SpecURL)

	var b strings.Builder
	b.Grow(2048)

	b.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>`)
	b.WriteString(safeTitle)
	b.WriteString(`</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />`)

	if cfg.DarkMode {
		b.WriteString(`
    <style>
    body { background: #1a1a2e; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .microlight { filter: invert(100%) hue-rotate(180deg); }
    .swagger-ui svg.arrow { filter: invert(100%) hue-rotate(180deg); }
    </style>`)
	}

	b.WriteString(`
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
    SwaggerUIBundle({
        url: "`)
	b.WriteString(safeSpecURL)
	b.WriteString(`",
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
        layout: "StandaloneLayout"
    })
    </script>
</body>
</html>`)

	return b.String()
}
