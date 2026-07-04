package ws

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/go-kruda/kruda"
)

// hijacker is the optional interface a wrapped ResponseWriter exposes to reach
// the underlying http.ResponseWriter.
type hijacker interface {
	Unwrap() http.ResponseWriter
}

// upgradeHijacker performs the WebSocket upgrade over any transport whose
// c.ResponseWriter() implements the standard http.Hijacker contract. It is
// transport-agnostic: net/http and Wing both route here.
func upgradeHijacker(c *kruda.Ctx, acceptKey string, handler func(*Conn), cfg Config) error {
	w := c.ResponseWriter()
	if w == nil {
		return fmt.Errorf("ws: response writer is nil")
	}

	var netConn net.Conn
	var brw *bufio.ReadWriter
	if hj, ok := w.(http.Hijacker); ok {
		var err error
		netConn, brw, err = hj.Hijack()
		if err != nil {
			return fmt.Errorf("ws: hijack failed: %w", err)
		}
	} else if unwrapper, ok := w.(hijacker); ok {
		hw := unwrapper.Unwrap()
		hj, ok := hw.(http.Hijacker)
		if !ok {
			return fmt.Errorf("ws: response writer does not support hijack")
		}
		var err error
		netConn, brw, err = hj.Hijack()
		if err != nil {
			return fmt.Errorf("ws: hijack failed: %w", err)
		}
	} else {
		return fmt.Errorf("ws: response writer does not support hijack")
	}

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

	conn := newConn(netConn, brw, cfg)
	defer conn.Close(CloseGoingAway, "")
	handler(conn)
	return nil
}
