//go:build !kruda_stdjson && cgo

// Package json provides a pluggable JSON engine for Kruda.
// This file uses github.com/bytedance/sonic for SIMD-accelerated JSON processing.
// It is compiled by default when CGO is enabled and the kruda_stdjson tag is not set.
package json

import "github.com/bytedance/sonic"

// Marshal encodes v as JSON using Sonic (SIMD-accelerated).
func Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal decodes JSON data into v using Sonic.
func Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}
