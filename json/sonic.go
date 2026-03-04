//go:build !kruda_stdjson && cgo

// Package json provides a pluggable JSON engine for Kruda.
// This file uses github.com/bytedance/sonic for SIMD-accelerated JSON processing.
// It is compiled by default when CGO is enabled and the kruda_stdjson tag is not set.
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
func MarshalToBuffer(buf *bytes.Buffer, v any) error {
	data, err := sonic.Marshal(v)
	if err != nil {
		return err
	}
	buf.Write(data)
	return nil
}

// Unmarshal decodes JSON data into v using Sonic.
func Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}
