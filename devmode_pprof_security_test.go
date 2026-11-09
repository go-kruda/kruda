package kruda

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPprofRequiresExplicitDevMode(t *testing.T) {
	t.Setenv("KRUDA_ENV", "development")
	for _, tc := range []struct {
		name    string
		options []Option
		want    int
	}{
		{"environment only", nil, 404},
		{"explicit enable", []Option{WithDevMode(true)}, 200},
		{"explicit disable", []Option{WithDevMode(false)}, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(append([]Option{NetHTTP()}, tc.options...)...)
			app.Compile()
			resp := httptest.NewRecorder()
			app.ServeHTTP(resp, httptest.NewRequest("GET", "/debug/pprof/cmdline", nil))
			if resp.Code != tc.want {
				t.Fatalf("status=%d want=%d", resp.Code, tc.want)
			}
		})
	}
}

func TestDevModeStartupWarning(t *testing.T) {
	t.Setenv("KRUDA_ENV", "development")
	for _, startup := range []string{"Listen", "Serve"} {
		for _, enabled := range []bool{false, true} {
			t.Run(startup+map[bool]string{true: "/enabled", false: "/disabled"}[enabled], func(t *testing.T) {
				var log bytes.Buffer
				tr := &startupProbeTransport{t: t}
				app := New(WithTransport(tr), WithDevMode(enabled), WithLogger(slog.New(slog.NewTextHandler(&log, nil))))
				var err error
				if startup == "Listen" {
					err = app.Listen("127.0.0.1:0")
				} else {
					ln, e := net.Listen("tcp", "127.0.0.1:0")
					if e != nil {
						t.Fatal(e)
					}
					defer ln.Close()
					err = app.Serve(ln)
				}
				if !errors.Is(err, errStartupTransportDone) {
					t.Fatalf("startup err=%v", err)
				}
				want := 0
				if enabled {
					want = 1
				}
				if n := strings.Count(log.String(), "level=WARN"); n != want {
					t.Fatalf("warnings=%d want=%d log=%s", n, want, log.String())
				}
			})
		}
	}
}
