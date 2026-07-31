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

- **The vulnerability has been reproduced.** Not reported, not accepted,
  not written up — reproduced, by the maintainer, with the reproduction
  in the repository:

  - **In Kruda's own code** — the fix's diff contains a regression test
    that fails without the fix and passes with it. That is already how
    this repo works: `TestWingParser_RejectsMalformedHeaderLines` and its
    neighbours exist because specific parser defects were reproduced
    first.
  - **In a dependency** — upstream's published advisory (`CVE-…`,
    `GHSA-…`, `GO-YYYY-NNNN`) supplies the analysis, and govulncheck
    supplies the reachability from Kruda. That is exactly how
    GO-2026-6061 was handled: govulncheck reported it reachable from
    `observability.buildSDK`, which is what made it Kruda's problem
    rather than a line in someone else's changelog.

  **Reproduction is the requirement because no document can carry it.**
  Four earlier drafts of this ADR looked for an artifact that would prove
  the vulnerability was confirmed — a report, an advisory id, a completed
  assessment, a filled-in advisory — and each was satisfiable without
  anyone having confirmed anything. The last is the clearest: `SECURITY.md`
  asks reporters to supply affected versions and an impact assessment, so
  a filled-in advisory is often just the reporter's own claim, restated.
  A failing test cannot be delegated to the reporter, cannot be produced
  by clicking anything, and is checkable by anyone reading the diff.

  Nothing about *provenance* matters, therefore. A report from a stranger
  and a hit from `FuzzParserDifferential` take the identical path, because
  the test they both have to produce is the same one.

  So the disqualifying cases are:

  - A report still in `Triage`, or one accepted into a draft while the
    investigation runs. Neither is a reproduction. However urgent it
    sounds, `SECURITY.md` allows 7 days for assessment; a fix that cannot
    wait ships as a minor.
  - An assessment that concluded *not* a vulnerability, out of scope, or
    won't-fix.
  - A fix whose diff carries no test that fails without it. If the
    defect cannot be demonstrated, it has not been confirmed — and a fix
    for something undemonstrated is not safe to install unread, which is
    the whole test in obligation 1.

  Publication may lag reproduction — an embargoed fix still qualifies.

  Proactive hardening has no specific defect to reproduce, so no such
  test can be written for it and it takes the ordinary route. That is why
  v1.4.0's accept-side DoS caps were a minor, not a patch.

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
- Most of the gate is now objective rather than self-attested: a test
  either fails against the unfixed tree or it does not, and anyone
  reading the diff — or CI — can check which. That is a deliberate
  improvement over four earlier drafts that each rested on a document
  somebody could simply write.
- The residual risk is named rather than papered over, and reproduction
  shrinks it to one question it cannot answer: whether a demonstrated
  defect is a *security* defect rather than an ordinary bug. A failing
  test proves the behaviour exists; it cannot prove the behaviour is
  exploitable. That call stays with the maintainer, and in a
  single-maintainer project no gate closes it. What the gate does is
  leave a runnable artifact behind, so a wrong call is discoverable
  afterwards instead of invisible.
- ADR 0001 remains the record of why v1.3.0 broke the API. It no longer
  governs new decisions.
