//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "uring-http-probe is only supported on Linux")
	os.Exit(1)
}
