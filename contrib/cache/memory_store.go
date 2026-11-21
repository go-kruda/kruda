package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxBytes is the byte budget of a store from NewMemoryStore. Entry
// count alone does not bound memory: distinct query strings on one large
// endpoint would otherwise fill every slot with a large body.
const DefaultMaxBytes int64 = 64 << 20

// memoryEntry wraps a cached response with expiration for the in-memory store.
type memoryEntry struct {
	resp      *CachedResponse
	expiresAt time.Time
	size      int64
	elem      *list.Element // position in MemoryStore.order
}

// MemoryStore is a thread-safe, in-memory cache store with TTL-based expiration.
// It includes a background cleanup goroutine that removes expired entries at a
// configurable interval, plus MaxEntries and byte limits that evict the oldest
// entries when either is reached.
//
// Suitable for single-instance deployments and development.
// For multi-instance or production use, implement Store with Redis or similar.
type MemoryStore struct {
	mu              sync.RWMutex
	entries         map[string]memoryEntry
	order           *list.List // keys, oldest first
	bytes           int64
	maxBytes        int64
	maxEntries      int
	lastPurge       time.Time // last expired sweep made to free room in Set
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	stopped         atomic.Bool
}

// NewMemoryStore creates a new in-memory cache store.
// maxEntries limits the number of cached responses (0 = unlimited). The total
// size is limited to DefaultMaxBytes; change it with SetMaxBytes.
// An optional cleanup interval can be provided (default: 1 minute).
// Call Close() when the store is no longer needed to stop the cleanup goroutine.
func NewMemoryStore(maxEntries int, cleanupInterval ...time.Duration) *MemoryStore {
	interval := time.Minute
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		interval = cleanupInterval[0]
	}

	s := &MemoryStore{
		entries:         make(map[string]memoryEntry),
		order:           list.New(),
		maxBytes:        DefaultMaxBytes,
		maxEntries:      maxEntries,
		cleanupInterval: interval,
		stopCleanup:     make(chan struct{}),
	}

	go s.cleanupLoop()

	return s
}

// SetMaxBytes sets the total size budget of cached bodies, headers and keys
// (0 = unlimited) and returns the store. Call it before the store is in use.
func (s *MemoryStore) SetMaxBytes(n int64) *MemoryStore {
	s.mu.Lock()
	s.maxBytes = n
	s.mu.Unlock()
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
		if cur, ok := s.entries[key]; ok && cur.elem == entry.elem {
			s.remove(key)
		}
		s.mu.Unlock()
		return nil, nil
	}

	return entry.resp, nil
}

// Set stores a cached response with the given TTL. When the entry or byte
// limit would be exceeded, the oldest entries are evicted first. A response
// larger than the whole byte budget is not stored.
func (s *MemoryStore) Set(key string, resp *CachedResponse, ttl time.Duration) error {
	size := entrySize(key, resp)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.remove(key)
	if s.maxBytes > 0 && size > s.maxBytes {
		return nil
	}
	full := func() bool {
		return (s.maxEntries > 0 && len(s.entries) >= s.maxEntries) || (s.maxBytes > 0 && s.bytes+size > s.maxBytes)
	}
	// Free expired entries before live ones. The sweep is O(n), so a full store
	// under a stream of new keys runs it at most once a second.
	if now := time.Now(); full() && now.Sub(s.lastPurge) >= time.Second {
		s.lastPurge = now
		s.removeExpiredLocked(now)
	}
	for s.order.Len() > 0 && full() {
		s.remove(s.order.Front().Value.(string))
	}

	s.entries[key] = memoryEntry{
		resp:      resp,
		expiresAt: time.Now().Add(ttl),
		size:      size,
		elem:      s.order.PushBack(key),
	}
	s.bytes += size
	return nil
}

// remove deletes key and its accounting. Must be called with s.mu held.
func (s *MemoryStore) remove(key string) {
	e, ok := s.entries[key]
	if !ok {
		return
	}
	s.order.Remove(e.elem)
	s.bytes -= e.size
	delete(s.entries, key)
}

func entrySize(key string, resp *CachedResponse) int64 {
	n := len(key)
	if resp != nil {
		n += len(resp.Body)
		for k, v := range resp.Headers {
			n += len(k) + len(v)
		}
	}
	return int64(n)
}

// Delete removes a cached response from the store.
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	s.remove(key)
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
	s.mu.Lock()
	s.removeExpiredLocked(time.Now())
	s.mu.Unlock()
}

// removeExpiredLocked removes entries expired at now. Must be called with s.mu held.
func (s *MemoryStore) removeExpiredLocked(now time.Time) {
	for k, e := range s.entries {
		if now.After(e.expiresAt) {
			s.remove(k)
		}
	}
}
