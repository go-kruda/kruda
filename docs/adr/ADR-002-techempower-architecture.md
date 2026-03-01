# ADR-002: TechEmpower Benchmark Architecture

## Status: Proposed

## Context

Kruda needs to dominate TechEmpower Web Framework Benchmarks across all 7 scenarios to establish market leadership. Current performance shows promise:

**Micro-benchmark Results:**
- fasthttp mode: StaticGET 53ns/0alloc (ties Fiber)
- Real HTTP: 221k req/s (beats Elysia 159k req/s)

**Target Competition:**
- Go: Fiber, Gin, Echo (current leaders)
- Cross-language: Elysia (Bun/JS), Actix (Rust), Spring (Java)

**TechEmpower Scenarios:**
1. JSON serialization
2. Plaintext response  
3. Single database query
4. Multiple database queries (1-500)
5. Fortunes (database + HTML templating)
6. Database updates (1-500)
7. Cached queries

## Decision

**Use fasthttp transport exclusively for TechEmpower submission with zero-allocation optimizations.**

### Architecture Choice: fasthttp + Direct pgx + Manual Serialization

**Rationale:**
- fasthttp gives 4x allocation advantage over net/http (53ns/0alloc vs 216ns/8alloc)
- Direct pgx queries bypass ORM overhead
- Manual JSON/HTML serialization eliminates reflection
- Kruda's existing fasthttp integration is production-ready

**Trade-offs:**
- More complex implementation vs stdlib simplicity
- Linux-specific optimizations vs cross-platform compatibility
- Manual serialization vs type safety

## Component Design

### File: `techempower/optimized_handlers.go`
**Responsibilities:**
- Zero-allocation JSON serialization using byte buffers
- Direct pgx batch queries with prepared statements
- Manual HTML generation for Fortunes
- Sharded world cache for cached-queries scenario

**Dependencies:** pgxpool, sync.Pool, unsafe package
**Interfaces:** kruda.Handler

### File: `techempower/cache.go` 
**Responsibilities:**
- Sharded world cache (16 shards, RWMutex per shard)
- LRU eviction with 10k capacity per shard
- Background refresh every 60 seconds

**Dependencies:** sync.RWMutex, time.Ticker
**Interfaces:** WorldCache interface

### File: `techempower/pools.go`
**Responsibilities:**
- Buffer pools for JSON/HTML serialization
- World slice pools for batch operations
- Connection pool management

**Dependencies:** sync.Pool
**Interfaces:** Pool management functions

### File: `techempower/main.go`
**Responsibilities:**
- fasthttp transport configuration
- Database connection pool setup (256 connections)
- Linux kernel optimizations (SO_REUSEPORT, TCP_NODELAY)
- Route registration

**Dependencies:** pgxpool, kruda
**Interfaces:** main() entry point

## Data Flow

### Entry Points
```
HTTP Request → fasthttp.RequestCtx → kruda.ServeFastHTTP → Handler
```

### Transformations
```
1. JSON: Static response → zero-copy write
2. DB: ID generation → pgx query → manual JSON serialization
3. Queries: Batch generation → pgx.Batch → pooled slice → manual JSON
4. Fortunes: pgx query → sort → manual HTML generation
5. Updates: Read batch → update batch → transaction commit → JSON
6. Cached: Cache lookup → pooled slice → manual JSON
```

### Outputs
```
Handler → Response buffer → fasthttp.RequestCtx.Write → TCP
```

## Build Sequence

### Phase 1: Core Infrastructure (4 hours)
- [ ] Create `techempower/` directory structure
- [ ] Implement buffer pools and slice pools
- [ ] Setup pgxpool with optimized configuration
- [ ] Create manual JSON serialization functions
- [ ] Verify zero-allocation with benchmarks

### Phase 2: Database Scenarios (6 hours)  
- [ ] Implement DB single query with prepared statements
- [ ] Implement Queries with pgx.Batch for parallelization
- [ ] Implement Updates with read/write batching + transactions
- [ ] Add connection pool monitoring and tuning
- [ ] Benchmark against current implementation

### Phase 3: Advanced Scenarios (4 hours)
- [ ] Implement Fortunes with manual HTML generation
- [ ] Create sharded world cache for cached-queries
- [ ] Add background cache refresh mechanism
- [ ] Optimize HTML escaping with unsafe string conversion
- [ ] Validate HTML output compliance

### Phase 4: Linux Optimization (2 hours)
- [ ] Configure SO_REUSEPORT for multi-process scaling
- [ ] Enable TCP_NODELAY and TCP_QUICKACK
- [ ] Set optimal TCP buffer sizes
- [ ] Configure io_uring if available
- [ ] Add CPU affinity for database connections

### Phase 5: Validation (2 hours)
- [ ] Run local TechEmpower test suite
- [ ] Profile memory allocations (target 0 allocs for JSON/plaintext)
- [ ] Validate database query correctness
- [ ] Test under high concurrency (10k connections)
- [ ] Compare against Fiber/Gin implementations

