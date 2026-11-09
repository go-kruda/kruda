package ratelimit

import (
	"github.com/valyala/fasthttp"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-kruda/kruda/transport"
)

// Keep this private policy compatible with the core policy without requiring
// an unreleased core API from this independently versioned module.
type proxyPolicy struct{ trusted map[netip.Addr]struct{} }

func newProxyPolicy(ips []string) proxyPolicy {
	p := proxyPolicy{trusted: make(map[netip.Addr]struct{}, len(ips))}
	for _, raw := range ips {
		if ip, ok := proxyIP(raw); ok {
			p.trusted[ip] = struct{}{}
		}
	}
	return p
}

// clientIP returns the canonical socket peer or a validated forwarded client.
// Requests without raw peer support return an empty string.
func (p proxyPolicy) clientIP(r transport.Request) string {
	peer, ok := r.(interface{ PeerAddr() string })
	if !ok {
		return ""
	}
	raw := peer.PeerAddr()
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	ip, ok := proxyIP(raw)
	if !ok {
		return ""
	}
	remote := ip.String()
	if _, ok := p.trusted[ip]; !ok {
		return remote
	}
	h, ok := r.(interface {
		ForwardedHeaders() (string, int, string, int)
	})
	if !ok {
		return remote
	}
	xff, n, xri, m := h.ForwardedHeaders()
	if n > 1 || m > 1 {
		return remote
	}
	if n == 1 {
		rest := xff
		for {
			i := strings.LastIndexByte(rest, ',')
			part := rest
			if i >= 0 {
				part = rest[i+1:]
			}
			hop, ok := proxyIP(strings.TrimSpace(part))
			if !ok {
				return remote
			}
			if _, ok := p.trusted[hop]; !ok || i < 0 {
				return hop.String()
			}
			rest = rest[:i]
		}
	}
	if m == 1 {
		if hop, ok := proxyIP(strings.TrimSpace(xri)); ok {
			return hop.String()
		}
	}
	return remote
}

func proxyIP(raw string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(raw)
	if err != nil || ip.Zone() != "" {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

// Older cores expose the socket peer through their native request instead.
func requestWithPeer(r transport.Request) transport.Request {
	if r == nil {
		return nil
	}
	if _, ok := r.(interface{ PeerAddr() string }); ok {
		return r
	}
	peer := ""
	var xff, xri []string
	switch raw := r.RawRequest().(type) {
	case *http.Request:
		peer = raw.RemoteAddr
		xff = raw.Header.Values("X-Forwarded-For")
		xri = raw.Header.Values("X-Real-IP")
	case *fasthttp.RequestCtx:
		peer = raw.RemoteAddr().String()
		for _, v := range raw.Request.Header.PeekAll("X-Forwarded-For") {
			xff = append(xff, string(v))
		}
		for _, v := range raw.Request.Header.PeekAll("X-Real-IP") {
			xri = append(xri, string(v))
		}
	default:
		// RemoteAddr may already contain a forwarded address on older transports.
		if r.Header("X-Forwarded-For") != "" || r.Header("X-Real-IP") != "" {
			return r
		}
		peer = r.RemoteAddr()
	}
	a, b := "", ""
	if len(xff) > 0 {
		a = xff[0]
	}
	if len(xri) > 0 {
		b = xri[0]
	}
	return &requestPeer{Request: r, peer: peer, xff: a, xri: b, n: len(xff), m: len(xri)}
}

type requestPeer struct {
	transport.Request
	peer, xff, xri string
	n, m           int
}

func (r *requestPeer) PeerAddr() string                             { return r.peer }
func (r *requestPeer) ForwardedHeaders() (string, int, string, int) { return r.xff, r.n, r.xri, r.m }
