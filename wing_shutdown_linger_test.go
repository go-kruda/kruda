//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

type readResult struct {
	n   int
	err error
}

// readBodyAfter reads one response once start closes, as a client that is
// slow to read: the response is still queued in the socket until then.
func readBodyAfter(br *bufio.Reader, start <-chan struct{}) <-chan readResult {
	got := make(chan readResult, 1)
	go func() {
		<-start
		time.Sleep(50 * time.Millisecond)
		res, err := http.ReadResponse(br, nil)
		if err != nil {
			got <- readResult{0, err}
			return
		}
		body, err := io.ReadAll(res.Body)
		got <- readResult{len(body), err}
	}()
	return got
}

// TestWingShutdownKeepsAnEventLoopLinger shuts down while the event loop
// lingers behind a final response. Input that arrives after Shutdown begins
// must still be dropped, or closing over it resets the response's unread tail.
func TestWingShutdownKeepsAnEventLoopLinger(t *testing.T) {
	big := strings.Repeat("l", 256<<10)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Connection", "close")
		_, _ = w.Write([]byte(big))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1}, handler)
	conn, br := framingDial(t, addr)
	defer conn.Close()
	if _, err := io.WriteString(conn, framingGet("/a")+"trailing"); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	got := readBodyAfter(br, start)
	time.Sleep(100 * time.Millisecond) // the response is written and the loop lingers
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	time.Sleep(50 * time.Millisecond)
	if _, err := io.WriteString(conn, "more input after Shutdown began"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(start)
	r := <-got
	<-stopped
	if r.err != nil || r.n != len(big) {
		t.Fatalf("response %d of %d bytes, %v; want it whole", r.n, len(big), r.err)
	}
}

// TestWingFirstTakeoverRejectionKeepsEarlierResponse pipelines an event-loop
// response and a Hijack request that is refused after Shutdown cut its write
// side. The refusal is the Takeover's first request, but the response before
// it is still queued: closing over later input would reset it.
func TestWingFirstTakeoverRejectionKeepsEarlierResponse(t *testing.T) {
	t.Setenv("KRUDA_WORKERS", "1")
	big := strings.Repeat("h", 256<<10)
	entered, release := make(chan struct{}), make(chan struct{})
	app := New(Wing())
	app.Get("/big", func(c *Ctx) error { return c.Text(big) }, Bolt)
	app.Get("/ws", func(c *Ctx) error {
		close(entered)
		<-release
		return c.Status(http.StatusBadRequest).Text("no upgrade")
	}, Hijack)
	app.Compile()
	addr, _ := startWingHijackServer(t, app)
	conn, br := framingDial(t, addr)
	defer conn.Close()
	if _, err := io.WriteString(conn, framingGet("/big")+framingGet("/ws")); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	got := readBodyAfter(br, start)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Hijack handler never ran")
	}
	if _, err := io.WriteString(conn, framingGet("/late")); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopped <- app.transport.Shutdown(ctx)
	}()
	time.Sleep(100 * time.Millisecond) // Shutdown has cut the write side
	close(release)
	time.Sleep(50 * time.Millisecond)
	close(start)
	r := <-got
	if err := <-stopped; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if r.err != nil || r.n != len(big) {
		t.Fatalf("response %d of %d bytes, %v; want it whole", r.n, len(big), r.err)
	}
}
