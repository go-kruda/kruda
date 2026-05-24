//go:build bench_pprof

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func startPprof() {
	if os.Getenv("BENCH_ENABLE_PPROF") != "1" {
		return
	}
	go func() {
		fmt.Println("[pprof] listening on :6060")
		_ = http.ListenAndServe(":6060", nil)
	}()
}
