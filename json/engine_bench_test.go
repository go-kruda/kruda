package json

import (
	"bytes"
	"strconv"
	"testing"
)

// The CHANGELOG claims a 100-item payload for the MarshalToBuffer figures, so
// the benchmarks below use exactly that shape rather than a convenient one.
type benchItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
	Score  int    `json:"score"`
}

func benchPayload(n int) []benchItem {
	items := make([]benchItem, n)
	for i := range items {
		items[i] = benchItem{
			ID:     i,
			Name:   "user-" + strconv.Itoa(i),
			Email:  "user" + strconv.Itoa(i) + "@example.com",
			Active: i%2 == 0,
			Score:  i * 7,
		}
	}
	return items
}

var (
	benchLarge = benchPayload(100)
	benchSmall = benchPayload(1)
)

// BenchmarkMarshalToBufferLarge backs the "6967 B/op → 178 B/op, ~19% faster"
// claim: MarshalToBuffer used to call Marshal and copy the result, so it paid
// for an intermediate []byte the streaming encoder does not need.
func BenchmarkMarshalToBufferLarge(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if err := MarshalToBuffer(buf, benchLarge); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMarshalToBufferViaMarshal reproduces the implementation
// MarshalToBuffer had before it streamed: marshal into a fresh []byte, then copy
// that into the caller's buffer. This, not Marshal alone, is what the CHANGELOG
// figures compare against.
func BenchmarkMarshalToBufferViaMarshal(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		data, err := Marshal(benchLarge)
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(data)
	}
}

// BenchmarkMarshalLarge is the plain allocating encoder, for reference.
func BenchmarkMarshalLarge(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Marshal(benchLarge); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMarshalToBufferSmallViaMarshal is the small-payload counterpart.
func BenchmarkMarshalToBufferSmallViaMarshal(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		data, err := Marshal(benchSmall)
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(data)
	}
}

// BenchmarkMarshalToBufferSmall exists because the trade is not uniform: the
// handoff measured small payloads as slightly slower for fewer bytes.
func BenchmarkMarshalToBufferSmall(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if err := MarshalToBuffer(buf, benchSmall); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnmarshalLarge backs the "roughly 4–5× faster decode" claim for
// CGO_ENABLED=0 builds, which used to get encoding/json. Run it under both
// engines and compare:
//
//	go test -bench Unmarshal ./json/                        # Sonic
//	go test -tags kruda_stdjson -bench Unmarshal ./json/    # encoding/json
func BenchmarkUnmarshalLarge(b *testing.B) {
	data, err := Marshal(benchLarge)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		var out []benchItem
		if err := Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}
