//go:build linux

package kruda

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

// Socket option constants — defined locally to avoid golang.org/x/sys/unix dependency.
const (
	soReusePort    = 0x0F // SO_REUSEPORT
	tcpFastOpen    = 0x17 // TCP_FASTOPEN
	tcpDeferAccept = syscall.TCP_DEFER_ACCEPT
)

// fastOpenQueueLen is the max pending TFO connections.
const fastOpenQueueLen = 4096

// childEnvKey is the environment variable used to distinguish
// supervisor (unset) from child processes (set to child index).
const childEnvKey = "KRUDA_CHILD_ID"

// gomaxprocsEnvKey passes the GoMaxProcs setting from supervisor to children.
const gomaxprocsEnvKey = "KRUDA_GOMAXPROCS"

// ReuseportListener creates a TCP listener with SO_REUSEPORT enabled,
// plus TCP_DEFER_ACCEPT and TCP_FASTOPEN for reduced latency.
// TCP_DEFER_ACCEPT avoids waking the process until the client sends data.
// TCP_FASTOPEN allows data in the SYN packet, saving 1 RTT for repeat clients.
func ReuseportListener(addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				f := int(fd)
				if e := syscall.SetsockoptInt(f, syscall.SOL_SOCKET, soReusePort, 1); e != nil {
					opErr = e
					return
				}
				_ = syscall.SetsockoptInt(f, syscall.IPPROTO_TCP, tcpDeferAccept, 1)
				_ = syscall.SetsockoptInt(f, syscall.SOL_TCP, tcpFastOpen, fastOpenQueueLen)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}
	return lc.Listen(context.Background(), "tcp", addr)
}

// IsSupervisor returns true if this process is the supervisor (not a child).
func IsSupervisor() bool {
	return os.Getenv(childEnvKey) == ""
}

// IsChild returns true if this process is a forked child.
func IsChild() bool {
	return os.Getenv(childEnvKey) != ""
}

// ChildID returns the child process index (0-based), or -1 if this is the supervisor.
func ChildID() int {
	s := os.Getenv(childEnvKey)
	if s == "" {
		return -1
	}
	id, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return id
}

// Supervisor manages a pool of child processes for prefork mode.
// It re-executes the current binary with KRUDA_CHILD_ID set,
// monitors children, and respawns on crash.
type Supervisor struct {
	// Addr is the listen address (e.g., ":8080").
	Addr string

	// Processes is the number of child processes to fork.
	// 0 = auto (derived from CPUPercent or availableCPUs).
	Processes int

	// CPUPercent limits CPU usage as a percentage (1–99).
	// Ignored if Processes > 0.
	CPUPercent float64

	// GoMaxProcs sets GOMAXPROCS per child process.
	// 0 = 1 (default, optimal for CPU-bound).
	// Set to 2 for mixed CPU+DB workloads.
	GoMaxProcs int

	mu       sync.Mutex
	children []*os.Process
	stopping bool
}

