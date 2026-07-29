# Reproducing the JSON-engine claims — 2026-07-29

The CHANGELOG's figures for the JSON engine came from a harness that lived only
on a machine that no longer exists, so nothing in the repo could reproduce them.
All four are re-measured here, and every one holds. Two were stated
conservatively.

| environment | |
|---|---|
| Linux | 8-core 13th Gen Intel i5-13500, Go 1.25.11, linux/amd64 |
| macOS | M-series, Go 1.26.1, darwin/arm64 (contrast only — see the caveat) |

Benchmarks live in `json/engine_bench_test.go`; the cold-start harness in
`bench/reproducible/coldstart/`. Both are in-repo now, so these numbers stay
checkable.

## Marshal path, 100-item payload, Sonic on linux/amd64

`MarshalToBufferViaMarshal` reproduces the implementation `MarshalToBuffer` had
before it streamed — marshal into a fresh `[]byte`, then copy — because that,
not `Marshal` alone, is what the claim compares against.

Medians of 12 runs at 20,000 iterations each, linux/amd64.

Which baseline you pick changes the answer, so both are measured. `Legacy` is the
code that actually shipped — `sonic.Marshal` on the package **default** config,
then copy — while `ViaMarshal` routes through this package's `Marshal`, which
today means the frozen config with `SortMapKeys` and `ValidateString` on. The
old code never paid for those options, so only `Legacy` answers "what did
upgrading change for a caller".

| 100-item payload | ns/op | B/op |
|---|---|---|
| Legacy — what shipped before | 3,754 | 9,798 |
| ViaMarshal — config held equal | 3,840 | 9,818 |
| **streaming — today** | **2,485** | **113** |

Against `Legacy`: **34% faster, 87× fewer bytes.** Against `ViaMarshal`, isolating
streaming alone: 35%. The two baselines land within 2% of each other here, because
`ValidateString` is cheap on a struct of short ASCII strings — but that is a
property of this payload, not a general one, and the published figure uses
`Legacy`.

| single item | ns/op | B/op |
|---|---|---|
| Legacy | 104.4 | 127 |
| ViaMarshal | 115.8 | 127 |
| streaming — today | 129.8 | 112 |

Against `Legacy`, a single-item payload costs **25 ns more** for 15 fewer bytes.
An earlier pass published 14 ns by measuring against `ViaMarshal`; that
understated what a caller upgrading actually sees, and the CHANGELOG now carries
the 25 ns figure.

The CHANGELOG originally said "~19% faster" and "6967 → 178 B/op". The speed
claim was understated. The byte figures differ from the originals only because
the payload shape differs — neither the old harness's struct nor its field
widths were recorded, so the handoff's "~13 ns slower" for small payloads is not
comparable to the 25 ns measured here.

## Decode, Sonic vs encoding/json, linux/amd64

| | ns/op |
|---|---|
| encoding/json | 78,265 |
| sonic | **12,686** |
| | **6.2× faster** |

The CHANGELOG said "roughly 4–5×". Also understated.

## Cold start and startup RSS, linux/amd64

Median of 15 spawns, timed from `exec` to the first 200, RSS read from
`/proc/<pid>/status` at that instant.

| engine | routes | cold start | RSS at first response | RSS after 500 ms |
|---|---|---|---|---|
| encoding/json | 1 | 3.02 ms | 12.81 MB | 13.02 MB |
| sonic | 1 | 5.98 ms | 18.88 MB | 18.97 MB |
| encoding/json | 25 | 3.25 ms | 12.54 MB | 12.81 MB |
| sonic | 25 | 6.25 ms | 19.41 MB | 19.46 MB |

The CHANGELOG said 6.5 → 13.5 ms and 12.1 → 17.6 MB on a 4-core container. The
absolute milliseconds are lower here on faster bare metal, but the ratio holds
(claimed 2.08×, measured 1.92×) and the RSS figures land within about 2 MB of
the originals.

These numbers were confirmed a second time from a **fresh clone on the same
host**, with no local edits — sonic at 25 routes landed on 6.25 ms both times,
`encoding/json` on 3.25 and 3.13 ms. The harness carries a relative replace to
the core module, so it builds from any checkout.

**The "25 typed POST routes" framing was incidental.** Going from 1 route to 25
moves cold start by ~0.25 ms in both engines. Sonic's warm-up is a fixed cost
paid at process start, not something typed-handler registration scales. The
CHANGELOG now says so.

## Caveat: these are amd64 numbers

The same marshal benchmark on darwin/arm64 inverts:

| darwin/arm64, 100-item payload | ns/op |
|---|---|
| sonic | 17,970 |
| encoding/json | 10,940 |

Sonic's build constraints do cover arm64, and its **decode** is 4.3× faster there
— but **encode is slower than `encoding/json`**. Nothing in the CHANGELOG is
wrong because of this; the claims are about `CGO_ENABLED=0` Linux builds, which
is also where Kruda runs in production. It is worth knowing before anyone quotes
the encode figures as cross-platform, and worth a separate look.
