# Reproducing the JSON-engine claims — 2026-07-29

The CHANGELOG's figures for the JSON engine came from a harness that lived only
on a machine that no longer exists, so nothing in the repo could reproduce them.
All four are re-measured here, and every one holds. Two were stated
conservatively.

| environment | |
|---|---|
| Linux | 8-core 13th Gen Intel i5-13500, Go 1.25.11, linux/amd64 |
| macOS | M-series, Go 1.26.1, darwin/arm64 (contrast only — see the caveat) |

Benchmarks live in `json/engine_bench_test.go` and `json/engine_bench_legacy_test.go`; the cold-start harness in
`bench/reproducible/coldstart/`. Both are in-repo now, so these numbers stay
checkable.

## Marshal path, 100-item payload, Sonic on linux/amd64

`MarshalToBufferViaMarshal` reproduces the implementation `MarshalToBuffer` had
before it streamed — marshal into a fresh `[]byte`, then copy — because that,
not `Marshal` alone, is what the claim compares against.

Medians of 12 runs at 20,000 iterations each, linux/amd64. Raw output for both
engines is in `json-bench-sonic.txt` and `json-bench-stdjson.txt`; every figure
quoted below appears there.

Which baseline you pick changes the answer, so both are measured.
`BenchmarkMarshalToBufferLegacy` (in `json/engine_bench_legacy_test.go`) is the
code that actually shipped — `sonic.Marshal` on the package **default** config,
then copy. `ViaMarshal` (in `json/engine_bench_test.go`) routes through this
package's `Marshal`, which today means the frozen config with `SortMapKeys` and
`ValidateString` on. The old code never paid for those options, so only `Legacy`
answers "what did upgrading change for a caller".

Two independent 12-run samples were taken, because the first pass published a
figure that moved when re-measured. Both are shown rather than the flattering
one. Only sample B's raw output is kept here — sample A's was overwritten before
it was archived, so its numbers are reported as observed but are not
independently checkable from this directory. Sample B is.

| 100-item payload | sample A | sample B |
|---|---|---|
| Legacy — what shipped before | 3,754 ns | 3,879 ns |
| ViaMarshal — config held equal | 3,840 ns | 3,530 ns |
| **streaming — today** | **2,485 ns** | **2,475 ns** |
| gain against Legacy | 33.8% | 36.2% |

Bytes are stable across samples: 9,798 / 9,816 B/op against **113 B/op**, so
about 87× fewer. The published figure is "about 35% faster", which is the honest
resolution of a 33.8–36.2% spread.

| single item | sample A | sample B |
|---|---|---|
| Legacy | 104.4 ns | 103.4 ns |
| streaming — today | 129.8 ns | 131.3 ns |
| cost against Legacy | +25.4 ns | +27.9 ns |

Published as "about 26 ns more" for 127 → 112 B/op. An earlier pass published
14 ns by measuring against `ViaMarshal`; that understated what a caller
upgrading actually sees, because the baseline carried config costs the old code
never had.

The CHANGELOG originally said "~19% faster" and "6967 → 178 B/op". The speed
claim was understated on either baseline. The byte figures differ from the
originals only because the payload shape differs — neither the old harness's
struct nor its field widths were recorded, so its "~13 ns slower" for small
payloads is not comparable to the ~26 ns measured here.

## Decode, Sonic vs encoding/json, linux/amd64

| | ns/op |
|---|---|
| encoding/json | 78,133 |
| sonic | **13,015** |
| | **6.0× faster** |

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
