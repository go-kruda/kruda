# Kruda — TechEmpower Framework Benchmarks

[Kruda](https://github.com/go-kruda/kruda) (ครุฑ) is a high-performance Go web framework combining speed with type-safety through Go generics.

## Test Variants

| Variant | Display Name | Description | Dockerfile |
|---------|-------------|-------------|------------|
| default | Kruda | Single-process mode | `Dockerfile` |
| turbo | Kruda [turbo] | SO_REUSEPORT prefork mode (1 process per CPU core) | `kruda-turbo.dockerfile` |

Both variants implement all 7 TFB test types:

- `/json` — JSON serialization
- `/plaintext` — Plaintext
- `/db` — Single database query
- `/queries?queries=N` — Multiple database queries
- `/updates?queries=N` — Database updates
- `/fortunes` — Fortunes (HTML rendering with XSS escaping)
- `/cached-queries?count=N` — Cached queries

## Build & Run (Local)

```bash
docker compose up --build
```

- Default variant: http://localhost:8080
- Turbo variant: http://localhost:8081
- PostgreSQL: localhost:5432

## Key Optimizations

- Zero-allocation JSON/plaintext handlers (pre-allocated response bytes)
- Manual JSON serializer using `strconv.AppendInt` (no `encoding/json`)
- Manual HTML builder with byte-level XSS escaping (no `html/template`)
- Tiered `sync.Pool` buffer pools (1KB / 8KB / 32KB)
- `pgx.Batch` for single-roundtrip multi-query and update operations
- Flat-array in-memory cache for cached queries (0-alloc lookups)
- Atomic date header cache (1-second refresh)
- SO_REUSEPORT prefork in turbo mode (1 process per CPU, `GOMAXPROCS(1)`)
