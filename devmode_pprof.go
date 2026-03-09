package kruda

import (
	"net/http"
	"net/http/pprof"
)

// registerPprofRoutes adds pprof endpoints when DevMode is enabled.
func (app *App) registerPprofRoutes() {
	app.Get("/debug/pprof/", wrapPprofHandler(pprof.Index))
	app.Get("/debug/pprof/cmdline", wrapPprofHandler(pprof.Cmdline))
	app.Get("/debug/pprof/profile", wrapPprofHandler(pprof.Profile))
	app.Get("/debug/pprof/symbol", wrapPprofHandler(pprof.Symbol))
	app.Get("/debug/pprof/trace", wrapPprofHandler(pprof.Trace))
	app.Get("/debug/pprof/allocs", wrapPprofHandler(pprof.Handler("allocs").ServeHTTP))
	app.Get("/debug/pprof/block", wrapPprofHandler(pprof.Handler("block").ServeHTTP))
	app.Get("/debug/pprof/goroutine", wrapPprofHandler(pprof.Handler("goroutine").ServeHTTP))
	app.Get("/debug/pprof/heap", wrapPprofHandler(pprof.Handler("heap").ServeHTTP))
	app.Get("/debug/pprof/mutex", wrapPprofHandler(pprof.Handler("mutex").ServeHTTP))
	app.Get("/debug/pprof/threadcreate", wrapPprofHandler(pprof.Handler("threadcreate").ServeHTTP))
}

// wrapPprofHandler adapts net/http pprof handlers to Kruda's handler signature.
func wrapPprofHandler(h http.HandlerFunc) HandlerFunc {
	return func(c *Ctx) error {
		if req := c.request.RawRequest(); req != nil {
			if r, ok := req.(*http.Request); ok {
				if w := c.ResponseWriter(); w != nil {
					if rw, ok := w.(httpResponseWriter); ok {
						c.responded = true
						h(rw.Unwrap(), r)
						return nil
					}
				}
			}
		}
		return InternalError("pprof requires net/http transport")
	}
}
