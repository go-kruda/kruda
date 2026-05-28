# Wing Phase 5 Closure

Phase 5 fair-handler runtime research is closed with no runtime candidate accepted.

## Scope

- Workload: CPU-bound fair handler routes.
- Routes: `plaintext-handler`, `json-static`, and `json-serialize`.
- Goal: find whether a credible path exists toward a broad 20% median RPS advantage over Actix without weakening correctness, security, or default framework behavior.

## Evidence

Merged evidence:

- `2026-05-28-phase5-v124-baseline-evidence.md`
- `2026-05-28-wing-cpu-profile-evidence.md`
- `2026-05-28-wing-default-profile-evidence.md`
- `2026-05-28-wing-affinity-rejection-evidence.md`
- `2026-05-28-wing-epoll-idle-spin-rejection-evidence.md`
- `2026-05-28-wing-phase5-candidate-rejections.md`

The Phase 5 candidate evidence was merged in PR #73.

## Decision

No Phase 5 runtime candidate cleared the balanced fair-handler gate.

The accepted baseline remains:

- `KRUDA_WORKERS=4`
- `GOMAXPROCS=8`
- `KRUDA_READ_BUF_SIZE=4096`

The public wording remains conservative. The Phase 5 evidence does not support a broad 20% Actix win claim. It supports the existing CPU-bound fair-handler messaging and the existing evidence-specific claim gates.

## Rejected Directions

- CPU affinity for Wing workers.
- Epoll idle-spin variants.
- Removing the speculative post-send read.
- CPU Spear takeover for fair handler routes.
- Switching the benchmark baseline to Go 1.26.3.
- Changing `GOMAXPROCS` or `KRUDA_WORKERS` away from the current balanced profile.

## Next Direction

Future performance work should be opened as a new track instead of extending Phase 5:

- Workload-specific Wing profiles and explicit Feather composition.
- JSON-specific handler-path Feathers with JSON-only claim boundaries.
- Real-world API credibility work outside the fair-handler CPU-bound claim.
- A larger Linux I/O architecture design only if it starts from new evidence and a rollback-safe plan.
