// Package session provides cookie-backed session management for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/session"
//
//	app := kruda.New()
//	app.Use(session.New())
//
//	app.Post("/login", func(c *kruda.Ctx) error {
//	    sess := session.GetSession(c)
//	    sess.Set("user_id", 42)
//	    return c.JSON(kruda.Map{"ok": true})
//	})
//
//	app.Get("/profile", func(c *kruda.Ctx) error {
//	    sess := session.GetSession(c)
//	    return c.JSON(kruda.Map{"user_id": sess.Get("user_id")})
//	})
//
// # What it does
//
// On every request the middleware looks for a session cookie. If found and
// valid, the corresponding [Session] is loaded from the configured [Store].
// Otherwise a fresh session is created lazily on first write. Modified
// sessions are persisted (and the cookie refreshed) on the response path.
// Calling Session.Destroy clears the cookie and deletes the entry from the
// store.
//
// The default in-memory store ([NewMemoryStore]) is single-instance only;
// implement [Store] to plug in Redis, SQL, or any other backend for
// horizontally-scaled deployments.
//
// # Configuration
//
//   - CookieName:     session cookie name (default "_session")
//   - CookiePath:     cookie Path attribute (default "/")
//   - CookieDomain:   cookie Domain attribute (default empty)
//   - CookieSecure:   set Secure flag (default false; enable on HTTPS)
//   - CookieHTTPOnly: HttpOnly flag (default true; disable via DisableHTTPOnly)
//   - CookieSameSite: SameSite attribute (default http.SameSiteLaxMode)
//   - MaxAge:         cookie max-age in seconds (default 86400)
//   - IdleTimeout:    refresh-on-use lifetime (default 30 minutes)
//   - Store:          backing storage (default in-memory)
//   - Skip:           per-request bypass function
//
// # See also
//
//   - [Store] — interface for plugging in custom session backends
//   - RFC 6265 (HTTP cookies)
package session
