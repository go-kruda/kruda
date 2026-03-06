# Typed Handlers

Demonstrates `C[T]` — generic typed context that auto-parses path params, query strings, and JSON body into one struct with validation.

## Run

```bash
go run -tags kruda_stdjson ./examples/typed-handlers/
```

## Test

```bash
# Path param binding
curl http://localhost:3000/products/42

# Query param binding
curl "http://localhost:3000/products?search=phone&page=1&limit=5"

# JSON body with validation
curl -X POST http://localhost:3000/products \
  -H 'Content-Type: application/json' \
  -d '{"name":"Laptop","price":999.99,"category":"electronics"}'

# Combined: path param + JSON body
curl -X PUT http://localhost:3000/products/42 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Updated Laptop","price":1099.99}'
```
