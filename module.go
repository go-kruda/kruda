package kruda

import "fmt"

// Module is the interface for modular DI registration.
// Modules group related service registrations into reusable units.
type Module interface {
	Install(c *Container) error
}

// Module installs a DI module into the App.
// If no container is configured, a new one is created automatically.
// Panics if Install returns an error — module registration failures are
// considered programming errors (similar to http.HandleFunc panicking on
// duplicate routes). Returns the App for method chaining.
func (app *App) Module(m Module) *App {
	if app.container == nil {
		app.container = NewContainer()
	}
	if err := m.Install(app.container); err != nil {
		panic(fmt.Sprintf("kruda: module install failed: %v", err))
	}
	return app
}
