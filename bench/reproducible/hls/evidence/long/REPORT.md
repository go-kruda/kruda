# Linux DEV 60-second HLS proxy comparison

11,776 simulated viewers at 4 Mbps each, 2 pinned vCPUs per proxy, HTTP/1.1 loopback with the same RAM origin and load driver. Five interleaved blocks of three arms, each with 6 seconds of warm-up and 60 scored seconds. Outcomes follow scheduled phase, even if the response completes across a boundary. Process CPU 100% equals 1 vCPU.

| Stack | Full / measured passes | Warm bad/attempts | Measured bad/attempts | p99 ms across passing measured phases | CPU %, median (range) | RSS median MiB, median (range) | Maximum sampled RSS MiB |
|---|---:|---:|---:|---:|---:|---:|---:|
| wing-256 | 4/5 / 5/5 | 69/58,880 | 0/588,800 | 139.30 (45.20–224.60) | 163.16 (154.01–170.50) | 44.92 (34.65–50.53) | 138.42 |
| fiber | 0/5 / 0/5 | 4,912/58,880 | 63,478/588,800 | — | 199.80 (199.77–199.82) | 54.73 (51.04–57.20) | 57.27 |
| actix-tuned | 0/5 / 0/5 | 17,953/58,880 | 211,288/588,800 | — | 199.97 (199.97–200.00) | 348.54 (344.98–351.00) | 361.74 |

Resource summaries describe the same offered load. Delivered volume can differ. For substantive p99 and CPU comparisons use same-block joint full passes only. Per-arm p99 for any failed measurement can include the driver's cap-miss penalty.

## Joint full passes

- fiber had no jointly passing blocks with Wing.
- actix-tuned had no jointly passing blocks with Wing.

## Measured rounds

| Block | Stack | Full / measured pass | Warm dispatched bad / unsent | Measured dispatched bad / unsent | p99 ms | Proxy CPU % | Median / sampled peak RSS MiB | Measured reuse | Origin hits incl. warm | Dispatch p99 ms | Origin / driver CPU % | Max CPU endpoint offset ms | Proxy max steal % |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| block01 | wing-256 | True / True | 0 / 0 | 0 / 0 | 159.05 | 163.16 | 39.47 / 83.68 | 99.853% | 129,536/129,536 | 1.86 | 195.74 / 141.93 | 56.0 | 0.076 |
| block01 | fiber | False / False | 0 / 1,074 | 0 / 12,700 | 1001.00 | 199.82 | 51.04 / 56.71 | 89.215% | 115,762/129,536 | 1.39 | 235.23 / 162.62 | 31.0 | 0.015 |
| block01 | actix-tuned | False / False | 0 / 3,057 | 0 / 42,049 | 1001.00 | 199.97 | 351.00 / 361.74 | 64.293% | 84,430/129,536 | 1.67 | 191.14 / 101.86 | 31.7 | 0.015 |
| block02 | fiber | False / False | 0 / 1,093 | 0 / 12,480 | 1001.00 | 199.77 | 51.73 / 51.86 | 89.402% | 115,963/129,536 | 1.37 | 232.76 / 165.12 | 29.8 | 0.030 |
| block02 | actix-tuned | False / False | 0 / 4,308 | 0 / 42,184 | 1001.00 | 199.99 | 347.12 / 353.20 | 64.178% | 83,044/129,536 | 1.68 | 192.42 / 101.24 | 27.6 | 0.015 |
| block02 | wing-256 | True / True | 0 / 0 | 0 / 0 | 224.60 | 170.50 | 49.72 / 133.21 | 99.889% | 129,536/129,536 | 1.84 | 213.96 / 146.35 | 41.5 | 0.046 |
| block03 | actix-tuned | False / False | 0 / 4,339 | 0 / 42,365 | 1001.00 | 199.97 | 348.54 / 358.45 | 64.024% | 82,832/129,536 | 1.67 | 191.63 / 101.19 | 27.1 | 0.015 |
| block03 | wing-256 | True / True | 0 / 0 | 0 / 0 | 70.06 | 159.58 | 44.92 / 89.00 | 100.000% | 129,536/129,536 | 1.86 | 198.10 / 142.92 | 53.0 | 0.046 |
| block03 | fiber | False / False | 0 / 881 | 0 / 12,355 | 1001.00 | 199.80 | 57.17 / 57.24 | 89.508% | 116,300/129,536 | 1.41 | 233.90 / 164.21 | 32.1 | 0.030 |
| block04 | actix-tuned | False / False | 0 / 2,081 | 0 / 42,227 | 1001.00 | 200.00 | 344.98 / 352.83 | 64.141% | 85,228/129,536 | 1.68 | 191.89 / 101.46 | 30.0 | 0.000 |
| block04 | fiber | False / False | 0 / 943 | 0 / 13,261 | 1001.00 | 199.80 | 54.73 / 55.64 | 88.739% | 115,332/129,536 | 1.42 | 234.26 / 162.57 | 30.3 | 0.015 |
| block04 | wing-256 | False / True | 0 / 69 | 0 / 0 | 139.30 | 163.88 | 50.53 / 138.42 | 100.000% | 129,467/129,536 | 1.86 | 201.29 / 144.50 | 50.2 | 0.046 |
| block05 | fiber | False / False | 0 / 921 | 0 / 12,682 | 1001.00 | 199.77 | 57.20 / 57.27 | 89.231% | 115,933/129,536 | 1.40 | 237.85 / 163.49 | 33.0 | 0.015 |
| block05 | wing-256 | True / True | 0 / 0 | 0 / 0 | 45.20 | 154.01 | 34.65 / 73.65 | 100.000% | 129,536/129,536 | 1.88 | 192.43 / 144.31 | 63.0 | 0.046 |
| block05 | actix-tuned | False / False | 0 / 4,168 | 0 / 42,463 | 1001.00 | 199.97 | 350.46 / 361.07 | 63.941% | 82,905/129,536 | 1.68 | 190.56 / 101.62 | 30.3 | 0.015 |

## Error categories by scheduled phase

- wing-256 warm {'client inflight guard': 69}, measured none.
- fiber warm {'client inflight guard': 4912}, measured {'client inflight guard': 63478}.
- actix-tuned warm {'client inflight guard': 17953}, measured {'client inflight guard': 211288}.

See PROTOCOL.md and BUILD.md for the build identities, scope and run settings. The five loopback blocks describe the tested configuration; production capacity requires a separate measurement.
