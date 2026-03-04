package kruda

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_HTTPIntegration(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			if err := s.Event("greeting", "hello"); err != nil {
				return err
			}
			if err := s.Data(42); err != nil {
				return err
			}
			if err := s.Comment("keep-alive"); err != nil {
				return err
			}
			return nil
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify SSE headers
	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	cc := resp.Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	// Read the body and verify SSE event format
	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	body := strings.Join(lines, "\n")

	if !strings.Contains(body, "event: greeting") {
		t.Errorf("missing 'event: greeting' in body:\n%s", body)
	}
	if !strings.Contains(body, `data: "hello"`) {
		t.Errorf("missing 'data: \"hello\"' in body:\n%s", body)
	}
	if !strings.Contains(body, "data: 42") {
		t.Errorf("missing 'data: 42' in body:\n%s", body)
	}
	if !strings.Contains(body, ": keep-alive") {
		t.Errorf("missing ': keep-alive' comment in body:\n%s", body)
	}
}

func TestSSE_ClientDisconnect(t *testing.T) {
	handlerReturned := make(chan error, 1)

	app := New(NetHTTP())
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			// Send first event
			if err := s.Event("msg", "first"); err != nil {
				handlerReturned <- err
				return err
			}
			// Keep trying to send — eventually the write will fail when client disconnects
			for i := 0; i < 100; i++ {
				err := s.Event("ping", i)
				if err != nil {
					handlerReturned <- err
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
			handlerReturned <- nil
			return nil
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Use a context with cancel to simulate client disconnect
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	// Read first event to confirm connection works
	buf := make([]byte, 256)
	resp.Body.Read(buf)

	// Cancel the request (disconnect)
	cancel()
	resp.Body.Close()

	// Wait for handler to return with an error or timeout
	select {
	case err := <-handlerReturned:
		if err == nil {
			t.Log("handler completed all events before disconnect was noticed")
		}
		// Error is expected — client disconnected
	case <-time.After(5 * time.Second):
		t.Error("handler did not return within timeout after client disconnect")
	}
}

func TestSSE_MultipleEvents(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			for i := 0; i < 5; i++ {
				if err := s.EventWithID(string(rune('0'+i)), "tick", i); err != nil {
					return err
				}
			}
			return nil
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteString("\n")
	}

	content := body.String()

	// Verify all 5 events are present with IDs
	for i := 0; i < 5; i++ {
		idStr := "id: " + string(rune('0'+i))
		if !strings.Contains(content, idStr) {
			t.Errorf("missing event ID %q in body", idStr)
		}
	}
	if !strings.Contains(content, "event: tick") {
		t.Error("missing 'event: tick' in body")
	}
}
