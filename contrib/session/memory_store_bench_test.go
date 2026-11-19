package session

import (
	"strconv"
	"testing"
	"time"
)

// BenchmarkMemoryStoreGetParallel reads distinct sessions from many
// goroutines, each holding about 1 KiB of nested values.
func BenchmarkMemoryStoreGetParallel(b *testing.B) {
	s := NewMemoryStore()
	defer s.Close()
	const n = 1024
	for i := range n {
		items := make([]any, 32)
		for j := range items {
			items[j] = map[string]any{"k": "value-" + strconv.Itoa(j)}
		}
		_ = s.Save("s"+strconv.Itoa(i), &SessionData{Values: map[string]any{"user": "u", "items": items}}, time.Hour)
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if d, _ := s.Get("s" + strconv.Itoa(i%n)); d == nil {
				b.Fatal("missing")
			}
			i++
		}
	})
}
