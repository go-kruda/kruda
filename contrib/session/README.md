# Session

Session management middleware with in-memory store and cookie-based session ID.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/session
```

## Usage

Session cookies work with Kruda's default transport. When `Set-Cookie` is
present, Wing automatically leaves its header-minimal fast path, so no
`NetHTTP` override is required.

```go
import "github.com/go-kruda/kruda/contrib/session"

app := kruda.New()
app.Use(session.New())

app.Get("/login", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    if err := sess.Regenerate(); err != nil {
        return err
    }
    sess.Set("user", "Tiger")
    return c.JSON(kruda.Map{"status": "logged in"})
})

app.Get("/profile", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    user := sess.GetString("user")
    return c.JSON(kruda.Map{"user": user})
})
```

After verifying credentials, call `sess.Regenerate()` before setting the
authenticated identity. This atomically moves even a valid anonymous session to
a new ID and queues the replacement cookie before `c.JSON` sends headers.
Existing values are preserved; arbitrary calls to `Set` do not rotate the ID.
Call `Regenerate` before writing the response; it returns `kruda.ErrAlreadyResponded`
if the response has already been sent, or `session.ErrSessionNotFound` if another
request already deleted the session. New, unsaved sessions are persisted with
their new ID without deleting a nonexistent entry.

## Production Cookie Settings

For HTTPS deployments, enable the `Secure` flag and choose a `SameSite` mode
that matches your app's cross-site flow. Keep `CookieHTTPOnly` enabled unless
JavaScript must read the session cookie.

```go
app.Use(session.New(session.Config{
    CookieSecure:   true,
    CookieHTTPOnly: true,
    CookieSameSite: http.SameSiteLaxMode,
}))
```

Use `http.SameSiteStrictMode` for same-site apps that do not need cross-site
login or callback flows. If you set `http.SameSiteNoneMode`, browsers require
`CookieSecure: true`.

Call `sess.Destroy()` on logout. The delete cookie keeps the configured path,
domain, `Secure`, `HttpOnly`, and `SameSite` attributes so browsers remove the
same cookie that was issued.

Logout deletes the ID loaded by that request. Rotation and deletion are ordered
atomically: logout completed before rotation prevents the old session from being
rotated; a rotation completed first creates a new ID that an overlapping logout
of the old ID does not revoke. These APIs do not revoke every ID associated with
a user; an explicit login may establish a new session.

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| CookieName | string | "_session" | Session cookie name |
| MaxAge | int | 86400 | Session cookie max-age in seconds |
| CookiePath | string | "/" | Cookie path |
| CookieSecure | bool | false | HTTPS only cookie |
| CookieHTTPOnly | bool | true | HTTP only cookie |
| CookieSameSite | http.SameSite | http.SameSiteLaxMode | SameSite policy |
| Store | Store | MemoryStore | Session storage backend |

Untouched new sessions are not written to the store. A cookie can still be
issued for them; the next request creates a new session if no values were saved.
MemoryStore refreshes existing sessions on read-only requests without replacing
their values. Custom stores can implement `Touch(string, time.Duration) error`
for the same behavior. Touch must not recreate deleted or expired sessions.
Stores without Touch keep their existing expiration behavior on read-only requests.

MemoryStore recursively copies maps, slices, and arrays, including those held in
interfaces, so requests own their nested session containers. Types, nil values, map cycles, and cycles of identical slice views are
preserved. Slice copies contain only visible elements, with no
spare capacity; views with different lengths or starting elements are copied
independently. After modifying a value returned by `Get`, call `Set`
to persist the change. Structs, pointers, map keys, and other reference-bearing
values are kept as supplied; their shared contents must be immutable or
synchronized by the caller. Do not mutate data while a store operation copies it.

MemoryStore implements `ExistingSessionStore`: saving a loaded session updates
only an existing, unexpired entry, atomically with deletion. A modified request
that finishes after logout or ID regeneration cannot recreate the old session.
Custom stores should implement `SaveExisting(string, *SessionData, time.Duration)
(bool, error)` with the same atomic guarantee (for example, a conditional SQL
update or Redis script). A separate `Get` followed by `Save` is insufficient.

For ID rotation, implement `RegeneratingStore` as well. Its
`Regenerate(oldID, newID string, data *SessionData, ttl time.Duration) (bool, error)`
must atomically verify that the old session exists and is unexpired, persist the
supplied snapshot under the new ID, and remove the old ID. Return false without
writing if the old session is gone. The rotated entry is persisted before its
cookie is issued; later changes use `SaveExisting`, so a concurrent logout of the
new ID cannot be undone by the rotating request.

Existing `Store` implementations still compile, but every request managed by
session middleware requires `ExistingSessionStore`. Without it, middleware
returns `session.ErrAtomicStoreRequired` before invoking the handler, accessing
the store, or issuing a cookie, including on the first request. Requests excluded
by `Skip` still bypass session management. Rotating an existing session also
requires `RegeneratingStore`. Migrate custom stores to these atomic operations
before using session middleware. MemoryStore implements both capabilities.
