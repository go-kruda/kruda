# ADR 0001: Ship the Preset API redesign as a breaking change in a v1 minor release

Date: 2026-06-12
Status: Accepted

## Context

The Preset API redesign renames the per-route tuning surface
(`Feather` → `Preset`, presets become `RouteOption` values, the
`WingFeather`/`WingPlaintext`/`WingJSON`/`WingQuery`/`WingRender` helpers
are removed, and `WingConfig` route-tuning fields are renamed). This is a
breaking change to the public API.

Semantic versioning says breaking changes require a new major version,
and Go modules enforce a `/v2` import-path suffix for v2.0.0+. However,
the project currently has effectively zero external adopters — the
maintainer is the only known user.

A `/v2` import suffix is permanent: every future user types
`github.com/go-kruda/kruda/v2` for the lifetime of that major version.
Moving to v2 today would also force import rewrites across the repo's own
22 examples and 11 sub-modules, repeating the release churn that v1.1.x
suffered.

## Decision

Ship the redesign as **v1.3.0** with a prominent breaking-changes section
in the CHANGELOG and a migration table in the release notes. Do not move
to v2.0.0.

## Consequences

- The v1 line is not strictly semver-compliant across v1.2.x → v1.3.0.
  This is accepted because the blast radius is effectively zero and the
  compatibility promise exists to protect users who do not yet exist.
- Future users keep the clean import path `github.com/go-kruda/kruda`.
- v2.0.0 remains available as a deliberate milestone for a future
  breaking change made *after* real adoption, when the major-version
  signal actually serves someone.
- Anyone pinned to v1.2.x is unaffected until they upgrade explicitly;
  Go's minimal version selection never auto-upgrades them.
