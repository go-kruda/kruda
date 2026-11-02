package transport

import "testing"

type proxyTestRequest struct {
	Request
	peer, xff, xri string
	n, m           int
}

func (r *proxyTestRequest) PeerAddr() string { return r.peer }
func (r *proxyTestRequest) ForwardedHeaders() (string, int, string, int) {
	return r.xff, r.n, r.xri, r.m
}

func TestProxyPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		peer    string
		trusted []string
		xff     string
		n       int
		xri     string
		m       int
		want    string
	}{
		{name: "empty allowlist", peer: "127.0.0.1:8", xff: "192.0.2.1", n: 1, want: "127.0.0.1"},
		{name: "untrusted peer", peer: "198.51.100.1:8", trusted: []string{"127.0.0.1"}, xff: "192.0.2.1", n: 1, want: "198.51.100.1"},
		{name: "nearest untrusted", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1", "10.0.0.2"}, xff: "attacker, 192.0.2.1, 10.0.0.2", n: 1, want: "192.0.2.1"},
		{name: "all trusted", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1", "10.0.0.2"}, xff: "10.0.0.2, 127.0.0.1", n: 1, want: "10.0.0.2"},
		{name: "invalid visited hop", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, xff: "192.0.2.1, invalid", n: 1, xri: "192.0.2.2", m: 1, want: "127.0.0.1"},
		{name: "empty present", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, n: 1, xri: "192.0.2.2", m: 1, want: "127.0.0.1"},
		{name: "duplicates", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, xff: "192.0.2.1", n: 2, want: "127.0.0.1"},
		{name: "xri duplicate", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, xri: "192.0.2.2", m: 2, want: "127.0.0.1"},
		{name: "xri", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, xri: "192.0.2.2", m: 1, want: "192.0.2.2"},
		{name: "mapped", peer: "127.0.0.1:8", trusted: []string{"::ffff:127.0.0.1"}, xff: "::ffff:192.0.2.1", n: 1, want: "192.0.2.1"},
		{name: "ipv6", peer: "[::1]:8", trusted: []string{"::1"}, xff: "2001:0db8:0:0:0:0:0:1", n: 1, want: "2001:db8::1"},
		{name: "zone denied", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1"}, xff: "fe80::1%en0", n: 1, want: "127.0.0.1"},
		{name: "invalid config", peer: "127.0.0.1:8", trusted: []string{"127.0.0.1/8", "bad"}, xff: "192.0.2.1", n: 1, want: "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProxyPolicy(tc.trusted)
			r := &proxyTestRequest{peer: tc.peer, xff: tc.xff, n: tc.n, xri: tc.xri, m: tc.m}
			if got := p.ClientIP(r); got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
	if got := NewProxyPolicy(nil).ClientIP(nil); got != "" {
		t.Fatalf("nil request: %q", got)
	}
	if got := NewProxyPolicy(nil).ClientIP(directPeerOnlyRequest{}); got != "127.0.0.1" {
		t.Fatalf("no raw peer: %q", got)
	}
}

type directPeerOnlyRequest struct{ Request }

func (directPeerOnlyRequest) RemoteAddr() string { return "127.0.0.1:1234" }
