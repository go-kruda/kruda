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

**1. A patch must be safe to install unread.** Patch is the channel
people upgrade through without reading anything — `go get -u`,
Dependabot, `@latest` in CI. Everything else about patches follows from
that one property, so state it that way rather than as a list of bans.

An ordinary behaviour change is not safe to install unread, so it forces
at least a minor. The size of the diff is not the size of the change.

A fix for a **specific, identified vulnerability** is the case where the
test comes out the other way: not installing it is the greater danger, so
it belongs in the unread channel. Kruda already depends on this working —
`contrib/observability/v1.0.1` delivered the GO-2026-6061 grpc fix as a
patch on 2026-07-30.

"Specific and identified" is the whole gate, and it is deliberately
narrow, because a loose one would swallow the rule: almost any tightening
can be described as hardening, and validation-by-default — which rejects
malformed input — is exactly the kind of change that could be relabelled
into a patch. The exemption applies only when **both** hold:

- **The vulnerability has an advisory id.** Upstream (`CVE-…`, `GHSA-…`,
  `GO-YYYY-NNNN`) for a dependency, or one Kruda issues itself through
  GitHub Security Advisories, which is where `SECURITY.md` already routes
  reports. A draft advisory id counts, so an embargo never forces the
  wrong version number.

  The id is what makes this checkable, and the reason to require the id
  rather than the report is that `SECURITY.md` separates acknowledgment
  (48 hours) from assessment (7 days). A *received* report is an
  unassessed claim from anyone; it must not authorize a
  behaviour-changing patch. A report earns an id only by surviving
  assessment, so requiring the id is what puts the triage step inside the
  gate. If a report is still being assessed and a fix cannot wait, ship
  it as a minor.

  Proactive hardening has no id and so takes the ordinary route — that is
  why v1.4.0's accept-side DoS caps were a minor, not a patch.

- **The fix is narrow enough to be safe to install unread.** One that
  moves a floor every adopter must follow goes in a minor with a
  `### Security` section, as v1.5.0's Go-version bump did — even though
  it carried two advisory ids.

Obligation 2 also changes shape here: an opt-out that reinstates the
vulnerability must not exist. What has to stay tunable is any *limit* the
fix introduces (v1.4.0's caps ship with `WithMaxConns(0)`), never the
rejection itself.

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
- The residual risk is named rather than papered over: the maintainer can
  issue a Kruda advisory, so the gate is ultimately self-served. What it
  buys is that doing so is a deliberate, public, written act — a
  published advisory naming an affected component and a reporter — rather
  than a word chosen while writing a CHANGELOG entry. That is the honest
  ceiling for a single-maintainer project; the gate raises the cost of a
  wrong call, it cannot remove it.
- ADR 0001 remains the record of why v1.3.0 broke the API. It no longer
  governs new decisions.
