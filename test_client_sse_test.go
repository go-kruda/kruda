package kruda

import (
	"strings"
	"testing"
)

func TestSSETestClientHelper(t *testing.T) {
	app := New()
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(stream *SSEStream) error {
			if err := stream.Data("hello"); err != nil {
				return err
			}
			return stream.Event("ping", "world")
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	result, err := tc.SSE("/events")
	if err != nil {
		t.Fatalf("SSE() returned error: %v", err)
	}

	if result.Status != 200 {
		t.Fatalf("expected status 200, got %d", result.Status)
	}

	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d: %v", len(result.Events), result.Events)
	}

	if !strings.Contains(result.Events[0], "data: hello") {
		t.Errorf("event[0] = %q, want to contain 'data: hello'", result.Events[0])
	}

	if !strings.Contains(result.Events[1], "event: ping") || !strings.Contains(result.Events[1], "data: world") {
		t.Errorf("event[1] = %q, want to contain 'event: ping' and 'data: world'", result.Events[1])
	}
}
