package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptrace"
	"sort"
	"strconv"
	"sync"
	"time"
)

const segmentPeriod = 6 * time.Second

type fetchResult struct {
	latencyMS, dispatchMS, ttfbMS float64
	bytes                         int64
	code                          int
	err                           string
	originID                      string
	reused                        bool
}

func runLoad(base string, viewers, warmCycles, measureCycles int, closeConnections bool) {
	transport := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		DisableKeepAlives:   closeConnections,
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	var urls [8]string
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/media/%d.ts", base, i)
	}
	results := make([]fetchResult, (warmCycles+measureCycles)*viewers)
	warmRequests := warmCycles * viewers
	var active sync.WaitGroup
	inflight := make(chan struct{}, 512)
	start := time.Now().Add(250 * time.Millisecond)
	measurementStart := start.Add(time.Duration(warmCycles) * segmentPeriod)
	measurementEnd := measurementStart.Add(time.Duration(measureCycles) * segmentPeriod)
	next := 0
	for next < len(results) {
		elapsed := time.Duration(next) * segmentPeriod / time.Duration(viewers)
		if wait := time.Until(start.Add(elapsed)); wait > 0 {
			time.Sleep(max(wait, time.Millisecond))
		}
		now := time.Now()
		for next < len(results) {
			due := start.Add(time.Duration(next) * segmentPeriod / time.Duration(viewers))
			if due.After(now) {
				break
			}
			index := next
			select {
			case inflight <- struct{}{}:
				active.Go(func() {
					defer func() { <-inflight }()
					results[index] = fetch(client, urls[index%len(urls)], due)
				})
			default:
				delay := millis(time.Since(due))
				results[index] = fetchResult{
					err: "client inflight guard", dispatchMS: delay, latencyMS: max(delay, 1_001),
				}
			}
			next++
		}
	}
	active.Wait()

	ids := make(map[string]struct{}, len(results))
	warm := summarize(results[:warmRequests], ids)
	measure := summarize(results[warmRequests:], ids)
	measure["warm"] = warm
	measure["viewers"] = viewers
	measure["max_inflight"] = cap(inflight)
	measure["warm_start_ns"] = start.UnixNano()
	measure["measure_start_ns"] = measurementStart.UnixNano()
	measure["measure_end_ns"] = measurementEnd.UnixNano()
	measure["unique_origin_ids_all"] = len(ids)
	printJSON(measure)
}

func summarize(results []fetchResult, allIDs map[string]struct{}) map[string]any {
	latencies := make([]float64, 0, len(results))
	dispatches := make([]float64, 0, len(results))
	ttfbs := make([]float64, 0, len(results))
	ids := make(map[string]struct{}, len(results))
	codes := make(map[string]int)
	errs := make(map[string]int)
	var bytes int64
	ok, duplicate, reused := 0, 0, 0
	for _, result := range results {
		latencies = append(latencies, result.latencyMS)
		dispatches = append(dispatches, result.dispatchMS)
		if result.ttfbMS > 0 {
			ttfbs = append(ttfbs, result.ttfbMS)
		}
		bytes += result.bytes
		codes[strconv.Itoa(result.code)]++
		if result.reused {
			reused++
		}
		if result.err != "" {
			errs[result.err]++
			continue
		}
		ok++
		if _, exists := ids[result.originID]; exists {
			duplicate++
		}
		ids[result.originID] = struct{}{}
		allIDs[result.originID] = struct{}{}
	}
	return map[string]any{
		"attempts": len(results), "ok": ok, "bad": len(results) - ok,
		"bytes": bytes, "origin_ids": len(ids), "duplicate_origin_ids": duplicate,
		"status": codes, "error_kinds": errs, "connections_reused": reused,
		"p50_ms": percentile(latencies, .50), "p99_ms": percentile(latencies, .99),
		"p99_first_byte_ms": percentile(ttfbs, .99), "p99_dispatch_ms": percentile(dispatches, .99),
	}
}

func fetch(client *http.Client, url string, due time.Time) (result fetchResult) {
	start := time.Now()
	result.dispatchMS = millis(start.Sub(due))
	defer func() { result.latencyMS = millis(time.Since(due)) }()
	trace := &httptrace.ClientTrace{
		GotConn:              func(info httptrace.GotConnInfo) { result.reused = info.Reused },
		GotFirstResponseByte: func() { result.ttfbMS = millis(time.Since(due)) },
	}
	ctx, cancel := context.WithDeadline(context.Background(), due.Add(5*time.Second))
	defer cancel()
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.err = "deadline"
		} else {
			result.err = "request error"
		}
		return
	}
	defer res.Body.Close()
	result.code = res.StatusCode
	result.originID = res.Header.Get("X-Origin-Request")
	result.bytes, err = io.Copy(io.Discard, res.Body)
	switch {
	case res.StatusCode != http.StatusOK:
		result.err = "unexpected status"
	case err != nil:
		result.err = "body read error"
	case result.bytes != segmentBytes:
		result.err = "short or long body"
	case result.originID == "":
		result.err = "missing origin id"
	}
	return
}

func millis(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func percentile(values []float64, proportion float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[int(math.Ceil(float64(len(values))*proportion))-1]
}
