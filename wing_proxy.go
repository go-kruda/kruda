package kruda

// ProxyConfig returns forwarding settings and an owned copy of the allowlist.
func (t *Transport) ProxyConfig() (bool, []string) {
	return t.config.TrustProxy, append([]string(nil), t.config.TrustedProxies...)
}
