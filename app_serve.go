package kruda

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-kruda/kruda/transport"
)

// containsDotPercent checks if path contains . or % using a fast byte scan.
func containsDotPercent(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '%' {
			return true
		}
	}
	return false
}

// ServeHTTP implements http.Handler for net/http path (TLS, Windows, fallback).
// Includes full lifecycle pipeline (matching ServeFast) and panic recovery.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := app.ctxPool.Get().(*Ctx)

	c.embeddedReq.r = r
	c.embeddedReq.maxBody = app.config.BodyLimit
	c.embeddedReq.bodyRead = false
	c.embeddedReq.body = nil
	c.embeddedReq.queryDone = false
	c.embeddedResp.w = w
	c.embeddedResp.statusCode = 200
	c.embeddedResp.written = false
	c.embeddedResp.headerMap = nil

	if c.multipartForm != nil {
		_ = c.multipartForm.RemoveAll()
		c.multipartForm = nil
	}

	// Minimal inline reset — ONLY hot fields
	c.app = app
	c.method = r.Method
	c.path = r.URL.Path
	c.status = 200
	c.responded = false
	c.bodyParsed = false
	c.bodyBytes = nil
	c.bodyErr = nil
	c.routeIndex = 0
	c.handlers = nil
	c.writer = &c.embeddedResp
	c.request = &c.embeddedReq
	c.contentType = ""
	c.contentLength = -1
	c.cacheControl = ""
	c.location = ""
	c.body = nil
	c.logger = nil
	if len(c.respHeaders) > 0 {
		clear(c.respHeaders)
	}
	if len(c.cookies) > 0 {
		c.cookies = c.cookies[:0]
	}
	if len(c.headers) > 0 {
		clear(c.headers)
	}
	if len(c.locals) > 0 {
		clear(c.locals)
	}

	if len(c.path) == 0 {
		c.path = "/"
	}

	// Path traversal prevention (opt-in via WithPathTraversal or WithSecurity)
	if app.config.PathTraversal && len(c.path) > 1 && containsDotPercent(c.path) {
		if cleaned, err := cleanPath(c.path); err != nil {
			app.handleError(c, NewError(400, "bad request: "+err.Error()))
			c.shrinkMaps()
			app.ctxPool.Put(c)
			return
		} else {
			c.path = cleaned
		}
	}

	if c.params.count > 0 {
		c.params.reset()
	}

	// Write pre-computed security headers.
	if len(app.secHeaders) > 0 {
		h := w.Header()
		for _, kv := range app.secHeaders {
			h.Set(kv[0], kv[1])
		}
	}

	// Panic recovery — prevents unhandled panics from crashing the server.
	// Placed here (not as a named defer) to keep the fast path overhead minimal:
	// a single defer per request is negligible (~1-2ns on arm64).
	defer func() {
		if rec := recover(); rec != nil {
			app.config.Logger.Error("unrecovered panic in ServeHTTP",
				"panic", fmt.Sprintf("%v", rec),
				"method", c.method,
				"path", c.path,
			)
			if !c.responded {
				c.Status(500)
				_ = c.JSON(Map{
					"code":    500,
					"message": "internal server error",
				})
			}
		}
		c.shrinkMaps()
		app.ctxPool.Put(c)
	}()

	// === Lifecycle pipeline (mirrors ServeFast exactly) ===

	// OnRequest hooks — fire before route matching.
	if app.hasLifecycle {
		for _, hook := range app.hooks.OnRequest {
			if err := hook(c); err != nil {
				app.handleError(c, err)
				goto response
			}
		}
	}

	{
		handlers := app.router.find(c.method, c.path, &c.params)
		if handlers == nil {
			if allowed := app.router.findAllowedMethods(c.path); allowed != "" {
				c.SetHeader("Allow", allowed)
				app.handleError(c, NewError(405, "method not allowed"))
			} else {
				app.handleError(c, NotFound("not found"))
			}
			goto response
		}

		c.handlers = handlers
		c.routeIndex = 0

		// BeforeHandle hooks — fire after middleware chain is set, before handler.
		if app.hasLifecycle {
			for _, hook := range app.hooks.BeforeHandle {
				if err := hook(c); err != nil {
					app.handleError(c, err)
					goto afterHandle
				}
			}
		}

		if err := c.handlers[0](c); err != nil {
			app.handleError(c, err)
		}
	}

afterHandle:
	// AfterHandle hooks — fire after handler, before response flush.
	if app.hasLifecycle {
		for _, hook := range app.hooks.AfterHandle {
			if err := hook(c); err != nil {
				app.handleError(c, err)
			}
		}
	}

	// Flush lazy body before OnResponse hooks so hooks can inspect the response.
	if c.body != nil && !c.responded {
		c.responded = true
		c.contentLength = len(c.body)
		c.writeHeaders()
		c.writer.WriteHeader(c.status)
		_, _ = c.writer.Write(c.body)
		c.body = nil
	}

response:
	// OnResponse hooks — fire after body flush.
	// Always runs — even on 404 and OnRequest errors — so metrics/logging hooks work.
	if app.hasLifecycle {
		for _, hook := range app.hooks.OnResponse {
			_ = hook(c) // errors are logged but don't affect the response
		}
	}
}

