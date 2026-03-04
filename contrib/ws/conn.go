package ws

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Conn represents a WebSocket connection.
type Conn struct {
	rwc    net.Conn
	br     *bufio.Reader
	bw     *bufio.Writer
	mu     sync.Mutex // protects writes
	closed atomic.Bool

	maxMessageSize  int64
	readTimeout     time.Duration
	writeTimeout    time.Duration
	messageTimeout  time.Duration
	maxPingPerSec   int
	pingCount       int32
	pingWindowStart time.Time
}

// newConn wraps a hijacked net.Conn into a WebSocket connection.
func newConn(rwc net.Conn, brw *bufio.ReadWriter, cfg Config) *Conn {
	c := &Conn{
		rwc:            rwc,
		br:             brw.Reader,
		bw:             brw.Writer,
		maxMessageSize: cfg.MaxMessageSize,
		readTimeout:    cfg.ReadTimeout,
		writeTimeout:   cfg.WriteTimeout,
		messageTimeout: cfg.MessageTimeout,
		maxPingPerSec:  cfg.MaxPingPerSecond,
	}
	return c
}

// newConnFromRaw wraps a raw net.Conn (no existing bufio) into a WebSocket connection.
func newConnFromRaw(rwc net.Conn, cfg Config) *Conn {
	return &Conn{
		rwc:            rwc,
		br:             bufio.NewReaderSize(rwc, cfg.ReadBufferSize),
		bw:             bufio.NewWriterSize(rwc, cfg.WriteBufferSize),
		maxMessageSize: cfg.MaxMessageSize,
		readTimeout:    cfg.ReadTimeout,
		writeTimeout:   cfg.WriteTimeout,
		messageTimeout: cfg.MessageTimeout,
		maxPingPerSec:  cfg.MaxPingPerSecond,
	}
}

// ReadMessage reads the next complete message from the connection.
// It handles fragmented messages by assembling continuation frames.
// Returns the message type (TextMessage or BinaryMessage) and the payload.
func (c *Conn) ReadMessage() (messageType int, data []byte, err error) {
	for {
		if c.readTimeout > 0 {
			_ = c.rwc.SetReadDeadline(time.Now().Add(c.readTimeout))
		}

		f, err := readFrame(c.br, c.maxMessageSize)
		if err != nil {
			// If frame exceeded size limit, send close 1009 before returning.
			if c.maxMessageSize > 0 && strings.Contains(err.Error(), "exceeds max size") {
				_ = c.Close(CloseMessageTooBig, "message too big")
			}
			return 0, nil, err
		}

		// Handle control frames inline (can appear between fragmented data frames)
		if f.opcode >= 0x8 {
			if err := c.handleControl(f); err != nil {
				return 0, nil, err
			}
			if f.opcode == 0x8 {
				return 0, nil, io.EOF // close frame
			}
			continue
		}

		// First frame of a message
		messageType = int(f.opcode)
		data = f.payload

		if f.fin {
			// Single-frame message — size already checked in readFrame
			return messageType, data, nil
		}

		// Fragmented message — assemble continuation frames.
		// If MessageTimeout is set, apply a single deadline for the entire
		// message assembly instead of resetting per-frame.
		if c.messageTimeout > 0 {
			_ = c.rwc.SetReadDeadline(time.Now().Add(c.messageTimeout))
		}

		var totalSize int64
		totalSize = int64(len(data))

		for {
			if c.messageTimeout == 0 && c.readTimeout > 0 {
				_ = c.rwc.SetReadDeadline(time.Now().Add(c.readTimeout))
			}

			cont, err := readFrame(c.br, c.maxMessageSize)
			if err != nil {
				return 0, nil, err
			}

			// Control frames can interleave with fragmented data
			if cont.opcode >= 0x8 {
				if err := c.handleControl(cont); err != nil {
					return 0, nil, err
				}
				if cont.opcode == 0x8 {
					return 0, nil, io.EOF
				}
				continue
			}

			if cont.opcode != 0x0 {
				_ = c.Close(CloseProtocolError, "expected continuation frame")
				return 0, nil, fmt.Errorf("ws: expected continuation frame, got opcode %d", cont.opcode)
			}

			totalSize += int64(len(cont.payload))
			if c.maxMessageSize > 0 && totalSize > c.maxMessageSize {
				_ = c.Close(CloseMessageTooBig, "message too big")
				return 0, nil, fmt.Errorf("ws: message size %d exceeds limit %d", totalSize, c.maxMessageSize)
			}

			data = append(data, cont.payload...)

			if cont.fin {
				return messageType, data, nil
			}
		}
	}
}

// WriteMessage writes a complete message as a single frame.
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("ws: connection closed")
	}

	if c.writeTimeout > 0 {
		_ = c.rwc.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	if err := writeFrame(c.bw, true, byte(messageType), data); err != nil {
		return err
	}
	return c.bw.Flush()
}

// WriteText writes a text message.
func (c *Conn) WriteText(text string) error {
	return c.WriteMessage(TextMessage, []byte(text))
}

// WriteBinary writes a binary message.
func (c *Conn) WriteBinary(data []byte) error {
	return c.WriteMessage(BinaryMessage, data)
}

// Close sends a close frame and closes the underlying connection.
func (c *Conn) Close(code int, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return nil
	}
	c.closed.Store(true)

	// Best-effort close frame — ignore write errors
	if c.writeTimeout > 0 {
		_ = c.rwc.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	_ = writeCloseFrame(c.bw, code, reason)
	_ = c.bw.Flush()

	return c.rwc.Close()
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.rwc.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.rwc.SetWriteDeadline(t)
}

// handleControl processes control frames (ping, pong, close).
func (c *Conn) handleControl(f *frame) error {
	switch f.opcode {
	case 0x9: // Ping → respond with Pong
		if c.maxPingPerSec > 0 {
			now := time.Now()
			if now.Sub(c.pingWindowStart) >= time.Second {
				c.pingCount = 0
				c.pingWindowStart = now
			}
			c.pingCount++
			if int(c.pingCount) > c.maxPingPerSec {
				_ = c.Close(ClosePolicyViolation, "ping rate exceeded")
				return fmt.Errorf("ws: ping rate exceeded (%d/s)", c.maxPingPerSec)
			}
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.writeTimeout > 0 {
			_ = c.rwc.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		}
		if err := writeFrame(c.bw, true, 0xA, f.payload); err != nil {
			return err
		}
		return c.bw.Flush()

	case 0xA: // Pong — no action needed
		return nil

	case 0x8: // Close
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.closed.Load() {
			c.closed.Store(true)
			// Echo close frame back
			if c.writeTimeout > 0 {
				_ = c.rwc.SetWriteDeadline(time.Now().Add(c.writeTimeout))
			}
			_ = writeFrame(c.bw, true, 0x8, f.payload)
			_ = c.bw.Flush()
			_ = c.rwc.Close()
		}
		return nil

	default:
		// RFC 6455 §5.2: undefined control opcodes are protocol errors
		_ = c.Close(CloseProtocolError, "unknown control opcode")
		return fmt.Errorf("ws: unknown control opcode 0x%x", f.opcode)
	}
}
