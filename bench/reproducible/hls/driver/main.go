package main

import (
	"encoding/json"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
)

const segmentBytes = 3_000_000

func main() {
	if len(os.Args) != 5 {
		log.Fatal("usage: phase-load http://127.0.0.1:PORT VIEWERS WARM_CYCLES MEASURE_CYCLES")
	}
	base, err := url.Parse(os.Args[1])
	if err != nil || base.Scheme != "http" || base.Hostname() != "127.0.0.1" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" || base.Port() == "" {
		log.Fatal("expected a loopback HTTP origin without a path")
	}
	port, err := strconv.Atoi(base.Port())
	if err != nil || port < 1 || port > 65535 {
		log.Fatal("invalid loopback port")
	}
	viewers, warmCycles, measureCycles := positiveInt(os.Args[2]), positiveInt(os.Args[3]), positiveInt(os.Args[4])
	runLoad("http://"+net.JoinHostPort("127.0.0.1", base.Port()), viewers, warmCycles, measureCycles, false)
}

func positiveInt(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		log.Fatalf("expected a positive integer, got %q", value)
	}
	return n
}

func printJSON(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		log.Fatal(err)
	}
}
