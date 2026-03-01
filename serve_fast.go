//go:build !windows

package kruda

import (
	"context"
	"mime/multipart"
	"unsafe"

	"github.com/go-kruda/kruda/transport"
	"github.com/valyala/fasthttp"
)

// Pre-interned method strings avoid allocating a new string header per request.
// These are compile-time constants, so comparisons use pointer equality when possible.
var (
	methodStringGET     = "GET"
	methodStringPOST    = "POST"
	methodStringPUT     = "PUT"
	methodStringDELETE  = "DELETE"
	methodStringPATCH   = "PATCH"
	methodStringHEAD    = "HEAD"
	methodStringOPTIONS = "OPTIONS"
)

// internMethod returns a pre-interned string for standard HTTP methods,
// avoiding unsafe.String allocation on every request. Falls back to
// unsafe.String only for custom/non-standard methods.
func internMethod(b []byte) string {
	switch len(b) {
	case 3:
		if b[0] == 'G' {
			return methodStringGET
		}
		if b[0] == 'P' {
			return methodStringPUT
		}
	case 4:
		if b[0] == 'P' {
			return methodStringPOST
		}
		if b[0] == 'H' {
			return methodStringHEAD
		}
	case 5:
		return methodStringPATCH
	case 6:
		return methodStringDELETE
	case 7:
		return methodStringOPTIONS
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ServeFast handles a fasthttp request directly, bypassing transport selection.
// This is the fastest path for fasthttp-based benchmarks and production use.
func (app *App) ServeFast(ctx *fasthttp.RequestCtx) {
	c := app.ctxPool.Get().(*Ctx)

	// Only reset fields handlers actually read/write.
	// Cold fields use dirty flags — only cleaned when actually touched.
	c.app = app
	c.method = internMethod(ctx.Method())
	c.status = 200
	c.responded = false
	c.dirty = 0
	c.routeIndex = 0

	c.embeddedFHResp.ctx = ctx
	c.embeddedFHReq.ctx = ctx

	// Reset params — just count=0, next find() overwrites used slots.
	c.params.count = 0

	// Reset fixed-slot headers from previous request to prevent stale data.
	c.contentType = ""
	c.contentLength = -1
	c.cacheControl = ""
	c.location = ""
	c.bodyParsed = false
	c.logger = nil // lazy-init in Log() — must nil to avoid stale request attributes

	// Fast path — skip cleanPath for simple paths (opt-in via WithPathTraversal or WithSecurity)
	pathBytes := ctx.Path()
	path := unsafe.String(unsafe.SliceData(pathBytes), len(pathBytes))
	if path == "" {
		c.path = "/"
	} else if app.config.PathTraversal && len(path) > 1 && containsDotPercent(path) {
		cleaned, err := cleanPath(path)
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString("Bad Request")
			goto cleanup
		}
		c.path = cleaned
	} else {
		c.path = path
	}

	if len(app.secHeaders) > 0 {
		for _, kv := range app.secHeaders {
			ctx.Response.Header.Set(kv[0], kv[1])
		}
	}

	// === Lifecycle pipeline ===
	// When hasLifecycle=false (no hooks registered), this is a zero-cost branch —
	// the compiler sees a constant bool and the dead code is eliminated by PGO/inlining.

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

	// Post-handler flush: only needed if handler used SetBody() lazy path.
	// Most handlers call Send*/SendBytes* which set responded=true, so this is rarely taken.
	if !c.responded && c.body != nil {
		c.responded = true
		fhCtx := c.embeddedFHResp.ctx
		if c.contentType != "" {
			fhCtx.Response.Header.SetContentType(c.contentType)
		}
		bodyLen := len(c.body)
		if c.contentLength >= 0 {
			fhCtx.Response.Header.SetContentLength(c.contentLength)
		} else {
			fhCtx.Response.Header.SetContentLength(bodyLen)
		}
		if c.cacheControl != "" {
			fhCtx.Response.Header.Set("Cache-Control", c.cacheControl)
		}
		if c.location != "" {
			fhCtx.Response.Header.Set("Location", c.location)
		}
		if len(c.respHeaders) > 0 {
			for k, vals := range c.respHeaders {
				for i, v := range vals {
					if i == 0 {
						fhCtx.Response.Header.Set(k, v)
					} else {
						fhCtx.Response.Header.Add(k, v)
					}
				}
			}
		}
		if len(c.cookies) > 0 {
			for _, cookie := range c.cookies {
				fhCtx.Response.Header.Add("Set-Cookie", formatCookie(cookie))
			}
		}
		fhCtx.SetStatusCode(c.status)
		fhCtx.Response.SetBody(c.body)
		c.body = nil
	}

response:
	// OnResponse hooks — fire after body flush so hooks can inspect final state.
	// Always runs — even on 404 and OnRequest errors — so metrics/logging hooks work.
	if app.hasLifecycle {
		for _, hook := range app.hooks.OnResponse {
			_ = hook(c) // errors are logged but don't affect the response
		}
	}

cleanup:
	// Cleanup before returning to pool.
	// Clear transport references — they're per-request from fasthttp.
	c.embeddedFHResp.ctx = nil
	c.embeddedFHReq.ctx = nil
	c.handlers = nil

	// Dirty-flag cleanup: only clear fields that were actually touched.
	// On the typical plaintext/JSON hot path, dirty=0 and this entire block is skipped.
	if d := c.dirty; d != 0 {
		if d&dirtyHeaders != 0 {
			clear(c.headers)
		}
		if d&dirtyRespHdrs != 0 {
			clear(c.respHeaders)
		}
		if d&dirtyLocals != 0 {
			clear(c.locals)
		}
		// Shrink oversized maps to prevent unbounded pool memory growth.
		if d&(dirtyHeaders|dirtyRespHdrs|dirtyLocals) != 0 {
			c.shrinkMaps()
		}
		if d&dirtyCookies != 0 {
			c.cookies = c.cookies[:0]
		}
		if d&dirtyBody != 0 {
			c.body = nil
		}
		if d&dirtyBodyBytes != 0 {
			c.bodyBytes = nil
			c.bodyErr = nil
		}
		if d&dirtyCtx != 0 {
			c.ctx = nil
		}
		if d&dirtyMultipart != 0 {
			if c.multipartForm != nil {
				_ = c.multipartForm.RemoveAll()
				c.multipartForm = nil
			}
		}
	}

	app.ctxPool.Put(c)
}

type fhReqAdapter struct {
	ctx *fasthttp.RequestCtx
}

func (r *fhReqAdapter) Method() string               { return string(r.ctx.Method()) }
func (r *fhReqAdapter) Path() string                 { return string(r.ctx.Path()) }
func (r *fhReqAdapter) Header(key string) string     { return string(r.ctx.Request.Header.Peek(key)) }
func (r *fhReqAdapter) Body() ([]byte, error)        { return r.ctx.PostBody(), nil }
func (r *fhReqAdapter) QueryParam(key string) string { return string(r.ctx.QueryArgs().Peek(key)) }
func (r *fhReqAdapter) RemoteAddr() string           { return r.ctx.RemoteAddr().String() }
func (r *fhReqAdapter) Cookie(name string) string    { return string(r.ctx.Request.Header.Cookie(name)) }
func (r *fhReqAdapter) RawRequest() any              { return r.ctx }
func (r *fhReqAdapter) Context() context.Context     { return r.ctx }

func (r *fhReqAdapter) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	return r.ctx.Request.MultipartForm()
}

