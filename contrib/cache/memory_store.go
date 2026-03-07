package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// memoryEntry wraps a cached response with expiration for the in-memory store.
type memoryEntry struct {
	resp      *CachedResponse
	expiresAt time.Time
}

// MemoryStore is a thread-safe, in-memory cache store with TTL-based expiration.
// It includes a background cleanup goroutine that removes expired entries at a
// configurable interval, plus a MaxEntries limit that evicts the oldest entry
// when the capacity is reached.
//
// Suitable for single-instance deployments and development.
// For multi-instance or production use, implement Store with Redis or similar.
type MemoryStore struct {
	mu              sync.RWMutex
	entries         map[string]memoryEntry
	maxEntries      int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	stopped         atomic.Bool
}

// NewMemoryStore creates a new in-memory cache store.
// maxEntries limits the number of cached responses (0 = unlimited).
// An optional cleanup interval can be provided (default: 1 minute).
// Call Close() when the store is no longer needed to stop the cleanup goroutine.
func NewMemoryStore(maxEntries int, cleanupInterval ...time.Duration) *MemoryStore {
	interval := time.Minute
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		interval = cleanupInterval[0]
	}

	s := &MemoryStore{
		entries:         make(map[string]memoryEntry),
		maxEntries:      maxEntries,
		cleanupInterval: interval,
		stopCleanup:     make(chan struct{}),
	}

	go s.cleanupLoop()

	return s
}

// Get retrieves a cached response by key. Returns nil, nil if not found or expired.
func (s *MemoryStore) Get(key string) (*CachedResponse, error) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		// Expired -- remove lazily.
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return nil, nil
	}

	return entry.resp, nil
}

// Set stores a cached response with the given TTL.
// If maxEntries is exceeded, the oldest entry (by CachedAt) is evicted.
func (s *MemoryStore) Set(key string, resp *CachedResponse, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict if at capacity and key is new.
	if s.maxEntries > 0 && len(s.entries) >= s.maxEntries {
		if _, exists := s.entries[key]; !exists {
			s.evictOldest()
		}
	}

	s.entries[key] = memoryEntry{
		resp:      resp,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Delete removes a cached response from the store.
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}

// Close stops the background cleanup goroutine.
// Safe to call multiple times.
func (s *MemoryStore) Close() {
	if s.stopped.CompareAndSwap(false, true) {
		close(s.stopCleanup)
	}
}

// Len returns the number of entries (including possibly expired ones).
// Useful for testing.
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	n := len(s.entries)
	s.mu.RUnlock()
	return n
}

// evictOldest removes the oldest entry by CachedAt, or the first expired entry found.
// Must be called with s.mu held.
func (s *MemoryStore) evictOldest() {
	now := time.Now()
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, e := range s.entries {
		// Prefer evicting expired entries first.
		if now.After(e.expiresAt) {
			delete(s.entries, k)
			return
		}
		if first || e.resp.CachedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.resp.CachedAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(s.entries, oldestKey)
	}
}

// cleanupLoop periodically removes expired entries.
func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.removeExpired()
		case <-s.stopCleanup:
			return
		}
	}
}

// removeExpired removes all expired entries from the store.
func (s *MemoryStore) removeExpired() {
	now := time.Now()
	s.mu.Lock()
	for k, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, k)
		}
	}
	s.mu.Unlock()
}
