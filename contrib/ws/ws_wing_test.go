//go:build linux || darwin

package ws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// listenWingApp starts app on a real loopback listener via Wing and returns its
// address + a stop func. Public API only (package ws can't touch app.transport).
func listenWingApp(t *testing.T, app *kruda.App) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = ln.Addr().String()
	_ = ln.Close() // free the port; app.Listen re-binds it (readiness poll closes the race)
	go func() { _ = app.Listen(addr) }()
	// Readiness poll.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond); derr == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return addr, func() { _ = app.Shutdown(context.Background()) }
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