type fhRespAdapter struct {
	ctx *fasthttp.RequestCtx
}

func (w *fhRespAdapter) WriteHeader(code int)           { w.ctx.SetStatusCode(code) }
func (w *fhRespAdapter) Write(data []byte) (int, error) { return w.ctx.Write(data) }
func (w *fhRespAdapter) Header() transport.HeaderMap    { return &fhHeaderAdapter{ctx: w.ctx} }

type fhHeaderAdapter struct {
	ctx *fasthttp.RequestCtx
}

func (h *fhHeaderAdapter) Set(key, value string) { h.ctx.Response.Header.Set(key, value) }
func (h *fhHeaderAdapter) Get(key string) string { return string(h.ctx.Response.Header.Peek(key)) }
func (h *fhHeaderAdapter) Del(key string)        { h.ctx.Response.Header.Del(key) }
func (h *fhHeaderAdapter) Add(key, value string) { h.ctx.Response.Header.Add(key, value) }

// tryFastHTTPText attempts to send text response directly via fasthttp without transport interface.
// Returns true if successful (fasthttp context available), false otherwise.
func (c *Ctx) tryFastHTTPText(s string) bool {
	if c.embeddedFHResp.ctx != nil {
		c.responded = true
		ctx := c.embeddedFHResp.ctx
		ctx.SetStatusCode(c.status)
		ctx.Response.Header.SetContentType("text/plain; charset=utf-8")
		ctx.SetBodyString(s)
		return true
	}
	return false
}

// tryFastHTTPJSON attempts to send a JSON response directly via fasthttp without transport interface.
// This bypasses sendBytes() pool-copy overhead and writes directly to fasthttp's response buffer.
// Returns true if successful (fasthttp context available), false otherwise.
func (c *Ctx) tryFastHTTPJSON(data []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		c.responded = true
		ctx := c.embeddedFHResp.ctx
		ctx.SetStatusCode(c.status)
		ctx.Response.Header.SetContentTypeBytes(jsonContentType)
		ctx.Response.SetBody(data)
		return true
	}
	return false
}
