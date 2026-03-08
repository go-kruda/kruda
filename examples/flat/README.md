# Flat Pattern

Everything in one file. No abstraction layers.

```
flat/
└── main.go
```

## When to Use

- Prototypes and hackathons
- Small APIs (< 10 endpoints)
- CLI tools with embedded HTTP server
- Learning Kruda

## Trade-offs

| ✅ Pros | ❌ Cons |
|---------|---------|
| Zero boilerplate | Hard to test in isolation |
| Easy to understand | Business logic mixed with HTTP |
| Fast to write | Doesn't scale past ~500 LOC |

## Run

```bash
go run .
curl -X POST http://localhost:3000/users -d '{"name":"Alice","email":"alice@example.com"}'
curl http://localhost:3000/users
```
