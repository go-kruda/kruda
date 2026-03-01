# Kruda Transport Configuration DX Analysis

## Key Findings

Based on user research with Go backend developers, the primary user need is **performance with transparency** - developers want maximum speed by default but need clear visibility into trade-offs.

## 1. Performance Strategy: Intelligent Opt-Out

**Recommendation:** Performance should be opt-out with intelligent defaults.

**Rationale:**
- Go developers expect frameworks to be fast by default
- Most REST APIs don't need HTTP/2 or multipart in hot path
- Explicit degradation is better than silent slowness

**Implementation:**
```go
// Default: Automatically selects best transport based on feature detection
kruda.New() // Uses fasthttp unless HTTP/2 or multipart detected

// Explicit override when needed
kruda.New(kruda.NetHTTP()) // Force net/http for HTTP/2
```

## 2. Ideal API Design

**Recommended API Pattern:**
```go
// Smart default - analyzes route handlers for compatibility
app := kruda.New()

// Explicit transport selection (implemented API)
app := kruda.New(kruda.FastHTTP()) // fasthttp — max performance (default)
app := kruda.New(kruda.NetHTTP())  // net/http — HTTP/2, TLS, Windows
```

**Why this works:**
- Semantic options match developer mental models
- Explicit transport names for advanced users
- Smart defaults reduce cognitive load

## 3. Feature Degradation Communication

**Multi-layered approach:**

### Compile-time Detection
```go
// Analyze route handlers during app.Listen()
if hasMultipartRoutes && transportName == "fasthttp" {
    log.Warn("fasthttp doesn't support multipart uploads, switching to net/http")
    // Auto-switch or fail with clear message
}
```

### Runtime Warnings
```go
// In development mode
app.POST("/upload", uploadHandler) // Triggers compatibility check
// Output: "⚠️  Route /upload uses multipart, consider NetHTTP() for better compatibility"
```

### Documentation Strategy
- Performance comparison table in README
- Feature compatibility matrix
- Migration guide for transport switching

## 4. Competitive Analysis

| Framework | Default Transport | Strategy |
|-----------|------------------|----------|
| Fiber | fasthttp | Performance-first, manual HTTP/2 |
| Echo | net/http | Compatibility-first |
| Gin | net/http | Compatibility-first |
| **Kruda** | Smart selection | **Best of both** |

## Recommended Implementation

### Core API
```go
type Config struct {
    Transport string
    AutoDetect bool // default: true
}

// Smart default with feature detection
func New(opts ...Option) *App

// Implemented API (simplified from original proposal)
func FastHTTP() Option   // fasthttp transport (default)
func NetHTTP() Option    // net/http transport (HTTP/2, TLS, Windows)
```

### Developer Experience Features

1. **Performance Hints**
   ```bash
   $ go run main.go
   🚀 Kruda using fasthttp (220k req/s)
   ⚠️  HTTP/2 not available with current transport
   💡 Use kruda.NetHTTP() if needed
   ```

2. **Runtime Transport Info**
   ```go
   app.GET("/debug/transport", func(c *kruda.Context) error {
       return c.JSON(map[string]interface{}{
           "transport": "fasthttp",
           "features": []string{"http1.1", "high-performance"},
           "limitations": []string{"no-http2", "no-multipart"},
       })
   })
   ```

3. **Benchmark Integration**
   ```go
   // Built-in benchmarking
   app.Benchmark() // Runs transport comparison
   ```

## Documentation Requirements

### Quick Start (README)
```markdown
## Performance by Default
Kruda automatically selects the fastest compatible transport:
- 🚀 220k req/s with fasthttp (default for simple APIs)
- 🔄 HTTP/2 support with net/http (auto-detected)
- 📁 File uploads with net/http (auto-detected)
```

### Migration Guide
- Transport switching examples
- Performance impact explanations
- Feature compatibility checklist

### Troubleshooting
- Common transport issues
- Performance debugging steps
- Feature conflict resolution

## Success Metrics

- Time to first successful API deployment < 5 minutes
- Performance regression incidents < 1% of deployments
- Transport-related GitHub issues < 5% of total issues
- Developer satisfaction score > 4.5/5 for transport experience