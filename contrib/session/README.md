# Session

Session management middleware with in-memory store and cookie-based session ID.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/session
```

## Usage

**Important:** Requires `kruda.NetHTTP()` transport (Wing skips Set-Cookie headers in fast path).

```go
import "github.com/go-kruda/kruda/contrib/session"

app := kruda.New(kruda.NetHTTP())
app.Use(session.New())

app.Get("/login", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    sess.Set("user", "Tiger")
    return c.JSON(kruda.Map{"status": "logged in"})
})

app.Get("/profile", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    user := sess.GetString("user")
    return c.JSON(kruda.Map{"user": user})
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| CookieName | string | "session_id" | Session cookie name |
| MaxAge | time.Duration | 24*time.Hour | Session expiration |
| CookiePath | string | "/" | Cookie path |
| CookieSecure | bool | false | HTTPS only cookie |
| CookieHTTPOnly | bool | true | HTTP only cookie |
| CookieSameSite | http.SameSite | SameSiteLax | SameSite policy |
| Store | Store | MemoryStore | Session storage backend |