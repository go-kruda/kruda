# Middleware

Built-in middleware (Logger, Recovery, CORS, RequestID, Timeout) plus custom middleware examples.

## Run

```bash
go run -tags kruda_stdjson ./examples/middleware/
```

## Test

```bash
# Public routes
curl -v http://localhost:3000/
curl -v http://localhost:3000/public

# Protected routes (requires Bearer token)
curl http://localhost:3000/admin/dashboard -H "Authorization: Bearer secret-token"
curl http://localhost:3000/admin/users -H "Authorization: Bearer secret-token"

# Without token (returns 401)
curl http://localhost:3000/admin/dashboard

# Slow endpoint (demonstrates timeout middleware)
curl http://localhost:3000/slow
```
