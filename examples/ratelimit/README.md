# Rate Limit

Global and per-route rate limiting using `contrib/ratelimit` with skip functions and response headers.

## Run

```bash
cd examples/ratelimit
go run .
```

## Test

```bash
# Check rate limit headers
curl -v http://localhost:3000/api/data

# Exhaust login limit (5 req/min)
for i in $(seq 1 6); do
  curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:3000/api/login
done

# Health check (bypasses rate limit)
curl http://localhost:3000/health
```
