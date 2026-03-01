package kruda

import (
	"bytes"
	"strconv"
	"testing"
	"testing/quick"
)

// Feature: techempower-domination
// Property 12: SetBody Lazy-Send Round-Trip
//
// For any byte slice, SetBody(data) + send() writes exact bytes to the
// response writer with correct Content-Length header.
//
// **Validates: Requirements 18.2, 13.4**

func TestPropertySetBodyLazySendRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 200}

	t.Run("ExactBytesWritten", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			resp := newMockResponse()
			c := newCtx(app)
			c.reset(resp, &mockRequest{method: "GET", path: "/"})

			c.SetBody(data)
			err := c.send()
			if err != nil {
				return false
			}

			// Verify written bytes match input exactly
			return bytes.Equal(resp.body, data)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("ContentLengthHeader", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			resp := newMockResponse()
			c := newCtx(app)
			c.reset(resp, &mockRequest{method: "GET", path: "/"})

			c.SetBody(data)
			err := c.send()
			if err != nil {
				return false
			}

			// Verify Content-Length header equals len(data)
			clHeader := resp.headers.Get("Content-Length")
			expected := strconv.Itoa(len(data))
			return clHeader == expected
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("NilBodyWritesNothing", func(t *testing.T) {
		app := New()
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, &mockRequest{method: "GET", path: "/"})

		// Don't call SetBody — body is nil
		err := c.send()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.body) != 0 {
			t.Errorf("expected empty body, got %d bytes", len(resp.body))
		}
	})

	t.Run("EmptySliceWritesZeroBytes", func(t *testing.T) {
		app := New()
		resp := newMockResponse()
		c := newCtx(app)
		c.reset(resp, &mockRequest{method: "GET", path: "/"})

		c.SetBody([]byte{})
		err := c.send()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.body) != 0 {
			t.Errorf("expected empty body, got %d bytes", len(resp.body))
		}
		cl := resp.headers.Get("Content-Length")
		if cl != "0" {
			t.Errorf("Content-Length = %q, want \"0\"", cl)
		}
	})

	t.Run("BodyClearedAfterSend", func(t *testing.T) {
		f := func(data []byte) bool {
			if len(data) == 0 {
				return true // skip empty — tested above
			}
			app := New()
			resp := newMockResponse()
			c := newCtx(app)
			c.reset(resp, &mockRequest{method: "GET", path: "/"})

			c.SetBody(data)
			_ = c.send()

			// After send(), c.body should be nil (cleared to prevent stale refs)
			return c.body == nil
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("RespondedFlagSet", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			resp := newMockResponse()
			c := newCtx(app)
			c.reset(resp, &mockRequest{method: "GET", path: "/"})

			c.SetBody(data)
			_ = c.send()

			return c.responded
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("DoubleSendReturnsError", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			resp := newMockResponse()
			c := newCtx(app)
			c.reset(resp, &mockRequest{method: "GET", path: "/"})

			c.SetBody(data)
			_ = c.send()

			// Second send should return ErrAlreadyResponded
			err := c.send()
			return err == ErrAlreadyResponded
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}
