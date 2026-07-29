// Cold-start driver: spawn a server binary, time exec → first successful
// response, and sample RSS. Median of N spawns.
//
// This is a Go program rather than a shell loop for two reasons the shell
// version in footprint.sh gets wrong. It starts the clock immediately before
// exec, inside the process that spawns, so the measurement includes runtime
// init — which is exactly where a JIT's warm-up lands. And it polls with a
// tight TCP dial instead of curl at 2 ms granularity, which on a ~6 ms
// measurement is a 30% error bar.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	bin := flag.String("bin", "", "server binary to spawn")
	port := flag.Int("port", 3500, "port the server listens on")
	routes := flag.Int("routes", 25, "ROUTES passed to the server")
	spawns := flag.Int("spawns", 15, "number of spawns to measure")
	label := flag.String("label", "", "label for the output row")
	flag.Parse()
	if *bin == "" {
		fmt.Fprintln(os.Stderr, "-bin is required")
		os.Exit(2)
	}

	var readyMs, rssAtReady, rssSettled []float64
	for i := 0; i < *spawns; i++ {
		r, ra, rs, err := once(*bin, *port+i, *routes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spawn %d: %v\n", i, err)
			os.Exit(1)
		}
		readyMs = append(readyMs, r)
		rssAtReady = append(rssAtReady, ra)
		rssSettled = append(rssSettled, rs)
	}

	fmt.Printf("%-22s routes=%-3d spawns=%d  ready_ms=%.2f (min %.2f max %.2f)  rss_at_ready_mb=%.2f  rss_settled_mb=%.2f\n",
		*label, *routes, *spawns,
		median(readyMs), min(readyMs), max(readyMs),
		median(rssAtReady), median(rssSettled))
}

// once spawns the server, returns milliseconds from exec to the first 200 on
// /ready, RSS at that instant, and RSS after a settle window.
func once(bin string, port, routes int) (ms, rssReady, rssSettled float64, err error) {
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"PORT="+strconv.Itoa(port),
		"ROUTES="+strconv.Itoa(routes),
	)
	cmd.Stdout, cmd.Stderr = nil, nil

	start := time.Now()
	if err = cmd.Start(); err != nil {
		return 0, 0, 0, err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	addr := "127.0.0.1:" + strconv.Itoa(port)
	url := "http://" + addr + "/ready"
	client := &http.Client{Timeout: 200 * time.Millisecond}

	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return 0, 0, 0, fmt.Errorf("never became ready on %s", addr)
		}
		conn, derr := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if derr != nil {
			continue // no sleep: the whole point is to catch readiness immediately
		}
		conn.Close()
		resp, rerr := client.Get(url)
		if rerr != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			break
		}
	}
	ms = float64(time.Since(start).Microseconds()) / 1000

	rssReady = rssMB(cmd.Process.Pid)
	time.Sleep(500 * time.Millisecond)
	rssSettled = rssMB(cmd.Process.Pid)
	return ms, rssReady, rssSettled, nil
}

// rssMB reads VmRSS from /proc. Returns 0 where /proc is not available, which
// keeps the driver usable on macOS for smoke-testing even though the numbers
// that matter are measured on Linux.
func rssMB(pid int) float64 {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			return 0
		}
		kb, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			return 0
		}
		return kb / 1024
	}
	return 0
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func min(v []float64) float64 { s := append([]float64(nil), v...); sort.Float64s(s); return s[0] }
func max(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)-1]
}