// Run starts the supervisor loop. It forks Processes child processes,
// monitors them, and respawns any that crash. On SIGTERM/SIGINT, it
// propagates the signal to all children and waits for them to exit.
// Run blocks until all children have exited.
func (s *Supervisor) Run() error {
	if s.GoMaxProcs <= 0 {
		s.GoMaxProcs = 1
	}
	if s.Processes <= 0 {
		n := resolveCPUs(0, s.CPUPercent)
		// Auto-adjust: total parallelism = Processes × GoMaxProcs ≈ NumCPU
		// e.g. 8 cores + GoMaxProcs=2 → 4 processes
		s.Processes = max(n/s.GoMaxProcs, 1)
	}

	s.children = make([]*os.Process, s.Processes)

	for i := 0; i < s.Processes; i++ {
		proc, err := s.startChild(i)
		if err != nil {
			s.signalAll(syscall.SIGKILL)
			return fmt.Errorf("kruda: turbo: failed to start child %d: %w", i, err)
		}
		s.mu.Lock()
		s.children[i] = proc
		s.mu.Unlock()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	type childExit struct {
		index int
		state *os.ProcessState
		err   error
	}
	exitCh := make(chan childExit, s.Processes)

	for i := 0; i < s.Processes; i++ {
		go func(idx int) {
			s.mu.Lock()
			proc := s.children[idx]
			s.mu.Unlock()
			state, err := proc.Wait()
			exitCh <- childExit{index: idx, state: state, err: err}
		}(i)
	}

	alive := s.Processes
	for alive > 0 {
		select {
		case sig := <-sigCh:
			s.mu.Lock()
			s.stopping = true
			s.mu.Unlock()
			s.signalAll(sig.(syscall.Signal))
			for alive > 0 {
				<-exitCh
				alive--
			}
			return nil

		case exit := <-exitCh:
			s.mu.Lock()
			stopping := s.stopping
			s.mu.Unlock()

			if stopping {
				alive--
				continue
			}

			// Child crashed — respawn.
			proc, err := s.startChild(exit.index)
			if err != nil {
				s.mu.Lock()
				s.stopping = true
				s.mu.Unlock()
				s.signalAll(syscall.SIGTERM)
				for alive > 1 {
					<-exitCh
					alive--
				}
				return fmt.Errorf("kruda: turbo: failed to respawn child %d: %w", exit.index, err)
			}

			s.mu.Lock()
			s.children[exit.index] = proc
			s.mu.Unlock()

			go func(idx int) {
				s.mu.Lock()
				p := s.children[idx]
				s.mu.Unlock()
				state, err := p.Wait()
				exitCh <- childExit{index: idx, state: state, err: err}
			}(exit.index)
		}
	}

	return nil
}

// startChild forks a new child process with KRUDA_CHILD_ID=index.
func (s *Supervisor) startChild(index int) (*os.Process, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build environment: copy current env, set/replace KRUDA_CHILD_ID, KRUDA_WORKERS, KRUDA_GOMAXPROCS.
	env := os.Environ()
	childIDSet := false
	workersSet := false
	gmaxSet := false
	childTarget := childEnvKey + "="
	workersTarget := "KRUDA_WORKERS="
	gmaxTarget := gomaxprocsEnvKey + "="
	for i, e := range env {
		if len(e) >= len(childTarget) && e[:len(childTarget)] == childTarget {
			env[i] = childEnvKey + "=" + strconv.Itoa(index)
			childIDSet = true
		} else if len(e) >= len(workersTarget) && e[:len(workersTarget)] == workersTarget {
			env[i] = "KRUDA_WORKERS=" + strconv.Itoa(s.Processes)
			workersSet = true
		} else if len(e) >= len(gmaxTarget) && e[:len(gmaxTarget)] == gmaxTarget {
			env[i] = gomaxprocsEnvKey + "=" + strconv.Itoa(s.GoMaxProcs)
			gmaxSet = true
		}
	}
	if !childIDSet {
		env = append(env, childEnvKey+"="+strconv.Itoa(index))
	}
	if !workersSet {
		env = append(env, "KRUDA_WORKERS="+strconv.Itoa(s.Processes))
	}
	if !gmaxSet && s.GoMaxProcs > 0 {
		env = append(env, gomaxprocsEnvKey+"="+strconv.Itoa(s.GoMaxProcs))
	}

	proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return nil, err
	}
	return proc, nil
}

// signalAll sends a signal to all live child processes.
func (s *Supervisor) signalAll(sig syscall.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, proc := range s.children {
		if proc != nil {
			_ = proc.Signal(sig)
		}
	}
}

// SetupChild configures the current process as a turbo child.
// Sets GOMAXPROCS based on KRUDA_GOMAXPROCS env (default: 1).
//
// GOMAXPROCS=1 is optimal for CPU-bound workloads (json, plaintext, cached).
// GOMAXPROCS=2 is optimal for mixed CPU+DB workloads (queries, fortunes, updates)
// because it allows goroutine scheduling during I/O wait.
//
// CPU pinning is intentionally skipped — Go's runtime threads (sysmon, GC, network poller)
// would not be pinned, causing cross-core overhead with the pinned request thread.
func SetupChild() {
	if os.Getenv("KRUDA_NO_GOMAXPROCS") == "1" {
		return
	}
	gmax := 1
	if v, err := strconv.Atoi(os.Getenv(gomaxprocsEnvKey)); err == nil && v > 0 {
		gmax = v
	}
	runtime.GOMAXPROCS(gmax)
}
