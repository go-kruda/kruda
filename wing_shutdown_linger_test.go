//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
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

// TestWingShutdownKeepsAnEventLoopLinger runs Shutdown's linger pass over a
// connection lingering behind its final response. It must keep dropping input
// until the peer closes, not close at once: closing over input that arrives
// can reset the response's unread tail.
func TestWingShutdownKeepsAnEventLoopLinger(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	w, _ := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.lingerUntil = monoNow() + int64(5*time.Second)
	w.conns[c.fd] = c
	w.lingering++
	done := make(chan struct{})
	go func() { w.lingerAtShutdown(true); close(done) }()
	late := []byte("input after Shutdown began")
	time.Sleep(20 * time.Millisecond)
	if _, err := syscall.Write(fds[1], late); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("closed while the peer was still sending")
	default:
	}
	if err := syscall.Shutdown(fds[1], syscall.SHUT_WR); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("still lingering after the peer closed")
	}
	if c.discarded != len(late) || w.conns[c.fd] != nil {
		t.Fatalf("discarded %d of %d, still open %v", c.discarded, len(late), w.conns[c.fd] != nil)
	}
}

// TestWingFinalCloseCountsBufferedInput ends a connection whose trailing input
// was read along with its request. That input is unread by any handler too, so
// the close must linger rather than reset the response.
func TestWingFinalCloseCountsBufferedInput(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	w, _ := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.readBuf = make([]byte, 64)
	c.readN = copy(c.readBuf, "trailing")
	w.conns[c.fd] = c
	w.closeAfterFinal(c)
	if c.lingerUntil == 0 || w.conns[c.fd] != c {
		t.Fatalf("closed at once over %d buffered bytes", len("trailing"))
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

// TestWingContinueCountsAsAResponse begins an upload that asks for 100
// Continue. Once that is written, a response is in the socket: a Takeover that
// starts with this request must still drain behind it.
func TestWingContinueCountsAsAResponse(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	w, _ := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.readN = copy(c.readBuf, "POST /up HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\nExpect: 100-continue\r\n\r\n")
	w.conns[c.fd] = c
	if !w.beginBodyAccum(c, 5, true) {
		t.Fatal("body not accepted")
	}
	got := make([]byte, 64)
	n, _ := syscall.Read(fds[1], got)
	if string(got[:n]) != string(wing100Continue) {
		t.Fatalf("peer read %q, want 100 Continue", got[:n])
	}
	if c.responses != 1 {
		t.Fatalf("responses %d after 100 Continue, want 1", c.responses)
	}
}

// finalClosePair is a worker and a connection over a socketpair, with the
// peer end non-blocking so the test can see what it has been sent.
func finalClosePair(t *testing.T) (*worker, *conn, int) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Close(fds[0]); syscall.Close(fds[1]) })
	for _, fd := range fds {
		if err := syscall.SetNonblock(fd, true); err != nil {
			t.Fatal(err)
		}
	}
	w, _ := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	w.conns[c.fd] = c
	return w, c, fds[1]
}

// TestWingFinalCloseLingersOnAnEmptySocket ends a connection with nothing
// unread yet. A request can still land before the close and reset the
// response, so the close must end the write side and linger, not close.
func TestWingFinalCloseLingersOnAnEmptySocket(t *testing.T) {
	w, c, peer := finalClosePair(t)
	w.closeAfterFinal(c)
	if c.lingerUntil == 0 || w.conns[c.fd] != c {
		t.Fatal("closed at once with nothing unread yet")
	}
	if n, err := syscall.Read(peer, make([]byte, 8)); n != 0 || err != nil {
		t.Fatalf("peer read %d, %v; want the EOF of the write side", n, err)
	}
}

