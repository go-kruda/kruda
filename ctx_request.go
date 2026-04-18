package kruda

import (
	"net/http"
	"strconv"
)

// Method returns the HTTP method (GET, POST, etc.).
func (c *Ctx) Method() string { return c.method }

// Path returns the request path.
func (c *Ctx) Path() string { return c.path }

// Route returns the matched route pattern (e.g. "/users/:id").
// Returns the raw path if no pattern was matched (static routes).
func (c *Ctx) Route() string {
	if c.params.pattern != "" {
		return c.params.pattern
	}
	return c.path
}

// Param returns a path parameter value by name.
func (c *Ctx) Param(name string) string {
	return c.params.get(name)
}

// ParamInt returns a path parameter parsed as int.
func (c *Ctx) ParamInt(name string) (int, error) {
	return strconv.Atoi(c.params.get(name))
}

// Query returns a query parameter value by name, with optional default.
// An empty query value (?flag= or ?flag) returns the default.
func (c *Ctx) Query(name string, def ...string) string {
	if c.request != nil {
		if v := c.request.QueryParam(name); v != "" {
			return v
		}
	} else if v := c.tryQueryFastHTTP(name); v != "" {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// QueryInt returns a query parameter parsed as int.
func (c *Ctx) QueryInt(name string, def ...int) int {
	s := c.Query(name)
	if s == "" {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return v
}

// Header returns a request header value (lazy parsed, cached).
// Keys are normalized to canonical form so lookups are case-insensitive.
func (c *Ctx) Header(name string) string {
	key := http.CanonicalHeaderKey(name)
	if v, ok := c.headers[key]; ok {
		return v
	}
	if c.request != nil {
		v := c.request.Header(name)
		if v != "" {
			c.dirty |= dirtyHeaders
			c.headers[key] = v
		}
		return v
	}
	return ""
}

// Cookie returns the value of the named cookie via the transport interface.
func (c *Ctx) Cookie(name string) string {
	if c.request != nil {
		return c.request.Cookie(name)
	}
	return ""
}

// IP returns the client's IP address.
func (c *Ctx) IP() string {
	if c.request != nil {
		return c.request.RemoteAddr()
	}
	return ""
}

// BodyBytes returns the raw request body as a safe copy.
func (c *Ctx) BodyBytes() ([]byte, error) {
	if !c.bodyParsed {
		if c.request != nil {
			data, err := c.request.Body()
			c.bodyBytes = data
			c.bodyErr = err
		} else if data, ok := c.tryBodyBytesFastHTTP(); ok {
			// fasthttp path — PostBody() never returns an error
			c.bodyBytes = data
		}
		c.dirty |= dirtyBodyBytes
		c.bodyParsed = true
	}
	return c.bodyBytes, c.bodyErr
}

// BodyString returns the request body as a string.
// Discards body read errors for convenience — use BodyBytes() if you need error handling.
func (c *Ctx) BodyString() string {
	b, _ := c.BodyBytes()
	return string(b)
}

// Bind parses the request body into the given struct.
func (c *Ctx) Bind(v any) error {
	body, err := c.BodyBytes()
	if err != nil {
		if isBodyTooLarge(err) {
			return NewError(413, "request entity too large", err)
		}
		return BadRequest("failed to read request body")
	}
	if len(body) == 0 {
		return BadRequest("empty request body")
	}
	return c.app.config.JSONDecoder(body, v)
}
