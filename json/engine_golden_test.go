package json

import (
	"bytes"
	"testing"
)

// The sonic and encoding/json engines are selected by build tag, so no single
// test binary can run both. These tests instead pin exact bytes: each engine
// build asserts against the same golden values, so an engine whose output
// drifts from the other fails here rather than silently changing responses for
// whichever build a user happens to compile.

// goldenCases must encode identically on every engine. Multi-key maps belong
// here only because both engines now sort map keys.
var goldenCases = []struct {
	name string
	val  any
	want string
}{
	{"string", "hello", `"hello"`},
	{"emptyString", "", `""`},
	{"int", 42, `42`},
	{"negativeInt", -7, `-7`},
	{"int64Max", int64(9223372036854775807), `9223372036854775807`},
	{"float", 98.6, `98.6`},
	{"floatIntegral", 3.0, `3`},
	{"bigFloat", 1e21, `1e+21`},
	{"smallFloat", 0.000001, `0.000001`},
	{"true", true, `true`},
	{"false", false, `false`},
	{"nil", nil, `null`},
	{"nilSlice", []int(nil), `null`},
	{"nilMap", map[string]int(nil), `null`},
	{"emptySlice", []int{}, `[]`},
	{"emptyMap", map[string]int{}, `{}`},
	{"emptyStruct", struct{}{}, `{}`},
	{"slice", []int{1, 2, 3}, `[1,2,3]`},
	{"unicode", "สวัสดี 🦅", `"สวัสดี 🦅"`},
	{"escapes", "tab\there\nnewline\"quote\\slash", `"tab\there\nnewline\"quote\\slash"`},
	{"invalidUTF8", string([]byte{0xff, 0xfe}), `"\ufffd\ufffd"`},
	{"singleKeyMap", map[string]int{"a": 1}, `{"a":1}`},
	{"multiKeyMapSorted", map[string]int{"zebra": 1, "apple": 2, "mango": 3},
		`{"apple":2,"mango":3,"zebra":1}`},
	{"nestedMapSorted", map[string]any{"b": map[string]int{"y": 2, "x": 1}, "a": 1},
		`{"a":1,"b":{"x":1,"y":2}}`},
	{"structFieldOrder", struct {
		Zebra string `json:"zebra"`
		Apple string `json:"apple"`
	}{"z", "a"}, `{"zebra":"z","apple":"a"}`},
	{"structOmitempty", struct {
		Name string `json:"name"`
		Skip string `json:"skip,omitempty"`
	}{Name: "n"}, `{"name":"n"}`},
}

// TestGoldenBytesMatchAcrossEngines is the cross-engine regression guard: both
// engine builds must produce these exact bytes, via Marshal and via
// MarshalToBuffer.
func TestGoldenBytesMatchAcrossEngines(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Marshal(tc.val)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("engine %s: Marshal = %s, want %s", ActiveEngine(), got, tc.want)
			}

			buf := &bytes.Buffer{}
			if err := MarshalToBuffer(buf, tc.val); err != nil {
				t.Fatalf("MarshalToBuffer failed: %v", err)
			}
			if buf.String() != tc.want {
				t.Errorf("engine %s: MarshalToBuffer = %s, want %s", ActiveEngine(), buf.String(), tc.want)
			}
		})
	}
}

// TestMapMarshalIsDeterministic is the direct regression guard for the ETag
// breakage that unsorted map keys caused: repeated marshals of one multi-key map
// must produce one byte sequence, or anything hashing the response body (ETag,
// response caching, snapshot tests) sees a different value on every request.
func TestMapMarshalIsDeterministic(t *testing.T) {
	m := map[string]any{"status": "ok", "count": 3, "name": "aom", "ver": 2, "zz": nil}

	first, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	for i := 0; i < 500; i++ {
		got, err := Marshal(m)
		if err != nil {
			t.Fatalf("Marshal failed on iteration %d: %v", i, err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("engine %s: marshal %d = %s, first = %s (map key order is not deterministic)",
				ActiveEngine(), i, got, first)
		}

		buf := &bytes.Buffer{}
		if err := MarshalToBuffer(buf, m); err != nil {
			t.Fatalf("MarshalToBuffer failed on iteration %d: %v", i, err)
		}
		if !bytes.Equal(buf.Bytes(), first) {
			t.Fatalf("engine %s: MarshalToBuffer %d = %s, Marshal = %s",
				ActiveEngine(), i, buf.Bytes(), first)
		}
	}
}

// TestKnownCrossEngineDivergences pins the one byte-level difference that
// remains between engines, so it cannot drift unnoticed and closing it is a
// deliberate, reviewed change rather than an accident.
//
// encoding/json escapes <, > and & so its output is safe to paste directly
// inside an HTML <script> tag. The sonic engine leaves EscapeHTML off because it
// costs about 41% on every response containing a string, which is a poor default
// when responses go out as application/json — no browser executes those — and
// the html/template renderer escapes JSON it embeds. An application that
// hand-embeds a response body into HTML can opt in with kruda.WithJSONEncoder.
//
// Invalid UTF-8 used to differ here too. The sonic engine now enables
// ValidateString, so both engines substitute U+FFFD and that case has moved to
// goldenCases.
func TestKnownCrossEngineDivergences(t *testing.T) {
	cases := []struct {
		name      string
		val       any
		wantSonic string
		wantStd   string
	}{
		{"htmlTags", "<script>",
			`"<script>"`, `"\u003cscript\u003e"`},
		{"ampersand", "a&b",
			`"a&b"`, `"a\u0026b"`},
		{"htmlInMap", map[string]string{"k": "<b>"},
			`{"k":"<b>"}`, `{"k":"\u003cb\u003e"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// EncoderName, deliberately, not EngineIsStdlib. This table is about
			// bytes, and bytes follow the config, which follows the tag: when
			// sonic falls back its compat layer still honours the Config it was
			// frozen with — compat.go calls SetEscapeHTML(cfg.EscapeHTML) — so a
			// fallback build emits sonic's unescaped output, not the standard
			// library's escaped default. Only speed changes on fallback, not
			// bytes. EngineIsStdlib answers "is the fast implementation present",
			// which is a different question and the wrong one here.
			want := tc.wantStd
			if EncoderName == "sonic" {
				want = tc.wantSonic
			}
			got, err := Marshal(tc.val)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(got) != want {
				t.Errorf("engine %s: Marshal = %q, want %q\n"+
					"If this engine's escaping behavior changed on purpose, update this "+
					"table and the sonic.Config comment in json/sonic.go.",
					ActiveEngine(), got, want)
			}
		})
	}
}
