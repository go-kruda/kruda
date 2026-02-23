//go:build kruda_stdjson || !cgo

// Package json provides a pluggable JSON engine for Kruda.
// This file uses encoding/json from the Go standard library.
// It is compiled when the kruda_stdjson build tag is set or when CGO is disabled.
package json

import stdjson "encoding/json"

// Marshal encodes v as JSON using encoding/json.
func Marshal(v any) ([]byte, error) {
	return stdjson.Marshal(v)
}

// Unmarshal decodes JSON data into v using encoding/json.
func Unmarshal(data []byte, v any) error {
	return stdjson.Unmarshal(data, v)
}
