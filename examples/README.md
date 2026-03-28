# Examples

All examples use `kruda.NetHTTP()` and run on every OS.

```bash
cd examples/hello && go run .
```

## Getting Started

| Example | Description |
|---------|-------------|
| [hello](hello/) | Minimal app — ping, JSON, HTML |
| [json-api](json-api/) | REST API with in-memory store |
| [static-files](static-files/) | Serve static assets |
| [error-handling](error-handling/) | Error mapping, custom error pages |

## Features

| Example | Description |
|---------|-------------|
| [typed-handlers](typed-handlers/) | `C[T]` typed input with validation |
| [openapi](openapi/) | Auto-generated OpenAPI 3.1 spec from typed handlers |
| [auto-crud](auto-crud/) | `kruda.Resource[T, ID]()` auto-generates 5 endpoints |
| [crud](crud/) | Manual CRUD with DI container |
| [di-services](di-services/) | Full DI: Give, GiveLazy, GiveNamed, Modules |
| [database](database/) | PostgreSQL with connection pooling |
| [auth](auth/) | JWT authentication + role-based access |
| [session](session/) | Cookie sessions (requires NetHTTP) |
| [middleware](middleware/) | Custom middleware, chaining, groups |
| [sse](sse/) | Server-Sent Events (requires NetHTTP) |
| [websocket](websocket/) | WebSocket upgrade + echo |
| [ratelimit](ratelimit/) | Token bucket rate limiting |
| [health-checks](health-checks/) | Liveness + readiness probes |
| [testing](testing/) | Test client, table-driven tests |

## Architecture Patterns

Same User CRUD API in 4 different patterns — compare and pick.

| Example | Pattern | When to use |
|---------|---------|-------------|
| [flat](flat/) | Everything in one file | Prototypes, small APIs |
| [layered](layered/) | Handler → Service → Repo | Typical CRUD apps |
| [clean-arch](clean-arch/) | Domain core with interfaces | Swappable dependencies |
| [hexagonal](hexagonal/) | Ports & Adapters | Multiple adapters (REST + gRPC) |