// TestWingFinalCloseHoldsPastTheCap ends a connection whose peer sends past
// maxDiscardInput. The write side ends first, and the connection reads no more
// but still lingers: closing over the rest would reset the response.
func TestWingFinalCloseHoldsPastTheCap(t *testing.T) {
	w, c, peer := finalClosePair(t)
	c.discarded = maxDiscardInput - 4
	if _, err := syscall.Write(peer, make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	eng := w.eng.(*mockEngine)
	w.closeAfterFinal(c)
	if c.lingerUntil == 0 || w.conns[c.fd] != c {
		t.Fatal("closed at the cap")
	}
	// The unread rest keeps the socket readable: re-arming it would hand the
	// worker the same event until the deadline.
	if eng.recvArmed != 0 || eng.detached != 1 {
		t.Fatalf("read armed %d times, detached %d; want no events until the deadline", eng.recvArmed, eng.detached)
	}
	if n, err := syscall.Read(peer, make([]byte, 8)); n != 0 || err != nil {
		t.Fatalf("peer read %d, %v; want the EOF of the write side", n, err)
	}
}

// TestWingLingerReleasesTheBodyBudget retires an upload that timed out after
// an earlier response. Its body budget must come back while it lingers, or
// the next upload is refused until the socket closes.
func TestWingLingerReleasesTheBodyBudget(t *testing.T) {
	w, c, _ := finalClosePair(t)
	w.maxInflightBody, w.inflightBody = 64, 10
	c.bodyNeed, c.bodyBuf, c.responses = 10, make([]byte, 0, 10), 1
	w.readTimeout, c.readDeadline = int64(time.Second), 1
	w.sweepTimeouts(int64(time.Hour))
	if c.lingerUntil == 0 {
		t.Fatal("did not linger")
	}
	if w.inflightBody != 0 || c.bodyNeed != 0 {
		t.Fatalf("lingering upload holds %d of the budget, bodyNeed %d", w.inflightBody, c.bodyNeed)
	}
	w.closeConn(c.fd)
	if w.inflightBody != 0 {
		t.Fatalf("budget returned twice: %d", w.inflightBody)
	}
}

// TestWingFinalResponseSurvivesInputPastTheCap pipelines a large response and
// more input than maxDiscardInput behind it, from a client slow to read. Both
// the event loop and a Takeover must end the write side before dropping that
// input, and keep the socket until the client has read the response.
func TestWingFinalResponseSurvivesInputPastTheCap(t *testing.T) {
	for name, preset := range map[string]Preset{"event loop": Bolt, "Takeover": Spear} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("KRUDA_WORKERS", "1")
			big := strings.Repeat("h", 256<<10)
			app := New(Wing())
			app.Get("/big", func(c *Ctx) error { return c.Text(big) }, preset)
			app.Compile()
			addr, _ := startWingHijackServer(t, app)
			conn, br := framingDial(t, addr)
			defer conn.Close()
			req := "GET /big HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"
			go func() {
				_, _ = io.WriteString(conn, req)
				_, _ = conn.Write(make([]byte, maxDiscardInput+(64<<10)))
			}()
			start := make(chan struct{})
			got := readBodyAfter(br, start)
			time.Sleep(150 * time.Millisecond)
			close(start)
			if r := <-got; r.err != nil || r.n != len(big) {
				t.Fatalf("response %d of %d bytes, %v; want it whole", r.n, len(big), r.err)
			}
		})
	}
}

// TestWingRetireDrainsBehindAnAnswer ends live connections for a timeout or a
// malformed request. Once a response may be in the socket, each must linger
// behind it, as a final close does; one that never answered closes at once.
func TestWingRetireDrainsBehindAnAnswer(t *testing.T) {
	retires := map[string]func(w *worker, c *conn){
		"read timeout": func(w *worker, c *conn) {
			w.readTimeout = int64(time.Second)
			c.readDeadline = 1
			w.sweepTimeouts(int64(time.Hour))
		},
		"idle timeout": func(w *worker, c *conn) {
			w.idleTimeout = int64(time.Second)
			w.sweepTimeouts(int64(time.Hour))
		},
		"write timeout": func(w *worker, c *conn) {
			w.writeTimeout = int64(time.Second)
			c.sendBuf, c.sendN = []byte("unsent"), 0
			w.sweepTimeouts(int64(time.Hour))
		},
		"malformed request": func(w *worker, c *conn) {
			c.readN = copy(c.readBuf, "\n\r\n\r\n")
			w.tryParse(c)
		},
	}
	for name, retire := range retires {
		for _, answered := range []bool{true, false} {
			w, c, _ := finalClosePair(t)
			if answered {
				c.responses = 1
			}
			retire(w, c)
			if open := w.conns[c.fd] == c; open != answered || answered && c.lingerUntil == 0 {
				t.Errorf("%s, answered %v: open %v, lingering %v", name, answered, open, c.lingerUntil != 0)
			}
		}
	}
}

// TestWingShutdownDrainsBehindAKeepAliveReply leaves a whole keep-alive reply
// unread in the socket. A Pool handler on another connection holds Shutdown
// while the stopped loop no longer reads, and a late request arrives. Closing
// over it would reset the reply before the client reads it.
func TestWingShutdownDrainsBehindAKeepAliveReply(t *testing.T) {
	t.Setenv("KRUDA_WORKERS", "1")
	reply := strings.Repeat("k", 256<<10)
	entered, release := make(chan struct{}), make(chan struct{})
	app := New(Wing())
	app.Get("/reply", func(c *Ctx) error { return c.Text(reply) })
	app.Get("/hold", func(c *Ctx) error {
		close(entered)
		<-release
		return c.Text("held")
	}, Arrow)
	app.Compile()
	addr, _ := startWingHijackServer(t, app)
	conn, br := framingDial(t, addr)
	defer conn.Close()
	if _, err := io.WriteString(conn, framingGet("/reply")); err != nil {
		t.Fatal(err)
	}
	hold, _ := framingDial(t, addr)
	defer hold.Close()
	if _, err := io.WriteString(hold, framingGet("/hold")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Pool handler never ran")
	}
	time.Sleep(50 * time.Millisecond) // the reply is written, and not read
	stopped := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopped <- app.transport.Shutdown(ctx)
	}()
	time.Sleep(100 * time.Millisecond) // the loop has stopped; Shutdown waits for the Pool
	if _, err := io.WriteString(conn, framingGet("/late")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	start := make(chan struct{})
	got := readBodyAfter(br, start)
	close(start)
	r := <-got
	if err := <-stopped; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if r.err != nil || r.n != len(reply) {
		t.Fatalf("reply %d of %d bytes, %v; want it whole", r.n, len(reply), r.err)
	}
}

