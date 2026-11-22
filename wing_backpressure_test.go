//go:build linux || darwin

package kruda

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingUnreadResponsesAreBounded pipelines requests for large responses and
// reads none of them.
func TestWingUnreadResponsesAreBounded(t *testing.T) {
	const bodySize, queueLimit = 64 << 10, 256 << 10
	body := strings.Repeat("b", bodySize)
	calls := 0
	w, c, peer, _ := socketWorker(t, 8192, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		calls++
		_, _ = rw.Write([]byte(body))
	}))
	if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	batch := strings.Repeat("GET / HTTP/1.1\r\nHost: x\r\n\r\n", 100)
	sent := 0
	for i := range 8 {
		if _, err := io.WriteString(peer, batch); err != nil {
			t.Fatal(err)
		}
		sent += 100
		w.handleRecv(event{Op: opRecv, Fd: c.fd})
		if queued := len(c.sendBuf) - c.sendN; queued > queueLimit+bodySize+512 {
			t.Fatalf("read event %d left %d bytes queued for a client that reads nothing", i, queued)
		}
	}
	// Once the client reads, every request is still answered.
	done := make(chan error, 1)
	go func() {
		br := bufio.NewReader(peer)
		for range sent {
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				done <- err
				return
			}
			if n, err := io.Copy(io.Discard, res.Body); err != nil || n != bodySize {
				done <- errors.Join(err, errors.New("short response"))
				return
			}
		}
		done <- nil
	}()
	for {
		select {
		case err := <-done:
			if err != nil || calls != sent {
				t.Fatalf("served %d of %d: %v", calls, sent, err)
			}
			return
		default:
			w.handleSend(event{Op: opSend, Fd: c.fd})
			w.handleRecv(event{Op: opRecv, Fd: c.fd}) // kqueue reports leftover input again
			time.Sleep(10 * time.Microsecond)
		}
	}
}

// TestWingSuppressedReadinessSeesEOF lets one short read consume the data in
// front of an EOF whose readiness was suppressed. No other edge will report it.
func TestWingSuppressedReadinessSeesEOF(t *testing.T) {
	w, c, peer, eng := socketWorker(t, 512, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		_, _ = rw.Write([]byte("ok"))
	}))
	if _, err := io.WriteString(peer, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := peer.(*net.UnixConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	c.readSuppressed = true
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if len(eng.closedFds) != 1 {
		t.Fatal("connection kept after an EOF that will not be reported again")
	}
}

// TestWingYieldedInputSurvivesEOF reports EOF while a complete request waits
// for its connection's next turn.
func TestWingYieldedInputSurvivesEOF(t *testing.T) {
	calls := 0
	w, c, peer, eng := socketWorker(t, 512, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		calls++
		_, _ = rw.Write([]byte("ok"))
	}))
	if err := peer.(*net.UnixConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	c.readN = copy(c.readBuf, "GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	c.keepAlive, c.resumeParse = true, true
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if len(eng.closedFds) != 0 {
		t.Fatal("closed over a buffered request")
	}
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if calls != 1 || len(eng.closedFds) != 1 {
		t.Fatalf("buffered request ran %d times, closed=%v", calls, eng.closedFds)
	}
}

// TestWingStaleReadinessKeepsConnection reports readiness for input an earlier
// read already took.
func TestWingStaleReadinessKeepsConnection(t *testing.T) {
	w, c, _, eng := socketWorker(t, 512, nil)
	c.readSuppressed = true
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if len(eng.closedFds) != 0 || c.readSuppressed {
		t.Fatalf("stale readiness: closed=%v suppressed=%v", eng.closedFds, c.readSuppressed)
	}
}

// TestWingBackpressureResumes pauses reading behind large responses on every
// dispatch, then reads them all. No timeout may be needed to finish.
func TestWingBackpressureResumes(t *testing.T) {
	const bodySize, requests = 64 << 10, 400
	body := strings.Repeat("b", bodySize)
	for _, tc := range []struct {
		name   string
		preset Preset
	}{{"Inline", Bolt}, {"Pool", Arrow}, {"Spawn", Preset{Dispatch: Spawn}}, {"Takeover", Spear}} {
		for _, halfClose := range []bool{false, true} {
			name := tc.name
			if halfClose {
				name += " half-close"
			}
			t.Run(name, func(t *testing.T) {
				addr, stop := startWingServerWithConfig(t, WingConfig{
					Workers: 1, HandlerPoolSize: 2, DefaultPreset: Bolt,
					Presets: map[string]Preset{"GET /mixed": tc.preset},
				}, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
					id := r.Header("X-N")
					_, _ = rw.Write([]byte(id + body[len(id):]))
				}))
				defer stop()
				conn, br := framingDial(t, addr)
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
				var halves [2]strings.Builder
				for i := range requests {
					path := "/inline"
					if i%(requests/2) >= requests/4 {
						path = "/mixed"
					}
					halves[i/(requests/2)].WriteString("GET " + path + " HTTP/1.1\r\nHost: x\r\nX-N: " + strconv.Itoa(i) + "|\r\n\r\n")
				}
				for i := range halves {
					if i > 0 {
						time.Sleep(100 * time.Millisecond) // arrives while the responses wait
					}
					if _, err := io.WriteString(conn, halves[i].String()); err != nil {
						t.Fatal(err)
					}
				}
				if halfClose {
					if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
						t.Fatal(err)
					}
					time.Sleep(50 * time.Millisecond)
				}
				for i := range requests {
					res, err := http.ReadResponse(br, nil)
					if err != nil {
						t.Fatalf("response %d of %d: %v", i, requests, err)
					}
					got, err := io.ReadAll(res.Body)
					if err != nil || len(got) != bodySize {
						t.Fatalf("response %d: %d bytes, %v", i, len(got), err)
					}
					if id, _, _ := strings.Cut(string(got[:16]), "|"); id != strconv.Itoa(i) {
						t.Fatalf("response %d answers request %s", i, id)
					}
				}
				if halfClose {
					if _, err := br.ReadByte(); !errors.Is(err, io.EOF) {
						t.Fatalf("connection held after the half-close: %v", err)
					}
				}
			})
		}
	}
}
