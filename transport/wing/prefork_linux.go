//go:build linux

package wing

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/go-kruda/kruda/transport"
)

const preforkChildEnv = "KRUDA_PREFORK_CHILD"

func (t *Transport) listenAndServePrefork(addr string, handler transport.Handler) error {
	if os.Getenv(preforkChildEnv) != "" {
		return t.preforkChild(addr, handler)
	}
	return t.preforkParent(addr)
}

func (t *Transport) preforkParent(addr string) error {
	n := t.config.Workers
	children := make([]*exec.Cmd, n)
	for i := 0; i < n; i++ {
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), preforkChildEnv+"=1")
		if err := cmd.Start(); err != nil {
			// Kill already started children.
			for j := 0; j < i; j++ {
				children[j].Process.Kill()
			}
			return fmt.Errorf("wing: prefork child %d: %w", i, err)
		}
		children[i] = cmd
	}

	// Wait for signal to shutdown.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	for _, cmd := range children {
		cmd.Process.Signal(syscall.SIGTERM)
	}
	for _, cmd := range children {
		cmd.Wait()
	}
	return nil
}

func (t *Transport) preforkChild(addr string, handler transport.Handler) error {
	runtime.GOMAXPROCS(1)
	t.config.Workers = 1
	t.workers = make([]*worker, 1)
	fd, err := createListenFd(addr)
	if err != nil {
		return err
	}
	w, err := newWorker(0, fd, t.config, handler)
	if err != nil {
		closeFd(fd)
		return err
	}
	t.workers[0] = w
	close(t.ready)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		w.run(&t.shutdown)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	t.shutdown.Store(true)
	w.wake()
	t.wg.Wait()
	return nil
}
