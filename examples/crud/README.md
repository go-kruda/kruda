# CRUD

DI container, `kruda.Resource()` auto-CRUD, and health checks in one example.

## Run

```bash
go run -tags kruda_stdjson ./examples/crud/
```

## Test

```bash
# Auto CRUD endpoints
curl http://localhost:3000/users
curl -X POST http://localhost:3000/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice"}'
curl http://localhost:3000/users/1
curl -X PUT http://localhost:3000/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Updated"}'
curl -X DELETE http://localhost:3000/users/1

# Health check and stats
curl http://localhost:3000/health
curl http://localhost:3000/stats
```
