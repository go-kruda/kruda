# Auth

JWT-like token authentication using HMAC-SHA256 with no external dependencies. Includes login, auth middleware, and protected route groups.

## Run

```bash
go run -tags kruda_stdjson ./examples/auth/
```

## Test

```bash
# Public endpoint
curl http://localhost:3000/

# Login (get token)
curl -X POST http://localhost:3000/login \
  -d '{"username":"admin","password":"secret"}'

# Access protected route with token
curl http://localhost:3000/protected \
  -H "Authorization: Bearer <token>"

# Get profile
curl http://localhost:3000/protected/me \
  -H "Authorization: Bearer <token>"
```
