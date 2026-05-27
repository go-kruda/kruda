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
	port := os.Getenv("PPROF_PORT")
	if port == "" {
		port = "6060"
	}
	go func() {
		fmt.Println("[pprof] listening on :" + port)
		_ = http.ListenAndServe(":"+port, nil)
	}()
}
