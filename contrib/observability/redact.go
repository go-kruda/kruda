package observability

import "strings"

var defaultRedactedHeaders = []string{
	"authorization", "proxy-authorization", "cookie", "set-cookie",
	"x-api-key", "x-auth-token", "x-csrf-token", "x-amz-security-token",
}

// redactor decides whether a header name is sensitive (case-insensitive).
type redactor struct{ set map[string]struct{} }

func newRedactor(extra []string) *redactor {
	m := make(map[string]struct{}, len(defaultRedactedHeaders)+len(extra))
	for _, h := range defaultRedactedHeaders {
		m[h] = struct{}{}
	}
	for _, h := range extra {
		m[strings.ToLower(h)] = struct{}{}
	}
	return &redactor{set: m}
}

func (r *redactor) isRedacted(header string) bool {
	_, ok := r.set[strings.ToLower(header)]
	return ok
}
