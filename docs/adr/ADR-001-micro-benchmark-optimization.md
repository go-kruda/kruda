# ADR-001: Micro-Benchmark Optimization Strategy

## Status: Proposed

## Context

Kruda currently loses micro-benchmarks against Echo/Gin due to transport abstraction overhead:

**Current Results (httptest.NewRecorder path):**
- Echo: 238ns/op, 10 allocs/op
- Gin: 250ns/op, 9 allocs/op  
- Kruda: 392ns/op, 19 allocs/op

**Root Cause Analysis:**
1. **Transport Wrapper Overhead**: Kruda wraps `http.ResponseWriter`/`*http.Request` via `transport.NewNetHTTPResponseWriter` + `NewNetHTTPRequestWithLimit`, adding ~9 extra allocations
2. **Allocation Sources**: 
   - `netHTTPRequest` struct allocation
   - `netHTTPResponseWriter` struct allocation  
   - `netHTTPHeaderMap` struct allocation
   - Internal slices/maps for query parsing, headers
   - Context pool get/put (though this is optimized)

**Business Impact:**
- Micro-benchmarks are used for framework comparisons in blog posts, GitHub stars, adoption decisions
- Real-world performance (220k req/s vs 100k req/s) shows Kruda wins, but perception matters
- Need to win both micro-benchmarks AND real-world benchmarks

## Decision

**Implement dual-path architecture with zero-allocation fast path for net/http:**

### 1. Add `ServeHTTP` Method to App
```go
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Direct net/http path - zero transport wrapper allocations
    // Use stack-allocated request/response adapters
    // Bypass transport.NewNetHTTP* entirely
}
```

### 2. Optimize Transport Wrappers (Secondary)
- Pool `netHTTPRequest`/`netHTTPResponseWriter` structs
- Use sync.Pool for header maps
- Lazy query parsing (only allocate when accessed)

### 3. Maintain Transport Abstraction
- Keep `ServeKruda(transport.ResponseWriter, transport.Request)` for fasthttp/QUIC
- `ServeHTTP` becomes the micro-benchmark fast path
- Both paths share the same core routing/middleware logic

## Architecture Design

### Component Responsibilities

**File: `kruda.go`**
- Add `ServeHTTP(w http.ResponseWriter, r *http.Request)` method
- Implement stack-allocated request/response adapters
- Share core logic with `ServeKruda` via internal `serveRequest` function

**File: `transport/nethttp.go`** 
- Optimize existing wrappers with object pooling
- Keep for non-micro-benchmark use cases (fasthttp compatibility)

**File: `fast_adapters.go`** (new)
- Stack-allocated `fastHTTPRequest`/`fastHTTPResponseWriter` structs
- Zero-allocation implementations of transport interfaces
- Embedded directly in `ServeHTTP` method (no heap allocation)

### Data Flow

**Micro-benchmark path (ServeHTTP):**
```
http.ResponseWriter/Request → stack adapters → core logic → response
```

**Production path (ServeKruda):**
```  
transport.ResponseWriter/Request → core logic → response
```

**Shared core logic:**
```go
func (app *App) serveRequest(w transport.ResponseWriter, r transport.Request) {
    // Context pool, routing, middleware, error handling
    // Same logic for both paths
}
```

## Implementation Blueprint

### Phase 1: Add ServeHTTP Method
```go
// In kruda.go
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Stack-allocated adapters (zero heap allocation)
    req := &fastHTTPRequest{r: r, path: r.URL.Path}
    resp := &fastHTTPResponseWriter{w: w, statusCode: 200}
    
    app.serveRequest(resp, req)
}

func (app *App) ServeKruda(w transport.ResponseWriter, r transport.Request) {
    app.serveRequest(w, r)
}

func (app *App) serveRequest(w transport.ResponseWriter, r transport.Request) {
    // Extract existing ServeKruda logic here
    // Context pool, routing, middleware execution
}
```

