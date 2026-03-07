# JSON API

Full REST API using typed handlers (`C[T]`) for automatic request body and path parameter parsing.

## Run

```bash
go run -tags kruda_stdjson ./examples/json-api/
```

## Test

```bash
# Create
curl -X POST http://localhost:3000/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'

# List
curl http://localhost:3000/users

# Get by ID
curl http://localhost:3000/users/1

# Update
curl -X PUT http://localhost:3000/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Updated","email":"alice@example.com"}'

# Delete
curl -X DELETE http://localhost:3000/users/1
```
