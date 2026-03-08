# DI Container

Dependency injection with `Give`/`Use` pattern — repo, service, and module layers.

## Run

```bash
go run -tags kruda_stdjson ./examples/di-services/
```

## Test

```bash
curl http://localhost:3000/users

curl -X POST http://localhost:3000/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'

curl http://localhost:3000/users/1
curl http://localhost:3000/stats
curl http://localhost:3000/health
```
