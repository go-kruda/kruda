package kruda

import (
	"encoding/json"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 18: TestClient Request Round-Trip
// For random ASCII header values and body, the handler receives the correct
// method, path, header value, and body content.
// ---------------------------------------------------------------------------

func TestPropertyTestClientRequestRoundTrip(t *testing.T) {
	f := func(headerVal string) bool {
		// Filter to printable ASCII, len 1-50
		if len(headerVal) == 0 || len(headerVal) > 50 {
			return true // skip
		}
		for _, c := range headerVal {
			if c < 32 || c > 126 {
				return true // skip non-printable
			}
		}

		type roundTripResult struct {
			Method    string `json:"method"`
			Path      string `json:"path"`
			HeaderVal string `json:"header_val"`
			Body      string `json:"body"`
		}

		app := New()
		app.Post("/test", func(c *Ctx) error {
			return c.JSON(roundTripResult{
				Method:    c.Method(),
				Path:      c.Path(),
				HeaderVal: c.Header("X-Test"),
				Body:      c.BodyString(),
			})
		})
		app.Compile()

		bodyPayload := map[string]string{"data": headerVal}
		bodyBytes, err := json.Marshal(bodyPayload)
		if err != nil {
			return false
		}

		tc := NewTestClient(app)
		resp, err := tc.Request("POST", "/test").
			Header("X-Test", headerVal).
			Body(bodyPayload).
			Send()
		if err != nil {
			return false
		}
		if resp.StatusCode() != 200 {
			return false
		}

		var result roundTripResult
		if err := resp.JSON(&result); err != nil {
			return false
		}

		return result.Method == "POST" &&
			result.Path == "/test" &&
			result.HeaderVal == headerVal &&
			result.Body == string(bodyBytes)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 19: TestClient Query Delivery
// For random alphanumeric query key-value pairs, c.Query(key) returns the
// correct value.
// ---------------------------------------------------------------------------

func TestPropertyTestClientQueryDelivery(t *testing.T) {
	f := func(key, value string) bool {
		// Filter to alphanumeric only, len 1-20
		if !isAlphanumRange(key, 1, 20) || !isAlphanumRange(value, 1, 20) {
			return true // skip
		}

		app := New()
		// Capture the key in the closure for the handler
		capturedKey := key
		app.Get("/q", func(c *Ctx) error {
			return c.Text(c.Query(capturedKey))
		})
		app.Compile()

		tc := NewTestClient(app)
		resp, err := tc.Request("GET", "/q").
			Query(key, value).
			Send()
		if err != nil {
			return false
		}
		if resp.StatusCode() != 200 {
			return false
		}

		return resp.BodyString() == value
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 20: TestClient Raw Body Passthrough
// For random byte slices, the handler receives the exact bytes without
// JSON marshaling.
// ---------------------------------------------------------------------------

func TestPropertyTestClientRawBodyPassthrough(t *testing.T) {
	f := func(data []byte) bool {
		// Filter len 1-200
		if len(data) == 0 || len(data) > 200 {
			return true // skip
		}

		app := New()
		app.Post("/raw", func(c *Ctx) error {
			return c.Text(c.BodyString())
		})
		app.Compile()

		tc := NewTestClient(app)
		resp, err := tc.Request("POST", "/raw").
			Body(data).
			Send()
		if err != nil {
			return false
		}
		if resp.StatusCode() != 200 {
			return false
		}

		return resp.BodyString() == string(data)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// isAlphanumRange checks that s contains only [a-zA-Z0-9] and has length in [minLen, maxLen].
func isAlphanumRange(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
