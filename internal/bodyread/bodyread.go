// Package bodyread reads request bodies into one buffer sized from the
// declared Content-Length.
package bodyread

import "io"

// maxPrealloc bounds the first buffer, so a declared length that the body
// never delivers costs at most this much up front.
const maxPrealloc = 1 << 20

// Read reads r to EOF. A positive contentLength sizes the first buffer. A
// positive limit stops reading once more than limit bytes have arrived and
// reports tooLarge; Read never holds more than limit+1 bytes plus growth
// headroom in that case. Partial data is returned with a read error.
func Read(r io.Reader, contentLength int64, limit int) (b []byte, tooLarge bool, err error) {
	size := 512
	if contentLength > 0 {
		size = int(min(contentLength, maxPrealloc)) + 1 // +1 leaves room to read EOF without growing
	}
	if limit > 0 && size > limit+1 {
		size = limit + 1
	}
	b = make([]byte, 0, size)
	for {
		if len(b) == cap(b) {
			b = append(b, 0)[:len(b)]
		}
		end := cap(b)
		if limit > 0 && end > limit+1 {
			end = limit + 1
		}
		n, rerr := r.Read(b[len(b):end])
		b = b[:len(b)+n]
		if limit > 0 && len(b) > limit {
			return b, true, nil
		}
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			return b, false, rerr
		}
	}
}
