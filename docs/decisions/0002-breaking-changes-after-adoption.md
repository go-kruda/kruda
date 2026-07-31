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

Breaking and behaviour changes keep shipping inside the v1 line, and the
version number is a signal rather than the protection. What protects
adopters is no longer their absence; it is four obligations that every
such change must meet.

**1. The version number does not carry the warning — the CHANGELOG
does.** Patch is the channel people upgrade through without reading
anything: `go get -u`, Dependabot, `@latest` in CI. The strict conclusion
to draw from that is "a behaviour change must never ship in a patch", and
this ADR deliberately does **not** adopt it. Kruda's v1 line has been
outside strict semver since ADR 0001; release levels here are chosen by
how much attention a release needs, not derived mechanically. The
validation change ships as **v1.7.1** on that basis, despite changing
behaviour and adding two exported symbols.

The obligation that replaces the ban is disclosure, and it is
unconditional where the ban was categorical:

- Every release that can change what an application does carries a
  `### Breaking` section with the opt-out (obligation 2) and the concrete
  before/after (obligation 3) — **whatever its version number**.
- Because a Kruda patch may change behaviour, the patch channel is not
  safe to install unread, and the project must say so plainly rather than
  let the number imply otherwise. An adopter who auto-merges patches
  needs that stated somewhere they will see it, not inferred from semver.

The cost is accepted openly rather than argued away: `gorelease` and
`apidiff` will flag a patch that adds exported API, and an adopter who
trusts semver mechanically will be surprised at some point. Obligations 2
and 3 exist so that the surprise is documented and recoverable instead of
silent.

**2. Ship a documented way to keep the old behaviour.** v1.7.0 has
`-tags kruda_stdjson`; validation-by-default has
`kruda.New(kruda.WithoutValidation())`. The opt-out belongs in the same
CHANGELOG entry as the change, because an adopter who is surprised is
reading that entry and nothing else. A change that cannot offer an
opt-out is a signal to reconsider the change, not to skip the opt-out.

*This is what separates a compile-time break from a behaviour change*, and
obligation 1's indifference to version levels does **not** extend to one.
A removed or renamed exported symbol cannot offer a way to keep the old
behaviour — once it is gone, no flag brings it back.

**A removal can never satisfy this obligation, and a deprecation window
does not change that.** It is tempting to say the window *is* the
opt-out, but the window belongs to an earlier release; in the entry that
actually removes the symbol there is nothing to opt into, and obligation
2 asks for the opt-out in *that* entry, because that is the one a
surprised adopter is reading. Obligation 1 makes this sharper rather than
softer: adopters may now pass through releases without reading them, so
"they had a window" cannot be assumed to mean they saw it.

A removal therefore takes one of two routes, and only one.

**Route A — a deprecation window.** It does not satisfy obligation 2; it
substitutes for it, and the substitution is paid for on both ends:

- **At the deprecation release** — the new symbol lands, the old one
  keeps working, and that entry says the old one is going away. Here the
  opt-out is real and present: keep using the old symbol.
- **At the removal release** — obligation 2 is unsatisfiable, so
  obligation 3 carries the whole load. The entry must give the
  replacement mapping concretely, and must name the version where the
  deprecation was announced, so an adopter who jumped straight over that
  release can find what they missed.
- **Never in a patch.** This is the one place a version level is still
  load-bearing, and for a specific reason: it is the only change with no
  opt-out at all, so the number is the last signal left. A removal ships
  at a minor or above.

**Route B — v2.0.0**, when no window is possible or wanted. Route A's
three requirements do not apply to it: there was no deprecation release
to point back at, and the major version carries the signal on its own.
The removal entry still owes the replacement mapping, because obligation
3 has no escapes.

This ADR is therefore **not** a standing permission slip for removals.
Where the release checklist has always allowed a removal to proceed on
the strength of an accepted ADR, the route this one provides is the
deprecation window, not its own signature. ADR 0001 remains the accepted
exception for v1.3.0's window-less removals; it covers those and nothing
later.

**3. State it concretely.** v1.7.0's before/after byte table is the
standard: what an adopter should grep for, and what they will see if they
do nothing. With obligation 1 no longer reserving behaviour changes to
minors, this is the obligation doing the work the version number used to
be trusted for.

