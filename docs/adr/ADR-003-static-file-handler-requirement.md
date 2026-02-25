# ADR-003: Static File Handler Requirement Resolution

## Status: Proposed

## Context
R2.4 specifies behavior for static file handler, but Kruda Phase 1-4 has no built-in static file handler. This creates an unimplementable acceptance criteria.

Current framework focuses on API development, not full-stack web serving. Adding static file serving would expand scope significantly.

## Decision
**Option C: Reword R2.4 to be conditional**

Change R2.4 from:
"IF a static file handler is configured..."

To:
"WHEN static file serving is added in a future phase, it SHALL..."

This preserves the specification intent while acknowledging current implementation reality.

## Consequences
**Positive:**
- Maintains specification completeness for future development
- Doesn't force premature feature implementation
- Keeps Phase 5 scope focused on ecosystem tools
- Provides clear guidance for future static file implementation

**Negative:**
- Delays static file serving capability
- May require users to use external solutions (nginx, CDN)
- Creates forward compatibility obligation

## Alternatives Considered
- **Option A (cut R2.4)**: Loses valuable specification for future phases
- **Option B (add static handler)**: Significant scope creep, delays Phase 5 completion