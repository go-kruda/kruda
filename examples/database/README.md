# Database

Repository pattern with an in-memory store, DI module registration, and interface-based dependency resolution.

## Run

```bash
go run -tags kruda_stdjson ./examples/database/
```

## Test

```bash
curl http://localhost:3000/users
curl http://localhost:3000/users/1
curl -X POST http://localhost:3000/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Charlie","email":"charlie@example.com"}'
```
