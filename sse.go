package kruda

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEStream writes Server-Sent Events to the client.
type SSEStream struct {
	writer  io.Writer
	flusher http.Flusher
	encode  func(v any) ([]byte, error)
	ctx     context.Context
}

// sanitizeSSE truncates at first newline to prevent SSE injection.
func sanitizeSSE(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// encodeData returns data as bytes. Strings and []byte are passed through;
// all other types are JSON-encoded. This prevents double-encoding when
// the caller already holds a JSON string.
func (s *SSEStream) encodeData(data any) ([]byte, error) {
	switch v := data.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		b, err := s.encode(data)
		if err != nil {
			return nil, fmt.Errorf("SSE encode error: %w", err)
		}
		return b, nil
	}
}

// formatDataField formats encoded bytes as one or more "data: " lines per SSE spec.
// Multiline data must have each line prefixed with "data: ".
func formatDataField(b []byte) []byte {
	s := string(b)
	if !strings.ContainsAny(s, "\r\n") {
		return b
	}
	// Normalize \r\n and lone \r to \n
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	var buf strings.Builder
	for i, line := range lines {
		if i > 0 {
			buf.WriteString("\ndata: ")
		}
		buf.WriteString(line)
	}
	return []byte(buf.String())
}

// Event sends a named event with data (JSON-encoded unless string or []byte).
func (s *SSEStream) Event(event string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encodeData(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", sanitizeSSE(event), formatDataField(b))
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// EventWithID sends a named event with an ID (for client reconnection).
func (s *SSEStream) EventWithID(id, event string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encodeData(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "id: %s\nevent: %s\ndata: %s\n\n", sanitizeSSE(id), sanitizeSSE(event), formatDataField(b))
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Data sends an unnamed event with data (JSON-encoded unless string or []byte).
func (s *SSEStream) Data(data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	b, err := s.encodeData(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "data: %s\n\n", formatDataField(b))
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Comment sends an SSE comment (keep-alive).
func (s *SSEStream) Comment(text string) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(s.writer, ": %s\n\n", sanitizeSSE(text))
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Retry sends a retry directive (reconnect interval in ms).
func (s *SSEStream) Retry(ms int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(s.writer, "retry: %d\n\n", ms)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Done returns a channel that closes when the client disconnects.
func (s *SSEStream) Done() <-chan struct{} {
	return s.ctx.Done()
}
