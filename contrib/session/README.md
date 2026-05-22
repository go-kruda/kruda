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