**4. Keep a release to one attributable cause, and stage them.** An
adopter must never have to absorb two *independent* behaviour changes in
one upgrade, so that any breakage they hit has a single suspect. A change
together with the prerequisites it needs to be safe counts as one item —
v1.7.1 carries validation-by-default plus the unknown-rule change that
had to precede it, and that is one cause, not two. The response-byte
change was a second cause, which is why it went out separately as v1.7.0.

Staging is the half with teeth: v1.7.1 waits until Rianhub has absorbed
v1.7.0, so that if something breaks in production there is one thing to
look at. Waiting is the default, and jumping the queue is what has to be
justified.

### The exception: a real attack may jump the queue

A fix for a **specific, identified vulnerability** ships immediately,
out of band, without waiting for the adopter to absorb the previous
release. Not installing it is the greater danger, and Kruda already
depends on this working — `contrib/observability/v1.0.1` delivered the
GO-2026-6061 grpc fix on 2026-07-30 without staging behind anything.

That latitude is the thing worth laundering, so the gate is deliberately
narrow. Almost any tightening can be described as hardening, and
validation-by-default — which rejects malformed input — is exactly the
kind of change that could be relabelled to skip its wait. The exception
applies only when **all three** hold — the vulnerability is
**reproduced**, the reproduction demonstrates an **attack**, and the fix
is **narrow**:

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
  Earlier drafts of this ADR looked for an artifact that would prove the
  vulnerability was confirmed — a received report, an advisory id, a
  completed assessment, a maintainer's accept click, a filled-in advisory
  — and every one was satisfiable without anyone having confirmed
  anything. The last is the clearest: `SECURITY.md` asks reporters to
  supply affected versions and an impact assessment, so a filled-in
  advisory is often just the reporter's own claim, restated. A failing
  test cannot be delegated to the reporter, cannot be produced by
  clicking anything, and is checkable by anyone reading the diff.

  Nothing about *provenance* matters, therefore. A report from a stranger
  and a hit from `FuzzParserDifferential` take the identical path, because
  the test they both have to produce is the same one.

  So the disqualifying cases are:

  - A report still in `Triage`, or one accepted into a draft while the
    investigation runs. Neither is a reproduction. However urgent it
    sounds, `SECURITY.md` allows 7 days for assessment; a fix that cannot
    wait for it takes its turn in the queue like anything else.
  - An assessment that concluded *not* a vulnerability, out of scope, or
    won't-fix.
  - A fix whose diff carries no test that fails without it. A defect that
    cannot be demonstrated has not been confirmed, and an unconfirmed
    defect is not a reason to skip staging.

  Publication may lag reproduction — an embargoed fix still qualifies.

  Proactive hardening has no specific defect to reproduce, so no such
  test can be written for it and it waits its turn. That is why v1.4.0's
  accept-side DoS caps shipped as an ordinary release.

- **The reproduction demonstrates an attack, not merely a defect.** This
  second condition is what does the work, because the first admits
  everything on its own: *every* bug fix carries a test that fails
  without it, so a gate that stops at "there is a failing test" lets any
  change jump the queue under a security label.

  The distinction that has to be made, and written into the advisory:

  - An ordinary bug is the software doing the wrong thing for a
    legitimate user.
  - A vulnerability is an **attacker** causing the software to violate a
    security property — confidentiality, integrity, or availability —
    across a boundary Kruda states or implies it enforces.

  So the advisory must answer three questions the failing test cannot:
  **who is the attacker, what do they gain, and which boundary breaks.**
  If the honest answer to "what do they gain" is "nothing they did not
  already have", it is an ordinary bug and waits its turn, however
  severe.

  The reproduction has to be shaped accordingly: it drives adversarial
  input across the boundary and shows the boundary broken.
  `TestWingParser_RejectsMalformedHeaderLines` is the shape — a crafted
  request, a parser that must not be fooled by it — rather than a test
  asserting a correct value for well-formed input.

  Worked example, because it is the case most likely to be relabelled.
  Validation-by-default is easy to reproduce: a test showing a malformed
  email reaching a handler fails without the change. It is still not a
  vulnerability. The attacker gains nothing Kruda ever prevented —
  `validate` tags were inert, so an application relying on them was
  relying on something that never ran, and whether that harms the
  application is a property of the application, not of a Kruda boundary.
  So v1.7.1 waits for Rianhub to absorb v1.7.0; it does not jump.

