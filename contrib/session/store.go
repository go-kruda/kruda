package session

import (
	"errors"
	"time"
)

// ErrAtomicStoreRequired reports that a store lacks an atomic capability required
// by the session middleware or session ID regeneration.
var ErrAtomicStoreRequired = errors.New("session: store does not support the required atomic operation")

// ErrSessionNotFound reports that the session being regenerated was deleted or expired.
var ErrSessionNotFound = errors.New("session: session no longer exists")

// SessionData holds the session's internal state.
type SessionData struct {
	// Values is the key-value store for session data.
	Values map[string]any

	// CreatedAt is the time the session was first created.
	CreatedAt time.Time

	// ExpiresAt is the absolute expiration time for this session entry.
	ExpiresAt time.Time
}

// Store defines the interface for session storage backends.
// Implementations must be safe for concurrent use.
// Session middleware requires ExistingSessionStore before any session handler
// runs. Regenerating existing sessions also requires RegeneratingStore.
type Store interface {
	// Get retrieves session data by ID.
	// Returns nil, nil if the session does not exist or has expired.
	Get(id string) (*SessionData, error)

	// Save persists session data with the given TTL.
	// If the session already exists, it is overwritten.
	Save(id string, data *SessionData, ttl time.Duration) error

	// Delete removes a session from the store.
	Delete(id string) error
}

// ExistingSessionStore prevents an in-flight request from recreating a session
// after another request deletes it. The middleware uses SaveExisting for loaded
// sessions and Save for new sessions.
// Without this capability, middleware rejects session requests with
// ErrAtomicStoreRequired before invoking the handler.
type ExistingSessionStore interface {
	Store

	// SaveExisting atomically replaces an existing, unexpired session and its TTL.
	// It returns false without writing if the session was deleted or expired.
	// The existence check and write must be atomic with respect to Delete.
	SaveExisting(id string, data *SessionData, ttl time.Duration) (bool, error)
}

// RegeneratingStore atomically rotates existing sessions without reviving a
// session deleted by another request.
type RegeneratingStore interface {
	ExistingSessionStore

	// Regenerate atomically saves the supplied snapshot under newID and deletes oldID.
	// It returns false without writing if oldID was deleted or expired.
	// The source check, new entry, and deletion must be atomic with respect to Delete.
	Regenerate(oldID, newID string, data *SessionData, ttl time.Duration) (bool, error)
}
