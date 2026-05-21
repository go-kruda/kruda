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

// Verify token at an auth/security boundary
claims, err := jwt.VerifyWithAlgorithm(token, secret, "HS256")
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Secret | []byte | required | Secret key for HS256/384/512 |
| PublicKey | *rsa.PublicKey | nil | Public key for RS256 verification |
| PrivateKey | *rsa.PrivateKey | nil | Private key for RS256 signing |
| Algorithm | string | "HS256" | JWT algorithm |
| Lookup | string | "header:Authorization" | Token lookup source |
| Skip | func(*kruda.Ctx) bool | nil | Skip middleware function |
| GracePeriod | time.Duration | 0 | Accepted refresh grace window |

HMAC algorithms reject empty secrets. Configure a non-empty secret before using
the middleware or signing tokens.

Use `VerifyWithAlgorithm` at authentication boundaries so verification requires
the algorithm your application configured. The middleware already enforces the
configured algorithm.
