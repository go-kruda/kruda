//go:build linux || darwin

package kruda

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// TestWingContentLengthIsUnambiguous covers the two paths that send a handler's
// Content-Length: a stream, and a HEAD that wrote no body.
func TestWingContentLengthIsUnambiguous(t *testing.T) {
	for _, tc := range []struct {
		name         string
		values, want []string
	}{
		{"conflict", []string{"4", "7"}, nil},
		{"conflict after eight headers", []string{"4", "4", "4", "4", "4", "4", "4", "4", "7"}, nil},
		{"same value twice", []string{"4", "4"}, []string{"4"}},
		{"not a number", []string{"four"}, nil},
		{"negative", []string{"-1"}, nil},
		{"valid", []string{"4"}, []string{"4"}},
	} {
		for _, route := range []struct {
			name, method string
			preset       Preset
		}{{"stream", "GET", Stream}, {"HEAD", "HEAD", Bolt}} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
					for _, v := range tc.values {
						w.Header().Add("Content-Length", v)
					}
					w.WriteHeader(200)
					if route.method != "HEAD" {
						_, _ = w.Write([]byte("data"))
					}
				})
				addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, DefaultPreset: route.preset}, handler)
				defer stop()
				conn, br := framingDial(t, addr)
				if _, err := io.WriteString(conn, route.method+" /x HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
					t.Fatal(err)
				}
				var got []string
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						t.Fatal(err)
					}
					if line == "\r\n" {
						break
					}
					if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "Content-Length") {
						got = append(got, strings.TrimSpace(value))
					}
				}
				if !slices.Equal(got, tc.want) {
					t.Fatalf("Content-Length lines %q, want %q", got, tc.want)
				}
			})
		}
	}
}
