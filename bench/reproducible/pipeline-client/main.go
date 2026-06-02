package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const userAgent = "kruda-pipeline-client/1"

type config struct {
	target      string
	connections int
	depth       int
	duration    time.Duration
	warmup      time.Duration
	timeout     time.Duration
}

type target struct {
	url        string
	addr       string
	hostHeader string
	requestURI string
}

type result struct {
	requests     int64
	socketErrors int64
	non2xx       int64
	latencies    []int64
}

func main() {
	cfg := parseFlags()
	target, err := parseTarget(cfg.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target: %v\n", err)
		os.Exit(2)
	}

	if cfg.warmup > 0 {
		warmupCfg := cfg
		warmupCfg.duration = cfg.warmup
		warm, elapsed := run(warmupCfg, target, false)
		fmt.Printf("Warmup requests: %d\n", warm.requests)
		fmt.Printf("Warmup elapsed: %.3fs\n", elapsed.Seconds())
		fmt.Printf("Warmup socket errors: %d\n", warm.socketErrors)
	}

	measured, elapsed := run(cfg, target, true)
	printSummary(cfg, target, measured, elapsed)
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.target, "url", "http://127.0.0.1:3000/plaintext-handler", "HTTP URL to benchmark")
	flag.IntVar(&cfg.connections, "connections", 128, "parallel TCP connections")
	flag.IntVar(&cfg.depth, "depth", 8, "HTTP/1.1 pipelined requests per connection write")
	flag.DurationVar(&cfg.duration, "duration", 15*time.Second, "measured run duration")
	flag.DurationVar(&cfg.warmup, "warmup", 5*time.Second, "warmup duration before the measured run")
	flag.DurationVar(&cfg.timeout, "timeout", 5*time.Second, "per-read and per-write timeout")
	flag.Parse()

	if cfg.connections < 1 {
		failFlag("connections must be >= 1")
	}
	if cfg.depth < 1 {
		failFlag("depth must be >= 1")
	}
	if cfg.duration <= 0 {
		failFlag("duration must be > 0")
	}
	if cfg.timeout <= 0 {
		failFlag("timeout must be > 0")
	}
	return cfg
}

func failFlag(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	flag.Usage()
	os.Exit(2)
}

func parseTarget(raw string) (target, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return target{}, err
	}
	if parsed.Scheme != "http" {
		return target{}, fmt.Errorf("only http URLs are supported, got %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return target{}, errors.New("URL host is required")
	}

	port := parsed.Port()
	if port == "" {
		port = "80"
	}
	requestURI := parsed.RequestURI()
	if requestURI == "" {
		requestURI = "/"
	}

	return target{
		url:        raw,
		addr:       net.JoinHostPort(parsed.Hostname(), port),
		hostHeader: parsed.Host,
		requestURI: requestURI,
	}, nil
}

func run(cfg config, target target, collectLatencies bool) (result, time.Duration) {
	startCh := make(chan struct{})
	results := make(chan result, cfg.connections)
	var wg sync.WaitGroup
	wg.Add(cfg.connections)

	for id := 0; id < cfg.connections; id++ {
		go func() {
			defer wg.Done()
			<-startCh
			results <- runConnection(cfg, target, collectLatencies)
		}()
	}

	start := time.Now()
	close(startCh)
	wg.Wait()
	elapsed := time.Since(start)
	close(results)

	var total result
	for r := range results {
		total.requests += r.requests
		total.socketErrors += r.socketErrors
		total.non2xx += r.non2xx
		if collectLatencies {
			total.latencies = append(total.latencies, r.latencies...)
		}
	}
	return total, elapsed
}

