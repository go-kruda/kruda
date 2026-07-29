//go:build !kruda_stdjson

package json

import (
	"bytes"
	"testing"

	"github.com/bytedance/sonic"
)

// BenchmarkMarshalToBufferLegacy is the implementation MarshalToBuffer actually
// shipped with before it streamed, copied verbatim from json/sonic.go at
// 460ba2b^:
//
//	data, err := sonic.Marshal(v)
//	if err != nil { return err }
//	buf.Write(data)
//
// The distinction from BenchmarkMarshalToBufferViaMarshal matters. That one
// routes through this package's Marshal, which today means the frozen api
// config with SortMapKeys and ValidateString enabled — options the old code
// never paid for, and ValidateString alone costs about 8%. Benchmarking the old
// implementation against today's therefore needs sonic.Marshal directly, or the
// baseline is inflated and the speedup overstated.
//
// Keep both: this one answers "what did upgrading actually change for a caller",
// ViaMarshal answers "what did streaming alone buy, holding config equal".
func BenchmarkMarshalToBufferLegacy(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		data, err := sonic.Marshal(benchLarge)
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(data)
	}
}

// BenchmarkMarshalToBufferLegacySmall is the single-item counterpart.
func BenchmarkMarshalToBufferLegacySmall(b *testing.B) {
	buf := &bytes.Buffer{}
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		data, err := sonic.Marshal(benchSmall)
		if err != nil {
			b.Fatal(err)
		}
		buf.Write(data)
	}
}
