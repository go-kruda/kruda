package session

import "time"

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
