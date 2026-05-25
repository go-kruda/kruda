//go:build linux || darwin

package kruda

import (
	"context"
	"testing"
)

func TestCtxResetWingUsesLazyRequestContext(t *testing.T) {
	app := New()
	c := newCtx(app)
	req := &wingRequest{ctx: context.WithValue(context.Background(), contextValueKey("wing-lazy"), "ok")}
	resp := acquireResponse()
	defer releaseResponse(resp)

	c.resetWing(resp, req)

	if got := c.Context().Value(contextValueKey("wing-lazy")); got != "ok" {
		t.Fatalf("Context value = %v, want lazy Wing request context value", got)
	}
}
