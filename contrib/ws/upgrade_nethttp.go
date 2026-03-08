package ws

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/go-kruda/kruda"
)

// hijacker is the interface for accessing the underlying http.ResponseWriter.
type hijacker interface {
	Unwrap() http.ResponseWriter
}

// upgradeNetHTTP performs WebSocket upgrade on net/http transport via http.Hijacker.
func (u *Upgrader) upgradeNetHTTP(c *kruda.Ctx, acceptKey string, handler func(*Conn)) error {
	// Get the underlying http.ResponseWriter through the transport adapter
	raw := c.Request().RawRequest()
	if raw == nil {
		return fmt.Errorf("ws: cannot access raw request")
	}

	// The ResponseWriter is accessible through the writer chain.
	// We need to find the http.Hijacker interface.
	var netConn net.Conn
	var brw *bufio.ReadWriter

	// Try to get the response writer that supports hijacking.
	// The Kruda transport wraps http.ResponseWriter — we need to unwrap it.
	w := c.ResponseWriter()
	if w == nil {
		return fmt.Errorf("ws: response writer is nil")
	}

	// Check if the writer itself or its underlying type supports Hijack
	if hj, ok := w.(http.Hijacker); ok {
		var err error
		netConn, brw, err = hj.Hijack()
		if err != nil {
			return fmt.Errorf("ws: hijack failed: %w", err)
		}
	} else if unwrapper, ok := w.(hijacker); ok {
		hw := unwrapper.Unwrap()
		if hj, ok := hw.(http.Hijacker); ok {
			var err error
			netConn, brw, err = hj.Hijack()
			if err != nil {
				return fmt.Errorf("ws: hijack failed: %w", err)
			}
		} else {
			return fmt.Errorf("ws: response writer does not support hijack")
		}
	} else {
		return fmt.Errorf("ws: response writer does not support hijack")
	}

	// Write the HTTP 101 Switching Protocols response
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n" +
		"\r\n"

	if _, err := brw.WriteString(resp); err != nil {
		netConn.Close()
		return fmt.Errorf("ws: failed to write upgrade response: %w", err)
	}
	if err := brw.Flush(); err != nil {
		netConn.Close()
		return fmt.Errorf("ws: failed to flush upgrade response: %w", err)
	}

	// Create WebSocket connection and run handler
	conn := newConn(netConn, brw, u.Config)
	defer conn.Close(CloseGoingAway, "")

	handler(conn)
	return nil
}
