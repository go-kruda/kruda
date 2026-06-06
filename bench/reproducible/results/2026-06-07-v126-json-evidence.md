# v1.2.6 JSON Serialization Recheck

Date: 2026-06-07
Host: local Mac
Commit under test: `095d6df6be28c196f250257ac0a705ddd7ef01b8`
Scope: local `kruda_stdjson` microbenchmark sanity recheck only. This is not a
cross-runtime throughput claim and not a release trigger.

## Command

```bash
/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local \
  GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase6 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase6 \
  go test -run '^$' -tags kruda_stdjson \
    -bench 'BenchmarkCPU(ResponseJSON|HandlerJSON(Static|StaticBytes|Serialize)Feather)$' \
    -benchmem -count=10 .
```

## Environment

- OS/arch: `darwin/arm64`
- Kernel: `Darwin 25.5.0`
- CPU reported by `go test`: `Apple M3`
- Online processors: `8`
- Go command: `go version go1.26.1 darwin/arm64`
- Module `go` directive: `1.25.10`
- Build tags: `kruda_stdjson`
- Network path: none; in-process Go microbenchmark
- Coordinated omission: not applicable; no request generator or network client

## Median Results

| Benchmark | Median ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkCPUResponseJSON` | 38.48 | 160 | 1 |
| `BenchmarkCPUHandlerJSONStaticFeather` | 201.25 | 160 | 1 |
| `BenchmarkCPUHandlerJSONStaticBytesFeather` | 201.95 | 160 | 1 |
| `BenchmarkCPUHandlerJSONSerializeFeather` | 266.05 | 160 | 1 |

## Decision

No runtime change.

The current `main` JSON handler-path benchmark remains healthy after the
previous stdjson stream response work. The targeted serialization benchmark is
at 160 B/op and 1 alloc/op, matching the intended allocation profile from
`2026-06-02-wing-json-stream-response-evidence.md`, and the local median
ns/op is below the earlier kept candidate median. This does not create a new
optimization target.

Keep the JSON track closed unless a new same-runner baseline-versus-candidate
proof shows an actionable regression or a narrow improvement that clears the
roadmap gate without adding bytes or allocations on the static JSON paths.