## Critical Details

### Error Handling
- Panic recovery middleware disabled for benchmarks
- Database errors return 500 status immediately
- No error logging to avoid I/O overhead

### State Management  
- Context pool reuse with embedded fasthttp adapters
- No per-request allocations in hot path
- Buffer pools with appropriate sizing (1KB initial, 64KB max)

### Testing Strategy
- Unit tests for each handler with allocation verification
- Integration tests against PostgreSQL test database
- Load tests with wrk/bombardier matching TechEmpower methodology
- Memory profiling to ensure zero-allocation targets

### Performance Targets

| Scenario | Target req/s | Target Allocations | Key Optimization |
|----------|-------------:|-------------------:|------------------|
| JSON | 2M+ | 0 allocs/op | Static response buffer |
| Plaintext | 2M+ | 0 allocs/op | Static response buffer |
| DB | 500k+ | 1 alloc/op | Prepared statements + manual JSON |
| Queries | 100k+ | <queries allocs/op | pgx.Batch + pooled slices |
| Fortunes | 50k+ | 2 allocs/op | Manual HTML + sort optimization |
| Updates | 50k+ | <queries allocs/op | Batch read/write + transactions |
| Cached | 800k+ | 0 allocs/op | Sharded cache + zero-copy |

### Security Considerations
- Disable security headers for benchmark compliance
- SQL injection protection via prepared statements
- HTML escaping for Fortunes scenario
- Input validation for queries parameter (1-500 range)

## Linux Kernel Tuning

### Required sysctl settings:
```bash
net.core.somaxconn = 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
net.ipv4.ip_local_port_range = 1024 65535
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
```

### Database Connection Pool:
```go
config.MaxConns = 256
config.MinConns = 256  
config.MaxConnLifetime = 0 // No limit
config.MaxConnIdleTime = 30 * time.Minute
```

## Risk Assessment

### High Risk - Database Connection Exhaustion
**Mitigation:** Connection pool monitoring, graceful degradation, circuit breaker

### Medium Risk - Memory Leaks in Buffer Pools  
**Mitigation:** Buffer size limits, periodic pool reset, memory profiling

### Medium Risk - Cache Invalidation Complexity
**Mitigation:** Simple TTL-based invalidation, background refresh, cache bypass fallback

### Low Risk - JSON Serialization Bugs
**Mitigation:** Comprehensive test suite, comparison with stdlib output

## Expected Rankings

### Conservative Estimates (vs current leaders):
- **JSON/Plaintext:** Top 3 (behind only C/Rust frameworks)
- **DB scenarios:** Top 5 in Go, competitive with Fiber
- **Fortunes:** Top 10 (HTML generation is CPU-intensive)
- **Updates:** Top 5 (transaction overhead limits all frameworks)
- **Cached:** Top 3 (cache hit rate advantage)

### Aggressive Targets (with full optimization):
- **JSON/Plaintext:** #1 in Go, top 5 overall
- **DB scenarios:** #1 in Go, competitive with Actix-Web
- **Fortunes:** Top 5 in Go
- **Updates:** Top 3 in Go  
- **Cached:** #1 in Go, competitive with C frameworks

## Success Criteria

1. **Beat all Go frameworks** (Fiber, Gin, Echo) in at least 5/7 scenarios
2. **Top 10 overall** in JSON and Plaintext scenarios
3. **Zero allocations** for JSON, Plaintext, and Cached scenarios
4. **Sub-1ms P99 latency** under 10k concurrent connections
5. **Stable performance** across 15-minute benchmark runs

## Implementation Notes

The TechEmpower implementation will be separate from the main framework - it's a showcase of what's possible with Kruda's fasthttp transport and zero-allocation design, not a general-purpose application template.

Focus on absolute performance over code maintainability for this specific use case.


## Security vs Performance Options (TODO: Doc)

Pattern ที่ Kruda ใช้: security features เป็น opt-out ผ่าน `kruda.New(WithXxx(false))`

Features ที่ต้อง document พร้อม example:
- `kruda.New()` — security headers enabled by default (X-Frame-Options, CSP ฯลฯ) สำหรับ production
- `WithSecureHeaders()` — explicitly enable security headers
- `WithPathTraversal(false)` — ปิด path traversal check สำหรับ non-file-serving apps
- Query string safety — `string(b)` copy by default, safe for all use cases
- Multipart limits — `BodyLimit` config controls max multipart size

Doc ที่ต้องเขียน:
1. Security guide: อธิบาย default-on features และเมื่อไหรควรปิด
2. Performance guide: อธิบาย tradeoffs ของแต่ละ option พร้อม benchmark numbers
3. Example: `examples/high-perf-api/` — app ที่ปิด security features สำหรับ internal use
