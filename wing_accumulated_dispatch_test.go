//go:build linux || darwin

package kruda

import (
	"bufio"
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

// TestWingAccumulatedBodyKeepsDispatch sends a POST whose body arrives after
// its header, to a Pool or Takeover route whose handler waits. While it waits,
// a GET on another connection to the same single worker must still be
// answered: the handler must not run on the event loop.
func TestWingAccumulatedBodyKeepsDispatch(t *testing.T) {
	for _, tt := range []struct {
		name   string
		preset Preset
	}{{"Arrow", Arrow}, {"Spawn", Preset{Dispatch: Spawn}}, {"Spear", Spear}} {
		t.Run(tt.name, func(t *testing.T) {
			release := make(chan struct{})
			entered := make(chan struct{}, 1)
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/slow" {
					body, _ := r.Body()
					entered <- struct{}{}
					select {
					case <-release:
					case <-time.After(5 * time.Second):
					}
					_, _ = w.Write([]byte(strconv.Itoa(len(body))))
					return
				}
				_, _ = w.Write([]byte("fast"))
			})
			cfg := WingConfig{
				Workers:         1,
				HandlerPoolSize: 4,
				DefaultPreset:   Bolt,
				Presets:         map[string]Preset{"POST /slow": tt.preset},
			}
			addr, stop := startWingServerWithConfig(t, cfg, handler)
			defer stop()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()

			slow, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer slow.Close()
			body := strings.Repeat("b", 20000)
			if _, err := io.WriteString(slow, "POST /slow HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"+body[:100]); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond)
			if _, err := io.WriteString(slow, body[100:]); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("slow handler never ran")
			}

			fast, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer fast.Close()
			_ = fast.SetDeadline(time.Now().Add(time.Second))
			if _, err := io.WriteString(fast, "GET /fast HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			resp, err := http.ReadResponse(bufio.NewReader(fast), nil)
			if err != nil {
				t.Fatalf("GET blocked behind a waiting %s handler: %v", tt.name, err)
			}
			resp.Body.Close()

			close(release)
			_ = slow.SetDeadline(time.Now().Add(3 * time.Second))
			resp, err = http.ReadResponse(bufio.NewReader(slow), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if string(got) != strconv.Itoa(len(body)) {
				t.Fatalf("slow handler saw a %s-byte body, want %d", got, len(body))
			}
		})
	}
}

// TestWingAccumulatedTakeoverQueuesBehindUnsentResponse completes a POST body
// for a Takeover route while a reply below the queue limit is unsent. Takeover
// cannot detach then, so the handler runs off the loop and replies after it.
func TestWingAccumulatedTakeoverQueuesBehindUnsentResponse(t *testing.T) {
	big := strings.Repeat("g", 128<<10)
	release := make(chan struct{})
	w, c, peer, _ := socketWorker(t, 8192, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/big" {
			_, _ = rw.Write([]byte(big))
			return
		}
		<-release
		body, _ := r.Body()
		_, _ = rw.Write([]byte(strconv.Itoa(len(body))))
	}))
	w.presets = NewPresetTable(map[string]Preset{"POST /slow": Spear}, Bolt)
	w.doneCh = make(chan doneMsg, 1)
	if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(peer, "GET /big HTTP/1.1\r\nHost: x\r\n\r\nPOST /slow HTTP/1.1\r\nHost: x\r\nContent-Length: 20000\r\n\r\n"+strings.Repeat("b", 20000)); err != nil {
		t.Fatal(err)
	}
	returned := make(chan struct{})
	go func() {
		w.handleRecv(event{Op: opRecv, Fd: c.fd})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("handler ran on the event loop")
	}
	if c.pending != 1 || c.takenOver {
		close(release)
		t.Fatalf("pending=%d takenOver=%v", c.pending, c.takenOver)
	}
	close(release)
	w.handleDone(<-w.doneCh)

	got := make(chan string, 1)
	go func() {
		br := bufio.NewReader(peer)
		var replies []string
		for range 2 {
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				got <- err.Error()
				return
			}
			body, _ := io.ReadAll(res.Body)
			replies = append(replies, strconv.Itoa(len(body)))
		}
		got <- strings.Join(replies, ",")
	}()
	want := strconv.Itoa(len(big)) + ",5"
	for {
		select {
		case replies := <-got:
			if replies != want {
				t.Fatalf("replies %s, want %s", replies, want)
			}
			return
		default:
			w.handleSend(event{Op: opSend, Fd: c.fd})
			time.Sleep(10 * time.Microsecond)
		}
	}
}

// TestWingAccumulatedTakeoverBehindUnsentResponse pipelines a fragmented POST
// to a Takeover route behind a GET reply over the queue limit. The POST waits
// for that reply, then must run off the event loop and reply in order.
func TestWingAccumulatedTakeoverBehindUnsentResponse(t *testing.T) {
	big := strings.Repeat("g", 8<<20)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		switch r.Path() {
		case "/slow":
			body, _ := r.Body()
			entered <- struct{}{}
			select {
			case <-release:
			case <-time.After(5 * time.Second):
			}
			_, _ = w.Write([]byte(strconv.Itoa(len(body))))
		case "/big":
			_, _ = w.Write([]byte(big))
		default:
			_, _ = w.Write([]byte("fast"))
		}
	})
	cfg := WingConfig{Workers: 1, DefaultPreset: Bolt, Presets: map[string]Preset{"POST /slow": Spear}}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	defer stop()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	slow, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()
	body := strings.Repeat("b", 20000)
	if _, err := io.WriteString(slow, "GET /big HTTP/1.1\r\nHost: x\r\n\r\nPOST /slow HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"+body[:100]); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := io.WriteString(slow, body[100:]); err != nil {
		t.Fatal(err)
	}
	_ = slow.SetDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(slow)
	read := func(want string) {
		r, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if string(got) != want {
			t.Fatalf("reply out of order or wrong: %d bytes, want %d", len(got), len(want))
		}
	}
	read(big)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow handler never ran")
	}

	fast, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer fast.Close()
	_ = fast.SetDeadline(time.Now().Add(time.Second))
	if _, err := io.WriteString(fast, "GET /fast HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(fast), nil)
	if err != nil {
		t.Fatalf("GET blocked behind the Takeover handler: %v", err)
	}
	resp.Body.Close()
	close(release)
	read("20000")
}
