# Clean Architecture

Domain core defines interfaces (ports). Outer layers implement them.
Dependencies always point inward — domain never imports handler or repository.

```
clean-arch/
├── main.go                    # Wiring
├── domain/
│   └── user.go                # Entity + Service + Repository interface
├── handler/
│   └── user.go                # HTTP adapter (imports domain)
└── repository/
    └── user_memory.go         # Storage adapter (imports domain)
```

## Dependency Rule

```
handler ──→ domain ←── repository
              ↑
           main.go (wires everything)
```

`domain/` has zero imports from other packages. The `UserRepository` interface
lives in domain — repository package implements it.

## When to Use

- Medium-to-large applications
- When you need to swap DB, cache, or external services
- When business logic must be testable without HTTP

## Trade-offs

| ✅ Pros | ❌ Cons |
|---------|---------|
| Domain is pure and testable | More files and packages |
| Easy to swap implementations | Indirection can confuse newcomers |
| Enforced dependency rule | Overkill for simple CRUD |

## Run

```bash
go run .
curl -X POST http://localhost:3000/users -d '{"name":"Alice","email":"alice@example.com"}'
curl http://localhost:3000/users
```
