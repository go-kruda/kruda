# Transport Configuration User Flow

```mermaid
graph TD
    A[Start New Project] --> B{Performance Critical?}
    B -->|Yes| C[Check Feature Requirements]
    B -->|No| D[Use Default net/http]
    
    C --> E{Need HTTP/2?}
    E -->|Yes| F[Use net/http]
    E -->|No| G{Need Multipart Upload?}
    
    G -->|Yes| H[Use net/http or netpoll]
    G -->|No| I[Use fasthttp for max perf]
    
    D --> J[kruda.New()]
    F --> K[kruda.New(kruda.WithNetHTTP())]
    H --> L[kruda.New(kruda.WithNetpoll())]
    I --> M[kruda.New(kruda.WithFastHTTP())]
    
    J --> N[Development]
    K --> N
    L --> N
    M --> N
    
    N --> O{Performance Issues?}
    O -->|Yes| P[Benchmark & Switch Transport]
    O -->|No| Q[Production Ready]
    
    P --> R{Feature Conflicts?}
    R -->|Yes| S[Choose Compromise Transport]
    R -->|No| T[Use Fastest Compatible]
    
    S --> Q
    T --> Q
```