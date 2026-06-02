# Sonic Buffer Encoder Candidate Rejection

Date: 2026-06-02
Host: local Apple M3
Branch: `perf/sonic-buffer-encoder`

## Scope

This records a rejected local microbenchmark candidate for Kruda's default
Sonic JSON stream path. It is not tiger route evidence, not cross-runtime
benchmark evidence, and not a public performance claim.

The candidate changed `json/sonic.go` `MarshalToBuffer` from:

- `sonic.Marshal(v)` followed by `buf.Write(data)`

to:

- `sonic.ConfigDefault.NewEncoder(buf).Encode(v)` followed by trimming the
  trailing newline to preserve existing output semantics.

The runtime change was reverted before commit.

## Command

Baseline and candidate were both measured with:

```bash
/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local \
  GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase7 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase7 \
  go test -run '^$' \
    -bench 'Benchmark(JSON|CPUResponseJSON|CPUHandlerJSONStaticFeather|CPUHandlerJSONSerializeFeather)$' \
    -benchmem -count=5
```

Environment:

- `goos=darwin`
- `goarch=arm64`
- `cpu=Apple M3`
- `go version`: local toolchain reported `go1.26.1`
- Build tags: default, so Kruda compiled the Sonic JSON path

## Median Summary

| Benchmark | Baseline median | Candidate median | Decision |
|---|---:|---:|---|
| `BenchmarkJSON` | 420.5 ns/op, ~496 B/op, 5 allocs/op | 428.9 ns/op, ~532 B/op, 6 allocs/op | Reject |
| `BenchmarkCPUResponseJSON` | 41.83 ns/op, 160 B/op, 1 alloc/op | 41.58 ns/op, 160 B/op, 1 alloc/op | Neutral |
| `BenchmarkCPUHandlerJSONStaticFeather` | 212.6 ns/op, 160 B/op, 1 alloc/op | 210.1 ns/op, 160 B/op, 1 alloc/op | Neutral |
| `BenchmarkCPUHandlerJSONSerializeFeather` | 378.3 ns/op, ~218 B/op, 3 allocs/op | 383.5 ns/op, ~219 B/op, 3 allocs/op | Reject |

## Decision

Reject the Sonic stream-encoder candidate.

It regressed the generic Kruda JSON microbenchmark by adding one allocation and
did not improve the Wing JSON serialization handler path. This candidate should
not be repeated unless a new Sonic API or Go runtime version changes the
allocation behavior and fresh before/after microbenchmarks show a clear win.

This result does not change public CPU-bound Actix comparison wording, because
the default reproducible harness uses `KRUDA_GO_TAGS=kruda_stdjson` for portable
cross-runtime evidence.
