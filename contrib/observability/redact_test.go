package observability

import "testing"

func TestRedact_DefaultSensitiveHeaders(t *testing.T) {
	r := newRedactor(nil)
	for _, h := range []string{"Authorization", "Cookie", "Set-Cookie", "X-Api-Key", "X-Auth-Token"} {
		if !r.isRedacted(h) {
			t.Fatalf("%q must be redacted by default", h)
		}
	}
	if r.isRedacted("X-Request-Id") {
		t.Fatal("non-sensitive header must not be redacted")
	}
}

func TestRedact_CaseInsensitiveAndExtra(t *testing.T) {
	r := newRedactor([]string{"X-Custom-Secret"})
	if !r.isRedacted("authorization") {
		t.Fatal("redaction must be case-insensitive")
	}
	if !r.isRedacted("x-custom-secret") {
		t.Fatal("extra redacted header not honored")
	}
}
