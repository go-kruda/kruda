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

Medians of 12 runs at 20,000 iterations each. An early 6-run sample put the
speed gain at 27%; the larger sample settles it at 30%, which is why the
published figure comes from the larger one.

| 100-item payload | ns/op | B/op |
|---|---|---|
| old: Marshal + copy | 3,593 | 9,792 |
| new: streaming | **2,506** | **113** |
| | **30% faster** | **87× less** |

| single item | ns/op | B/op |
|---|---|---|
| old: Marshal + copy | 115.5 | 127 |
| new: streaming | 129.8 | 112 |
| | **14 ns slower** | 15 B less |

The CHANGELOG said "~19% faster" and "6967 → 178 B/op". The speed claim was
understated; the byte figures differ only because the payload shape differs —
neither the old harness's struct nor its field widths were recorded. The
small-payload trade the handoff described as "~13 ns slower" measures at 14.3 ns
here, which is close corroboration from an independent harness.

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
