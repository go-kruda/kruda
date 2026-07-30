package json

import (
	"bytes"
	"strings"
	"testing"
)

// parityValues covers the cases where a streaming encoder is most likely to
// diverge from the one-shot Marshal path: HTML-sensitive runes, unicode, empty
// containers, and nested structures.
//
// Multi-key maps are deliberately excluded. The sonic engine uses
// sonic.ConfigDefault, which leaves SortMapKeys off, so two marshals of the
// same multi-key map may emit keys in different orders — Marshal and
// MarshalToBuffer would differ by iteration order rather than by encoder
// behavior. Single-key maps have only one possible ordering and stay useful
// here; multi-key maps are covered semantically by
// TestMarshalToBufferMultiKeyMapSemanticParity.
var parityValues = []struct {
	name string
	val  any
}{
	{"string", "hello"},
	{"html", "<script>&\"'"},
	{"unicode", "สวัสดี 🦅"},
	{"int", 42},
	{"float", 98.6},
	{"bool", true},
	{"nil", nil},
	{"emptySlice", []int{}},
	{"emptyMap", map[string]int{}},
	{"slice", []int{1, 2, 3}},
	{"singleKeyMap", map[string]int{"a": 1}},
	{"struct", struct {
		Name string `json:"name"`
		Tags []string
		Nest struct{ N int }
	}{Name: "<widget>", Tags: []string{"x", "y"}}},
	{"nestedSingleKeyMap", map[string]any{"k": []any{1, "two", nil, map[string]int{"z": 3}}}},
}

// TestMarshalToBufferMatchesMarshal pins the invariant that makes the sonic and
// encoding/json engines interchangeable: MarshalToBuffer must produce exactly
// the bytes Marshal produces, with no trailing newline from the streaming
// encoder. Without this, swapping engines could silently change response bytes.
func TestMarshalToBufferMatchesMarshal(t *testing.T) {
	for _, tc := range parityValues {
		t.Run(tc.name, func(t *testing.T) {
			want, err := Marshal(tc.val)
			if err != nil {
				t.Fatalf("Marshal(%v) failed: %v", tc.val, err)
			}
			buf := &bytes.Buffer{}
			if err := MarshalToBuffer(buf, tc.val); err != nil {
				t.Fatalf("MarshalToBuffer(%v) failed: %v", tc.val, err)
			}
			if got := buf.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("engine %s: MarshalToBuffer = %q, Marshal = %q", ActiveEngine(), got, want)
			}
			if bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
				t.Errorf("engine %s: trailing newline not trimmed: %q", ActiveEngine(), buf.Bytes())
			}
		})
	}
}

// TestMarshalToBufferMultiKeyMapSemanticParity covers multi-key maps, where
// byte comparison is not meaningful on an engine that does not sort map keys.
// The guarantee that still must hold is semantic: the buffered output decodes
// to the same value as the one-shot Marshal output.
func TestMarshalToBufferMultiKeyMapSemanticParity(t *testing.T) {
	in := map[string]int{"zebra": 1, "apple": 2, "mango": 3, "kiwi": 4}

	want, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := MarshalToBuffer(buf, in); err != nil {
		t.Fatalf("MarshalToBuffer failed: %v", err)
	}

	var fromMarshal, fromBuffer map[string]int
	if err := Unmarshal(want, &fromMarshal); err != nil {
		t.Fatalf("Unmarshal of Marshal output failed: %v", err)
	}
	if err := Unmarshal(buf.Bytes(), &fromBuffer); err != nil {
		t.Fatalf("Unmarshal of MarshalToBuffer output failed: %v (bytes: %q)", err, buf.Bytes())
	}

	if len(fromBuffer) != len(in) {
		t.Fatalf("engine %s: got %d keys, want %d", ActiveEngine(), len(fromBuffer), len(in))
	}
	for k, v := range fromMarshal {
		if fromBuffer[k] != v {
			t.Errorf("engine %s: key %q = %d, want %d", ActiveEngine(), k, fromBuffer[k], v)
		}
	}
	if bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Errorf("engine %s: trailing newline not trimmed: %q", ActiveEngine(), buf.Bytes())
	}
}

// TestMarshalToBufferAppendsToExistingContent guards the sync.Pool reuse path:
// MarshalToBuffer must append without disturbing bytes already in the buffer,
// and must only trim the newline it added itself.
func TestMarshalToBufferAppendsToExistingContent(t *testing.T) {
	for _, prefix := range []string{"", "prefix:", "ends-in-newline\n"} {
		buf := &bytes.Buffer{}
		buf.WriteString(prefix)
		if err := MarshalToBuffer(buf, map[string]int{"a": 1}); err != nil {
			t.Fatalf("MarshalToBuffer failed: %v", err)
		}
		want, err := Marshal(map[string]int{"a": 1})
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if got := buf.String(); got != prefix+string(want) {
			t.Errorf("engine %s: prefix %q → got %q, want %q", ActiveEngine(), prefix, got, prefix+string(want))
		}
	}
}

// TestMarshalToBufferRoundTrips confirms buffered output is still valid JSON
// that decodes back to an equal value on the active engine.
func TestMarshalToBufferRoundTrips(t *testing.T) {
	type payload struct {
		Name  string  `json:"name"`
		Age   int     `json:"age"`
		Score float64 `json:"score"`
		Tags  []string
	}
	in := payload{Name: "<Aom> & co", Age: 30, Score: 98.6, Tags: []string{"a", "b"}}

	buf := &bytes.Buffer{}
	if err := MarshalToBuffer(buf, in); err != nil {
		t.Fatalf("MarshalToBuffer failed: %v", err)
	}
	var out payload
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal of buffered output failed: %v (bytes: %q)", err, buf.Bytes())
	}
	if out.Name != in.Name || out.Age != in.Age || out.Score != in.Score || strings.Join(out.Tags, ",") != strings.Join(in.Tags, ",") {
		t.Errorf("engine %s: round trip mismatch: got %+v, want %+v", ActiveEngine(), out, in)
	}
}

// TestMarshalToBufferErrorLeavesBufferUsable checks that a failed encode is
// reported and does not leave a partial value that a pooled buffer would carry
// into the next response.
func TestMarshalToBufferErrorLeavesBufferUsable(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := MarshalToBuffer(buf, make(chan int)); err == nil {
		t.Fatalf("engine %s: expected error for unsupported type", ActiveEngine())
	}
	buf.Reset()
	if err := MarshalToBuffer(buf, "ok"); err != nil {
		t.Fatalf("MarshalToBuffer after error failed: %v", err)
	}
	if got := buf.String(); got != `"ok"` {
		t.Errorf("engine %s: got %q, want %q", ActiveEngine(), got, `"ok"`)
	}
}
