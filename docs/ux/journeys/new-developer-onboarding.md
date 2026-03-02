# User Flow: New Developer Onboarding

## Flow Diagram

```mermaid
graph TD
    A[Developer discovers Kruda] --> B{Has Go experience?}
    B -->|Yes| C[Read README & Quick Start]
    B -->|No| D[Learn Go basics first]
    D --> C
    
    C --> E[Install: go get kruda]
    E --> F[Copy Hello World example]
    F --> G[Run: go run main.go]
    G --> H{Works correctly?}
    
    H -->|No| I[Check error message]
    I --> J[Fix Go module/version issue]
    J --> G
    
    H -->|Yes| K[Success! Server running]
    K --> L[Try basic routing]
    L --> M[Add middleware]
    M --> N{Ready for typed handlers?}
    
    N -->|Not yet| O[Build more basic routes]
    O --> N
    
    N -->|Yes| P[Read typed handler docs]
    P --> Q[Create first C[T] handler]
    Q --> R{Compile error?}
    
    R -->|Yes| S[Fix struct tags/generics]
    S --> Q
    
    R -->|No| T[Test validation]
    T --> U[See structured errors]
    U --> V[Excited about DX!]
    
    V --> W[Explore advanced features]
    W --> X[Production deployment]
    X --> Y[Advocate to team]
```

## Critical Success Factors

### Entry Points
- **Documentation clarity** - README must hook developers in 30 seconds
- **Installation simplicity** - Single `go get` command
- **Quick wins** - Hello World must work immediately

### Potential Friction Points
- **Generic syntax** - `C[T]` may intimidate Go developers new to generics
- **Struct tag learning** - Validation syntax requires documentation
- **Error messages** - Compiler errors for generic types can be cryptic

### Success Metrics
- Time from discovery to first successful request: **< 5 minutes**
- Time to first typed handler: **< 30 minutes**  
- Developer satisfaction at each checkpoint: **> 4/5**

## Optimization Opportunities

### Reduce Cognitive Load
- Provide copy-paste examples for common patterns
- Interactive playground for testing concepts
- Progressive disclosure of advanced features

### Improve Error Recovery
- Better error messages for common mistakes
- Suggested fixes for validation syntax errors
- Links to relevant documentation sections

### Accelerate Learning
- Video tutorials for visual learners
- Migration guides from popular frameworks
- Community examples and patterns