package ws

import (
	"encoding/binary"
	"fmt"
	"io"
)

// frame represents a single WebSocket frame per RFC 6455 §5.2.
type frame struct {
	fin     bool
	opcode  byte
	masked  bool
	payload []byte
}

// readFrame reads a single WebSocket frame from r.
// maxSize limits payload allocation to prevent OOM attacks.
// Pass maxSize <= 0 to disable the limit (not recommended for production).
func readFrame(r io.Reader, maxSize int64) (*frame, error) {
	// Read first 2 bytes: FIN + opcode, MASK + payload length
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}

	f := &frame{
		fin:    hdr[0]&0x80 != 0,
		opcode: hdr[0] & 0x0F,
		masked: hdr[1]&0x80 != 0,
	}

	// RSV bits must be 0 (no extensions)
	if hdr[0]&0x70 != 0 {
		return nil, fmt.Errorf("ws: reserved bits set")
	}

	// Payload length
	length := uint64(hdr[1] & 0x7F)
	switch {
	case length == 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case length == 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
		if length>>63 != 0 {
			return nil, fmt.Errorf("ws: payload length overflow")
		}
	}

	// Guard against OOM: reject frames exceeding maxSize before allocating.
	// Control frames (opcode >= 0x8) are always <= 125 bytes per RFC 6455 §5.5,
	// so we only enforce this for data frames.
	if maxSize > 0 && f.opcode < 0x8 && length > uint64(maxSize) {
		return nil, fmt.Errorf("ws: frame payload %d exceeds max size %d", length, maxSize)
	}

	// Masking key (4 bytes if masked)
	var maskKey [4]byte
	if f.masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return nil, err
		}
	}

	// Read payload
	if length > 0 {
		f.payload = make([]byte, length)
		if _, err := io.ReadFull(r, f.payload); err != nil {
			return nil, err
		}
		// Unmask payload
		if f.masked {
			maskBytes(maskKey, f.payload)
		}
	}

	return f, nil
}

// writeFrame writes a single WebSocket frame to w.
// Server-to-client frames are NOT masked per RFC 6455 §5.1.
func writeFrame(w io.Writer, fin bool, opcode byte, payload []byte) error {
	// Calculate frame size
	length := len(payload)
	headerSize := 2
	if length >= 126 && length <= 65535 {
		headerSize += 2
	} else if length > 65535 {
		headerSize += 8
	}

	buf := make([]byte, headerSize+length)

	// Byte 0: FIN + opcode
	buf[0] = opcode
	if fin {
		buf[0] |= 0x80
	}

	// Byte 1+: payload length (no mask for server frames)
	offset := 2
	switch {
	case length < 126:
		buf[1] = byte(length)
	case length <= 65535:
		buf[1] = 126
		binary.BigEndian.PutUint16(buf[2:4], uint16(length))
		offset = 4
	default:
		buf[1] = 127
		binary.BigEndian.PutUint64(buf[2:10], uint64(length))
		offset = 10
	}

	// Copy payload
	copy(buf[offset:], payload)

	_, err := w.Write(buf)
	return err
}

// maskBytes applies XOR masking per RFC 6455 §5.3.
func maskBytes(key [4]byte, data []byte) {
	for i := range data {
		data[i] ^= key[i%4]
	}
}

// writeCloseFrame writes a close frame with status code and reason.
func writeCloseFrame(w io.Writer, code int, reason string) error {
	var payload []byte
	if code != 0 {
		payload = make([]byte, 2+len(reason))
		binary.BigEndian.PutUint16(payload[:2], uint16(code))
		copy(payload[2:], reason)
	}
	return writeFrame(w, true, 0x8, payload)
}

// parseClosePayload extracts status code and reason from a close frame payload.
func parseClosePayload(payload []byte) (code int, reason string) {
	if len(payload) >= 2 {
		code = int(binary.BigEndian.Uint16(payload[:2]))
		if len(payload) > 2 {
			reason = string(payload[2:])
		}
	}
	return
}
