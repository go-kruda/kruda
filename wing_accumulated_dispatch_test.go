//go:build linux || darwin

package kruda

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
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
