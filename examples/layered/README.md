# Layered Pattern (MVC)

Handler → Service → Repository. Each layer only calls the one below.

```
layered/
├── main.go
├── handler/
│   └── user.go        # HTTP layer
├── service/
│   └── user.go        # Business logic
├── model/
│   └── user.go        # Data structures
└── repository/
    └── user.go        # Data access
```

## When to Use

- Typical CRUD applications
- Small-to-medium APIs (10–50 endpoints)
- Teams familiar with MVC

## Trade-offs

| ✅ Pros | ❌ Cons |
|---------|---------|
| Clear separation of concerns | Service layer imports repo directly |
| Easy to understand | Harder to swap implementations |
| Good enough for most apps | Can become "lasagna code" |

## Run

```bash
go run .
curl -X POST http://localhost:3000/users -d '{"name":"Alice","email":"alice@example.com"}'
curl http://localhost:3000/users
```
