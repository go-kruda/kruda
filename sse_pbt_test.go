package kruda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/quick"
)

// Property: SSE Event Formatting
//
// For any event name (non-empty string) and any JSON-serializable data value,
// Event(name, data) should produce output matching "event: {name}\ndata: {json}\n\n".
// For any JSON-serializable data value, Data(data) should produce "data: {json}\n\n".
// For any comment text, Comment(text) should produce ": {text}\n\n".
// For any positive integer ms, Retry(ms) should produce "retry: {ms}\n\n".

func TestPropertySSEEventFormatting(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	// Sub-property: Event formatting
	t.Run("Event", func(t *testing.T) {
		f := func(name, data string) bool {
			if name == "" {
				return true // skip empty names
			}
			// Filter out strings with control chars that break formatting
			if strings.ContainsAny(name, "\n\r") || strings.ContainsAny(data, "\n\r") {
				return true
			}

			var buf bytes.Buffer
			flushCount := 0
			s := &SSEStream{
				writer:  &buf,
				flusher: &mockFlush{count: &flushCount},
				encode:  json.Marshal,
				ctx:     context.Background(),
			}

			err := s.Event(name, data)
			if err != nil {
				return false
			}

			got := buf.String()
			jsonData, _ := json.Marshal(data)
			want := fmt.Sprintf("event: %s\ndata: %s\n\n", name, jsonData)
			return got == want && flushCount == 1
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	// Sub-property: Data formatting
	t.Run("Data", func(t *testing.T) {
		f := func(n int) bool {
			var buf bytes.Buffer
			flushCount := 0
			s := &SSEStream{
				writer:  &buf,
				flusher: &mockFlush{count: &flushCount},
				encode:  json.Marshal,
				ctx:     context.Background(),
			}

			err := s.Data(n)
			if err != nil {
				return false
			}

			got := buf.String()
			jsonData, _ := json.Marshal(n)
			want := fmt.Sprintf("data: %s\n\n", jsonData)
			return got == want && flushCount == 1
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	// Sub-property: Comment formatting
	t.Run("Comment", func(t *testing.T) {
		f := func(text string) bool {
			if strings.ContainsAny(text, "\n\r") {
				return true // skip multiline
			}

			var buf bytes.Buffer
			flushCount := 0
			s := &SSEStream{
				writer:  &buf,
				flusher: &mockFlush{count: &flushCount},
				encode:  json.Marshal,
				ctx:     context.Background(),
			}

			err := s.Comment(text)
			if err != nil {
				return false
			}

			got := buf.String()
			want := fmt.Sprintf(": %s\n\n", text)
			return got == want && flushCount == 1
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	// Sub-property: Retry formatting
	t.Run("Retry", func(t *testing.T) {
		f := func(ms uint16) bool {
			n := int(ms)
			var buf bytes.Buffer
			flushCount := 0
			s := &SSEStream{
				writer:  &buf,
				flusher: &mockFlush{count: &flushCount},
				encode:  json.Marshal,
				ctx:     context.Background(),
			}

			err := s.Retry(n)
			if err != nil {
				return false
			}

			got := buf.String()
			want := fmt.Sprintf("retry: %d\n\n", n)
			return got == want && flushCount == 1
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}
