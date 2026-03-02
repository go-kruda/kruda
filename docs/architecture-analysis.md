# Kruda System Architecture Analysis

## Executive Summary

Kruda is a high-performance Go web framework designed around zero-allocation principles and pluggable transport interfaces. The architecture prioritizes performance through careful memory management, efficient routing, and transport abstraction while maintaining developer ergonomics.

## Core Architectural Components

### 1. Pluggable Transport Interface

**Design Pattern**: Strategy Pattern with Transport Abstraction
- **Interface**: `transport.Transport` provides unified API across HTTP implementations
- **Implementations**: fasthttp, net/http, Wing (io_uring/kqueue)
- **Selection Logic**: Automatic fallback based on platform and TLS requirements

**Strengths**:
- Transport-agnostic application code
- Performance optimization per platform (Wing on Linux, fasthttp elsewhere)
- Graceful degradation (TLS → net/http, Windows → net/http)
- Zero-cost abstraction when using embedded adapters

**Risks**:
- Transport feature parity challenges (HTTP/2, WebSocket support varies)
- Debugging complexity across different transport implementations
- Potential performance regression if abstraction layer adds overhead

### 2. Radix Tree Router

**Design Pattern**: Compressed Trie with Method-Specific Trees
- **Structure**: Separate radix tree per HTTP method with O(1) method dispatch
- **Optimization**: Static route map for exact matches, frequency-based child sorting
- **Parameters**: Fixed-size array (8 slots) avoiding map allocations

**Strengths**:
- O(1) static route lookup via pre-computed map
- Zero-allocation parameter extraction using fixed arrays
- AOT optimizations (tree flattening, frequency sorting)
- Efficient wildcard and regex constraint support

**Risks**:
- 8-parameter limit may constrain complex APIs
- Memory usage scales with route count (separate trees per method)
- Route compilation time increases with large route sets

### 3. Middleware Chain Execution

**Design Pattern**: Pre-built Handler Chains with Lifecycle Hooks
- **Chain Building**: Compile-time handler chain construction
- **Execution**: Index-based traversal with `Next()` calls
- **Hooks**: Lifecycle events (OnRequest, BeforeHandle, AfterHandle, OnResponse)

**Strengths**:
- Zero allocation during request processing (pre-built chains)
- Flexible hook system for cross-cutting concerns
- Single boolean flag (`hasLifecycle`) enables fast path when no hooks
- Panic recovery prevents server crashes

**Risks**:
- Hook execution order complexity in error scenarios
- Middleware registration order dependency
- Potential for hook proliferation affecting performance

### 4. Context Lifecycle Management

**Design Pattern**: Object Pool with Dirty Field Tracking
- **Pooling**: `sync.Pool` for context reuse with intelligent cleanup
- **Memory Layout**: Cache-line optimized field ordering (hot fields first)
- **Cleanup**: Dirty flags track which fields need reset

**Strengths**:
- Zero allocation on hot path through context pooling
- Cache-friendly memory layout reduces CPU cache misses
- Selective cleanup based on usage patterns
- Map shrinking prevents unbounded pool memory growth

**Risks**:
- Pool contention under extreme concurrency
- Memory leaks if cleanup logic has bugs
- Context state bleeding between requests if reset incomplete

### 5. Graceful Shutdown System

**Design Pattern**: Coordinated Shutdown with Timeout Management
- **Signal Handling**: SIGINT/SIGTERM trigger graceful shutdown sequence
- **Phases**: Connection draining → hook execution → transport shutdown
- **Hooks**: LIFO execution order with panic recovery

**Strengths**:
- Configurable shutdown timeout prevents hanging
- Hook panic isolation ensures other hooks execute
- DI container integration for resource cleanup
- Connection draining minimizes request loss

**Risks**:
- Shutdown timeout too short may terminate active requests
- Hook dependencies not explicitly managed
- No shutdown progress visibility for operations teams

## Performance Optimizations

### Memory Management
- **Context Pooling**: Eliminates per-request allocations
- **Fixed-Size Arrays**: Route parameters use arrays vs maps
- **String Interning**: Header key canonicalization cache
- **Buffer Pools**: JSON encoding and response buffers

