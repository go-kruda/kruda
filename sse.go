package kruda

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEStream provides methods for writing Server-Sent Events.
// Created by Ctx.SSE() — do not construct directly.
type SSEStream struct {
	writer  io.Writer
	flusher http.Flusher
	encode  func(v any) ([]byte, error)
	ctx     context.Context
}

// Event writes a named SSE event with JSON-serialized data.
// Format: "event: {name}\ndata: {json}\n\n"
// Newlines and carriage returns in name are stripped to prevent SSE injection.
func (s *SSEStream) Event(name string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	jsonData, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("kruda: SSE encode error: %w", err)
	}
	_, err = fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", sanitizeSSEField(name), jsonData)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// EventWithID writes a named SSE event with an ID for reconnection support.
// The client stores the last event ID and sends it as Last-Event-ID on reconnect.
// Format: "id: {id}\nevent: {name}\ndata: {json}\n\n"
// Newlines and carriage returns in id and name are stripped to prevent SSE injection.
func (s *SSEStream) EventWithID(id, name string, data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	jsonData, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("kruda: SSE encode error: %w", err)
	}
	_, err = fmt.Fprintf(s.writer, "id: %s\nevent: %s\ndata: %s\n\n", sanitizeSSEField(id), sanitizeSSEField(name), jsonData)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Data writes an unnamed SSE event (data-only) with JSON-serialized data.
// Format: "data: {json}\n\n"
func (s *SSEStream) Data(data any) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	jsonData, err := s.encode(data)
	if err != nil {
		return fmt.Errorf("kruda: SSE encode error: %w", err)
	}
	_, err = fmt.Fprintf(s.writer, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// sanitizeSSEField strips newlines and carriage returns from SSE field values
// to prevent protocol injection. Only allocates if the string contains \n or \r.
func sanitizeSSEField(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	return strings.NewReplacer("\n", "", "\r", "").Replace(s)
}

// Comment writes an SSE comment line (prefixed with ':').
// Typically used as keep-alive pings.
func (s *SSEStream) Comment(text string) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(s.writer, ": %s\n\n", text)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Retry sets the client reconnection interval in milliseconds.
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
