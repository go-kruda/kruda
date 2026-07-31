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

- **The vulnerability is confirmed real and written down as an
  advisory.** One condition; how it gets satisfied depends on where the
  vulnerability came from, and all three routes must work:

  - **In a dependency** — upstream's published advisory (`CVE-…`,
    `GHSA-…`, `GO-YYYY-NNNN`) is the confirmation. Upstream did the
    assessing. This is the GO-2026-6061 grpc case.
  - **Reported by someone else** — the maintainer accepts it. GitHub's
    states make this checkable: a privately reported vulnerability
    arrives with status **`Triage`**, is a separate object from a draft
    advisory, and becomes one only when the maintainer clicks **Accept
    and open as draft**. That accept step *is* the confirmation.
  - **Found by the maintainer** — a fuzz finding, a govulncheck result, a
    code review. There is no report to accept, so the maintainer drafts
    the advisory directly. Requiring an external reporter would be
    perverse here: `FuzzParserDifferential` and the govulncheck workflow
    exist precisely to find these, and a smuggling vector Kruda's own
    fuzzing catches needs the patch channel as much as one a stranger
    emails in.

  What all three share is the artifact: an advisory naming the affected
  component, the affected versions, and the impact. That is the line
  against relabelling, because a vulnerability can be written up that way
  and general hardening cannot. Two disqualifying cases follow, and both
  are worth stating rather than implying:

  - A report still in `Triage` — an unassessed claim from anyone. It must
    never authorize a behaviour-changing patch, however urgent it sounds.
    `SECURITY.md` allows 7 days for assessment; if a fix cannot wait for
    it, ship the fix as a minor.
  - An assessment that finished and concluded *not* a vulnerability, out
    of scope, or won't-fix. "Assessed" is not the test; **confirmed** is.
    A closed report qualifies for nothing.

  Publication may lag confirmation — an embargoed fix still qualifies,
  because the confirming has already happened.

  Proactive hardening identifies no specific exploitable defect, so no
  advisory can be written for it and it takes the ordinary route — that
  is why v1.4.0's accept-side DoS caps were a minor, not a patch.

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
- The residual risk is named rather than papered over, and it sits in the
  maintainer-discovered route specifically: there, the maintainer both
  finds the vulnerability and confirms it, so that route is self-served
  by construction. Closing it is not possible in a single-maintainer
  project — any gate loops back to the same person. What the gate buys is
  that qualifying requires drafting an advisory that names an affected
  component, affected versions, and an impact, which is a deliberate
  written act rather than a word chosen while writing a CHANGELOG entry.
  It raises the cost of a wrong call; it cannot remove it.
- ADR 0001 remains the record of why v1.3.0 broke the API. It no longer
  governs new decisions.
