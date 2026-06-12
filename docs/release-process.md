# Releasing Kruda

Kruda follows a **single-tag release model** since v1.2.0. One tag, one release.

> Internal release-runner notes live in the gitignored `docs/RELEASING.md`. This file is the public, contributor-facing version.

## Release gate

Every release must go through the same guardrail:

1. Create a descriptive release or hotfix branch.
2. Open a pull request.
3. Wait for the PR test and benchmark checks to pass.
4. Merge the PR.
5. Wait for `main` to finish green `Tests` and `Benchmark` runs for the merge commit.
6. Tag that green `main` commit only when the change justifies a new version.

The release workflow refuses to publish if the tag is not on `origin/main` or if no successful `Tests` and `Benchmark` workflow runs exist for the tagged commit.

Docs-only, CI-only, and maintainer-process changes can merge without a new tag.
Cut a version when users receive a framework fix, security fix, compatibility
fix, or feature that is worth asking them to upgrade for.

## Pre-release checklist

Before opening the release PR, run `./scripts/pre-release.sh` for local release validation. The script covers local checks; CI provides the required cross-platform and benchmark evidence:

- [ ] Working tree clean (`git status` shows nothing to commit)
- [ ] All tests pass on the host platform: `go test -race -tags kruda_stdjson ./...`
- [ ] Cross-platform builds: `GOOS=linux go build ./...`, `GOOS=windows go build ./...`, `GOOS=darwin go build ./...`
- [ ] Wing tests pass: covered by the same `go test ./...` since v1.2.0 (flattened into core)
- [ ] Native fuzz tests don't crash within 30s each: `go test -fuzz=FuzzRouterPattern -fuzztime=30s -run=^$ .` (and FuzzRouterMatch, FuzzBindJSON, FuzzValidateString)
- [ ] PR benchmark check has no same-runner `benchstat` regression above 10% on the hot path
- [ ] Any `ns/op` movement is reviewed against `B/op`, `allocs/op`, same-runner `main`, and whether the changed code is on the default hot path
- [ ] CHANGELOG.md has a section for the new version with date
- [ ] No `replace` directives left in `cmd/kruda/go.mod` or any `contrib/*/go.mod`
- [ ] Every released submodule's `go.mod` requires the intended core version
- [ ] Public API surface diff reviewed — additions OK; removals require a major bump or an accepted ADR (see docs/decisions/0001-break-api-in-v1-minor.md for the v1.3.0 exception)

## Tagging

```bash
# after the release PR is merged and main is green for Tests + Benchmark
git fetch origin main --tags
git switch main
git pull --ff-only origin main
git tag v1.2.0 -m "Release v1.2.0"
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

If that succeeds, the GitHub release created by the release workflow is ready to publish.

## Hotfix flow

For a v1.2.x patch:

1. `git switch -c hotfix/v1.2.1 origin/main`
2. Apply the fix + tests
3. Update CHANGELOG.md
4. Run `./scripts/pre-release.sh`
5. Open a PR and wait for test + benchmark checks
6. Merge the PR
7. Wait for `main` checks to pass on the merge commit
8. Tag the green `main` commit and push as above

## NEVER do this

- Don't force-push tags. The Go module proxy caches sums permanently — a re-tag with different content = permanent breakage. Use a `retract` directive in `go.mod` and ship a new patch version instead.
- Don't release with a `replace` directive in any committed `go.mod`. The proxy honors local replaces only during `go mod tidy` in the user's project, not when downloading the module — but a stray `replace` in your own `go.mod` is a bug magnet.
- Don't tag from a branch that hasn't been merged to `main`. Tag SHAs that aren't on `main` are confusing for `go get @latest`.
- Don't rerun a failed release workflow if it could publish artifacts before the failed cause is understood.
