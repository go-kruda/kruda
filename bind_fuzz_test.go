package kruda

import (
	"testing"
)

// fuzzBindTarget is a representative struct used by FuzzBindJSON. It mixes
// scalar types, a slice, and a nested struct to cover several JSON decoder
// code paths.
type fuzzBindTarget struct {
	Name  string         `json:"name"`
	Email string         `json:"email"`
	Age   int            `json:"age"`
	Tags  []string       `json:"tags"`
	Meta  map[string]int `json:"meta"`
	Inner struct {
		ID   int64   `json:"id"`
		Note string  `json:"note"`
		Pi   float64 `json:"pi"`
	} `json:"inner"`
}

// FuzzBindJSON checks that Ctx.Bind never panics on arbitrary request
// bodies — malformed JSON, deeply nested arrays, integer overflow, NaN/Inf,
// trailing garbage, etc. Errors are expected; panics are bugs.
//
// Uses the same Ctx-construction pattern as bindCtx() in bind_test.go to
// keep this fuzz target close to real production behavior.
func FuzzBindJSON(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"name":"a","email":"a@b.com","age":1}`),
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(`{"name":"","email":"x"}`),
		[]byte(`{"age":"not-a-number"}`),
		[]byte(`[]`),
		[]byte(``),
		[]byte(`{"tags":["a","b"]}`),
		[]byte(`{"meta":{"x":1,"y":2}}`),
		[]byte(`{"inner":{"id":9223372036854775807,"pi":3.14}}`),
		[]byte(`{"age":99999999999999999999}`),
		[]byte(`{"name":"\u0000\uffff"}`),
		// Deeply nested arrays — stress the decoder's recursion handling.
		[]byte(`[[[[[[[[[[]]]]]]]]]]`),
		// Trailing garbage after a valid value.
		[]byte(`{"age":1}garbage`),
		// Unterminated string.
		[]byte(`{"name":"unterminated`),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Bind panicked on body (len=%d) %q: %v", len(body), body, r)
			}
		}()

		// Construct a Ctx the same way bindCtx does in bind_test.go — this
		// exercises the real production path: BodyBytes() → JSONDecoder().
		c := bindCtx("POST", "/", nil, nil, body)

		var v fuzzBindTarget
		_ = c.Bind(&v) // err is fine; panic is the bug
	})
}
