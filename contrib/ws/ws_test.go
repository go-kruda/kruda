package ws

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// --- Frame tests ---

func TestFrameRoundTrip_Text(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hello world")
	if err := writeFrame(&buf, true, 0x1, payload); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !f.fin {
		t.Error("expected FIN")
	}
	if f.opcode != 0x1 {
		t.Errorf("expected opcode 1, got %d", f.opcode)
	}
	if string(f.payload) != "hello world" {
		t.Errorf("payload mismatch: %q", f.payload)
	}
}

func TestFrameRoundTrip_Binary(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0x00, 0xFF, 0xAB, 0xCD}
	if err := writeFrame(&buf, true, 0x2, payload); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.opcode != 0x2 {
		t.Errorf("expected opcode 2, got %d", f.opcode)
	}
	if !bytes.Equal(f.payload, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestFrameRoundTrip_LargePayload(t *testing.T) {
	var buf bytes.Buffer
	payload := make([]byte, 70000) // > 65535, uses 8-byte extended length
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	if err := writeFrame(&buf, true, 0x2, payload); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.payload) != 70000 {
		t.Errorf("expected 70000 bytes, got %d", len(f.payload))
	}
	if !bytes.Equal(f.payload, payload) {
		t.Error("large payload mismatch")
	}
}

func TestFrameRoundTrip_MediumPayload(t *testing.T) {
	var buf bytes.Buffer
	payload := make([]byte, 300) // 126-65535 range, uses 2-byte extended length
	if err := writeFrame(&buf, true, 0x1, payload); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.payload) != 300 {
		t.Errorf("expected 300 bytes, got %d", len(f.payload))
	}
}

func TestFrameRoundTrip_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, true, 0x1, nil); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(f.payload))
	}
}

func TestMaskBytes(t *testing.T) {
	key := [4]byte{0x37, 0xFA, 0x21, 0x3D}
	data := []byte("Hello")
	original := make([]byte, len(data))
	copy(original, data)

	maskBytes(key, data)
	// Masked data should differ
	if bytes.Equal(data, original) {
		t.Error("masking should change data")
	}

	// Mask again to unmask (XOR is self-inverse)
	maskBytes(key, data)
	if !bytes.Equal(data, original) {
		t.Error("double masking should restore original")
	}
}

func TestReadFrame_Masked(t *testing.T) {
	// Build a masked frame manually
	key := [4]byte{0x37, 0xFA, 0x21, 0x3D}
	payload := []byte("Hello")
	masked := make([]byte, len(payload))
	copy(masked, payload)
	maskBytes(key, masked)

	var buf bytes.Buffer
	buf.WriteByte(0x81)                      // FIN + text
	buf.WriteByte(0x80 | byte(len(payload))) // MASK bit + length
	buf.Write(key[:])
	buf.Write(masked)

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(f.payload) != "Hello" {
		t.Errorf("expected 'Hello', got %q", f.payload)
	}
	if !f.masked {
		t.Error("expected masked flag")
	}
}

