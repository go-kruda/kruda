# ADR-004: Example Selection Strategy

## Status: Proposed

## Context
Two conflicts in example specifications:
1. WebSocket example exists but Kruda has no WebSocket support
2. Auth example choice: stdlib-only vs JWT contrib package

Framework positioning: high-performance API framework, not full-stack solution. Examples should demonstrate real-world patterns users will actually implement.

## Decision
**WebSocket: Option B - Remove entirely**
**Auth: Use JWT contrib package example**

WebSocket removal rationale:
- No current implementation planned
- Aspirational examples create confusion
- Focus examples on available features

JWT auth example rationale:
- More practical for API authentication
- Demonstrates contrib package usage pattern
- Stdlib-only auth is too basic for real applications
- Shows integration between core framework and extensions

## Consequences
**Positive:**
- Examples demonstrate actual framework capabilities
- JWT example shows realistic authentication patterns
- Contrib package integration example valuable for ecosystem
- Reduces user confusion about available features

**Negative:**
- Fewer total examples (9 instead of 10)
- JWT example has external dependency
- May need WebSocket example later if support is added

## Alternatives Considered
- **WebSocket Option A (aspirational)**: Creates user confusion
- **Auth stdlib-only**: Too simplistic for real-world usage
- **Both auth examples**: Redundant, dilutes focus