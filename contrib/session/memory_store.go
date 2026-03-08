package session

import (
	"sync"
	"time"
)

// memoryEntry wraps session data with expiration for the in-memory store.
type memoryEntry struct {
	data      *SessionData
	expiresAt time.Time
}

// MemoryStore is a thread-safe, in-memory session store.
// It includes a background cleanup goroutine that removes expired sessions
// at a configurable interval.
//
// Suitable for single-instance deployments and development.
// For multi-instance or production use, implement Store with Redis or similar.
type MemoryStore struct {
	mu              sync.RWMutex
	entries         map[string]memoryEntry
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewMemoryStore creates a new in-memory session store.
// An optional cleanup interval can be provided (default: 1 minute).
// Call Close() when the store is no longer needed to stop the cleanup goroutine.
func NewMemoryStore(cleanupInterval ...time.Duration) *MemoryStore {
	interval := time.Minute
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		interval = cleanupInterval[0]
	}

	s := &MemoryStore{
		entries:         make(map[string]memoryEntry),
		cleanupInterval: interval,
		stopCleanup:     make(chan struct{}),
	}

	go s.cleanupLoop()

	return s
}

// Get retrieves session data by ID. Returns nil, nil if not found or expired.
func (s *MemoryStore) Get(id string) (*SessionData, error) {
	s.mu.RLock()
	entry, ok := s.entries[id]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		// Expired — remove lazily.
		s.mu.Lock()
		delete(s.entries, id)
		s.mu.Unlock()
		return nil, nil
	}

	return entry.data, nil
}

// Save persists session data with the given TTL.
func (s *MemoryStore) Save(id string, data *SessionData, ttl time.Duration) error {
	s.mu.Lock()
	s.entries[id] = memoryEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
	return nil
}

// Delete removes a session from the store.
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	delete(s.entries, id)
	s.mu.Unlock()
	return nil
}

// Close stops the background cleanup goroutine.
func (s *MemoryStore) Close() {
	close(s.stopCleanup)
}

// Len returns the number of entries (including possibly expired ones).
// Useful for testing.
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	n := len(s.entries)
	s.mu.RUnlock()
	return n
}

// cleanupLoop periodically removes expired sessions.
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

// removeExpired removes all expired sessions from the store.
func (s *MemoryStore) removeExpired() {
	now := time.Now()
	s.mu.Lock()
	for id, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, id)
		}
	}
	s.mu.Unlock()
}