func TestCloseFrame(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCloseFrame(&buf, CloseNormalClosure, "bye"); err != nil {
		t.Fatal(err)
	}

	f, err := readFrame(bufio.NewReader(&buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.opcode != 0x8 {
		t.Errorf("expected close opcode, got %d", f.opcode)
	}
	code, reason := parseClosePayload(f.payload)
	if code != CloseNormalClosure {
		t.Errorf("expected code 1000, got %d", code)
	}
	if reason != "bye" {
		t.Errorf("expected reason 'bye', got %q", reason)
	}
}

func TestReadFrame_RSVBitsReject(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(0x91) // FIN + RSV1 set + text opcode
	buf.WriteByte(0x00) // no mask, 0 length

	_, err := readFrame(bufio.NewReader(&buf), 0)
	if err == nil {
		t.Error("expected error for RSV bits set")
	}
}

// --- Handshake / Accept Key tests ---

func TestComputeAcceptKey(t *testing.T) {
	// RFC 6455 §4.2.2 example
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	expected := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="

	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(magicGUID))
	got := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if got != expected {
		t.Errorf("RFC 6455 example: expected %q, got %q", expected, got)
	}

	// Our function should match
	if computeAcceptKey(key) != expected {
		t.Errorf("computeAcceptKey mismatch")
	}
}

func TestHeaderContains(t *testing.T) {
	tests := []struct {
		header string
		token  string
		want   bool
	}{
		{"Upgrade", "upgrade", true},
		{"keep-alive, Upgrade", "upgrade", true},
		{"keep-alive", "upgrade", false},
		{"", "upgrade", false},
	}
	for _, tt := range tests {
		got := headerContains(tt.header, tt.token)
		if got != tt.want {
			t.Errorf("headerContains(%q, %q) = %v, want %v", tt.header, tt.token, got, tt.want)
		}
	}
}

// --- Integration test with real HTTP server ---

func TestUpgrade_FullHandshake(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			// Echo server: read one message, write it back
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(msgType, data)
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Perform WebSocket handshake manually
	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	// Send a text message (masked, as client)
	sendClientFrame(t, conn, true, 0x1, []byte("hello"))

	// Read echo response
	f := readServerFrame(t, conn)
	if f.opcode != 0x1 {
		t.Errorf("expected text frame, got opcode %d", f.opcode)
	}
	if string(f.payload) != "hello" {
		t.Errorf("expected 'hello', got %q", f.payload)
	}
}

func TestUpgrade_PingPong(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			// Just read messages until close/error
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	// Send ping
	sendClientFrame(t, conn, true, 0x9, []byte("ping-data"))

	// Expect pong with same payload
	f := readServerFrame(t, conn)
	if f.opcode != 0xA {
		t.Errorf("expected pong (0xA), got opcode %d", f.opcode)
	}
	if string(f.payload) != "ping-data" {
		t.Errorf("expected 'ping-data', got %q", f.payload)
	}
}

func TestUpgrade_CloseHandshake(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	// Send close frame
	closePayload := make([]byte, 2)
	binary.BigEndian.PutUint16(closePayload, uint16(CloseNormalClosure))
	sendClientFrame(t, conn, true, 0x8, closePayload)

	// Expect close frame back
	f := readServerFrame(t, conn)
	if f.opcode != 0x8 {
		t.Errorf("expected close frame, got opcode %d", f.opcode)
	}
}

func TestUpgrade_BinaryMessage(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(msgType, data)
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	binData := []byte{0x00, 0xFF, 0xAB, 0xCD}
	sendClientFrame(t, conn, true, 0x2, binData)

	f := readServerFrame(t, conn)
	if f.opcode != 0x2 {
		t.Errorf("expected binary frame, got opcode %d", f.opcode)
	}
	if !bytes.Equal(f.payload, binData) {
		t.Error("binary payload mismatch")
	}
}

func TestUpgrade_MaxMessageSize(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{MaxMessageSize: 10})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return // expected: message too big
			}
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	// Send message exceeding limit
	bigData := make([]byte, 20)
	sendClientFrame(t, conn, true, 0x1, bigData)

	// Server should send close frame with 1009
	f := readServerFrame(t, conn)
	if f.opcode != 0x8 {
		t.Errorf("expected close frame, got opcode %d", f.opcode)
	}
	if len(f.payload) >= 2 {
		code := int(binary.BigEndian.Uint16(f.payload[:2]))
		if code != CloseMessageTooBig {
			t.Errorf("expected close code 1009, got %d", code)
		}
	}
}

func TestUpgrade_MissingHeaders(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Request without upgrade headers → should get 400
	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpgrade_OriginRejection(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{
		AllowedOrigins: []string{"https://allowed.example.com"},
	})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Dial with wrong origin
	url := strings.Replace(srv.URL, "http://", "", 1)
	conn, err := net.DialTimeout("tcp", url, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: " + url + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Origin: https://evil.example.com\r\n" +
		"\r\n"
	conn.Write([]byte(req))

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("expected 403 for bad origin, got %d", resp.StatusCode)
	}
}

func TestUpgrade_OriginAllowAll(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{}) // empty AllowedOrigins = allow all

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			conn.Close(CloseNormalClosure, "")
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()
	// If we got here, the upgrade succeeded
}

func TestUpgrade_FragmentedMessage(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(msgType, data)
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	conn := dialWS(t, srv.URL+"/ws")
	defer conn.Close()

	// Send fragmented text message: first fragment + continuation + final
	sendClientFrame(t, conn, false, 0x1, []byte("hel"))  // first fragment (not FIN)
	sendClientFrame(t, conn, false, 0x0, []byte("lo "))  // continuation (not FIN)
	sendClientFrame(t, conn, true, 0x0, []byte("world")) // final continuation (FIN)

	// Read assembled message
	f := readServerFrame(t, conn)
	if f.opcode != 0x1 {
		t.Errorf("expected text frame, got opcode %d", f.opcode)
	}
	if string(f.payload) != "hello world" {
		t.Errorf("expected 'hello world', got %q", f.payload)
	}
}

// --- Test helpers ---

// dialWS performs a WebSocket handshake and returns the raw TCP connection.
func dialWS(t *testing.T, rawURL string) net.Conn {
	t.Helper()
	url := strings.Replace(rawURL, "http://", "", 1)
	path := "/"
	if idx := strings.Index(url, "/"); idx >= 0 {
		path = url[idx:]
		url = url[:idx]
	}

	conn, err := net.DialTimeout("tcp", url, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + url + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		t.Fatalf("write handshake: %v", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		conn.Close()
		t.Fatalf("read handshake response: %v", err)
	}
	if resp.StatusCode != 101 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		conn.Close()
		t.Fatalf("expected 101, got %d: %s", resp.StatusCode, body)
	}

	return conn
}

