# SendStaticJSON Wing-First Candidate Evidence

Date: 2026-06-02

Scope: local microbenchmark evidence for a narrow Wing handler-path JSON static
response candidate. This is not tiger route evidence, not cross-runtime Actix
evidence, and not a public benchmark claim.

## Candidate

`Ctx.SendStaticJSON` now checks Wing's `JSONResponder` fast path before the
fasthttp static body fast path.

Rationale:

- On Wing, `c.writer` is a `JSONResponder`.
- On Wing, checking the fasthttp embedded context first is a guaranteed miss.
- The same `canBypassHeaderWrite(true)` guard remains in the Wing fast path, so
  custom headers, cookies, CORS, secure headers, and other application behavior
  still fall back to the generic response path.
- The fasthttp and generic fallbacks remain unchanged after the Wing check.

## Rejected Pre-Candidate

Changing the `JSON` Feather preset from `Bolt` to
`Feather{Dispatch: Inline, ResponseMode: responseJSON}` was rejected.

`responseJSON` is currently not read by the runtime JSON fast path; `c.JSON`,
`SendStaticJSON`, and `SendStaticWithTypeBytes` use `JSONResponder`/`jsonFast`
instead. Keeping a response-mode-only preset change would look like a real
optimization while being functionally inert for the measured Wing JSON paths.

## Command

```bash
/usr/bin/env \
  PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local \
  GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase7 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase7 \
  go test -run '^$' -tags kruda_stdjson \
  -bench 'Benchmark(CPUResponseJSON|CPUHandlerJSONStaticFeather|CPUHandlerJSONSerializeFeather)$' \
  -benchmem -count=10
```

Baseline worktree: `origin/main` at `f60ea8917b43536990b679aae78881b59b2ba9f8`.

Candidate branch: `perf/send-static-json-wing-first`.

Environment:

- Host: local Apple M3
- OS/arch: darwin/arm64
- Toolchain: local Go toolchain
- Build tags: `kruda_stdjson`

## Benchstat

```text
name                              main sec/op       candidate sec/op       delta
CPUResponseJSON-8                 42.38n +/- 6%     45.97n +/- 11%        +8.50% (p=0.001 n=10)
CPUHandlerJSONStaticFeather-8     221.3n +/- 22%    211.0n +/- 3%         -4.70% (p=0.000 n=10)
CPUHandlerJSONSerializeFeather-8  309.1n +/- 8%     285.6n +/- 2%         -7.62% (p=0.000 n=10)
geomean                           142.6n            140.4n                -1.52%

name                              main B/op         candidate B/op         delta
CPUResponseJSON-8                 160.0 +/- 0%      160.0 +/- 0%          no change
CPUHandlerJSONStaticFeather-8     160.0 +/- 0%      160.0 +/- 0%          no change
CPUHandlerJSONSerializeFeather-8  192.0 +/- 0%      192.0 +/- 0%          no change

name                              main alloc/op     candidate alloc/op     delta
CPUResponseJSON-8                 1.000 +/- 0%      1.000 +/- 0%          no change
CPUHandlerJSONStaticFeather-8     1.000 +/- 0%      1.000 +/- 0%          no change
CPUHandlerJSONSerializeFeather-8  2.000 +/- 0%      2.000 +/- 0%          no change
```

Interpretation:

- The only code path changed by the candidate is `SendStaticJSON`.
- The relevant static JSON handler benchmark improved by 4.70% with no
  allocation increase.
- `BenchmarkCPUResponseJSON` does not call `SendStaticJSON`; its regression is
  treated as unrelated local benchmark noise and is not used as supporting
  evidence for this candidate.
- `BenchmarkCPUHandlerJSONSerializeFeather` uses `c.JSON`, not
  `SendStaticJSON`; its improvement is also not used as supporting evidence.

## Verification

```bash
/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase7 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase7 \
  go test -count=1 -tags kruda_stdjson ./...

/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase7 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase7 \
  go test -race -count=1 -tags kruda_stdjson ./...
```

Both commands passed locally.

## Decision

Keep the `SendStaticJSON` Wing-first ordering as a narrow normal handler-path
static JSON fast-path improvement. Do not use this as an Actix claim. Tiger
route evidence is still required before making any public throughput claim.
