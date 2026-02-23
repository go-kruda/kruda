package kruda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock flusher for SSE tests
// ---------------------------------------------------------------------------

type mockFlusherWriter struct {
	mockResponseWriter
	flushCount int
}

func (m *mockFlusherWriter) Flush() {
	m.flushCount++
}

func newSSECtx() (*Ctx, *mockFlusherWriter) {
	app := New()
	fw := &mockFlusherWriter{
		mockResponseWriter: *newMockResponse(),
	}
	c := newCtx(app)
	c.reset(fw, &mockRequest{method: "GET", path: "/events"})
	return c, fw
}

// ---------------------------------------------------------------------------
// Task 7.7: SSEStream.Event formatting
// ---------------------------------------------------------------------------

func TestSSEStream_Event(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	err := s.Event("ping", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "event: ping\ndata: \"hello\"\n\n"
	if got != want {
		t.Errorf("Event output = %q, want %q", got, want)
	}
	if flushCount != 1 {
		t.Errorf("flush count = %d, want 1", flushCount)
	}
}

func TestSSEStream_EventWithID(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	err := s.EventWithID("42", "ping", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "id: 42\nevent: ping\ndata: \"hello\"\n\n"
	if got != want {
		t.Errorf("EventWithID output = %q, want %q", got, want)
	}
	if flushCount != 1 {
		t.Errorf("flush count = %d, want 1", flushCount)
	}
}

func TestSSEStream_EventWithID_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     ctx,
	}

	if err := s.EventWithID("1", "test", "data"); err == nil {
		t.Error("EventWithID should return error when context cancelled")
	}
	if buf.Len() != 0 {
		t.Error("nothing should be written when context is cancelled")
	}
}

func TestSSEStream_Event_Object(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	data := map[string]string{"msg": "hi"}
	err := s.Event("message", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.HasPrefix(got, "event: message\ndata: ") {
		t.Errorf("Event output prefix wrong: %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("Event output should end with \\n\\n: %q", got)
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: SSEStream.Data formatting
// ---------------------------------------------------------------------------

func TestSSEStream_Data(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	err := s.Data(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "data: 42\n\n"
	if got != want {
		t.Errorf("Data output = %q, want %q", got, want)
	}
	if flushCount != 1 {
		t.Errorf("flush count = %d, want 1", flushCount)
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: SSEStream.Comment formatting
// ---------------------------------------------------------------------------

func TestSSEStream_Comment(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	err := s.Comment("keep-alive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := ": keep-alive\n\n"
	if got != want {
		t.Errorf("Comment output = %q, want %q", got, want)
	}
	if flushCount != 1 {
		t.Errorf("flush count = %d, want 1", flushCount)
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: SSEStream.Retry formatting
// ---------------------------------------------------------------------------

func TestSSEStream_Retry(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     context.Background(),
	}

	err := s.Retry(3000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "retry: 3000\n\n"
	if got != want {
		t.Errorf("Retry output = %q, want %q", got, want)
	}
	if flushCount != 1 {
		t.Errorf("flush count = %d, want 1", flushCount)
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: Context cancellation
// ---------------------------------------------------------------------------

func TestSSEStream_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var buf bytes.Buffer
	flushCount := 0
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  json.Marshal,
		ctx:     ctx,
	}

	if err := s.Event("test", "data"); err == nil {
		t.Error("Event should return error when context cancelled")
	}
	if err := s.Data("data"); err == nil {
		t.Error("Data should return error when context cancelled")
	}
	if err := s.Comment("ping"); err == nil {
		t.Error("Comment should return error when context cancelled")
	}
	if err := s.Retry(1000); err == nil {
		t.Error("Retry should return error when context cancelled")
	}
	if buf.Len() != 0 {
		t.Error("nothing should be written when context is cancelled")
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: Encode error
// ---------------------------------------------------------------------------

func TestSSEStream_EncodeError(t *testing.T) {
	var buf bytes.Buffer
	flushCount := 0
	badEncoder := func(v any) ([]byte, error) {
		return nil, errors.New("encode failed")
	}
	s := &SSEStream{
		writer:  &buf,
		flusher: &mockFlush{count: &flushCount},
		encode:  badEncoder,
		ctx:     context.Background(),
	}

	err := s.Event("test", "data")
	if err == nil {
		t.Fatal("expected encode error")
	}
	if !strings.Contains(err.Error(), "SSE encode error") {
		t.Errorf("error = %q, want to contain 'SSE encode error'", err.Error())
	}

	err = s.Data("data")
	if err == nil {
		t.Fatal("expected encode error for Data")
	}
}

// ---------------------------------------------------------------------------
// Task 7.7: c.SSE() integration — headers and flusher
// ---------------------------------------------------------------------------

func TestCtx_SSE_SetsHeaders(t *testing.T) {
	c, fw := newSSECtx()

	err := c.SSE(func(s *SSEStream) error {
		return s.Data("hello")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h := fw.Header()
	if ct := h.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := h.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if conn := h.Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
	if !c.Responded() {
		t.Error("Responded() should be true after SSE")
	}
}

func TestCtx_SSE_NoFlusher(t *testing.T) {
	app := New()
	// Use a plain mockResponseWriter that doesn't implement http.Flusher
	w := newMockResponse()
	c := newCtx(app)
	c.reset(w, &mockRequest{method: "GET", path: "/events"})

	err := c.SSE(func(s *SSEStream) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for non-flusher writer")
	}
	var ke *KrudaError
	if !errors.As(err, &ke) {
		t.Fatalf("expected *KrudaError, got %T", err)
	}
	if ke.Code != 500 {
		t.Errorf("error code = %d, want 500", ke.Code)
	}
}

func TestCtx_SSE_CallbackError(t *testing.T) {
	c, _ := newSSECtx()
	sentinel := errors.New("stream error")

	err := c.SSE(func(s *SSEStream) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel", err)
	}
}

func TestCtx_SSE_MultipleEvents(t *testing.T) {
	c, fw := newSSECtx()

	err := c.SSE(func(s *SSEStream) error {
		if err := s.Event("msg", "one"); err != nil {
			return err
		}
		if err := s.Data("two"); err != nil {
			return err
		}
		if err := s.Comment("ping"); err != nil {
			return err
		}
		return s.Retry(5000)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := string(fw.body)
	if !strings.Contains(body, "event: msg\ndata: \"one\"\n\n") {
		t.Errorf("missing Event output in body: %q", body)
	}
	if !strings.Contains(body, "data: \"two\"\n\n") {
		t.Errorf("missing Data output in body: %q", body)
	}
	if !strings.Contains(body, ": ping\n\n") {
		t.Errorf("missing Comment output in body: %q", body)
	}
	if !strings.Contains(body, "retry: 5000\n\n") {
		t.Errorf("missing Retry output in body: %q", body)
	}
	if fw.flushCount < 4 {
		t.Errorf("flush count = %d, want >= 4", fw.flushCount)
	}
}

// ---------------------------------------------------------------------------
// Helper: mockFlush for standalone SSEStream tests
// ---------------------------------------------------------------------------

type mockFlush struct {
	count *int
}

func (m *mockFlush) Flush() {
	*m.count++
}

// Ensure mockFlusherWriter implements http.Flusher
var _ http.Flusher = (*mockFlusherWriter)(nil)
