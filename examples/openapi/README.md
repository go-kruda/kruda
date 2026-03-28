# OpenAPI 3.1

Auto-generates an OpenAPI spec from your typed handlers — no YAML, no codegen, no extra tools. Just define `C[T]` handlers and the spec builds itself from struct tags.

## Run

```bash
go run -tags kruda_stdjson ./examples/openapi/
```

## Test

```bash
# Grab the generated spec
curl http://localhost:3000/openapi.json | jq .

# Create a task
curl -X POST http://localhost:3000/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy milk","priority":"high"}'

# List with query filters
curl "http://localhost:3000/tasks?status=pending&limit=5"

# Get by ID
curl http://localhost:3000/tasks/1

# Update
curl -X PUT http://localhost:3000/tasks/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy oat milk","priority":"low","status":"done"}'

# Add a comment
curl -X POST http://localhost:3000/tasks/1/comments \
  -H 'Content-Type: application/json' \
  -d '{"author":"tiger","body":"switched to oat milk"}'
```

## How it works

```go
app := kruda.New(
    kruda.WithOpenAPIInfo("Task Manager API", "1.0.0", "..."),
    kruda.WithOpenAPITag("tasks", "Task management operations"),
)
```

That's it. Struct tags do the rest:

- `json` → request/response properties
- `param` → path parameters
- `query` → query parameters
- `validate` → schema constraints (required, min/max, etc.)

Per-route metadata via `WithDescription` and `WithTags`:

```go
kruda.Post[CreateTaskInput, Task](app, "/tasks", handler,
    kruda.WithDescription("Create a new task"),
    kruda.WithTags("tasks"),
)
```

Spec is served at `/openapi.json` by default. Change it with `WithOpenAPIPath("/api/spec.json")`.
