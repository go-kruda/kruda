package kruda

import (
	"strings"
	"testing"
)

// --- SSEStream: Done channel + EventWithID/Comment/Retry ---

func TestSSE_Done(t *testing.T) {
	app := New()
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			s.Event("test", "data")
			select {
			case <-s.Done():
			default:
			}
			return nil
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/events")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestSSE_EventWithID(t *testing.T) {
	app := New()
	app.Get("/sse-events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			s.EventWithID("1", "update", "hello")
			s.Comment("heartbeat")
			s.Retry(1000)
			return nil
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/sse-events")
	body := resp.BodyString()
	if !strings.Contains(body, "id: 1") {
		t.Errorf("missing event ID in body: %q", body)
	}
}
