// bench/transport-compare — Wing vs fasthttp transport benchmark
//
// This is a standalone benchmark binary that starts both transports
// and runs identical workloads against each, reporting latency and throughput.
//
// Usage:
//   go run . [flags]
//
// Flags:
//   -duration  Benchmark duration per test (default: 10s)
//   -conns     Concurrent connections (default: 256)
//   -workers   Workers (default: 0 = NumCPU)
//   -pipeline  HTTP pipeline depth (default: 16)
//
// Requirements:
//   - Linux (epoll), macOS (kqueue)
//   - wrk installed (apt install wrk) for external benchmarks
//   - Run as: go run . OR go build -o bench && ./bench

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kruda "github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport/wing"
)

// ── Flags ────────────────────────────────────────────────────────────────────

var (
	duration  = flag.Duration("duration", 10*time.Second, "benchmark duration per test")
	conns     = flag.Int("conns", 256, "concurrent connections")
	workers   = flag.Int("workers", 0, "Workers (0=NumCPU)")
	pipeline  = flag.Int("pipeline", 16, "wrk pipeline depth")
	useWrk    = flag.Bool("wrk", false, "use wrk for external benchmarking (must be installed)")
	warmup    = flag.Duration("warmup", 2*time.Second, "warmup duration before measurement")
	bodySize  = flag.Int("body", 0, "POST body size in bytes (0=GET only)")
	scenarios = flag.String("scenarios", "all", "comma-separated: plaintext,json,echo,all")
)

// ── Scenario definitions ─────────────────────────────────────────────────────

type scenario struct {
	name        string
	method      string
	path        string
	contentType string
	body        []byte
}

func getScenarios() []scenario {
	all := []scenario{
		{name: "plaintext", method: "GET", path: "/plaintext"},
		{name: "json", method: "GET", path: "/json"},
	}
	if *bodySize > 0 {
		body := bytes.Repeat([]byte("A"), *bodySize)
		all = append(all, scenario{
			name:        fmt.Sprintf("echo-%dB", *bodySize),
			method:      "POST",
			path:        "/echo",
			contentType: "application/octet-stream",
			body:        body,
		})
	} else {
		// Default echo test with small body
		all = append(all, scenario{
			name:        "echo-100B",
			method:      "POST",
			path:        "/echo",
			contentType: "application/octet-stream",
			body:        bytes.Repeat([]byte("A"), 100),
		})
	}

	if *scenarios == "all" {
		return all
	}

	selected := strings.Split(*scenarios, ",")
	var result []scenario
	for _, s := range all {
		for _, sel := range selected {
			if strings.TrimSpace(sel) == s.name || strings.HasPrefix(s.name, strings.TrimSpace(sel)) {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// ── Handlers (identical for both transports) ────────────────────────────────

var (
	helloJSON  = []byte(`{"message":"Hello, World!"}`)
	helloPlain = []byte("Hello, World!")
)

func setupApp(opts ...kruda.Option) *kruda.App {
	app := kruda.New(opts...)

	app.Get("/plaintext", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "text/plain")
		return c.SendBytes(helloPlain)
	})

	app.Get("/json", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "application/json")
		return c.SendBytes(helloJSON)
	})

	app.Post("/echo", func(c *kruda.Ctx) error {
		body, err := c.BodyBytes()
		if err != nil {
			return err
		}
		c.SetHeader("Content-Type", "application/octet-stream")
		return c.SendBytes(body)
	})

	app.Compile()
	return app
}

// ── Transport runners ────────────────────────────────────────────────────────

func getFreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("getFreePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

type transportRunner struct {
	name     string
	addr     string
	shutdown func()
}

func startFastHTTP() *transportRunner {
	port := getFreePort()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	app := setupApp(kruda.FastHTTP())

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(addr)
	}()

	waitReady(addr)

	return &transportRunner{
		name: "fasthttp",
		addr: addr,
		shutdown: func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			app.Shutdown(ctx)
		},
	}
}

func startWing() *transportRunner {
	port := getFreePort()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	wingTransport := wing.New(wing.Config{
		Workers:     *workers,
		RingSize:    4096,
		ReadBufSize: 16384,
	})

	app := setupApp(kruda.WithTransport(wingTransport))

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(addr)
	}()

	waitReady(addr)

	return &transportRunner{
		name: "wing",
		addr: addr,
		shutdown: func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			app.Shutdown(ctx)
		},
	}
}

func waitReady(addr string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.Fatalf("server at %s not ready after 5s", addr)
}

// ── Built-in Go benchmark (no external tools) ───────────────────────────────

type benchResult struct {
	transport string
	scenario  string
	requests  int64
	duration  time.Duration
	errors    int64
	latencies []time.Duration // sampled
	bytes     int64
}

