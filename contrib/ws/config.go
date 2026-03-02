// Package ws provides WebSocket support for Kruda.
//
// It implements RFC 6455 WebSocket protocol with support for text/binary frames,
// fragmented messages, ping/pong, close handshake, and origin validation.
//
// Transport compatibility:
//   - net/http: ✅ (via http.Hijacker)
//   - fasthttp: ✅ (via RequestCtx.Hijack)
//   - Wing: ❌ v1 (no hijack API — Wing manages fd directly via io_uring/kqueue/IOCP)
package ws

import "time"

// Message type constants per RFC 6455.
const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

// Close status codes per RFC 6455 §7.4.1.
const (
	CloseNormalClosure    = 1000
	CloseGoingAway        = 1001
	CloseProtocolError    = 1002
	CloseUnsupportedData  = 1003
	CloseNoStatusReceived = 1005
	CloseAbnormalClosure  = 1006
	CloseInvalidPayload   = 1007
	ClosePolicyViolation  = 1008
	CloseMessageTooBig    = 1009
	CloseMandatoryExt     = 1010
	CloseInternalError    = 1011
)

// Config holds WebSocket upgrader configuration.
type Config struct {
	// AllowedOrigins is the list of allowed Origin header values.
	// Empty slice means all origins are allowed.
	AllowedOrigins []string

	// StrictOrigin rejects requests with no Origin header when AllowedOrigins is set.
	// By default (false), requests without an Origin header are allowed because
	// same-origin browser requests and non-browser clients may omit it (RFC 6455).
	// Set to true if you want to enforce that all WebSocket clients must send
	// an Origin header matching one of AllowedOrigins.
	StrictOrigin bool

	// MaxMessageSize is the maximum message size in bytes. 0 = unlimited.
	// This limit is enforced at both the frame level (prevents OOM on single
	// large frames) and the message level (prevents OOM on fragmented messages).
	MaxMessageSize int64

	// ReadTimeout is the deadline for reading a complete message.
	ReadTimeout time.Duration

	// WriteTimeout is the deadline for writing a complete message.
	WriteTimeout time.Duration

	// ReadBufferSize is the size of the read buffer. Default: 4096.
	ReadBufferSize int

	// WriteBufferSize is the size of the write buffer. Default: 4096.
	WriteBufferSize int
}

func (c *Config) defaults() {
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = 4096
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = 4096
	}
}
