//go:build linux || darwin

package kruda

import "fmt"

func (app *App) serveWingSingleHandler(w *wingResponse, r *wingRequest, handler HandlerFunc) (handled bool) {
	if app.hasLifecycle {
		return false
	}
	handled = true
	c := app.ctxPool.Get().(*Ctx)
	c.resetWing(w, r)
	defer func() {
		if rec := recover(); rec != nil {
			app.config.Logger.Error("unrecovered panic in ServeKruda", "panic", fmt.Sprintf("%v", rec))
			if !c.responded {
				c.Status(500)
				_ = c.JSON(Map{
					"code":    500,
					"message": "internal server error",
				})
			}
		}
		c.cleanup()
		app.ctxPool.Put(c)
	}()

	if err := handler(c); err != nil {
		app.handleError(c, err)
	}
	if c.body != nil && !c.responded {
		_ = c.send()
	}
	return handled
}
