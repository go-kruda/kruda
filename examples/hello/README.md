# Hello World

Basic Kruda app with routing, middleware, and groups.

## Run

```bash
go run -tags kruda_stdjson ./examples/hello/
```

## Test

```bash
curl http://localhost:3000/ping
curl http://localhost:3000/api/hello?name=Tiger
curl -X POST http://localhost:3000/api/echo -d '{"msg":"hi"}'
```
