package kruda

import (
	"net/http/httptest"
	"testing"
)

func TestEncodeJSONReturnsOwnedBytes(t *testing.T) {
	encoded := []byte(`"owned"`)
	app := New(WithJSONEncoder(func(any) ([]byte, error) { return encoded, nil }))
	app.Get("/", func(c *Ctx) error {
		data, err := c.EncodeJSON(1)
		if err != nil {
			return err
		}
		encoded[1] = 'x'
		return c.SendBytes(data)
	})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Body.String() != `"owned"` {
		t.Fatalf("encoding aliases encoder buffer: %s", rec.Body.String())
	}
}
