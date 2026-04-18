# Releasing Kruda

Kruda follows a **single-tag release model** since v1.2.0. One tag, one release.

> Internal release-runner notes live in the gitignored `docs/RELEASING.md`. This file is the public, contributor-facing version.

## Pre-release checklist

Before tagging, run `./scripts/pre-release.sh` — it executes every check this list describes. The script is the source of truth; this list is the prose explanation:

- [ ] Working tree clean (`git status` shows nothing to commit)
- [ ] All tests pass on the host platform: `go test -race ./...`
- [ ] Cross-platform builds: `GOOS=linux go build ./...`, `GOOS=windows go build ./...`, `GOOS=darwin go build ./...`
- [ ] Wing tests pass: covered by the same `go test ./...` since v1.2.0 (flattened into core)
- [ ] Native fuzz tests don't crash within 30s each: `go test -fuzz=FuzzRouterPattern -fuzztime=30s -run=^$ .` (and FuzzRouterMatch, FuzzBindJSON, FuzzValidateString)
- [ ] Benchmarks within 10% of previous release on hot path
- [ ] CHANGELOG.md has a section for the new version with date
- [ ] No `replace` directives left in `transport/wing/go.mod` or any `contrib/*/go.mod`
- [ ] Every contrib package's `go.mod` requires the new core version (not the previous one)
- [ ] Public API surface diff reviewed — additions OK, removals require a major bump

## Tagging

```bash
# from main, with all checks green
git tag v1.2.0 -m "Maturity release: A+ across all dimensions"
git push origin v1.2.0
```

## Verification

After pushing the tag, wait ~30s for the proxy, then verify the new version is fetchable in a fresh module:

```bash
mkdir /tmp/kruda-verify && cd /tmp/kruda-verify
go mod init verify
go get github.com/go-kruda/kruda@v1.2.0
go build ./...
```

If that succeeds, create the GitHub release with notes pulled from CHANGELOG.md.

## Hotfix flow

For a v1.2.x patch:

1. `git checkout -b hotfix/v1.2.1 v1.2.0`
2. Apply the fix + tests
3. Update CHANGELOG.md
4. Run `./scripts/pre-release.sh`
5. Tag `v1.2.1` and push as above

## NEVER do this

- Don't force-push tags. The Go module proxy caches sums permanently — a re-tag with different content = permanent breakage. Use a `retract` directive in `go.mod` and ship a new patch version instead.
- Don't release with a `replace` directive in any committed `go.mod`. The proxy honors local replaces only during `go mod tidy` in the user's project, not when downloading the module — but a stray `replace` in your own `go.mod` is a bug magnet.
- Don't tag from a branch that hasn't been merged to `main`. Tag SHAs that aren't on `main` are confusing for `go get @latest`.
