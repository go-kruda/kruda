package kruda

import (
	"bytes"
	"testing"
)

// TestPutJSONBufDropsLargeBuffers returns a buffer grown past the pool cutoff.
// The pool must not hand it out again, or one large response would keep its
// buffer alive for every later small one.
func TestPutJSONBufDropsLargeBuffers(t *testing.T) {
	big := bytes.NewBuffer(make([]byte, 0, maxPooledJSONBuf+1))
	putJSONBuf(big)
	got := jsonBufPool.Get().(*bytes.Buffer)
	defer putJSONBuf(got)
	if got == big || got.Cap() > maxPooledJSONBuf {
		t.Fatalf("pool returned a %d-byte buffer, cutoff is %d", got.Cap(), maxPooledJSONBuf)
	}
}