// TestWingHijackCloseDrainsBehindAnEarlierResponse pipelines a keep-alive
// response and an upgrade the app hijacks, answers and closes while a client
// frame is still unread. The response before the upgrade is still queued:
// reclaiming the socket over that frame would reset it.
func TestWingHijackCloseDrainsBehindAnEarlierResponse(t *testing.T) {
	t.Setenv("KRUDA_WORKERS", "1")
	big := strings.Repeat("h", 256<<10)
	entered, release := make(chan struct{}), make(chan struct{})
	app := New(Wing())
	app.Get("/big", func(c *Ctx) error { return c.Text(big) }, Bolt)
	app.Get("/ws", func(c *Ctx) error {
		nc, _, err := c.ResponseWriter().(http.Hijacker).Hijack()
		if err != nil {
			return err
		}
		close(entered)
		<-release
		return nc.Close()
	}, Hijack)
	app.Compile()
	addr, _ := startWingHijackServer(t, app)
	conn, br := framingDial(t, addr)
	defer conn.Close()
	if _, err := io.WriteString(conn, framingGet("/big")+framingGet("/ws")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Hijack handler never ran")
	}
	if _, err := io.WriteString(conn, "a frame the app never reads"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	time.Sleep(100 * time.Millisecond)
	start := make(chan struct{})
	got := readBodyAfter(br, start)
	close(start)
	if r := <-got; r.err != nil || r.n != len(big) {
		t.Fatalf("response %d of %d bytes, %v; want it whole", r.n, len(big), r.err)
	}
}

// TestWingHijackReadsStayEndedPastAppDeadlines wakes a hijacked socket for
// Shutdown while one app goroutine is blocked reading and another keeps
// pushing the read deadline out, as a read loop with a timeout does. The
// blocked read must still end, every later read must report EOF, and the
// owner must be able to wait for app reads before it drains.
func TestWingHijackReadsStayEndedPastAppDeadlines(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	f := os.NewFile(uintptr(fds[0]), "hijacked")
	defer f.Close()
	g := &takeoverGate{}
	nc := newWingHijackConn(f, int32(fds[0]), "", context.Background(), g)
	blocked := make(chan error, 1)
	go func() {
		_, err := nc.Read(make([]byte, 8))
		blocked <- err
	}()
	stop := make(chan struct{})
	pushed := make(chan struct{})
	go func() {
		defer close(pushed)
		for {
			select {
			case <-stop:
				return
			default:
				_ = nc.SetReadDeadline(time.Now().Add(time.Hour))
			}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	g.wake(f, int32(fds[0]))
	select {
	case err := <-blocked:
		if err != io.EOF {
			t.Fatalf("blocked read ended with %v, want EOF", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a read blocked at Shutdown never ended")
	}
	close(stop)
	<-pushed
	if _, err := nc.Read(make([]byte, 8)); err != io.EOF {
		t.Fatalf("read after Shutdown: %v, want EOF", err)
	}
	if err := nc.Close(); err != nil {
		t.Fatal(err)
	}
	joined := make(chan struct{})
	go func() { nc.reads.Wait(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(2 * time.Second):
		t.Fatal("app reads never returned after Close")
	}
}

// TestWingTakeoverDrainHoldsWithoutADeadline drains a Takeover socket whose
// file cannot take a read deadline, as when it never reached the runtime
// poller. Its write side still ends, and the socket must be held through the
// linger rather than closed over the input queued on it.
func TestWingTakeoverDrainHoldsWithoutADeadline(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[1])
	f := os.NewFile(uintptr(fds[0]), "blocking") // a blocking fd: no deadlines
	defer f.Close()
	if err := f.SetReadDeadline(time.Now()); err == nil {
		t.Skip("this platform gave a blocking file a deadline")
	}
	if _, err := syscall.Write(fds[1], []byte("queued input")); err != nil {
		t.Fatal(err)
	}
	old := lingerDelay
	lingerDelay = int64(100 * time.Millisecond)
	defer func() { lingerDelay = old }()
	start := time.Now()
	lingerTakeover(f, int32(fds[0]), make([]byte, 64), &takeoverGate{})
	if held := time.Since(start); held < 90*time.Millisecond {
		t.Fatalf("held %v, want the linger", held)
	}
	if n, err := syscall.Read(fds[1], make([]byte, 8)); n != 0 || err != nil {
		t.Fatalf("peer read %d, %v; want the EOF of the write side", n, err)
	}
}
