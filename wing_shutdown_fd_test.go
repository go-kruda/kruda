//go:build linux || darwin

package kruda

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWing_TakeoverShutdownNoFdDoubleClose reproduces the takeover server-
// shutdown fd double-close. cleanup()'s doneCh drain discarded the takeover
// *os.File without closing it while the cleanup loop raw-closed the same fd, so
// the File's finalizer later closed a recycled fd ("bad file descriptor").
//
// Each iteration parks a takeover goroutine on a keep-alive read, shuts the
// server down (leaking the File), grabs the just-freed fd numbers with victim
// listeners, then forces GC so the leaked finalizer fires now. If the finalizer
// closes a recycled fd, a victim listener's fd is yanked and Accept returns
// EBADF instead of timing out.
func TestWing_TakeoverShutdownNoFdDoubleClose(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	for i := 0; i < 60; i++ {
		cfg := WingConfig{Workers: 1, DefaultPreset: DB} // DB = takeover dispatch
		addr, stop := startWingServerWithConfig(t, cfg, handler)

		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("iter %d dial: %v", i, err)
		}
		fmt.Fprintf(c, "GET / HTTP/1.1\r\nHost: x\r\n\r\n")
		br := bufio.NewReader(c)
		if _, err := br.ReadString('\n'); err != nil { // status line => handler ran, takeover active
			t.Fatalf("iter %d read status: %v", i, err)
		}
		// Let the takeover goroutine finish the write and park on the next read.
		time.Sleep(2 * time.Millisecond)

		stop() // Shutdown: SHUT_RD -> takeover exits -> doneMsg{file} drained (leaked)
		c.Close()

		// Occupy the just-freed fd numbers before the finalizer runs.
		victims := make([]*net.TCPListener, 0, 12)
		for k := 0; k < 12; k++ {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("iter %d victim listen: %v", i, err)
			}
			victims = append(victims, ln.(*net.TCPListener))
		}

		// Force the leaked takeover File's finalizer to run now.
		runtime.GC()
		runtime.GC()
		time.Sleep(2 * time.Millisecond) // give the finalizer goroutine a turn

		for _, ln := range victims {
			_ = ln.SetDeadline(time.Now().Add(time.Millisecond))
			if _, aerr := ln.Accept(); aerr != nil && !errors.Is(aerr, os.ErrDeadlineExceeded) {
				t.Fatalf("iter %d: victim listener fd closed by stale takeover finalizer: %v", i, aerr)
			}
		}
		for _, ln := range victims {
			ln.Close()
		}
	}
}
