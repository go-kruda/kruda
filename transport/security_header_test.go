package transport

import (
	"bytes"
	"testing"
)

func TestStaticResponseDoesNotEmitServerHeader(t *testing.T) {
	body := "ok-" + t.Name()
	resp := GetStaticResponseString(200, "text/plain", body)

	if bytes.Contains(resp, []byte("\r\nServer:")) {
		t.Fatal("static response should not emit Server header")
	}
}

func TestStaticResponseCacheKeyIncludesStatus(t *testing.T) {
	staticCache.Clear()

	body := "same-body-" + t.Name()
	ok := GetStaticResponseString(200, "text/plain", body)
	notFound := GetStaticResponseString(404, "text/plain", body)

	if bytes.Equal(ok, notFound) {
		t.Fatal("static response cache must not reuse responses across status codes")
	}
	if !bytes.HasPrefix(ok, []byte("HTTP/1.1 200 OK\r\n")) {
		t.Fatalf("200 static response has wrong status line: %q", ok)
	}
	if !bytes.HasPrefix(notFound, []byte("HTTP/1.1 404 Not Found\r\n")) {
		t.Fatalf("404 static response has wrong status line: %q", notFound)
	}
}

func BenchmarkGetStaticResponseStringCached(b *testing.B) {
	staticCache.Clear()
	want := GetStaticResponseString(200, "text/plain", "ok")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := GetStaticResponseString(200, "text/plain", "ok")
		if len(got) != len(want) {
			b.Fatalf("len = %d, want %d", len(got), len(want))
		}
	}
}
