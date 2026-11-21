package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// HandleFunc registers a WebSocket route: it wires the route with the
// kruda.Hijack preset (needed by the Wing transport; a harmless no-op on
// net/http and fasthttp, which ignore route presets) and performs the upgrade,
// so the same call works identically on every transport. For manual control,
// use Upgrader.Upgrade directly (on Wing that requires adding kruda.Hijack to
// the route yourself — which is exactly what this does for you).
func HandleFunc(app *kruda.App, path string, handler func(*Conn), cfg ...Config) *kruda.App {
	u := New(cfg...)
	return app.Get(path, func(c *kruda.Ctx) error {
		return u.Upgrade(c, handler)
	}, kruda.Hijack)
}

// Upgrade performs the WebSocket handshake and calls handler with the connection.
// The handler runs synchronously — when it returns, the connection is closed.
//
// Integrates with Kruda middleware chain: middleware executes before Upgrade is called.
// Returns an error if the transport doesn't support hijacking or headers are invalid.
func (u *Upgrader) Upgrade(c *kruda.Ctx, handler func(*Conn)) error {
	// R10.16-17: transport compatibility check
	transport := c.Transport()

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
	case "wing":
		return upgradeHijacker(c, accept, handler, u.Config)
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

	// RFC 6455: the upgrade is a bodyless GET. Reject a body here — in the
	// transport-agnostic validator, not just at a Wing-specific boundary — so the
	// outcome does not depend on TCP packetization (a body arriving in the same
	// read as the headers would otherwise reach the hijack path while a split
	// body is rejected earlier by Wing). Check the parsed body rather than the
	// Content-Length header, which not every transport exposes via c.Header.
	body, err := c.BodyBytes()
	if err != nil {
		return fmt.Errorf("ws: failed to read upgrade request body")
	}
	if len(body) > 0 {
		return fmt.Errorf("ws: upgrade request must not carry a body")
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
// When AllowedOrigins is empty, the scheme, host and port must match. Browsers send Origin on every
// WebSocket handshake, so this stops another site from opening a socket with
// the user's cookies. AllowedOrigins: []string{"*"} permits every origin.
// When AllowedOrigins is set:
//   - Requests with a matching Origin header are allowed
//   - Requests with no Origin header are allowed by default (same-origin / non-browser)
//   - Set StrictOrigin=true to reject requests with no Origin header
func (u *Upgrader) checkOrigin(c *kruda.Ctx) error {
	origin := c.Header("Origin")
	if len(u.Config.AllowedOrigins) == 0 {
		if origin == "" {
			return nil // non-browser client
		}
		o, err := url.Parse(origin)
		if err == nil && o.Host != "" && o.User == nil && o.Path == "" &&
			!o.ForceQuery && o.RawQuery == "" && o.Fragment == "" {
			scheme, authority := requestOrigin(c)
			h, err := url.Parse(scheme + "://" + authority)
			if err == nil && h.Hostname() != "" && o.Scheme == scheme &&
				strings.EqualFold(originHostPort(o), originHostPort(h)) {
				return nil
			}
		}
		return fmt.Errorf("ws: origin %q is not the same origin; set AllowedOrigins to permit it", origin)
	}

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

func requestOrigin(c *kruda.Ctx) (scheme, host string) {
	host = c.Header("Host")
	if r := c.Request(); r != nil {
		if hr, ok := r.RawRequest().(*http.Request); ok {
			host = hr.Host
			if hr.TLS != nil {
				return "https", host
			}
		}
	}
	// Older core versions can use AllowedOrigins for TLS-terminating proxies.
	if peer, ok := any(c).(interface{ IsTrustedProxy() bool }); ok &&
		peer.IsTrustedProxy() && strings.EqualFold(c.Header("X-Forwarded-Proto"), "https") {
		return "https", host
	}
	return "http", host
}

func originHostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
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
