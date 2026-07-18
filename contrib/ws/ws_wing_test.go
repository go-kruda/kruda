//go:build linux || darwin

package ws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// listenWingApp starts app on a real loopback listener via Wing and returns its
// address + a stop func. Readiness requires an application-level HTTP response;
// a bare TCP connect can succeed before Wing has installed its request loop.
func listenWingApp(t *testing.T, app *kruda.App) (addr string, stop func()) {
	t.Helper()
	rt := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: 250 * time.Millisecond, Transport: rt}
	defer rt.CloseIdleConnections()
	var lastErr error

attempts:
	for attempt := 0; attempt < 12; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr = ln.Addr().String()
		errc := make(chan error, 1)
		go func() { errc <- app.Serve(ln) }()

		shutdown := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = app.Shutdown(ctx)
			select {
			case <-errc:
			case <-ctx.Done():
				t.Errorf("listenWingApp: shutdown timed out: %v", ctx.Err())
			}
		}

		readyURL := "http://" + addr + "/ws"
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case serveErr := <-errc:
				lastErr = fmt.Errorf("server stopped before readiness: %v", serveErr)
				_ = ln.Close()
				continue attempts
			default:
			}

			resp, getErr := client.Get(readyURL)
			if getErr == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusBadRequest {
					select {
					case serveErr := <-errc:
						lastErr = fmt.Errorf("server stopped after readiness probe: %v", serveErr)
						_ = ln.Close()
						continue attempts
					default:
						return addr, shutdown
					}
				}
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
			} else {
				lastErr = getErr
			}
			time.Sleep(10 * time.Millisecond)
		}

		shutdown()
		t.Fatalf("listenWingApp: server did not become ready at %s: %v", readyURL, lastErr)
	}

	t.Fatalf("listenWingApp: bind kept failing after retries: %v", lastErr)
	return "", func() {}
}

func TestWSOverWing_Echo(t *testing.T) {
	app := kruda.New(kruda.Wing()) // force Wing on every platform
	HandleFunc(app, "/ws", func(conn *Conn) {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(mt, data)
	})
	app.Compile()
	addr, stop := listenWingApp(t, app)
	defer stop()

	conn := dialWSAddr(t, addr, "/ws") // reuse dialWS logic against a raw addr
	defer conn.Close()
	sendClientFrame(t, conn, true, 0x1, []byte("hello-wing"))
	br := bufio.NewReader(conn)
	f := readServerFrame(t, conn, br)
	if f.opcode != 0x1 || string(f.payload) != "hello-wing" {
		t.Errorf("echo = 0x%X %q", f.opcode, f.payload)
	}
}

func TestWSOverWing_PingPong(t *testing.T) {
	app := kruda.New(kruda.Wing())
	HandleFunc(app, "/ws", func(conn *Conn) {
		// Just read messages until close/error
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	app.Compile()
	addr, stop := listenWingApp(t, app)
	defer stop()

	conn := dialWSAddr(t, addr, "/ws")
	defer conn.Close()

	// Send ping
	sendClientFrame(t, conn, true, 0x9, []byte("ping-data"))

	// Expect pong with same payload
	br := bufio.NewReader(conn)
	f := readServerFrame(t, conn, br)
	if f.opcode != 0xA {
		t.Errorf("expected pong (0xA), got opcode %d", f.opcode)
	}
	if string(f.payload) != "ping-data" {
		t.Errorf("expected 'ping-data', got %q", f.payload)
	}
}

func TestWSOverWing_CloseHandshake(t *testing.T) {
	app := kruda.New(kruda.Wing())
	HandleFunc(app, "/ws", func(conn *Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	app.Compile()
	addr, stop := listenWingApp(t, app)
	defer stop()

	conn := dialWSAddr(t, addr, "/ws")
	defer conn.Close()

	// Send close frame
	closePayload := make([]byte, 2)
	binary.BigEndian.PutUint16(closePayload, uint16(CloseNormalClosure))
	sendClientFrame(t, conn, true, 0x8, closePayload)

	// Expect close frame back
	br := bufio.NewReader(conn)
	f := readServerFrame(t, conn, br)
	if f.opcode != 0x8 {
		t.Errorf("expected close frame, got opcode %d", f.opcode)
	}
}

// TestWSOverWing_RejectUpgradeWithBodyOnePacket sends a full, valid WS upgrade
// GET that ALSO carries a body, all in a single write (so the whole request is
// parsed in one read and reaches the hijack path — not the split-body
// accumulation path). validateUpgrade must reject the body, so the outcome is a
// 4xx regardless of TCP packetization, never a 101 protocol switch.
func TestWSOverWing_RejectUpgradeWithBodyOnePacket(t *testing.T) {
	app := kruda.New(kruda.Wing())
	HandleFunc(app, "/ws", func(conn *Conn) {})
	app.Compile()
	addr, stop := listenWingApp(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	req := "GET /ws HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\nContent-Length: 5\r\n\r\nhello"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 101 {
		t.Fatal("bodied upgrade must not switch protocols, got 101")
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for bodied upgrade, got %d", resp.StatusCode)
	}
}

func TestWSOverWing_PipelinedFirstFrame(t *testing.T) {
	app := kruda.New(kruda.Wing())
	HandleFunc(app, "/ws", func(conn *Conn) {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(mt, data)
	})
	app.Compile()
	addr, stop := listenWingApp(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Build handshake + first masked frame as ONE buffer, write once.
	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	handshake := "GET /ws HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\n" +
		"Connection: Upgrade\r\nSec-WebSocket-Key: " + key +
		"\r\nSec-WebSocket-Version: 13\r\n\r\n"
	var one bytes.Buffer
	one.WriteString(handshake)
	appendMaskedFrame(&one, true, 0x1, []byte("pipelined")) // helper: same masking as sendClientFrame
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(one.Bytes()); err != nil {
		t.Fatal(err)
	}

	// Read the 101, then the echoed first frame.
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != 101 {
		t.Fatalf("handshake: %v status %v", err, resp)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	f := readServerFrame(t, conn, br)
	if f.opcode != 0x1 || string(f.payload) != "pipelined" {
		t.Errorf("pipelined first frame lost: got 0x%X %q", f.opcode, f.payload)
	}
}
