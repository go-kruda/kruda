//go:build linux || darwin

package kruda

import (
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingYieldWaitingToWriteTimesOut yields while the kernel send buffer is
// too full to report writable. The client reads nothing, so no event follows.
func TestWingYieldWaitingToWriteTimesOut(t *testing.T) {
	w, eng := newTestWorker(0)
	w.writeTimeout, w.idleTimeout = int64(time.Second), int64(time.Second)
	c := newTestConn(42)
	c.keepAlive, c.resumeParse = true, true
	c.readN = copy(c.readBuf, "GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	w.conns[c.fd] = c
	w.sweepTimeouts(int64(3 * time.Second))
	if len(eng.closedFds) != 1 {
		t.Fatal("yielded connection is bound by no timeout")
	}
}

// TestWingReadDeadlineRestartsAfterSend finishes a send whose partial next
// request was seen long before. The client gets the whole interval from there.
func TestWingReadDeadlineRestartsAfterSend(t *testing.T) {
	w, c, peer, eng := socketWorker(t, 512, nil)
	w.readTimeout = int64(time.Second)
	w.now = int64(5 * time.Second)
	c.keepAlive = true
	c.sendBuf = []byte("reply")
	c.readN = copy(c.readBuf, "GET /par")
	c.readDeadline = int64(2 * time.Second) // expired while sending
	w.handleSend(event{Op: opSend, Fd: c.fd})
	// The worker's batch clock is stale by however long the batch took.
	w.sweepTimeouts(time.Now().UnixNano() + int64(time.Second)/2)
	if len(eng.closedFds) != 0 {
		t.Fatal("inherited a deadline that ran out during the send")
	}
	_, _ = peer.Read(make([]byte, 16))
}

// TestWingUploadDoesNotExtendWriteTimeout trickles a body behind a reply the
// client does not read.
func TestWingUploadDoesNotExtendWriteTimeout(t *testing.T) {
	w, c, peer, _ := socketWorker(t, 512, nil)
	w.hasTimeout = true
	w.writeTimeout = int64(time.Second)
	w.now = int64(5 * time.Second)
	c.keepAlive = true
	c.sendBuf = []byte("reply nobody reads")
	c.lastActive = int64(time.Second)
	c.bodyNeed = 100
	if _, err := io.WriteString(peer, "x"); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if c.lastActive != int64(time.Second) {
		t.Fatal("an upload byte restarted the write timeout")
	}
}

// TestWingWriteEventReadsBodyThatArrivedWithIt delivers one event for both
// readiness kinds, as epoll does. No read event follows.
func TestWingWriteEventReadsBodyThatArrivedWithIt(t *testing.T) {
	var got string
	w, c, peer, _ := socketWorker(t, 512, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		got = string(b)
		_, _ = rw.Write([]byte("ok"))
	}))
	c.keepAlive = true
	c.sendBuf = []byte(wing100Continue)
	c.bodyNeed = 4
	c.bodyBuf = make([]byte, 0, 4)
	c.headerSnapshot = []byte("POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\n")
	if _, err := io.WriteString(peer, "body"); err != nil {
		t.Fatal(err)
	}
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if got != "body" {
		t.Fatal("body left in the socket with no event to come")
	}
}

// TestWingUploadEOFKeepsReplyBeingSent delivers a write event that cannot
// finish an earlier reply while the upload behind it has ended short.
func TestWingUploadEOFKeepsReplyBeingSent(t *testing.T) {
	w, c, peer, eng := socketWorker(t, 512, nil)
	if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	c.keepAlive = true
	c.sendBuf = make([]byte, 4<<20)
	c.bodyNeed = 4
	c.bodyBuf = make([]byte, 0, 4)
	if err := peer.(*net.UnixConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if len(eng.closedFds) != 0 {
		t.Fatalf("closed over a reply with %d bytes unsent", len(c.sendBuf)-c.sendN)
	}
}

// TestWingWriteEventKeepsHiddenEOF reports only writability while a request
// and an EOF are readable.
func TestWingWriteEventKeepsHiddenEOF(t *testing.T) {
	calls := 0
	w, c, peer, eng := socketWorker(t, 512, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		calls++
		_, _ = rw.Write([]byte("ok"))
	}))
	c.keepAlive = true
	c.sendBuf = []byte("earlier reply")
	if _, err := io.WriteString(peer, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := peer.(*net.UnixConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if calls != 1 || len(eng.closedFds) != 1 {
		t.Fatalf("calls=%d closed=%v, want the request served and the EOF seen", calls, eng.closedFds)
	}
}

// TestWingReplyIsSentBeforeWaitingForUpload parks on a body behind a complete
// request. Its reply must not wait for, or time out under, the upload.
func TestWingReplyIsSentBeforeWaitingForUpload(t *testing.T) {
	w, c, peer, _ := socketWorker(t, 512, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		_, _ = rw.Write([]byte("ok"))
	}))
	if _, err := io.WriteString(peer, "GET / HTTP/1.1\r\nHost: x\r\n\r\nPOST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\npart"); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if c.bodyNeed != 100 {
		t.Fatalf("bodyNeed = %d, want an upload in progress", c.bodyNeed)
	}
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 256)); err != nil {
		t.Fatalf("reply held back for the upload: %v", err)
	}
}

// TestWingReadDeadlineArmsFromFreshClock sees a partial request for the first
// time after a send, late in a slow event batch.
func TestWingReadDeadlineArmsFromFreshClock(t *testing.T) {
	w, c, _, _ := socketWorker(t, 512, nil)
	w.readTimeout = int64(time.Second)
	w.now = int64(5 * time.Second) // the batch's clock
	c.keepAlive = true
	c.readN = copy(c.readBuf, "GET /par")
	w.tryParse(c)
	if c.readDeadline <= time.Now().UnixNano() {
		t.Fatal("deadline armed in the past")
	}
}
