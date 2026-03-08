# JWT

JWT sign, verify, refresh with HS256/384/512 and RS256 support.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/jwt
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/jwt"

// Middleware for protected routes
app.Use(jwt.New(jwt.Config{
    Secret: []byte(os.Getenv("JWT_SECRET")),
}))

// Sign token
token, err := jwt.Sign(claims, secret)

// Verify token
claims, err := jwt.Verify(token, secret)
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Secret | []byte | required | Secret key for HS256/384/512 |
| Algorithm | string | "HS256" | JWT algorithm |
| Skipper | func | nil | Skip middleware function |
| ErrorHandler | func | nil | Custom error handler |