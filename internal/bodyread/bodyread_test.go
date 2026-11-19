package bodyread

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

func TestRead(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 5000)
	for _, tc := range []struct {
		name     string
		cl       int64
		limit    int
		wantLen  int
		tooLarge bool
	}{
		{"known length", 5000, 0, 5000, false},
		{"unknown length", -1, 0, 5000, false},
		{"under limit", 5000, 5000, 5000, false},
		{"over limit chunked", -1, 4999, 5000, true},
		{"declared too small", 10, 0, 5000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range []io.Reader{bytes.NewReader(body), iotest.OneByteReader(bytes.NewReader(body)), iotest.HalfReader(bytes.NewReader(body))} {
				got, tooLarge, err := Read(r, tc.cl, tc.limit)
				if err != nil || tooLarge != tc.tooLarge || len(got) != tc.wantLen {
					t.Fatalf("len=%d tooLarge=%v err=%v", len(got), tooLarge, err)
				}
				if !tooLarge && !bytes.Equal(got, body) {
					t.Fatal("body mismatch")
				}
			}
		})
	}
}

func TestReadKeepsPartialDataOnError(t *testing.T) {
	boom := errors.New("boom")
	r := io.MultiReader(bytes.NewReader([]byte("abc")), iotest.ErrReader(boom))
	got, tooLarge, err := Read(r, 10, 0)
	if !errors.Is(err, boom) || tooLarge || string(got) != "abc" {
		t.Fatalf("got %q tooLarge=%v err=%v", got, tooLarge, err)
	}
}

func TestReadEmpty(t *testing.T) {
	got, tooLarge, err := Read(bytes.NewReader(nil), -1, 10)
	if err != nil || tooLarge || len(got) != 0 {
		t.Fatalf("got %q tooLarge=%v err=%v", got, tooLarge, err)
	}
}
