# KRUDA-29 typed-binding evidence

Date: 2026-09-20  
Kruda commit: `67c6727c9ed8e0c4967191fd01b7fe3fb1aac63a`  
Competitors: Fiber v3.5.0, Actix Web v4.15.0  
Toolchains: Go 1.27.1, rustc 1.98.1  
Host scope: one controlled native Linux x86-64 host; benchmark services and 43 unrelated containers stopped during measurement and restored afterward.

## Workload

The route binds 30 query values into generated typed input, runs compiled validation, and returns an exact 30-field JSON response. Every server also had to return the same structured 422 response for an invalid value before load was accepted.

The comparison covers this generated binding and validation path only. It is not a universal framework ranking.

## Capacity

Capacity used `wrk -t4 -c256`, a 2-second warmup, and an 8-second closed-loop measurement. Each profile ran all six server permutations once, giving six samples per server and 36 measured samples total. Every server occupied each position twice and every pair order was balanced 3/3.

| Profile | Server | Median RPS | Median p99 |
|---|---|---:|---:|
| 4 | Kruda | 527,676.58 | 1.480 ms |
| 4 | Fiber | 259,618.50 | 3.195 ms |
| 4 | Actix | 521,487.36 | 0.759 ms |
| 8 | Kruda | 577,700.53 | 1.240 ms |
| 8 | Fiber | 389,797.64 | 3.405 ms |
| 8 | Actix | 677,955.01 | 0.643 ms |

Interpretation:

- Profile 4: Kruda's median is 103.3% above Fiber. Its median is 1.2% above Actix, but the Actix comparison is fragile: Kruda won four of six paired rounds and the within-run spread is larger than the median gap. Treat Kruda and Actix as the same ballpark here.
- Profile 8: Kruda's median is 48.2% above Fiber. Actix leads Kruda by 17.4% when the lead is expressed relative to Kruda; equivalently, Kruda trails Actix by 14.8% relative to Actix.
- Capacity p99 is saturation evidence and is not used as a common-load latency claim.

## Matched-rate corrected latency

The offered rate for each profile was frozen before latency measurement as 50% of the slowest fresh capacity median, rounded down to 1,000 requests/second: 129,000 RPS for profile 4 and 194,000 RPS for profile 8.

The pinned wrk2 client recorded corrected HDR latency and per-thread post-calibration completion windows. Samples were accepted only when the post-calibration achieved rate was 99-101% of target, all error counters were zero, every thread calibrated once, and histogram/window counts reconciled within the 256-connection boundary.

The initial four rounds per profile were extended with the two missing permutations. The combined evidence therefore contains all six permutations at each profile: six samples per server and 36 measured samples total.

| Profile / offered rate | Server | Median p50 | Median p90 | Median p99 |
|---|---|---:|---:|---:|
| 4 / 129k | Kruda | 0.955 ms | 1.600 ms | 1.955 ms |
| 4 / 129k | Fiber | 1.150 ms | 2.125 ms | 3.070 ms |
| 4 / 129k | Actix | 0.900 ms | 1.480 ms | 1.915 ms |
| 8 / 194k | Kruda | 1.110 ms | 1.860 ms | 2.405 ms |
| 8 / 194k | Fiber | 1.255 ms | 2.315 ms | 3.355 ms |
| 8 / 194k | Actix | 1.030 ms | 1.710 ms | 2.220 ms |

Interpretation using wrk2's approximately 1 ms reporting granularity:

- Profile 4: Kruda's corrected-p99 median is 1.115 ms below Fiber and lower in all six paired rounds. Paired gaps are 0.99-1.19 ms. This is a resolved Kruda win, with the smallest round at the granularity boundary.
- Profile 8: Kruda's corrected-p99 median is 0.95 ms below Fiber and lower in all six paired rounds. The direction is consistent, but the 0.89-1.01 ms paired gaps remain at the tool boundary, so this result is directional rather than resolved.
- Kruda and Actix remain unresolved at both matched rates. Actix is lower in all six rounds, but the median gaps are only 0.040 ms at profile 4 and 0.185 ms at profile 8.

## Gates and provenance

- Capacity: 36/36 samples complete, 142,214,065 measured requests, zero connect/read/write/timeout/non-2xx errors.
- Matched rate: 36/36 samples complete, achieved-rate fraction 0.99996-1.00007, zero connect/read/write/timeout/non-2xx errors.
- Original matched-rate evidence remained frozen while the 12 balance-extension samples were added.
- All originally healthy containers returned healthy; all four paused service units returned active. The pre-existing unhealthy Plane container remained outside the restoration health requirement.
- Kruda binary SHA-256: `f0536c268d343ae79e10cdd0ae5995458d5c609b530aaeea39c24c08884b479b`.
- Fiber binary SHA-256: `e1212e3aedffdaf1f8b8c14a1df705e15918478f1784535244a9b227754d5101`.
- Actix binary SHA-256: `6c81a0c70ebfa78e00a618803d23635b94456ef1a894027ae0642f84550e2815`.
- Patched wrk2 binary SHA-256: `9c25f03760d80ae11439badd5d57eb2d985c2e7329df4490b4a1e06a8102bc5e`.
- Capacity manifest SHA-256: `2e69c67543a13d263348ad9dece2598a5ce3fa3ad9b70682c0131fa5a4f850cd`.
- Matched-rate manifest SHA-256: `09352f4b7874a0e2aef67ec6c9e96a75bb853a18265659e0ecf0f7204be97da9`.
- Balance-extension manifest SHA-256: `2fc7a5a3963cdcbffac6c0a01924531b4bde13cd5c6fc235565726e36823cc93`.

Committed evidence files:

- [`capacity-summary.json`](2026-09-20-kruda29-typed-binding/capacity-summary.json)
- [`matched-rate-summary.json`](2026-09-20-kruda29-typed-binding/matched-rate-summary.json)
- [`CAPACITY-PROTOCOL.md`](2026-09-20-kruda29-typed-binding/CAPACITY-PROTOCOL.md)
- [`MATCHED-RATE-PROTOCOL.md`](2026-09-20-kruda29-typed-binding/MATCHED-RATE-PROTOCOL.md)
- [`MATCHED-RATE-EXTENSION.md`](2026-09-20-kruda29-typed-binding/MATCHED-RATE-EXTENSION.md)

## Limits

The evidence is from one host, one 30-field workload, two worker profiles, six rounds per profile, and pinned framework/toolchain versions. Closed-loop capacity and matched-rate latency answer different questions and must not be combined into a universal "fastest framework" claim.
