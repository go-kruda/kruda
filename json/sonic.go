//go:build !kruda_stdjson

// Package json provides a pluggable JSON engine for Kruda.
// This file uses github.com/bytedance/sonic for SIMD-accelerated JSON processing.
// It is compiled by default, and is selected regardless of whether CGO is
// enabled: Sonic is pure Go plus assembly and has no cgo dependency.
//
// Compiling this file is not the same as Sonic doing the work. Sonic carries its
// own build constraints — amd64/arm64, and only Go versions it has validated
// (v1.15.0 excludes go1.27 and newer) — and transparently routes its API to
// encoding/json outside them. This file stays correct everywhere as a result, but
// EncoderName below reports the tag's effect, not the outcome, so a build on an
// unsupported toolchain logs json=sonic while encoding/json runs.
package json

import (
	"bytes"

	"github.com/bytedance/sonic"
)

// EncoderName names the engine this build's tag selected. It is not necessarily
// the engine doing the work — see ActiveEngine.
const EncoderName = "sonic"

// EngineIsStdlib reports whether the standard library is doing the encoding
// work — true under the default tag exactly when sonic has routed its own API to
// encoding/json. A const, so the choice stays free on the hot path.
//
// This answers a question about cost, not about output. Anything asserting or
// depending on byte-level behaviour must key on EncoderName instead, which
// follows the build tag and therefore the frozen Config.
//
// Sonic's fallback honours EscapeHTML, so the HTML-escaping difference between
// the engines does not appear on one. It does not implement SortMapKeys or
// ValidateString — those agree only because encoding/json sorts and substitutes
// U+FFFD unconditionally — and its errors come from encoding/json. Output beyond
// escaping is therefore untested rather than known identical, and cannot be
// tested yet: sonic does not build on 32-bit at all, so no architecture reaches
// the fallback, and the Go-version route needs a go1.27 that does not exist.
const EngineIsStdlib = !sonicAccelerated

// ActiveEngine names what is actually encoding, for logs and for labelling
// benchmark output.
//
// A fallback build gets its own name rather than either plain answer. Calling it
// "encoding/json" would imply the kruda_stdjson build, whose HTML escaping
// differs; calling it "sonic" would imply the accelerated speed. Naming it as a
// fallback keeps a benchmark run from filing its numbers under an engine it did
// not use.
func ActiveEngine() string {
	if EngineIsStdlib {
		return "sonic (fallback: encoding/json)"
	}
	return "sonic"
}

// api is the Sonic configuration Kruda encodes and decodes with.
//
// SortMapKeys is enabled so that marshalling the same map twice always produces
// the same bytes. Sonic's own ConfigDefault leaves it off, which makes map
// output follow Go's randomized map iteration order: a handler returning a
// multi-key map emits different bytes on every request. That silently breaks
// anything deriving a value from response bytes — notably ETag generation, which
// hashes the body, so If-None-Match never matches and 304 is never returned.
// It also made map responses differ between this engine and the kruda_stdjson
// engine, which sorts map keys because encoding/json always does.
//
// Sorting costs nothing for structs, where field order is fixed at compile time,
// and Kruda's typed handlers and Resource CRUD return structs. Only map-valued
// responses pay for it.
//
// ValidateString is enabled so invalid UTF-8 in a string is replaced with U+FFFD
// rather than copied through. RFC 8259 section 8.1 requires JSON exchanged
// between systems to be UTF-8, and Sonic's default emitted the raw bytes, so a
// string holding invalid UTF-8 — anything read from a legacy encoding, a
// truncated multi-byte sequence, arbitrary binary put in a string field —
// produced a response that was not valid UTF-8 and could be rejected by a strict
// client or proxy. It costs about 8%, and makes this engine byte-identical to
// encoding/json for those inputs.
//
// EscapeHTML stays off, which is the one byte-level difference from the
// kruda_stdjson engine that remains: encoding/json emits < for '<' so its
// output is safe to paste directly inside an HTML <script> tag, and this engine
// emits the character. It costs about 41% on every response containing a string,
// which is a poor trade for the framework default — responses go out as
// application/json, which no browser executes, and the html/template renderer
// escapes JSON it embeds. An application that hand-embeds a response body into
// HTML can opt in with kruda.WithJSONEncoder.
// TestKnownCrossEngineDivergences pins the difference so a change is deliberate.
var api = sonic.Config{SortMapKeys: true, ValidateString: true}.Froze()

// Marshal encodes v as JSON using Sonic (SIMD-accelerated).
func Marshal(v any) ([]byte, error) {
	return api.Marshal(v)
}

// MarshalToBuffer encodes v as JSON into the provided buffer using Sonic's
// streaming encoder. This avoids the intermediate []byte allocation that
// Marshal performs, enabling callers to reuse buffers via sync.Pool.
//
// It encodes through the same configuration as Marshal, so the bytes produced
// here are identical to those from Marshal.
func MarshalToBuffer(buf *bytes.Buffer, v any) error {
	if err := api.NewEncoder(buf).Encode(v); err != nil {
		return err
	}
	// Encoder.Encode appends a trailing '\n' — trim it for clean JSON output.
	if b := buf.Bytes(); len(b) > 0 && b[len(b)-1] == '\n' {
		buf.Truncate(buf.Len() - 1)
	}
	return nil
}

// Unmarshal decodes JSON data into v using Sonic.
func Unmarshal(data []byte, v any) error {
	return api.Unmarshal(data, v)
}