func (r *benchResult) RPS() float64 {
	return float64(r.requests) / r.duration.Seconds()
}

func (r *benchResult) AvgLatency() time.Duration {
	if len(r.latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, l := range r.latencies {
		total += l
	}
	return total / time.Duration(len(r.latencies))
}

func (r *benchResult) P99Latency() time.Duration {
	if len(r.latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(r.latencies))
	copy(sorted, r.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(float64(len(sorted))*0.99)) - 1
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (r *benchResult) Throughput() float64 {
	return float64(r.bytes) / r.duration.Seconds() / 1024 / 1024
}

func runGoBenchmark(tr *transportRunner, sc scenario, dur time.Duration) *benchResult {
	result := &benchResult{
		transport: tr.name,
		scenario:  sc.name,
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *conns,
			MaxIdleConnsPerHost: *conns,
			MaxConnsPerHost:     *conns,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
		},
		Timeout: 10 * time.Second,
	}
	defer client.CloseIdleConnections()

	url := fmt.Sprintf("http://%s%s", tr.addr, sc.path)

	var (
		wg       sync.WaitGroup
		requests atomic.Int64
		errors   atomic.Int64
		bytesRx  atomic.Int64
		mu       sync.Mutex
		lats     []time.Duration
	)

	// Pre-allocate latency sample buffer
	sampleRate := 100 // sample 1 in N
	maxSamples := int(dur.Seconds()) * (*conns) * 1000 / sampleRate
	if maxSamples > 100000 {
		maxSamples = 100000
	}
	lats = make([]time.Duration, 0, maxSamples)

	done := make(chan struct{})
	go func() {
		time.Sleep(dur)
		close(done)
	}()

	start := time.Now()

	for i := 0; i < *conns; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reqNum := 0
			for {
				select {
				case <-done:
					return
				default:
				}

				var req *http.Request
				var err error
				if sc.method == "POST" {
					req, err = http.NewRequest("POST", url, bytes.NewReader(sc.body))
					if err != nil {
						errors.Add(1)
						continue
					}
					req.Header.Set("Content-Type", sc.contentType)
				} else {
					req, err = http.NewRequest("GET", url, nil)
					if err != nil {
						errors.Add(1)
						continue
					}
				}

				t0 := time.Now()
				resp, err := client.Do(req)
				lat := time.Since(t0)

				if err != nil {
					errors.Add(1)
					continue
				}

				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				bytesRx.Add(int64(len(body)))

				if resp.StatusCode != 200 {
					errors.Add(1)
					continue
				}

				requests.Add(1)
				reqNum++

				// Sample latency
				if reqNum%sampleRate == 0 {
					mu.Lock()
					if len(lats) < maxSamples {
						lats = append(lats, lat)
					}
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
	result.duration = time.Since(start)
	result.requests = requests.Load()
	result.errors = errors.Load()
	result.bytes = bytesRx.Load()
	result.latencies = lats

	return result
}

// ── wrk benchmark (external tool) ───────────────────────────────────────────

func runWrkBenchmark(tr *transportRunner, sc scenario, dur time.Duration) string {
	args := []string{
		"-t", fmt.Sprintf("%d", runtime.NumCPU()),
		"-c", fmt.Sprintf("%d", *conns),
		"-d", fmt.Sprintf("%ds", int(dur.Seconds())),
	}

	if *pipeline > 1 {
		// Create a wrk pipeline script
		script := fmt.Sprintf(`
init = function(args)
   local r = wrk.format("%s", "%s")
   req = r
end

request = function()
   return req
end
`, sc.method, sc.path)

		tmpFile := fmt.Sprintf("/tmp/wrk_pipeline_%s.lua", sc.name)
		os.WriteFile(tmpFile, []byte(script), 0644)
		defer os.Remove(tmpFile)
		args = append(args, "-s", tmpFile)
	}

	url := fmt.Sprintf("http://%s%s", tr.addr, sc.path)
	args = append(args, url)

	cmd := exec.Command("wrk", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("wrk error: %v\n%s", err, string(out))
	}
	return string(out)
}

// ── Report printer ──────────────────────────────────────────────────────────

func printHeader() {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 90))
	fmt.Printf("  Kruda Transport Benchmark — Wing vs fasthttp\n")
	fmt.Printf("  Go %s | %s | %d CPU | %d connections | %v duration\n",
		runtime.Version(), runtime.GOARCH, runtime.NumCPU(), *conns, *duration)
	fmt.Println(strings.Repeat("═", 90))
}

func printResults(results []*benchResult) {
	// Group by scenario
	byScenario := map[string][]*benchResult{}
	for _, r := range results {
		byScenario[r.scenario] = append(byScenario[r.scenario], r)
	}

	for _, sc := range getScenarios() {
		pair := byScenario[sc.name]
		if len(pair) < 2 {
			continue
		}

		fmt.Println()
		fmt.Printf("  ┌─ %s (%s %s)\n", sc.name, sc.method, sc.path)
		fmt.Println("  │")
		fmt.Printf("  │  %-12s %12s %12s %12s %12s %8s\n",
			"Transport", "RPS", "Avg Lat", "P99 Lat", "MB/s", "Errors")
		fmt.Printf("  │  %-12s %12s %12s %12s %12s %8s\n",
			"─────────", "───────", "───────", "───────", "────", "──────")

		var fastRPS, wingRPS float64
		for _, r := range pair {
			rps := r.RPS()
			if r.transport == "fasthttp" {
				fastRPS = rps
			} else {
				wingRPS = rps
			}
			fmt.Printf("  │  %-12s %12s %12s %12s %12.1f %8d\n",
				r.transport,
				formatRPS(rps),
				r.AvgLatency().Round(time.Microsecond),
				r.P99Latency().Round(time.Microsecond),
				r.Throughput(),
				r.errors,
			)
		}

		if fastRPS > 0 && wingRPS > 0 {
			ratio := wingRPS / fastRPS
			diff := (ratio - 1) * 100
			symbol := "🔺"
			if diff < 0 {
				symbol = "🔻"
			}
			fmt.Println("  │")
			fmt.Printf("  │  %s Wing vs fasthttp: %.1fx (%.1f%%)\n", symbol, ratio, diff)
		}
		fmt.Println("  └" + strings.Repeat("─", 80))
	}
}

func formatRPS(rps float64) string {
	switch {
	case rps >= 1_000_000:
		return fmt.Sprintf("%.2fM", rps/1_000_000)
	case rps >= 1_000:
		return fmt.Sprintf("%.1fK", rps/1_000)
	default:
		return fmt.Sprintf("%.0f", rps)
	}
}

// ── Main ────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	printHeader()

	// Check platform support (Wing requires Linux, macOS, or Windows)
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		log.Fatal("Wing benchmarks require Linux, macOS, or Windows")
	}

	scenarios := getScenarios()
	if len(scenarios) == 0 {
		log.Fatal("no matching scenarios")
	}

	// ── Start both transports ────────────────────────────────────────────
	fmt.Println("\n  Starting transports...")

	fastRunner := startFastHTTP()
	defer fastRunner.shutdown()
	fmt.Printf("    ✓ fasthttp on %s\n", fastRunner.addr)

	wingRunner := startWing()
	defer wingRunner.shutdown()
	fmt.Printf("    ✓ wing on %s (workers=%d)\n", wingRunner.addr, func() int {
		if *workers == 0 {
			return runtime.NumCPU()
		}
		return *workers
	}())

	var results []*benchResult

	for _, sc := range scenarios {
		fmt.Printf("\n  Benchmarking: %s ...\n", sc.name)

		// ── Warmup ───────────────────────────────────────────────────
		if *warmup > 0 {
			fmt.Printf("    Warming up fasthttp (%v)...\n", *warmup)
			runGoBenchmark(fastRunner, sc, *warmup)
			fmt.Printf("    Warming up wing (%v)...\n", *warmup)
			runGoBenchmark(wingRunner, sc, *warmup)
		}

		// ── Benchmark ────────────────────────────────────────────────
		if *useWrk {
			fmt.Printf("    Running wrk: fasthttp ...\n")
			out := runWrkBenchmark(fastRunner, sc, *duration)
			fmt.Printf("    [fasthttp]\n%s\n", out)

			fmt.Printf("    Running wrk: wing ...\n")
			out = runWrkBenchmark(wingRunner, sc, *duration)
			fmt.Printf("    [wing]\n%s\n", out)
		} else {
			fmt.Printf("    Running: fasthttp (%v, %d conns)...\n", *duration, *conns)
			r := runGoBenchmark(fastRunner, sc, *duration)
			results = append(results, r)
			fmt.Printf("      → %s RPS, avg %v, p99 %v\n",
				formatRPS(r.RPS()), r.AvgLatency().Round(time.Microsecond), r.P99Latency().Round(time.Microsecond))

			// Brief pause between transports
			time.Sleep(1 * time.Second)

			fmt.Printf("    Running: wing (%v, %d conns)...\n", *duration, *conns)
			r = runGoBenchmark(wingRunner, sc, *duration)
			results = append(results, r)
			fmt.Printf("      → %s RPS, avg %v, p99 %v\n",
				formatRPS(r.RPS()), r.AvgLatency().Round(time.Microsecond), r.P99Latency().Round(time.Microsecond))
		}
	}

	// ── Final report ─────────────────────────────────────────────────────
	if !*useWrk {
		printResults(results)
	}

	fmt.Println()
	fmt.Println("  Done.")
}
