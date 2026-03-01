//go:build !windows

// ctx_fasthttp.go — fasthttp fast-path helpers for Ctx.
//
// Each try* method is an internal fast-path called by the corresponding public
// method in context.go. They write directly to the embedded fasthttp.RequestCtx,
// bypassing the transport interface entirely. If the Ctx is not running on
// fasthttp (e.g. net/http path), they return false and the caller falls back
// to the generic transport path.
//
// Typical call pattern in context.go:
//
//	func (c *Ctx) SendBytes(data []byte) error {
//	    if c.trySendBytesFastHTTP(data) {  // fast path
//	        return nil
//	    }
//	    // net/http fallback ...
//	}
//
// The three send variants differ in how they handle the body copy:
//   - trySendBytesFastHTTP            — copies data (safe for pooled buffers)
//   - trySendBytesWithTypeFastHTTP    — copies data, sets Content-Type (string)
//   - trySendBytesWithTypeBytesFastHTTP — copies data, sets Content-Type ([]byte, zero-alloc)
//   - trySendStaticWithTypeBytesFastHTTP — zero-copy via SetBodyRaw; data MUST be immutable

package kruda

import (
	"github.com/valyala/fasthttp"
)

// ctxFastHTTPFields holds embedded fasthttp adapters to avoid per-request allocation.
type ctxFastHTTPFields struct {
	embeddedFHReq    fhReqAdapter
	embeddedFHResp   fhRespAdapter
	embeddedFHHeader fhHeaderAdapter
}

// writeHeadersFastHTTP writes per-request response headers directly to the fasthttp
// response header API, mirroring writeHeaders()/writeHeadersNetHTTP() but without
// the transport interface overhead. Security headers are NOT written here — they are
// handled separately in ServeFastHTTP() via the pre-computed app.secHeaders slice.
func (c *Ctx) writeHeadersFastHTTP(ctx *fasthttp.RequestCtx) {
	if c.contentType != "" {
		ctx.Response.Header.SetContentType(c.contentType)
	}
	if c.contentLength >= 0 {
		ctx.Response.Header.SetContentLength(c.contentLength)
	}
	if c.cacheControl != "" {
		ctx.Response.Header.Set("Cache-Control", c.cacheControl)
	}
	if c.location != "" {
		ctx.Response.Header.Set("Location", c.location)
	}
	if len(c.respHeaders) > 0 {
		for k, vals := range c.respHeaders {
			for i, v := range vals {
				if i == 0 {
					ctx.Response.Header.Set(k, v)
				} else {
					ctx.Response.Header.Add(k, v)
				}
			}
		}
	}
	if len(c.cookies) > 0 {
		for _, cookie := range c.cookies {
			ctx.Response.Header.Add("Set-Cookie", formatCookie(cookie))
		}
	}
}

func (c *Ctx) trySendBytesFastHTTP(data []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		ctx := c.embeddedFHResp.ctx
		c.writeHeadersFastHTTP(ctx)
		ctx.SetStatusCode(c.status)
		ctx.Response.SetBody(data)
		return true
	}
	return false
}

func (c *Ctx) trySendFastHTTP() bool {
	if c.embeddedFHResp.ctx != nil {
		ctx := c.embeddedFHResp.ctx
		c.writeHeadersFastHTTP(ctx)
		ctx.SetStatusCode(c.status)
		if c.body != nil {
			ctx.Response.SetBody(c.body)
			c.body = nil
		}
		return true
	}
	return false
}

// trySetHeaderFastHTTP writes a response header directly to fasthttp, bypassing map storage.
func (c *Ctx) trySetHeaderFastHTTP(key, value string) bool {
	if c.embeddedFHResp.ctx != nil {
		c.embeddedFHResp.ctx.Response.Header.Set(key, value)
		return true
	}
	return false
}

// trySetHeaderBytesFastHTTP writes a response header with a []byte value directly
// to fasthttp, avoiding the []byte→string conversion. Zero-alloc.
func (c *Ctx) trySetHeaderBytesFastHTTP(key string, value []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		c.embeddedFHResp.ctx.Response.Header.SetBytesV(key, value)
		return true
	}
	return false
}

func (c *Ctx) trySendBytesWithTypeFastHTTP(contentType string, data []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		ctx := c.embeddedFHResp.ctx
		ctx.Response.Header.SetContentTypeBytes([]byte(contentType))
		ctx.SetStatusCode(c.status)
		ctx.Response.SetBody(data)
		return true
	}
	return false
}

// trySendBytesWithTypeBytesFastHTTP is the zero-alloc variant — takes content-type as []byte.
func (c *Ctx) trySendBytesWithTypeBytesFastHTTP(contentType []byte, data []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		ctx := c.embeddedFHResp.ctx
		ctx.Response.Header.SetContentTypeBytes(contentType)
		ctx.SetStatusCode(c.status)
		ctx.Response.SetBody(data)
		return true
	}
	return false
}

// trySendStaticWithTypeBytesFastHTTP is the zero-copy variant for static pre-allocated
// response bodies. Uses SetBodyRaw instead of SetBody to avoid memcopy.
// SAFETY: The caller MUST ensure data is never modified (e.g. package-level var).
// Do NOT use with pooled buffers — use trySendBytesWithTypeBytesFastHTTP instead.
func (c *Ctx) trySendStaticWithTypeBytesFastHTTP(contentType []byte, data []byte) bool {
	if c.embeddedFHResp.ctx != nil {
		ctx := c.embeddedFHResp.ctx
		ctx.Response.Header.SetContentTypeBytes(contentType)
		ctx.SetStatusCode(c.status)
		ctx.Response.SetBodyRaw(data)
		return true
	}
	return false
}

func (c *Ctx) tryQueryFastHTTP(name string) string {
	if c.embeddedFHReq.ctx != nil {
		if b := c.embeddedFHReq.ctx.QueryArgs().Peek(name); len(b) > 0 {
			return string(b)
		}
	}
	return ""
}

// tryBodyBytesFastHTTP reads the request body directly from fasthttp, bypassing
// the transport interface. Returns (body, true) if fasthttp context is available.
func (c *Ctx) tryBodyBytesFastHTTP() ([]byte, bool) {
	if c.embeddedFHReq.ctx != nil {
		return c.embeddedFHReq.ctx.PostBody(), true
	}
	return nil, false
}

// RawResponseHeader provides direct access to the fasthttp response header
// for zero-overhead header writes. Returns nil if not running on fasthttp.
// This bypasses all Kruda header abstractions for maximum performance.
func (c *Ctx) RawResponseHeader() interface {
	SetBytesV(key string, value []byte)
} {
	if c.embeddedFHResp.ctx != nil {
		return &c.embeddedFHResp.ctx.Response.Header
	}
	return nil
}
