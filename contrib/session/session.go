// Package session provides session management middleware for Kruda.
//
// Usage:
//
//	app.Use(session.New())
//
//	app.Get("/profile", func(c *kruda.Ctx) error {
//	    sess := session.GetSession(c)
//	    name := sess.Get("name")
//	    return c.JSON(kruda.Map{"name": name})
//	})
//
//	app.Post("/login", func(c *kruda.Ctx) error {
//	    sess := session.GetSession(c)
//	    sess.Set("user_id", 42)
//	    return c.JSON(kruda.Map{"ok": true})
//	})
package session

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"github.com/go-kruda/kruda"
)

const (
	// sessionKey is the context key for storing the Session object.
	sessionKey = "session"

	// sessionIDLength is the number of random bytes in a session ID.
	// 32 bytes = 64 hex characters.
	sessionIDLength = 32
)

// Session represents a user session with key-value storage.
// It is attached to the request context and accessible via GetSession().
type Session struct {
	id       string
	data     *SessionData
	store    Store
	config   *Config
	modified bool
	isNew    bool
	destroy  bool
}

// ID returns the session's unique identifier.
func (s *Session) ID() string { return s.id }

// IsNew returns true if this session was just created (no prior session cookie).
func (s *Session) IsNew() bool { return s.isNew }

// Get retrieves a value from the session by key.
// Returns nil if the key does not exist.
func (s *Session) Get(key string) any {
	if s.data == nil || s.data.Values == nil {
		return nil
	}
	return s.data.Values[key]
}

// GetString retrieves a string value from the session.
// Returns the default value if the key does not exist or is not a string.
func (s *Session) GetString(key string, def ...string) string {
	v := s.Get(key)
	if v == nil {
		if len(def) > 0 {
			return def[0]
		}
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// GetInt retrieves an int value from the session.
// Returns the default value if the key does not exist or is not an int.
func (s *Session) GetInt(key string, def ...int) int {
	v := s.Get(key)
	if v == nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	if n, ok := v.(int); ok {
		return n
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

// Set stores a value in the session.
func (s *Session) Set(key string, value any) {
	if s.data.Values == nil {
		s.data.Values = make(map[string]any)
	}
	s.data.Values[key] = value
	s.modified = true
}

// Delete removes a key from the session.
func (s *Session) Delete(key string) {
	if s.data.Values != nil {
		delete(s.data.Values, key)
		s.modified = true
	}
}

// Clear removes all values from the session.
func (s *Session) Clear() {
	s.data.Values = make(map[string]any)
	s.modified = true
}

// Destroy marks the session for deletion.
// The session data will be removed from the store and the cookie will be expired.
func (s *Session) Destroy() {
	s.destroy = true
	s.modified = true
}

// save persists the session to the store (called by middleware after c.Next()).
func (s *Session) save() error {
	if s.destroy {
		return s.store.Delete(s.id)
	}
	if s.modified || s.isNew {
		return s.store.Save(s.id, s.data, s.config.IdleTimeout)
	}
	return nil
}

// GetSession retrieves the Session from the request context.
// Returns nil if no session middleware is active.
func GetSession(c *kruda.Ctx) *Session {
	v := c.Get(sessionKey)
	if v == nil {
		return nil
	}
	return v.(*Session)
}

// New creates session middleware with the given configuration.
//
// The middleware:
//  1. Reads the session ID from the cookie (or generates a new one)
//  2. Loads session data from the store
//  3. Attaches the Session to the request context
//  4. Calls the next handler
//  5. Saves modified session data back to the store
//  6. Sets/refreshes the session cookie
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()

	// Use memory store if none provided.
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore()
	}

	// Default HTTPOnly to true unless explicitly set to false via config.
	httpOnly := true
	if len(config) > 0 {
		// If user provided a config, use their value.
		// Since bool zero-value is false, we check if they explicitly set it.
		// The Config struct's CookieHTTPOnly field: false means "user wants false" OR "user didn't set it".
		// We default to true for security. To disable, user must set CookieHTTPOnly = false explicitly.
		// This is the safer default.
		httpOnly = cfg.CookieHTTPOnly
		if !httpOnly && len(config) > 0 {
			// User explicitly provided config — respect their choice.
			httpOnly = false
		}
	}

	return func(c *kruda.Ctx) error {
		// Skip if custom skip function returns true.
		if cfg.Skip != nil && cfg.Skip(c.Method(), c.Path()) {
			return c.Next()
		}

		// Read session ID from cookie.
		sessionID := c.Cookie(cfg.CookieName)
		isNew := sessionID == ""

		var data *SessionData
		if !isNew {
			// Load existing session.
			var err error
			data, err = cfg.Store.Get(sessionID)
			if err != nil {
				return err
			}
			if data == nil {
				// Session expired or not found — create new.
				isNew = true
			}
		}

		if isNew {
			sessionID = generateSessionID()
			data = &SessionData{
				Values:    make(map[string]any),
				CreatedAt: time.Now(),
			}
		}

		// Create session object and attach to context.
		sess := &Session{
			id:       sessionID,
			data:     data,
			store:    cfg.Store,
			config:   &cfg,
			modified: false,
			isNew:    isNew,
		}
		c.Set(sessionKey, sess)

		// Pre-set cookie before handler runs — response headers must be
		// set before the body is written (c.JSON flushes immediately).
		if sess.isNew {
			c.SetCookie(&kruda.Cookie{
				Name:     cfg.CookieName,
				Value:    sessionID,
				Path:     cfg.CookiePath,
				Domain:   cfg.CookieDomain,
				MaxAge:   cfg.MaxAge,
				Secure:   cfg.CookieSecure,
				HTTPOnly: httpOnly,
				SameSite: cfg.CookieSameSite,
			})
		}

		// Call next handler.
		err := c.Next()

		// After handler: save session data to store.
		if saveErr := sess.save(); saveErr != nil {
			return saveErr
		}

		if sess.destroy {
			c.SetCookie(&kruda.Cookie{
				Name:   cfg.CookieName,
				Value:  "",
				Path:   cfg.CookiePath,
				Domain: cfg.CookieDomain,
				MaxAge: -1,
			})
		}

		return err
	}
}

// generateSessionID creates a cryptographically random session ID.
// Panics if the system's entropy source fails.
func generateSessionID() string {
	b := make([]byte, sessionIDLength)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("kruda/session: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