### Phase 2: Stack-Allocated Adapters
```go
// Embedded in ServeHTTP - no heap allocation
type fastHTTPRequest struct {
    r    *http.Request
    path string
}

type fastHTTPResponseWriter struct {
    w          http.ResponseWriter  
    statusCode int
    written    bool
}

type fastHTTPHeaderMap struct {
    h http.Header
}
```

### Phase 3: Optimize Transport Wrappers (Optional)
- Add sync.Pool for `netHTTPRequest`/`netHTTPResponseWriter`
- Lazy query parsing in `QueryParam()`
- Pool header map allocations

## Build Sequence

1. **Extract Core Logic** (1 hour)
   - Move `ServeKruda` logic to `serveRequest(w, r)`
   - Verify existing tests pass

2. **Add ServeHTTP Method** (2 hours)  
   - Implement stack-allocated adapters
   - Add `ServeHTTP` method calling `serveRequest`
   - Ensure http.Handler interface compliance

3. **Benchmark Validation** (1 hour)
   - Run micro-benchmarks: target <250ns/op, <10 allocs/op
   - Verify real-world benchmarks unchanged
   - Compare against Echo/Gin

4. **Integration Testing** (1 hour)
   - Test ServeHTTP with standard http.Server
   - Verify middleware/routing works identically
   - Test error handling parity

## Target Performance

**Micro-benchmark goals:**
- **Latency**: <250ns/op (beat Gin's 250ns)
- **Allocations**: <10 allocs/op (match Echo's 10)
- **Memory**: <1100B/op (beat Echo's 1024B)

**Success Criteria:**
- Win micro-benchmarks against Echo/Gin
- Maintain real-world performance advantage (220k req/s)
- Zero regression in existing transport.ServeKruda path

## Consequences

### Positive
- **Wins micro-benchmarks** - crucial for adoption/perception
- **stdlib compatibility** - can use Kruda as drop-in http.Handler
- **Maintains transport abstraction** - fasthttp/QUIC support unchanged
- **Minimal code duplication** - shared core logic

### Negative  
- **Dual maintenance paths** - ServeHTTP + ServeKruda must stay in sync
- **Code complexity** - two entry points instead of one
- **Testing overhead** - must test both paths for feature parity

### Risks
- **Logic divergence** - ServeHTTP and ServeKruda could drift apart
- **Performance regression** - shared core logic must stay optimized

### Mitigation
- **Shared core function** - `serveRequest()` prevents logic duplication
- **Comprehensive tests** - test both paths with identical scenarios  
- **Benchmark CI** - catch performance regressions automatically

## Alternatives Considered

### Alternative 1: Optimize Transport Wrappers Only
**Pros**: Single code path, simpler maintenance
**Cons**: Still has allocation overhead, harder to reach <10 allocs target
**Verdict**: Insufficient - transport abstraction inherently adds allocations

### Alternative 2: Remove Transport Abstraction Entirely  
**Pros**: Maximum performance, simple codebase
**Cons**: Lose fasthttp support, major breaking change, lose competitive advantage
**Verdict**: Rejected - fasthttp gives real-world performance edge

### Alternative 3: Conditional Compilation (Build Tags)
**Pros**: Zero overhead when not needed
**Cons**: Complex build matrix, harder testing, user confusion
**Verdict**: Rejected - adds complexity without clear benefit

## Implementation Notes

### Critical Details

**Error Handling**: Both paths must handle panics, errors, and edge cases identically

**State Management**: Context pool, security headers, hooks must work in both paths

**Testing Strategy**: 
- Unit tests for both ServeHTTP and ServeKruda
- Integration tests ensuring identical behavior
- Benchmark tests for performance validation

**Performance Monitoring**:
- CI benchmarks for both micro and real-world scenarios
- Allocation tracking to prevent regressions
- Memory profiling for optimization opportunities

**Security Considerations**:
- Both paths must apply security headers consistently
- Path traversal protection in both entry points
- Request size limits enforced identically