func runConnection(cfg config, target target, collectLatencies bool) result {
	deadline := time.Now().Add(cfg.duration)
	req := buildRequest(target)
	batch := strings.Repeat(req, cfg.depth)
	var out result
	if collectLatencies {
		out.latencies = make([]int64, 0, estimateLatencyCapacity(cfg))
	}

	var conn net.Conn
	var reader *bufio.Reader
	dial := func() bool {
		if conn != nil {
			_ = conn.Close()
		}
		c, err := net.DialTimeout("tcp", target.addr, cfg.timeout)
		if err != nil {
			out.socketErrors++
			return false
		}
		if tcp, ok := c.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
		conn = c
		reader = bufio.NewReaderSize(c, 64*1024)
		return true
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	if !dial() {
		return out
	}

	for time.Now().Before(deadline) {
		sentAt := time.Now()
		_ = conn.SetWriteDeadline(sentAt.Add(cfg.timeout))
		if _, err := io.WriteString(conn, batch); err != nil {
			out.socketErrors++
			if !dial() {
				return out
			}
			continue
		}

		batchOK := true
		for i := 0; i < cfg.depth; i++ {
			_ = conn.SetReadDeadline(time.Now().Add(cfg.timeout))
			status, err := readResponse(reader)
			if err != nil {
				out.socketErrors++
				batchOK = false
				break
			}
			out.requests++
			if status < 200 || status >= 300 {
				out.non2xx++
			}
			if collectLatencies {
				out.latencies = append(out.latencies, time.Since(sentAt).Nanoseconds())
			}
		}

		if !batchOK && !dial() {
			return out
		}
	}

	return out
}

func estimateLatencyCapacity(cfg config) int {
	const defaultPerConnection = 4096
	if cfg.depth > defaultPerConnection {
		return cfg.depth
	}
	return defaultPerConnection
}

func buildRequest(target target) string {
	return "GET " + target.requestURI + " HTTP/1.1\r\n" +
		"Host: " + target.hostHeader + "\r\n" +
		"User-Agent: " + userAgent + "\r\n" +
		"Accept: */*\r\n" +
		"Connection: keep-alive\r\n\r\n"
}

func readResponse(reader *bufio.Reader) (int, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return 0, err
	}
	status, err := parseStatus(line)
	if err != nil {
		return 0, err
	}

	contentLength := int64(-1)
	chunked := false
	for {
		line, err = reader.ReadSlice('\n')
		if err != nil {
			return 0, err
		}
		if isBlankLine(line) {
			break
		}
		name, value, ok := splitHeader(line)
		if !ok {
			continue
		}
		if asciiEqualFold(name, []byte("Content-Length")) {
			n, err := strconv.ParseInt(string(bytes.TrimSpace(value)), 10, 64)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("bad Content-Length header: %q", string(value))
			}
			contentLength = n
			continue
		}
		if asciiEqualFold(name, []byte("Transfer-Encoding")) && asciiContainsFold(value, []byte("chunked")) {
			chunked = true
		}
	}

	if chunked {
		return status, drainChunked(reader)
	}
	if contentLength >= 0 {
		if contentLength == 0 {
			return status, nil
		}
		_, err := io.CopyN(io.Discard, reader, contentLength)
		return status, err
	}
	return status, errors.New("response has no Content-Length or chunked Transfer-Encoding")
}

func parseStatus(line []byte) (int, error) {
	firstSpace := bytes.IndexByte(line, ' ')
	if firstSpace < 0 {
		return 0, fmt.Errorf("bad status line: %q", string(line))
	}
	value := bytes.TrimSpace(line[firstSpace+1:])
	if len(value) < 3 {
		return 0, fmt.Errorf("bad status line: %q", string(line))
	}
	status, err := strconv.Atoi(string(value[:3]))
	if err != nil {
		return 0, fmt.Errorf("bad status code: %q", string(value[:3]))
	}
	return status, nil
}

func splitHeader(line []byte) ([]byte, []byte, bool) {
	sep := bytes.IndexByte(line, ':')
	if sep <= 0 {
		return nil, nil, false
	}
	return bytes.TrimSpace(line[:sep]), bytes.TrimSpace(line[sep+1:]), true
}

func isBlankLine(line []byte) bool {
	return len(line) == 1 && line[0] == '\n' || len(line) == 2 && line[0] == '\r' && line[1] == '\n'
}

func drainChunked(reader *bufio.Reader) error {
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			return err
		}
		sizeText := bytes.TrimSpace(line)
		if semi := bytes.IndexByte(sizeText, ';'); semi >= 0 {
			sizeText = sizeText[:semi]
		}
		size, err := strconv.ParseInt(string(sizeText), 16, 64)
		if err != nil || size < 0 {
			return fmt.Errorf("bad chunk size: %q", string(sizeText))
		}
		if size == 0 {
			for {
				line, err = reader.ReadSlice('\n')
				if err != nil {
					return err
				}
				if isBlankLine(line) {
					return nil
				}
			}
		}
		if _, err := io.CopyN(io.Discard, reader, size+2); err != nil {
			return err
		}
	}
}

func asciiEqualFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func asciiContainsFold(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if asciiEqualFold(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func printSummary(cfg config, target target, r result, elapsed time.Duration) {
	latencies := r.latencies
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	rps := float64(r.requests) / elapsed.Seconds()
	fmt.Println("Pipeline benchmark")
	fmt.Printf("URL: %s\n", target.url)
	fmt.Printf("Connections: %d\n", cfg.connections)
	fmt.Printf("Pipeline depth: %d\n", cfg.depth)
	fmt.Printf("Duration: %.3fs\n", elapsed.Seconds())
	fmt.Printf("Requests: %d\n", r.requests)
	fmt.Printf("Requests/sec: %.2f\n", rps)
	fmt.Printf("Latency p50: %.3fms\n", nanosToMillis(percentile(latencies, 50)))
	fmt.Printf("Latency p90: %.3fms\n", nanosToMillis(percentile(latencies, 90)))
	fmt.Printf("Latency p99: %.3fms\n", nanosToMillis(percentile(latencies, 99)))
	fmt.Printf("Latency max: %.3fms\n", nanosToMillis(maxLatency(latencies)))
	fmt.Printf("Socket errors: %d\n", r.socketErrors)
	fmt.Printf("Non-2xx responses: %d\n", r.non2xx)
}

func percentile(sorted []int64, pct int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted)*pct + 99) / 100
	if idx < 1 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}

func maxLatency(sorted []int64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[len(sorted)-1]
}

func nanosToMillis(ns int64) float64 {
	return float64(ns) / float64(time.Millisecond)
}
