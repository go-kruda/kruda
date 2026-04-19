package kruda

// Wing Feather presets — select optimal dispatch per route type.
// Use as the last argument to app.Get(), app.Post(), etc.
//
//	app.Get("/health", handler, kruda.WingJSON())
//	app.Get("/db", handler, kruda.WingQuery())
//	app.Get("/fortunes", handler, kruda.WingRender())

func wingFeatherOpt(f Feather) RouteOption {
	return func(rc *routeConfig) { rc.wingFeather = &f }
}

// WingPlaintext — static text, health checks. Inline in ioLoop.
func WingPlaintext() RouteOption { return wingFeatherOpt(Plaintext) }

// WingJSON — JSON response, no external I/O. Inline in ioLoop.
func WingJSON() RouteOption { return wingFeatherOpt(JSON) }

// WingQuery — DB/Redis short I/O. Blocking goroutine per connection.
func WingQuery() RouteOption { return wingFeatherOpt(Query) }

// WingRender — DB + template/HTML rendering. Blocking goroutine per connection.
func WingRender() RouteOption { return wingFeatherOpt(Render) }
