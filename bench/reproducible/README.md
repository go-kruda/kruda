# Benchmark Reproduction

Source code for all 3 frameworks used in the Kruda benchmark comparison.
Anyone can clone this repo and reproduce the results.

## Results (2026-03-05)

| Test | Kruda (Go) | Fiber (Go) | Actix (Rust) | vs Fiber | vs Actix |
|------|--:|--:|--:|--:|--:|
| plaintext | **846,622** | 670,240 | 814,652 | +26% | +4% |
| JSON | **805,124** | 625,839 | 790,362 | +29% | +2% |
| db | **108,468** | 107,450 | 37,373 | +1% | +190% |
| fortunes | 104,144 | **106,623** | 45,078 | -2% | +131% |

## Hardware & Environment

- **CPU:** Intel i5-13500 (8P cores)
- **OS:** Ubuntu Linux
- **Go:** 1.25.7
- **Rust:** stable (latest)
- **DB:** PostgreSQL 16 (localhost)
- **Load tool:** `wrk -t4 -c256 -d5s`
- **GC tuning:** `GOGC=400`

## Directory Structure

```
bench/reproducible/
├── kruda/          # Kruda (Wing transport — epoll + eventfd)
│   ├── main.go
│   ├── go.mod
│   └── go.sum
├── fiber/          # Fiber v2 (fasthttp transport)
│   ├── main.go
│   ├── go.mod
│   └── go.sum
├── actix/          # Actix Web 4 (tokio + actix-rt)
│   ├── Cargo.toml
│   └── src/main.rs
├── bench.sh        # Automated benchmark script
└── README.md
```

## Prerequisites

```bash
# Go 1.25+
go version

# Rust (for Actix)
rustc --version

# wrk (load testing tool)
wrk --version

# PostgreSQL with TFB schema
psql -c "SELECT COUNT(*) FROM world" hello_world
```

## Database Setup

```bash
# Create database and tables (TFB standard schema)
createdb hello_world
psql hello_world <<'SQL'
CREATE TABLE world (
  id integer NOT NULL,
  randomnumber integer NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);
INSERT INTO world (id, randomnumber)
  SELECT x.id, floor(random() * 10000 + 1)
  FROM generate_series(1, 10000) AS x(id);

CREATE TABLE fortune (
  id integer NOT NULL,
  message varchar(2048) NOT NULL,
  PRIMARY KEY (id)
);
INSERT INTO fortune (id, message) VALUES
  (1, 'fortune: No such file or directory'),
  (2, 'A computer scientist is someone who fixes things that aren''t broken.'),
  (3, 'After all is said and done, more is said than done.'),
  (4, 'Any sufficiently advanced technology is indistinguishable from magic.'),
  (5, 'A SQL query walks into a bar, sees two tables and asks... Can I join you?'),
  (6, 'Change is inevitable, except from a vending machine.'),
  (7, 'Don''t worry about the world coming to an end today. It is already tomorrow in Australia.'),
  (8, 'Computers make very fast, very accurate mistakes.'),
  (9, 'Debugging is twice as hard as writing the code in the first place.'),
  (10, 'Feature: A bug with seniority.'),
  (11, 'フレームワークのベンチマーク'),
  (12, 'Premature optimization is the root of all evil.');
SQL
```

## How to Reproduce

### 1. Build all frameworks

```bash
# Kruda
cd kruda && go build -o kruda-bench . && cd ..

# Fiber
cd fiber && go build -o fiber-bench . && cd ..

# Actix
cd actix && cargo build --release && cd ..
```

### 2. Run benchmarks

```bash
# Start each server and test with wrk
# Kruda (port 3000)
GOMAXPROCS=8 ./kruda/kruda-bench &
wrk -t4 -c256 -d5s http://localhost:3000/       # plaintext
wrk -t4 -c256 -d5s http://localhost:3000/json    # JSON
wrk -t4 -c256 -d5s http://localhost:3000/db      # single DB query
wrk -t4 -c256 -d5s http://localhost:3000/fortunes # fortunes

# Fiber (port 3002)
GOMAXPROCS=8 ./fiber/fiber-bench &
wrk -t4 -c256 -d5s http://localhost:3002/
wrk -t4 -c256 -d5s http://localhost:3002/json
wrk -t4 -c256 -d5s http://localhost:3002/db
wrk -t4 -c256 -d5s http://localhost:3002/fortunes

# Actix (port 3003)
./actix/target/release/actix-bench &
wrk -t4 -c256 -d5s http://localhost:3003/
wrk -t4 -c256 -d5s http://localhost:3003/json
wrk -t4 -c256 -d5s http://localhost:3003/db
wrk -t4 -c256 -d5s http://localhost:3003/fortunes
```

Or use the automated script:

```bash
bash bench.sh
```

## Notes

- All 3 apps use the same PostgreSQL database, same schema, same `pool_max_conns=64`
- Kruda uses Wing transport (raw epoll + eventfd) — not fasthttp
- Fiber uses fasthttp transport (its default)
- Actix uses tokio + actix-rt (Rust async runtime)
- Actix DB numbers are lower likely due to `deadpool-postgres` vs `pgx` pool differences
- Results vary by hardware — the relative percentages are more meaningful than absolute numbers
