package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-kruda/kruda"
)

// magicGUID is the WebSocket magic GUID per RFC 6455 §4.2.2.
const magicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Upgrader performs WebSocket handshake and upgrades HTTP connections.
type Upgrader struct {
	Config Config
}

// New creates a new Upgrader with the given config.
func New(config ...Config) *Upgrader {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()
	return &Upgrader{Config: cfg}
}

// Upgrade performs the WebSocket handshake and calls handler with the connection.
// The handler runs synchronously — when it returns, the connection is closed.
//
// Integrates with Kruda middleware chain: middleware executes before Upgrade is called.
// Returns an error if the transport doesn't support hijacking or headers are invalid.
func (u *Upgrader) Upgrade(c *kruda.Ctx, handler func(*Conn)) error {
	// R10.16-17: transport compatibility check
	transport := c.Transport()
	if transport == "wing" {
		return fmt.Errorf("ws: WebSocket upgrade not supported on Wing transport — use net/http or fasthttp transport")
	}

	// Validate upgrade request headers
	if err := u.validateUpgrade(c); err != nil {
		c.Status(http.StatusBadRequest)
		return c.JSON(kruda.Map{"error": err.Error()})
	}

	// Origin validation
	if err := u.checkOrigin(c); err != nil {
		c.Status(http.StatusForbidden)
		return c.JSON(kruda.Map{"error": err.Error()})
	}

	key := c.Header("Sec-WebSocket-Key")
	accept := computeAcceptKey(key)

	// Hijack based on transport type
	switch transport {
	case "nethttp":
		return u.upgradeNetHTTP(c, accept, handler)
	case "fasthttp":
		return u.upgradeFastHTTP(c, accept, handler)
	default:
		return fmt.Errorf("ws: unsupported transport %q for WebSocket upgrade", transport)
	}
}

// validateUpgrade checks required WebSocket upgrade headers per RFC 6455 §4.2.1.
func (u *Upgrader) validateUpgrade(c *kruda.Ctx) error {
	if c.Method() != "GET" {
		return fmt.Errorf("ws: upgrade requires GET method")
	}

	upgrade := c.Header("Upgrade")
	if !strings.EqualFold(upgrade, "websocket") {
		return fmt.Errorf("ws: missing or invalid Upgrade header")
	}

	conn := c.Header("Connection")
	if !headerContains(conn, "upgrade") {
		return fmt.Errorf("ws: missing or invalid Connection header")
	}

	key := c.Header("Sec-WebSocket-Key")
	if key == "" {
		return fmt.Errorf("ws: missing Sec-WebSocket-Key header")
	}

	version := c.Header("Sec-WebSocket-Version")
	if version != "13" {
		return fmt.Errorf("ws: unsupported Sec-WebSocket-Version %q (expected 13)", version)
	}

	return nil
}

// checkOrigin validates the Origin header against allowed origins.
//
// When AllowedOrigins is empty, all origins are permitted.
// When AllowedOrigins is set:
//   - Requests with a matching Origin header are allowed
//   - Requests with no Origin header are allowed by default (same-origin / non-browser)
//   - Set StrictOrigin=true to reject requests with no Origin header
func (u *Upgrader) checkOrigin(c *kruda.Ctx) error {
	if len(u.Config.AllowedOrigins) == 0 {
		return nil // all origins allowed
	}

	origin := c.Header("Origin")
	if origin == "" {
		if u.Config.StrictOrigin {
			return fmt.Errorf("ws: Origin header required (strict mode)")
		}
		return nil // no origin header — same-origin request (RFC 6455 default)
	}

	for _, allowed := range u.Config.AllowedOrigins {
		if allowed == "*" || strings.EqualFold(origin, allowed) {
			return nil
		}
	}

	return fmt.Errorf("ws: origin %q not allowed", origin)
}

// computeAcceptKey computes the Sec-WebSocket-Accept value per RFC 6455 §4.2.2.
func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(magicGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// headerContains checks if a comma-separated header value contains a token (case-insensitive).
func headerContains(header, token string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
