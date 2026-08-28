//go:build linux

package kruda

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"
)

func TestWingResumesReadinessSuppressedByPendingHandler(t *testing.T) {
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	suppressedRead := make(chan struct{}, 1)
	testSuppressedRecvHook = func(int32) {
		select {
		case suppressedRead <- struct{}{}:
		default:
		}
	}

	app := New(Wing())
	app.Get("/async", func(c *Ctx) error {
		close(handlerStarted)
		<-releaseHandler
		return c.Text("async")
	}, Arrow)
	app.Get("/next", func(c *Ctx) error { return c.Text("next") }, Bolt)
	addr, stop := startWingApp(t, app)
	defer func() {
		select {
		case <-releaseHandler:
		default:
			close(releaseHandler)
		}
		stop()
		testSuppressedRecvHook = nil
	}()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET /async HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("async handler did not start")
	}
	if _, err := io.WriteString(conn, "GET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-suppressedRead:
	case <-time.After(time.Second):
		t.Fatal("EPOLLIN was not observed while the handler was pending")
	}
	close(releaseHandler)

	reader := bufio.NewReader(conn)
	assertWingPipelineResponse(t, reader, "async")
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	assertWingPipelineResponse(t, reader, "next")
}
