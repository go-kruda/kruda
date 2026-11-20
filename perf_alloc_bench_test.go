package kruda

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// retainedKB reports heap still reachable after one GC, which keeps pooled
// objects alive (sync.Pool moves them to its victim cache on the first GC).
// retainedKB reports live heap growth since before. Two GCs empty sync.Pool
// victim caches, as baselineHeap does; with one, a pooled buffer counts only
// when no GC happened after its last Put, so the metric flips run to run.
func retainedKB(b *testing.B, before runtime.MemStats) {
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	b.ReportMetric(float64(int64(after.HeapAlloc)-int64(before.HeapAlloc))/1024, "retained-KB")
}

func baselineHeap() runtime.MemStats {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}

func BenchmarkPerfValidateDive(b *testing.B) {
	type input struct {
		Tags   []string       `validate:"dive,min=2,max=16"`
		Limits map[string]int `validate:"dive,gte=1,lte=100"`
	}
	tags := make([]string, 100)
	for i := range tags {
		tags[i] = "tag-value"
	}
	limits := map[string]int{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		limits[k] = 5
	}
	in := input{Tags: tags, Limits: limits}
	v := NewValidator()
	validators := buildValidators[input](v)
	value := reflect.ValueOf(&in).Elem()
	b.ReportAllocs()
	for b.Loop() {
		if err := validate(validators, value, v.messages); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPerfValidateRules(b *testing.B) {
	type input struct {
		Name   string  `validate:"required,min=2,max=64"`
		Status string  `validate:"required,oneof=active pending closed"`
		Code   string  `validate:"len=6"`
		Age    int     `validate:"gte=18,lte=130"`
		Score  float64 `validate:"gt=0,lt=100"`
		Count  uint32  `validate:"required,gt=0"`
	}
	in := input{Name: "alice", Status: "pending", Code: "ABC123", Age: 30, Score: 50.5, Count: 3}
	v := NewValidator()
	validators := buildValidators[input](v)
	value := reflect.ValueOf(&in).Elem()
	b.ReportAllocs()
	for b.Loop() {
		if err := validate(validators, value, v.messages); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPerfNetHTTPBody(b *testing.B) {
	for _, size := range []int{4 << 10, 64 << 10, 1 << 20} {
		body := bytes.Repeat([]byte("x"), size)
		b.Run(byteSize(size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				req := &http.Request{Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(size)}
				r := &fastNetHTTPRequest{r: req, maxBody: 4 << 20}
				if got, err := r.Body(); err != nil || len(got) != size {
					b.Fatal(err)
				}
			}
		})
	}
}

func byteSize(n int) string {
	if n >= 1<<20 {
		return strconv.Itoa(n>>20) + "MiB"
	}
	return strconv.Itoa(n>>10) + "KiB"
}

// BenchmarkPerfJSONPoolRetention sends large JSON bodies through the pooled
// net/http path, then measures what the pool still holds.
func BenchmarkPerfJSONPoolRetention(b *testing.B) {
	big := map[string]string{"data": strings.Repeat("x", 1<<20)}
	app := New(NetHTTP())
	app.Get("/big", func(c *Ctx) error { return c.JSON(big) })
	app.Compile()
	before := baselineHeap()
	b.ReportAllocs()
	for b.Loop() {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/big", nil))
		if w.Code != 200 {
			b.Fatal(w.Code)
		}
	}
	b.StopTimer()
	retainedKB(b, before)
	runtime.KeepAlive(app)
}
