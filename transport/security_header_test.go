package transport

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// TestNetHTTPRawHeadersCannotAddFields checks that net/http itself drops
// invalid raw header names and cleans values, so the net/http transport needs
// no filter of its own.
func TestNetHTTPRawHeadersCannotAddFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		h := (&netHTTPResponseWriter{w: rw}).Header()
		h.Set("X-Value", "v\r\nX-Injected: 1")
		h.Add("X-Bad\r\nX-Injected", "2")
		_, _ = rw.Write([]byte("body"))
	}))
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(conn)
	if bytes.Contains(out, []byte("\r\nX-Injected")) {
		t.Fatalf("response %q", out)
	}
}
