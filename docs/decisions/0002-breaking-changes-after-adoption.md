# ADR 0002: Behaviour changes while Kruda's only user is its author

Date: 2026-07-31
Status: Accepted

Extends ADR 0001 from the API surface to runtime behaviour. It does not
replace it, and ADR 0001's premise still holds.

## Context

ADR 0001 allows breaking changes in v1 minor releases because Kruda has
effectively no external adopters — the maintainer is the only user. That
is still true.

What changed is not the adopter count but the *kind* of change. Kruda
runs in Rianhub, the maintainer's own browser-facing production service
on Linux, and the last two releases broke things ADR 0001 never spoke
about:

- v1.7.0 changed which JSON engine a `CGO_ENABLED=0` build gets, and so
  changed the bytes of every JSON response containing `<`, `>` or `&`.
- v1.7.1 enforces `validate` tags by default, so an application can start
  rejecting requests it previously accepted.

Neither removes a symbol. ADR 0001 reasoned entirely about the API
surface, where a break fails loudly at build time on a developer's
machine. These compile, pass a test suite that never asserted the old
behaviour, and surface in production.

Because the only affected service belongs to the maintainer, this is a
coordination problem rather than a compatibility promise: the same person
controls both sides, picks the upgrade moment, and can fix forward. The
rules below are sized for that, and are deliberately few.

## Decision

**1. Say what changed, concretely, in the CHANGELOG.** v1.7.0's
before/after byte table is the standard: what to grep for, and what you
will see if you do nothing. This is the rule that actually helps, because
the person debugging Rianhub at 2am has forgotten the change and is
reading that entry.

**2. Give an escape hatch when it is cheap.** v1.7.0 has
`-tags kruda_stdjson`; v1.7.1 has `kruda.New(kruda.WithoutValidation())`.
Both cost almost nothing and buy an instant rollback that does not need a
downgrade. When an escape hatch would be expensive, fixing forward is
fine — that is the advantage of owning both sides.

**3. One behaviour change per release.** Never ship two independent ones
together, so that when production misbehaves there is a single suspect.
A change plus the prerequisites that make it safe is one item: v1.7.1
carries validation-by-default together with the unknown-rule fix it
required. The response-byte change was a second cause, so it went out
separately as v1.7.0, and v1.7.1 waits until Rianhub has taken v1.7.0.

**4. Version numbers signal how much attention a release needs.** They
are not a semver contract — ADR 0001 already put the v1 line outside
strict semver. A patch may change behaviour and add exported API, which
is why validation-by-default is v1.7.1; `gorelease` and `apidiff` will
flag that, and the flag is expected rather than a stop.

A fix for a real vulnerability ships immediately and does not wait its
turn under rule 3.

## Consequences

- Kruda's patch releases are not safe to install without reading the
  CHANGELOG. That is acceptable while the only installer is the person
  writing it, and it is stated in the release notes rather than left for
  someone to discover.
- Rule 3 means two behaviour changes ready together are two releases, so
  the cadence is slower than the work rate.
- ADR 0001 is unchanged and still governs the API surface: removals may
  ship in a v1 minor. If that ever needs revisiting, it will be because
  the premise it rests on has actually expired.
- **This ADR should shrink or disappear if Kruda gains external users.**
  It is sized for a single self-coordinating user. Real adopters would
  need the opposite: deprecation windows, opt-outs that are mandatory
  rather than cheap, and a patch channel safe to install unread. Writing
  those rules now would be guessing at a situation that does not exist.
