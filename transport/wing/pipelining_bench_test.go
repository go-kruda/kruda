//go:build linux || darwin

package wing

import (
	"strconv"
	"strings"
	"testing"
)

// ============================================================================
// Pipelining Benchmarks
//
// Measures the cost of parsing N pipelined requests from a single buffer,
// simulating the transport's handleRecv → processPipelined chain.
// ============================================================================

// buildPipelinedGETs creates a buffer with n pipelined GET requests.
func buildPipelinedGETs(n int) []byte {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("GET /path/" + strconv.Itoa(i) + " HTTP/1.1\r\nHost: localhost\r\nAccept: text/plain\r\n\r\n")
	}
	return []byte(sb.String())
}

// buildPipelinedPOSTs creates a buffer with n pipelined POST requests.
func buildPipelinedPOSTs(n int) []byte {
	body := `{"id":1,"name":"bench","active":true}`
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("POST /api/items HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: ")
		sb.WriteString(strconv.Itoa(len(body)))
		sb.WriteString("\r\n\r\n")
		sb.WriteString(body)
	}
	return []byte(sb.String())
}

// parsePipelineAll iteratively parses all requests from buf,
// simulating the buffer-shift pattern used by the transport.
func parsePipelineAll(buf []byte) int {
	n := 0
	for len(buf) > 0 {
		_, consumed, ok := parseHTTPRequest(buf, noLimits)
		if !ok {
			break
		}
		buf = buf[consumed:]
		n++
	}
	return n
}

// --- Pipeline depth 1 (baseline, no pipelining overhead) ---

func BenchmarkPipelineGET_1(b *testing.B) {
	data := buildPipelinedGETs(1)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

func BenchmarkPipelinePOST_1(b *testing.B) {
	data := buildPipelinedPOSTs(1)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

// --- Pipeline depth 10 ---

func BenchmarkPipelineGET_10(b *testing.B) {
	data := buildPipelinedGETs(10)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

func BenchmarkPipelinePOST_10(b *testing.B) {
	data := buildPipelinedPOSTs(10)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

// --- Pipeline depth 100 ---

func BenchmarkPipelineGET_100(b *testing.B) {
	data := buildPipelinedGETs(100)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

func BenchmarkPipelinePOST_100(b *testing.B) {
	data := buildPipelinedPOSTs(100)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		parsePipelineAll(data)
	}
}

// --- Full cycle benchmark (parse + build response) ---

func BenchmarkPipelineFullCycle_10(b *testing.B) {
	data := buildPipelinedGETs(10)
	respBody := []byte("Hello, World!")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		buf := data
		for len(buf) > 0 {
			req, consumed, ok := parseHTTPRequest(buf, noLimits)
			if !ok {
				break
			}
			buf = buf[consumed:]
			_ = req.Path()
			r := acquireResponse()
			r.Header().Set("Content-Type", "text/plain")
			r.Write(respBody)
			_ = r.buildZeroCopy()
			r.buf = nil
			releaseResponse(r)
		}
	}
}

// --- Buffer shift simulation benchmark ---

func BenchmarkPipelineBufferShift_10(b *testing.B) {
	rawData := buildPipelinedGETs(10)
	readBuf := make([]byte, 8192)
	b.ReportAllocs()
	b.SetBytes(int64(len(rawData)))

	for b.Loop() {
		// Simulate: copy all data into readBuf, then parse + shift iteratively.
		readN := copy(readBuf, rawData)

		for readN > 0 {
			_, consumed, ok := parseHTTPRequest(readBuf[:readN], noLimits)
			if !ok {
				break
			}
			remaining := readN - consumed
			if remaining > 0 {
				copy(readBuf, readBuf[consumed:readN])
			}
			readN = remaining
		}
	}
}
