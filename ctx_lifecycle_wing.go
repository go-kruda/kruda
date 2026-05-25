//go:build linux || darwin

package kruda

import "time"

func (c *Ctx) resetWing(w *wingResponse, r *wingRequest) {
	c.method = r.method
	c.path = r.path
	c.status = 200
	c.responded = false
	c.dirty = 0
	c.bodyParsed = false
	c.routeIndex = 0
	c.handlers = nil
	c.startTime = time.Time{}
	c.writer = w
	c.request = r

	c.contentType = ""
	c.contentLength = -1
	c.cacheControl = ""
	c.location = ""
	c.ctx = nil

	if c.params.count > 0 {
		c.params.reset()
	}

	c.logger = nil
}
