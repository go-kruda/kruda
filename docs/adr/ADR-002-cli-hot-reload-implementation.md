# ADR-002: CLI Hot Reload Implementation Strategy

## Status: Proposed

## Context
Conflict between hot reload implementation and dependency policy:
- Decision D-003: Use fsnotify for hot reload in CLI tool
- NFR-9: CLI tool must have "zero external dependencies beyond stdlib and cobra"

fsnotify provides robust cross-platform file watching but violates the zero-dependency principle. Stdlib-only solutions require polling which is less efficient but maintains dependency minimalism.

## Decision
**Option B: Implement file watching with stdlib-only polling**

Use `os.Stat()` polling with intelligent optimizations:
- 500ms poll interval (configurable)
- Watch only Go files (*.go) in project directories
- Skip vendor/, .git/, node_modules/ directories
- Debounce rapid file changes (300ms window)
- Graceful degradation if file system is slow

## Consequences
**Positive:**
- Maintains zero external dependencies principle
- Simpler deployment (no CGO dependencies on some platforms)
- Predictable behavior across all platforms
- Easier to debug and maintain

**Negative:**
- Higher CPU usage than fsnotify (minimal impact for dev tool)
- Slight delay in change detection (500ms vs immediate)
- More complex implementation for recursive directory watching

## Alternatives Considered
- **Option A (fsnotify exception)**: Violates core architectural principle
- **Option C (rewrite NFR-9)**: Would weaken dependency discipline
- **Option D (plugin system)**: Over-engineering for single feature