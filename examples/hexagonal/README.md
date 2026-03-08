# Hexagonal Architecture (Ports & Adapters)

Domain is at the center. Ports define contracts. Adapters implement them.

```
hexagonal/
├── main.go                        # Wiring
├── domain/
│   └── user.go                    # Entity (pure, no deps)
├── port/
│   └── user.go                    # Driving + Driven port interfaces
├── service/
│   └── user.go                    # Business logic (implements driving port)
└── adapter/
    ├── http/
    │   └── user.go                # HTTP adapter (driving)
    └── storage/
        └── user_memory.go         # Storage adapter (driven)
```

## How It Differs from Clean Architecture

| | Clean Architecture | Hexagonal |
|---|---|---|
| Interface location | Inside domain | Separate `port/` package |
| Terminology | Use case / Gateway | Driving port / Driven port |
| Adapter grouping | By layer | By direction (inbound/outbound) |

Both enforce the same dependency rule — the difference is organizational.

## Ports

- **Driving port** (`UserService`) — inbound, called by HTTP adapter
- **Driven port** (`UserRepository`) — outbound, called by service, implemented by storage

## When to Use

- Large applications with multiple adapters (REST + gRPC + CLI)
- When you need to swap storage (memory → PostgreSQL → DynamoDB)
- Microservices with clear bounded contexts

## Trade-offs

| ✅ Pros | ❌ Cons |
|---------|---------|
| Maximum flexibility | Most boilerplate of all patterns |
| Every dependency is an interface | Over-engineering for small apps |
| Easy to add gRPC/CLI adapters | More packages to navigate |

## Run

```bash
go run .
curl -X POST http://localhost:3000/users -d '{"name":"Alice","email":"alice@example.com"}'
curl http://localhost:3000/users
```