// ServeKruda implements transport.Handler. Includes panic recovery to prevent
// server crashes from unhandled panics not caught by Recovery middleware.
func (app *App) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	c := app.ctxPool.Get().(*Ctx)
	c.reset(w, r)
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
	if app.config.PathTraversal {
		cleaned, err := cleanPath(c.path)
		if err != nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("Bad Request"))
			return
		}
		c.path = cleaned
	}

	// OnRequest hooks — fire before route matching.
	if app.hasLifecycle {
		for _, hook := range app.hooks.OnRequest {
			if err := hook(c); err != nil {
				app.handleError(c, err)
				goto response
			}
		}
	}

	{
		handlers := app.router.find(c.method, c.path, &c.params)
		if handlers == nil {
			if allowed := app.router.findAllowedMethods(c.path); allowed != "" {
				c.SetHeader("Allow", allowed)
				app.handleError(c, NewError(405, "method not allowed"))
			} else {
				app.handleError(c, NotFound("not found"))
			}
			goto response
		}

		c.handlers = handlers
		c.routeIndex = 0

		// BeforeHandle hooks — fire after middleware chain is set, before handler.
		if app.hasLifecycle {
			for _, hook := range app.hooks.BeforeHandle {
				if err := hook(c); err != nil {
					app.handleError(c, err)
					goto afterHandle
				}
			}
		}

		if err := c.handlers[0](c); err != nil {
			app.handleError(c, err)
		}
	}

afterHandle:
	// AfterHandle hooks — fire after handler, before response flush.
	if app.hasLifecycle {
		for _, hook := range app.hooks.AfterHandle {
			if err := hook(c); err != nil {
				app.handleError(c, err)
			}
		}
	}

	// Flush lazy body before OnResponse hooks so hooks can inspect the response.
	if c.body != nil && !c.responded {
		_ = c.send()
	}

response:
	// OnResponse hooks — fire after body flush.
	// Always runs — even on 404 and OnRequest errors — so metrics/logging hooks work.
	if app.hasLifecycle {
		for _, hook := range app.hooks.OnResponse {
			_ = hook(c) // errors are logged but don't affect the response
		}
	}
}

// handleError converts an error to a KrudaError, fires OnError hooks,
// and sends the appropriate JSON error response.
//
// To access the underlying *ValidationError from a custom ErrorHandler:
//
//	var ve *kruda.ValidationError
//	if errors.As(ke.Unwrap(), &ve) { ... }
func (app *App) handleError(c *Ctx, err error) {
	// Check for ValidationError first — it has its own JSON structure
	var ve *ValidationError
	if errors.As(err, &ve) {
		// OnError hooks still fire for validation errors
		ke := &KrudaError{Code: 422, Message: "Validation failed", Err: ve}
		for _, hook := range app.hooks.OnError {
			hook(c, ke)
		}

		// Don't write if response already sent
		if c.Responded() {
			return
		}

		// Use custom error handler if configured
		if app.config.ErrorHandler != nil {
			app.config.ErrorHandler(c, ke)
			return
		}

		// Default: use ValidationError's own JSON marshaling
		c.Status(422)
		data, _ := ve.MarshalJSON()
		c.SetHeader("Content-Type", "application/json; charset=utf-8")
		_ = c.sendBytes(data)
		return
	}

	ke := app.resolveError(err)

	// R7.7: Always log the full unsanitized error regardless of DevMode
	slog.Error("request error",
		"method", c.Method(),
		"path", c.Path(),
		"status", ke.Code,
		"error", err.Error(),
	)

	// Execute OnError hooks (always fire, even if already responded,
	// so logging/metrics hooks still work)
	for _, hook := range app.hooks.OnError {
		hook(c, ke)
	}

	// Don't write if response already sent
	if c.Responded() {
		return
	}

	// Try dev error page first (only in DevMode)
	if app.config.DevMode {
		if devErr := renderDevErrorPage(c, err, ke.Code); devErr != nil {
			return // dev page rendered successfully
		}
	}

	// R7.1-R7.6: Sanitize error response in production mode
	if !app.config.DevMode {
		var krudaErr *KrudaError
		isKrudaError := errors.As(err, &krudaErr)

		if !isKrudaError && !ke.mapped {
			// R7.6: Raw Go error (not KrudaError, not mapped) → replace message with HTTP status text
			ke.Message = http.StatusText(ke.Code)
			if ke.Message == "" {
				ke.Message = "Internal Server Error"
			}
			ke.Detail = ""
		} else if ke.Code >= 500 {
			// R7.2: KrudaError or mapped error with 5xx → preserve user-facing Message, strip Detail
			ke.Detail = ""
		}
		// R7.5: KrudaError/mapped error with 4xx → preserve Message and Detail as-is (user-facing)
	}

	// Use custom error handler if configured
	if app.config.ErrorHandler != nil {
		app.config.ErrorHandler(c, ke)
		return
	}

	// Default: set status and send JSON error response
	c.Status(ke.Code)
	_ = c.JSON(ke)
}

// buildChain creates a pre-built handler chain from global, group, and route-level handlers.
// Called once at registration time — the slice is reused for every request (zero alloc on hot path).
func buildChain(global, group []HandlerFunc, handler HandlerFunc) []HandlerFunc {
	chain := make([]HandlerFunc, 0, len(global)+len(group)+1)
	chain = append(chain, global...)
	chain = append(chain, group...)
	chain = append(chain, handler)
	return chain
}
