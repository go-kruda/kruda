# Auto CRUD

One line registers 5 REST endpoints using `kruda.Resource()`.

## Run

```bash
go run -tags kruda_stdjson ./examples/auto-crud/
```

## Test

```bash
# List
curl http://localhost:3000/users

# Create
curl -X POST http://localhost:3000/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice"}'

# Get
curl http://localhost:3000/users/1

# Update
curl -X PUT http://localhost:3000/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Updated"}'

# Delete
curl -X DELETE http://localhost:3000/users/1

# Health check (auto-discovers HealthChecker services)
curl http://localhost:3000/health
```
