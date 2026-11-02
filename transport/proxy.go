package transport

import (
	"net"
	"net/netip"
	"strings"
)

// ProxyConfigProvider exposes transport forwarding settings for application
// defaults. Explicit application proxy options take precedence.
type ProxyConfigProvider interface {
	ProxyConfig() (bool, []string)
}

// ProxyPolicy resolves client IPs through an exact IP allowlist.
// Invalid entries are ignored, so an empty or invalid list trusts no peer.
type ProxyPolicy struct{ trusted map[netip.Addr]struct{} }

// NewProxyPolicy builds a policy from exact IPv4 or IPv6 addresses, not CIDRs.
func NewProxyPolicy(ips []string) ProxyPolicy {
	p := ProxyPolicy{trusted: make(map[netip.Addr]struct{}, len(ips))}
	for _, raw := range ips {
		if ip, ok := proxyIP(raw); ok {
			p.trusted[ip] = struct{}{}
		}
	}
	return p
}

// ClientIP returns the canonical socket peer or a validated forwarded client.
// Without PeerAddr, RemoteAddr is treated as the direct peer and forwarding
// headers are ignored.
func (p ProxyPolicy) ClientIP(r Request) string {
	if r == nil {
		return ""
	}
	peer, hasPeer := r.(interface{ PeerAddr() string })
	raw := ""
	if hasPeer {
		raw = peer.PeerAddr()
	} else {
		raw = r.RemoteAddr()
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	ip, ok := proxyIP(raw)
	if !ok {
		return ""
	}
	remote := ip.String()
	if !hasPeer {
		return remote
	}
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
