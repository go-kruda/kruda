package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"
)

// Result holds the outcome of a rate limit check.
type Result struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
	RetryAt   time.Duration // time until next allowed request (only when !Allowed)
}

// Store is the interface for rate limit state backends.
type Store interface {
	// Allow checks if a request is allowed for the given key.
	Allow(key string, limit int, window time.Duration) Result
}

// entry holds per-client rate limit state.
type entry struct {
	mu          sync.Mutex
	tokens      float64   // current token count (token bucket)
	last        time.Time // last refill time
	count       int64     // request count in current window (sliding window)
	prevCount   int64     // request count in previous window
	windowStart time.Time // start of current window
}

// MemoryStore is an in-memory rate limit store using sync.Map.
type MemoryStore struct {
	entries sync.Map // map[string]*entry
	stopped atomic.Bool
	done    chan struct{}
}

// NewMemoryStore creates a new in-memory store with background cleanup.
func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	s := &MemoryStore{
		done: make(chan struct{}),
	}
	go s.cleanup(cleanupInterval)
	return s
}

// Stop terminates the background cleanup goroutine.
func (s *MemoryStore) Stop() {
	if s.stopped.CompareAndSwap(false, true) {
		close(s.done)
	}
}

// cleanup periodically removes expired entries.
func (s *MemoryStore) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			s.entries.Range(func(key, value any) bool {
				e := value.(*entry)
				e.mu.Lock()
				// Remove if idle for 2x window (conservative)
				if now.Sub(e.last) > 2*time.Minute {
					e.mu.Unlock()
					s.entries.Delete(key)
				} else {
					e.mu.Unlock()
				}
				return true
			})
		}
	}
}

// Allow implements the Store interface. Uses token bucket by default.
// For algorithm selection, use the middleware directly.
func (s *MemoryStore) Allow(key string, limit int, window time.Duration) Result {
	return tokenBucketAllow(s.getEntry(key), limit, window)
}

// getEntry returns or creates the entry for a key.
func (s *MemoryStore) getEntry(key string) *entry {
	if v, ok := s.entries.Load(key); ok {
		return v.(*entry)
	}
	e := &entry{}
	actual, _ := s.entries.LoadOrStore(key, e)
	return actual.(*entry)
}
