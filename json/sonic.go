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

// Marshal encodes v as JSON using Sonic (SIMD-accelerated).
func Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// MarshalToBuffer encodes v as JSON into the provided buffer using Sonic's
// streaming encoder. This avoids the intermediate []byte allocation that
// Marshal performs, enabling callers to reuse buffers via sync.Pool.
//
// ConfigDefault is the same configuration package-level sonic.Marshal uses, so
// the bytes produced here are identical to those from Marshal.
func MarshalToBuffer(buf *bytes.Buffer, v any) error {
	if err := sonic.ConfigDefault.NewEncoder(buf).Encode(v); err != nil {
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
	return sonic.Unmarshal(data, v)
}
