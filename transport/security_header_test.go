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

func TestStaticResponseStatusLineUsesStandardName(t *testing.T) {
	for status, want := range map[int]string{
		202:  "HTTP/1.1 202 Accepted\r\n",
		413:  "HTTP/1.1 413 Content Too Large\r\n",
		429:  "HTTP/1.1 429 Too Many Requests\r\n",
		503:  "HTTP/1.1 503 Service Unavailable\r\n",
		599:  "HTTP/1.1 599 Unknown\r\n",
		0:    "HTTP/1.1 200 OK\r\n",
		103:  "HTTP/1.1 200 OK\r\n",
		1000: "HTTP/1.1 200 OK\r\n",
	} {
		if resp := GetStaticResponseString(status, "text/plain", "ok"); !bytes.HasPrefix(resp, []byte(want)) {
			t.Errorf("status %d: response starts %q, want %q", status, resp[:min(len(resp), len(want))], want)
		}
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

func TestStaticResponseOmitsBodyFor204And304(t *testing.T) {
	for _, status := range []int{204, 304} {
		resp := GetStaticResponseString(status, "text/plain", "body")
		if bytes.HasSuffix(resp, []byte("body")) || bytes.Contains(resp, []byte("Content-Length:")) {
			t.Errorf("status %d: response %q carries a body or Content-Length", status, resp)
		}
		if !bytes.HasSuffix(resp, []byte("\r\n\r\n")) {
			t.Errorf("status %d: response %q does not end its header block", status, resp)
		}
	}
}
