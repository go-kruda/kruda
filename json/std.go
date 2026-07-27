//go:build kruda_stdjson

// Package json provides a pluggable JSON engine for Kruda.
// This file uses encoding/json from the Go standard library.
// It is compiled only when the kruda_stdjson build tag is set. Prefer this
// engine for workloads that spawn a process per request or scale to zero:
// Sonic's JIT warm-up dominates cold-start time there.
package json

import (
	"bytes"
	stdjson "encoding/json"
)

// EncoderName identifies the active JSON encoder for diagnostics.
const EncoderName = "encoding/json"

// Marshal encodes v as JSON using encoding/json.
func Marshal(v any) ([]byte, error) {
	return stdjson.Marshal(v)
}

// MarshalToBuffer encodes v as JSON into the provided buffer using
// encoding/json's streaming encoder. This avoids the intermediate []byte
// allocation that Marshal performs, enabling callers to reuse buffers via sync.Pool.
func MarshalToBuffer(buf *bytes.Buffer, v any) error {
	if err := stdjson.NewEncoder(buf).Encode(v); err != nil {
		return err
	}
	// Encoder.Encode appends a trailing '\n' — trim it for clean JSON output.
	if b := buf.Bytes(); len(b) > 0 && b[len(b)-1] == '\n' {
		buf.Truncate(buf.Len() - 1)
	}
	return nil
}

// Unmarshal decodes JSON data into v using encoding/json.
func Unmarshal(data []byte, v any) error {
	return stdjson.Unmarshal(data, v)
}
