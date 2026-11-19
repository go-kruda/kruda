// Package bodyread reads request bodies into one buffer, using the declared
// Content-Length to stop growth at the exact size.
package bodyread

import "io"

const (
	initialSize = 512
	maxInt      = int(^uint(0) >> 1)
)

// Read reads r to EOF. A positive limit stops reading once more than limit
// bytes have arrived and reports tooLarge. The buffer starts small and at most
// doubles what has arrived, so a declared Content-Length the client never
// sends costs nothing up front; once data arrives, growth stops at that length
// instead of overshooting it. Partial data is returned with a read error, and
// a read error is returned even when tooLarge is also set, so each caller
// keeps its own precedence.
func Read(r io.Reader, contentLength int64, limit int) (b []byte, tooLarge bool, err error) {
	window := 0 // limit+1 bytes detect an oversize body; 0 means no limit
	if limit > 0 && limit < maxInt {
		window = limit + 1
	}
	want := 0 // the declared size; growth stops there
	if contentLength > 0 && contentLength < int64(maxInt) {
		want = int(contentLength)
	}
	size := initialSize
	if want > 0 && want < size {
		size = want
	}
	if window > 0 && window < size {
		size = window
	}
	b = make([]byte, 0, size)
	var probe [1]byte
	for {
		var n int
		var rerr error
		if len(b) == cap(b) && len(b) == want {
			// Every declared byte is in: look for EOF without growing.
			n, rerr = r.Read(probe[:])
			if n > 0 {
				b = append(b, probe[0])
			}
		} else {
			if len(b) == cap(b) {
				b = grow(b, want, window)
			}
			end := cap(b)
			if window > 0 && end > window {
				end = window
			}
			n, rerr = r.Read(b[len(b):end])
			b = b[:len(b)+n]
		}
		eof := rerr == io.EOF
		if eof {
			rerr = nil
		}
		if window > 0 && len(b) >= window {
			return b, true, rerr
		}
		if eof || rerr != nil {
			return b, false, rerr
		}
	}
}

// grow doubles b, but not past the declared size or the limit window, so a
// body of known length ends in a buffer of its own size.
func grow(b []byte, want, window int) []byte {
	newCap := 2 * cap(b)
	if want > len(b) && newCap > want {
		newCap = want
	}
	if window > 0 && newCap > window {
		newCap = window
	}
	if newCap <= cap(b) {
		newCap = cap(b) + initialSize
	}
	nb := make([]byte, len(b), newCap)
	copy(nb, b)
	return nb
}
