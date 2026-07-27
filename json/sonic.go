//go:build !kruda_stdjson

// Package json provides a pluggable JSON engine for Kruda.
// This file uses github.com/bytedance/sonic for SIMD-accelerated JSON processing.
// It is compiled by default, and is selected regardless of whether CGO is
// enabled: Sonic is pure Go plus assembly and has no cgo dependency. On
// platforms or Go versions Sonic does not accelerate (anything other than
// amd64/arm64), Sonic's own build constraints transparently route its API to
// encoding/json, so this file stays correct everywhere.
package json

import (
	"bytes"

	"github.com/bytedance/sonic"
)

// EncoderName identifies the active JSON encoder for diagnostics.
const EncoderName = "sonic"

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
// Two byte-level differences from the kruda_stdjson engine remain by design,
// both because closing them would cost roughly 48% on every response containing
// a string: this engine does not escape HTML characters (encoding/json emits
// < for '<'), and it passes invalid UTF-8 through rather than substituting
// U+FFFD. TestKnownCrossEngineDivergences pins both so a change is deliberate.
var api = sonic.Config{SortMapKeys: true}.Froze()

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
