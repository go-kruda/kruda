package kruda

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// SSEStream writes Server-Sent Events to the client.
type SSEStream struct {
	writer  io.Writer
	flusher http.Flusher
	encode  func(v any) ([]byte, error)
	ctx     context.Context
}

// Event sends a named event with data (JSON-encoded).
func (s *SSEStream) Event(event string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("SSE encode error: %w", err)
	}
	fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", event, b)
	s.flusher.Flush()
	return nil
}

// EventWithID sends a named event with an ID (for client reconnection).
func (s *SSEStream) EventWithID(id, event string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("SSE encode error: %w", err)
	}
	fmt.Fprintf(s.writer, "id: %s\nevent: %s\ndata: %s\n\n", id, event, b)
	s.flusher.Flush()
	return nil
}

// Data sends an unnamed event with data (JSON-encoded).
func (s *SSEStream) Data(data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("SSE encode error: %w", err)
	}
	fmt.Fprintf(s.writer, "data: %s\n\n", b)
	s.flusher.Flush()
	return nil
}

// Comment sends an SSE comment (keep-alive).
func (s *SSEStream) Comment(text string) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(s.writer, ": %s\n\n", text)
	s.flusher.Flush()
	return nil
}

// Retry sends a retry directive (reconnect interval in ms).
func (s *SSEStream) Retry(ms int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(s.writer, "retry: %d\n\n", ms)
	s.flusher.Flush()
	return nil
}

// Done returns a channel that closes when the client disconnects.
func (s *SSEStream) Done() <-chan struct{} {
	return s.ctx.Done()
}