### CPU Efficiency
- **Cache Line Optimization**: Hot fields in first 64 bytes
- **Method Dispatch**: O(1) via array indexing vs string comparison
- **Fast Paths**: Embedded adapters bypass interface calls
- **AOT Compilation**: Route tree optimization at startup

### I/O Performance
- **Zero-Copy**: fasthttp SetBodyRaw for static responses
- **Streaming**: Direct buffer writing without intermediate copies
- **Transport Selection**: Platform-specific optimizations (Wing, fasthttp)

## Security Considerations

### Input Validation
- **Path Traversal**: Optional prevention with URL decoding
- **Header Injection**: CRLF sanitization in header values
- **Body Limits**: Configurable request size limits
- **Parameter Validation**: Regex constraints on route parameters

### Response Security
- **Security Headers**: Configurable default headers (HSTS, CSP, etc.)
- **Error Sanitization**: Production mode strips internal error details
- **Cookie Security**: SameSite, Secure, HttpOnly support

### Operational Security
- **Panic Recovery**: Prevents server crashes from handler panics
- **Resource Limits**: Body size, header size, shutdown timeout
- **Development Mode**: Separate security posture for dev vs production

## Scalability Analysis

### Horizontal Scaling
- **Stateless Design**: No shared state between requests
- **Process Isolation**: Turbo mode with SO_REUSEPORT
- **Resource Efficiency**: Low memory footprint per request

### Vertical Scaling
- **CPU Utilization**: GOMAXPROCS tuning in turbo mode
- **Memory Efficiency**: Object pooling and zero-allocation paths
- **I/O Optimization**: Platform-specific transport selection

### Bottleneck Identification
- **Router Performance**: O(1) static routes, O(log n) dynamic routes
- **Context Pool**: Potential contention point under extreme load
- **JSON Encoding**: Buffer pool contention in high-throughput scenarios

## Architectural Risks & Mitigations

### High-Risk Areas

1. **Transport Abstraction Complexity**
   - *Risk*: Feature parity issues between transports
   - *Mitigation*: Comprehensive integration tests, feature detection

2. **Context Pool Memory Management**
   - *Risk*: Memory leaks or state bleeding
   - *Mitigation*: Extensive cleanup testing, dirty flag validation

3. **Route Parameter Limits**
   - *Risk*: 8-parameter constraint limiting API design
   - *Mitigation*: Document limitation, consider dynamic expansion

### Medium-Risk Areas

1. **Middleware Order Dependencies**
   - *Risk*: Subtle bugs from registration order
   - *Mitigation*: Clear documentation, middleware testing patterns

2. **Shutdown Hook Coordination**
   - *Risk*: Resource cleanup ordering issues
   - *Mitigation*: LIFO execution, panic isolation

### Low-Risk Areas

1. **JSON Encoder Flexibility**
   - *Risk*: Performance regression with custom encoders
   - *Mitigation*: Benchmarking, streaming encoder support

## Recommendations

### Immediate Actions
1. **Monitoring**: Add metrics for pool hit rates, route lookup times
2. **Documentation**: Expand transport selection guidance
3. **Testing**: Increase coverage for edge cases in context cleanup

### Medium-Term Improvements
1. **Route Parameters**: Consider dynamic parameter array expansion
2. **Transport Features**: Standardize feature detection across transports
3. **Observability**: Add structured logging for performance debugging

### Long-Term Considerations
1. **HTTP/3**: Evaluate full HTTP/3 support across all transports
2. **Async I/O**: Explore async handler patterns for I/O-heavy workloads
3. **Resource Management**: Consider more sophisticated resource pooling

## Conclusion

Kruda's architecture successfully balances performance and maintainability through careful abstraction design and zero-allocation principles. The pluggable transport interface provides flexibility while the radix tree router and context pooling deliver excellent performance characteristics. Key risks center around the complexity of maintaining feature parity across transports and ensuring robust memory management in the context pool system.

The framework is well-positioned for high-performance applications requiring HTTP server capabilities, with particular strengths in CPU-bound workloads and scenarios where memory efficiency is critical.