- **The fix is narrow enough to ship on its own.** Jumping the queue
  means shipping without the soak the staging rule would have provided,
  so a queue-jumping fix must not carry anything that needed one. A fix
  that moves a floor every adopter must follow takes the ordinary route
  with a `### Security` section, as v1.5.0's Go-version bump did — even
  though it carried two advisory ids.

Obligation 2 is **waived** for a queue-jumping fix, not satisfied by some
narrower reading of it: an opt-out that reinstates the vulnerability must
not exist, so there is deliberately no way to keep the old behaviour.
What must stay tunable is any *limit* the fix introduces (v1.4.0's caps
ship with `WithMaxConns(0)`), never the rejection itself.

It is worth listing where the escapes are, since each is a place the
policy could quietly stop binding:

| obligation | escape | what it is |
|---|---|---|
| 1 — disclose | none | every change discloses, at every level |
| 2 — opt-out | removal, Route A | *substituted* by a deprecation window, paid on both ends |
| 2 — opt-out | removal, Route B | *waived*; v2.0.0 carries the signal instead |
| 2 — opt-out | queue-jumping security fix | *waived* outright |
| 3 — state concretely | none | and it absorbs the load wherever obligation 2 is escaped |
| 4 — one cause | none | two independent behaviour changes are always two releases |
| 4 — stage them | queue-jumping security fix | ships out of band, ahead of the queue |

Nothing here is dressed up as compliance. Where an obligation does not
hold, it says so.

Adopter count is no longer an argument in any of this. "Adopters ≈ 0" was
retired with ADR 0001 and must not be cited again.

## Consequences

- The v1 line remains not strictly semver-compliant, and this ADR widens
  that rather than narrowing it: a patch may now change behaviour and add
  exported API. This is a stated policy with obligations attached rather
  than a one-off exception, but the honest summary is that Kruda's
  version numbers describe intent, not a mechanical contract.
- **The patch channel is no longer safe to install unread.** That
  guarantee is what obligation 1 gave up, and nothing else replaces it —
  obligations 2 and 3 make the change documented and reversible, not
  invisible in advance. Adopters running unattended patch auto-merge need
  to know this, so it belongs in adopter-facing docs and not only here.
- Two independent behaviour changes ready together still mean two
  releases, not one. That, not the version level, is what keeps a single
  attributable cause per upgrade.
- The release checklist gains a question per breaking or behaviour
  change: is the opt-out documented, is the CHANGELOG note concrete
  enough to act on, and does any breakage this release could cause trace
  back to a single suspect.
- v2.0.0 stays available, and is now the answer for a change that cannot
  offer an opt-out at all — in practice a removal nobody wants to
  precede with a deprecation window — rather than a milestone deferred
  until adoption arrives.
- Compile-time breaks got *stricter* here while behaviour changes got
  looser, which is the opposite of how the two are usually ranked. It
  follows from obligation 2 rather than from taste: a behaviour change
  can be handed an opt-out, and a deleted symbol cannot.
- A real security fix ships out of band, ahead of the staging queue. The
  cost is that such a fix must stay narrow enough to go out without a
  soak behind it.
- Only the first of the three exception conditions is objective: a test
  either fails against the unfixed tree or it does not, and CI can check
  which. That is a deliberate improvement over earlier drafts of this
  ADR, which rested entirely on documents somebody could simply write.
- The second and third conditions — is it an attack, is the fix narrow —
  are judgements, and are stated as judgements rather than dressed up as
  checks. The second is not left as a footnote, because the first admits
  every bug fix on its own; it is a gate condition with three questions
  attached, and answering them in writing is the work. In a
  single-maintainer project no procedure closes either judgement. What
  the gate buys is that a wrong call leaves behind a runnable test and a
  written threat model, so it is discoverable afterwards rather than
  invisible.
- ADR 0001 remains the record of why v1.3.0 broke the API. It no longer
  governs new decisions.