// sendClientFrame writes a masked frame (client-to-server per RFC 6455).
func sendClientFrame(t *testing.T, conn net.Conn, fin bool, opcode byte, payload []byte) {
	t.Helper()
	length := len(payload)
	var buf bytes.Buffer

	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	buf.WriteByte(b0)

	// Masked + length
	key := [4]byte{0x12, 0x34, 0x56, 0x78}
	switch {
	case length < 126:
		buf.WriteByte(0x80 | byte(length))
	case length <= 65535:
		buf.WriteByte(0x80 | 126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(length))
		buf.Write(ext[:])
	default:
		buf.WriteByte(0x80 | 127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(length))
		buf.Write(ext[:])
	}

	buf.Write(key[:])

	masked := make([]byte, length)
	copy(masked, payload)
	maskBytes(key, masked)
	buf.Write(masked)

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(buf.Bytes()); err != nil {
		t.Fatalf("send frame: %v", err)
	}
}

// readServerFrame reads an unmasked frame from the server.
func readServerFrame(t *testing.T, conn net.Conn) *frame {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	f, err := readFrame(bufio.NewReader(conn), 0)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return f
}

// --- Concurrent safety and edge case tests ---

func TestConn_ConcurrentWrites(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			// Concurrently write 10 messages — should not panic or corrupt frames
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					msg := fmt.Sprintf("msg-%d", n)
					conn.WriteText(msg)
				}(i)
			}
			wg.Wait()

			// Send close to signal done
			conn.Close(CloseNormalClosure, "done")
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	wsConn := dialWS(t, srv.URL+"/ws")
	defer wsConn.Close()

	// Read all messages until close
	received := 0
	for {
		wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		f, err := readFrame(bufio.NewReader(wsConn), 0)
		if err != nil {
			break // connection closed or error
		}
		if f.opcode == 0x8 {
			break // close frame
		}
		if f.opcode == 0x1 {
			received++
		}
	}

	if received != 10 {
		t.Errorf("received %d messages, want 10", received)
	}
}

func TestUpgrade_InvalidMethod(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Post("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// POST with upgrade headers — should be rejected (WS requires GET)
	url := strings.Replace(srv.URL, "http://", "", 1)
	conn, err := net.DialTimeout("tcp", url, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	req := "POST /ws HTTP/1.1\r\n" +
		"Host: " + url + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"
	conn.Write([]byte(req))

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should be rejected — 400 or 405
	if resp.StatusCode == 101 {
		t.Error("POST should not upgrade to WebSocket")
	}
}

func TestConn_WriteAfterClose(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	var writeErr error
	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			conn.Close(CloseNormalClosure, "closing")
			// Write after close should fail
			writeErr = conn.WriteText("after close")
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	wsConn := dialWS(t, srv.URL+"/ws")
	defer wsConn.Close()

	// Wait for server to close and attempt write
	time.Sleep(100 * time.Millisecond)

	if writeErr == nil {
		t.Error("expected error writing to closed connection")
	}
}

func TestConn_ReadAfterClose(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	upgrader := New(Config{})

	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) {
			// Read one message, then close, then try to read again
			conn.ReadMessage()
			conn.Close(CloseNormalClosure, "closing")
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	wsConn := dialWS(t, srv.URL+"/ws")
	defer wsConn.Close()

	// Send a message, then close
	sendClientFrame(t, wsConn, true, 0x1, []byte("hello"))

	// Read server close frame
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	f, err := readFrame(bufio.NewReader(wsConn), 0)
	if err != nil {
		t.Logf("read after server close: %v (expected)", err)
		return
	}
	if f.opcode == 0x8 {
		// Good — server sent close frame
		return
	}
}

func TestConfig_Defaults(t *testing.T) {
	var cfg Config
	cfg.defaults()

	if cfg.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d, want 4096", cfg.ReadBufferSize)
	}
	if cfg.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d, want 4096", cfg.WriteBufferSize)
	}

	// Non-zero values should be preserved
	cfg2 := Config{ReadBufferSize: 8192, WriteBufferSize: 16384}
	cfg2.defaults()
	if cfg2.ReadBufferSize != 8192 {
		t.Errorf("ReadBufferSize = %d, want 8192 (preserved)", cfg2.ReadBufferSize)
	}
	if cfg2.WriteBufferSize != 16384 {
		t.Errorf("WriteBufferSize = %d, want 16384 (preserved)", cfg2.WriteBufferSize)
	}
}
