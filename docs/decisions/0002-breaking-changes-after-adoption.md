# ADR 0002: Breaking and behaviour changes now that Kruda has a production adopter

Date: 2026-07-31
Status: Accepted

Replaces the reasoning of ADR 0001, not its outcome.

## Context

ADR 0001 permitted breaking changes in v1 minor releases on a single
premise: "the project currently has effectively zero external adopters —
the maintainer is the only known user." That premise expired on
2026-06-29, when Rianhub — a browser-facing production service on Linux —
was confirmed to be running Kruda. Adopters are still few, but they are
no longer none, and "the compatibility promise exists to protect users
who do not yet exist" is no longer a true statement.

Since then two changes have been governed by a rule that was applied but
never written down:

- v1.7.0 changed which JSON engine a `CGO_ENABLED=0` build gets, and so
  changed the bytes of every JSON response containing `<`, `>` or `&`.
  No user's build flags changed; only Kruda's behaviour did.
- The unreleased change that enforces `validate` tags by default can make
  an application start rejecting requests it previously accepted.

Neither is a compile-time break, and that makes them the harder case.
A removed symbol fails loudly, at build time, on a developer's machine.
A behaviour change compiles, passes an unchanged test suite that never
asserted the old behaviour, and lands in production. ADR 0001 and the
release checklist both spoke only about the API surface.

## Decision

Breaking and behaviour changes continue to ship in v1 **minor** releases.
What protects adopters is no longer their absence; it is three
obligations that every such change must meet.

**1. Never in a patch — except to fix a vulnerability.** Patch is the
channel people upgrade through without reading anything — `go get -u`,
Dependabot, `@latest` in CI. Any change that can alter what an
application does, at compile time or at run time, forces at least a
minor. The size of the diff is not the size of the change.

A security fix inverts that argument rather than escaping it. Hardening a
parser or adding a limit *is* a behaviour change — requests that used to
be served now get rejected — but the unread auto-upgrade channel is
exactly where such a fix belongs, because the alternative is adopters
sitting on a known-vulnerable version until they next read release notes.
Kruda already depends on this working: `contrib/observability/v1.0.1`
delivered the GO-2026-6061 grpc fix as a patch on 2026-07-30.

Obligation 2 is what makes the two cases differ, and it is worth stating
as a derivation rather than a carve-out. An ordinary behaviour change
must ship a public opt-out; a public opt-out is new exported API; new
exported API is a minor by definition. So obligations 1 and 2 agree
without needing to be reconciled. A security fix is the case where
obligation 2 produces nothing — an opt-out that reinstates the
vulnerability must not exist — so no new API appears and the patch
channel stays open. What must remain tunable is any *limit* the fix
introduces (v1.4.0's accept-side caps ship with `WithMaxConns(0)`), never
the rejection itself.

The exchange for that latitude is narrowness: a security patch is not a
place to land unrelated tightening. A fix that cannot be kept narrow, or
that moves a floor every adopter must follow, goes in a minor with a
`### Security` section — as v1.5.0's Go-version bump did.

**2. Ship a documented way to keep the old behaviour.** v1.7.0 has
`-tags kruda_stdjson`; validation-by-default has
`kruda.New(kruda.WithoutValidation())`. The opt-out belongs in the same
CHANGELOG entry as the change, because an adopter who is surprised is
reading that entry and nothing else. A change that cannot offer an
opt-out is a signal to reconsider the change, not to skip the opt-out.

**3. State it concretely, and keep a release to one attributable cause.**
v1.7.0's before/after byte table is the standard for "concretely": what
an adopter should grep for, and what they will see if they do nothing.
And an adopter must never have to absorb two *independent* behaviour
changes in one upgrade, so that any breakage they hit has a single
suspect. A change together with the prerequisites it needs to be safe
counts as one item — v1.8.0 carries validation-by-default plus the
unknown-rule change that had to precede it, and that is one cause, not
two. The response-byte change was the second cause, which is why it went
out separately as v1.7.0 and Rianhub takes it first.

Adopter count is no longer an argument in any of this. "Adopters ≈ 0" was
retired with ADR 0001 and must not be cited again.

## Consequences

- The v1 line remains not strictly semver-compliant, but this is now a
  stated policy with obligations attached, rather than a one-off
  exception justified by an empty user base.
- Two behaviour changes ready together mean two minor releases, not one.
  A slower release cadence is the price of a single attributable cause
  per upgrade.
- The release checklist gains a question per breaking or behaviour
  change: is the opt-out documented, is the CHANGELOG note concrete
  enough to act on, and does any breakage this release could cause trace
  back to a single suspect.
- v2.0.0 stays available, and is now the answer for a non-security change
  that cannot offer an opt-out at all — not merely a milestone deferred
  until adoption arrives.
- Security fixes keep the patch channel, which is the one that reaches
  adopters without being read. The cost is that a security patch has to
  stay narrow enough to be safe to install unread.
- ADR 0001 remains the record of why v1.3.0 broke the API. It no longer
  governs new decisions.
