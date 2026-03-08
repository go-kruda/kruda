package swagger

// Config holds Swagger UI page configuration.
type Config struct {
	// SpecURL is the URL to the OpenAPI JSON spec.
	// Default: "/openapi.json"
	SpecURL string

	// Title is the HTML page title.
	// Default: "API Documentation"
	Title string

	// DarkMode enables the dark theme for Swagger UI.
	// Default: false
	DarkMode bool
}

// defaults applies default configuration values.
func (c *Config) defaults() {
	if c.SpecURL == "" {
		c.SpecURL = "/openapi.json"
	}
	if c.Title == "" {
		c.Title = "API Documentation"
	}
}
