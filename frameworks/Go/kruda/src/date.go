package main

import (
	"net/http"
	"sync/atomic"
	"time"
)

// dateHeader stores the pre-formatted Date header value as []byte.
// Updated every 1 second by a background goroutine.
var dateHeader atomic.Value

func init() {
	updateDate()
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			updateDate()
		}
	}()
}

// updateDate formats the current UTC time as RFC1123 and stores it.
func updateDate() {
	dateHeader.Store([]byte(time.Now().UTC().Format(http.TimeFormat)))
}

// GetDateHeader returns the cached Date header value as a byte slice.
func GetDateHeader() []byte {
	return dateHeader.Load().([]byte)
}
