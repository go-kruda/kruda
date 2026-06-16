// Package observability provides one-call OpenTelemetry wiring for a Kruda app:
// server-span tracing, RED metrics, trace/log correlation, and health probes.
//
// Enable MUST be called before any route is registered. The span middleware is
// installed via app.Use, and Kruda bakes middleware into a route's chain at
// registration time — routes registered before Enable are NOT instrumented.
//
//	app := kruda.New()
//	prov, err := observability.Enable(app, observability.Config{ServiceName: "checkout"})
//	if err != nil { /* ... */ }
//	defer prov.Flush(context.Background())
//	app.Get("/orders/:id", getOrder) // instrumented
//	app.Listen(":8080")
package observability
