package session

import (
	"reflect"
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
// Maps, slices, and arrays are recursively copied on reads and writes.
// Slice copies include only visible elements and do not retain spare capacity.
// Structs, pointers, and other reference-bearing values are not deep-copied;
// their shared contents must be immutable or synchronized by the caller.
//
// Suitable for single-instance deployments and development.
// For multi-instance or production use, implement Store with Redis or similar.
type MemoryStore struct {
	mu              sync.RWMutex
	entries         map[string]memoryEntry
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	closeOnce       sync.Once
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
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]

	if !ok {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		// Expired — remove lazily.
		delete(s.entries, id)
		return nil, nil
	}

	return cloneSessionData(entry.data), nil
}

// Save persists session data with the given TTL.
func (s *MemoryStore) Save(id string, data *SessionData, ttl time.Duration) error {
	s.mu.Lock()
	s.entries[id] = memoryEntry{
		data:      cloneSessionData(data),
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
	return nil
}

// SaveExisting atomically updates an existing, unexpired session.
func (s *MemoryStore) SaveExisting(id string, data *SessionData, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	now := time.Now()
	if !ok || now.After(entry.expiresAt) {
		return false, nil
	}
	s.entries[id] = memoryEntry{
		data:      cloneSessionData(data),
		expiresAt: now.Add(ttl),
	}
	return true, nil
}

// Regenerate atomically moves an existing, unexpired session to a new ID.
func (s *MemoryStore) Regenerate(oldID, newID string, data *SessionData, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[oldID]
	now := time.Now()
	if !ok || now.After(entry.expiresAt) {
		return false, nil
	}
	s.entries[newID] = memoryEntry{
		data:      cloneSessionData(data),
		expiresAt: now.Add(ttl),
	}
	delete(s.entries, oldID)
	return true, nil
}

// Touch refreshes an existing, unexpired session without replacing its values.
func (s *MemoryStore) Touch(id string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	entry.expiresAt = time.Now().Add(ttl)
	s.entries[id] = entry
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
	s.closeOnce.Do(func() { close(s.stopCleanup) })
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

func cloneSessionData(data *SessionData) *SessionData {
	if data == nil {
		return nil
	}
	clone := &SessionData{
		CreatedAt: data.CreatedAt,
		ExpiresAt: data.ExpiresAt,
	}
	if data.Values != nil {
		clone.Values = cloneSessionValue(reflect.ValueOf(data.Values), make(map[sessionValueRef]reflect.Value)).Interface().(map[string]any)
	}
	return clone
}

type sessionValueRef struct {
	typ     reflect.Type
	pointer uintptr
	length  int
}

func cloneSessionValue(value reflect.Value, seen map[sessionValueRef]reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		clone := reflect.New(value.Type()).Elem()
		clone.Set(cloneSessionValue(value.Elem(), seen))
		return clone
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		ref := sessionValueRef{typ: value.Type(), pointer: value.Pointer()}
		if clone, ok := seen[ref]; ok {
			return clone
		}
		clone := reflect.MakeMapWithSize(value.Type(), value.Len())
		seen[ref] = clone
		iter := value.MapRange()
		for iter.Next() {
			clone.SetMapIndex(iter.Key(), cloneSessionValue(iter.Value(), seen))
		}
		return clone
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		ref := sessionValueRef{typ: value.Type(), pointer: value.Pointer(), length: value.Len()}
		if clone, ok := seen[ref]; ok {
			return clone
		}
		clone := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		seen[ref] = clone
		for i := 0; i < value.Len(); i++ {
			clone.Index(i).Set(cloneSessionValue(value.Index(i), seen))
		}
		return clone
	case reflect.Array:
		clone := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			clone.Index(i).Set(cloneSessionValue(value.Index(i), seen))
		}
		return clone
	default:
		return value
	}
}
