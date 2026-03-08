# Error Handling

Demonstrates `MapError`, `MapErrorType`, `MapErrorFunc`, custom error handlers, and panic recovery.

## Run

```bash
go run -tags kruda_stdjson ./examples/error-handling/
```

## Test

```bash
# Sentinel error mapped to 404
curl http://localhost:3000/users/999

# Type error mapped to 422
curl http://localhost:3000/users/0

# Custom error func mapped to 409
curl -X POST http://localhost:3000/users

# Direct KrudaError (403)
curl http://localhost:3000/admin

# Panic caught by Recovery (500)
curl http://localhost:3000/panic
```
