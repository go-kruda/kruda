# ADR-001: Benchmark Regression Threshold

## Status: Proposed

## Context
Two conflicting specifications exist for benchmark regression thresholds:
- docs/features Task 10: >5% regression triggers warning
- .kiro spec R9.4: >10% regression triggers warning

Go benchmarks have inherent variance due to:
- GC timing variations (5-15% typical)
- CPU thermal throttling
- Background processes on CI runners
- Memory allocation patterns

CI pipeline needs balance between catching real regressions vs false positives.

## Decision
Use **10% regression threshold** for CI warnings with additional safeguards:
- Run benchmarks 3 times, use median result
- Compare against rolling 7-day average, not single baseline
- Require 2 consecutive builds with >10% regression before alerting
- Add manual benchmark review process for borderline cases

## Consequences
**Positive:**
- Reduces false positive alerts from normal Go benchmark variance
- Maintains developer velocity by avoiding noise
- Aligns with Go community practices (most projects use 8-15%)
- Allows focus on meaningful performance issues

**Negative:**
- May miss smaller but real performance regressions (5-10% range)
- Requires more sophisticated benchmark analysis tooling
- Need manual review process for edge cases

## Alternatives Considered
- **5% threshold**: Too sensitive for Go's GC variance, would create alert fatigue
- **15% threshold**: Too permissive, might miss significant regressions
- **Adaptive threshold**: Complex to implement, harder to reason about