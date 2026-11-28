# Linux DEV HLS matched comparison

All arms had one 6 s warm cycle and two scored 6 s cycles. CPU 100% equals one vCPU. RSS is sampled about every 0.2 s. The p99 column includes passing runs only, so the driver's 1,001 ms penalty for missed requests stays in raw results. Resource statistics across all arms describe behavior at the offered load. See same-block comparisons for equal-delivery results.

## 11,776 simulated viewers, 47.104 Gbps offered payload

| Stack | Passed | Bad / attempts | p99 ms, passing median (range) | CPU %, median (range) | RSS MiB, median (range) | Peak sampled RSS MiB, max |
|---|---:|---:|---:|---:|---:|---:|
| wing-256 | 4/5 | 0 / 117,760 | 54.61 (14.50–196.60) | 155.82 (151.90–168.54) | 32.84 (30.22–47.59) | 58.07 |
| fiber | 0/5 | 12,256 / 117,760 | — | 199.73 (199.63–199.88) | 50.78 (49.36–57.14) | 57.28 |
| actix-tuned | 0/5 | 42,814 / 117,760 | — | 199.96 (199.87–200.06) | 350.29 (341.13–352.67) | 362.51 |

### Same-block comparisons on equal completed work

- fiber. 0/5 jointly passing pairs.
- actix-tuned. 0/5 jointly passing pairs.

### Every scored arm

| Block | Stack | Pass | Complete | Bad | p99 ms | CPU % | RSS median / sampled peak MiB | Reuse / offered | Origin hits / expected incl. warm | Dispatch p99 ms | Origin / load CPU % | Max sampled CPU endpoint offset ms | Proxy max steal % |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| high01 | wing-256 | True | 23,552 | 0 | 45.89 | 151.90 | 36.44 / 38.55 | 100.000% | 35,328 / 35,328 | 1.89 | 183.64 / 143.34 | 34.1 | 0.055 |
| high01 | fiber | False | 21,219 | 2,333 | 1001.00 | 199.79 | 57.14 / 57.28 | 90.094% | 31,967 / 35,328 | 1.51 | 226.61 / 164.42 | 32.9 | 0.000 |
| high01 | actix-tuned | False | 15,009 | 8,543 | 1001.00 | 200.06 | 343.76 / 351.73 | 63.727% | 22,687 / 35,328 | 1.67 | 185.86 / 100.28 | 32.7 | 0.054 |
| high02 | fiber | False | 21,133 | 2,419 | 1001.00 | 199.63 | 50.78 / 50.78 | 89.729% | 31,958 / 35,328 | 1.46 | 228.36 / 163.01 | 33.0 | 0.054 |
| high02 | actix-tuned | False | 14,874 | 8,678 | 1001.00 | 199.98 | 341.13 / 346.99 | 63.154% | 22,377 / 35,328 | 1.67 | 191.77 / 99.49 | 31.6 | 0.000 |
| high02 | wing-256 | True | 23,552 | 0 | 196.60 | 168.54 | 31.96 / 58.07 | 99.592% | 35,328 / 35,328 | 1.84 | 218.88 / 137.56 | 35.0 | 0.055 |
| high03 | actix-tuned | False | 15,166 | 8,386 | 1001.00 | 199.96 | 350.29 / 359.68 | 64.394% | 23,064 / 35,328 | 1.67 | 185.68 / 101.72 | 30.6 | 0.054 |
| high03 | wing-256 | True | 23,552 | 0 | 14.50 | 153.98 | 32.84 / 34.82 | 100.000% | 35,328 / 35,328 | 1.88 | 184.15 / 144.51 | 35.6 | 0.055 |
| high03 | fiber | False | 21,139 | 2,413 | 1001.00 | 199.73 | 50.35 / 51.70 | 89.755% | 32,085 / 35,328 | 1.39 | 240.17 / 159.78 | 34.3 | 0.000 |
| high04 | actix-tuned | False | 14,968 | 8,584 | 1001.00 | 199.87 | 352.67 / 360.29 | 63.553% | 22,478 / 35,328 | 1.64 | 190.41 / 98.19 | 30.3 | 0.000 |
| high04 | fiber | False | 21,093 | 2,459 | 1001.00 | 199.63 | 49.36 / 49.36 | 89.559% | 31,436 / 35,328 | 1.37 | 230.77 / 159.60 | 33.3 | 0.054 |
| high04 | wing-256 | False | 23,552 | 0 | 54.56 | 165.86 | 47.59 / 51.53 | 100.000% | 35,323 / 35,328 | 1.82 | 203.42 / 140.93 | 31.5 | 0.055 |
| high05 | fiber | False | 20,920 | 2,632 | 1001.00 | 199.88 | 53.61 / 53.61 | 88.825% | 31,272 / 35,328 | 1.41 | 235.51 / 161.76 | 32.2 | 0.000 |
| high05 | wing-256 | True | 23,552 | 0 | 63.34 | 155.82 | 30.22 / 33.27 | 100.000% | 35,328 / 35,328 | 1.88 | 194.96 / 144.18 | 38.7 | 0.054 |
| high05 | actix-tuned | False | 14,929 | 8,623 | 1001.00 | 199.88 | 351.37 / 362.51 | 63.387% | 22,322 / 35,328 | 1.67 | 189.26 / 100.19 | 31.6 | 0.000 |

### Scored request misses

- wing-256. Zero scored misses.
- fiber. {'client inflight guard': 12256}
- actix-tuned. {'client inflight guard': 42814}

## 6,144 simulated viewers, 24.576 Gbps offered payload

| Stack | Passed | Bad / attempts | p99 ms, passing median (range) | CPU %, median (range) | RSS MiB, median (range) | Peak sampled RSS MiB, max |
|---|---:|---:|---:|---:|---:|---:|
| wing-256 | 5/5 | 0 / 61,440 | 3.42 (3.40–3.56) | 90.86 (90.12–95.77) | 20.66 (20.44–21.68) | 21.76 |
| fiber | 5/5 | 0 / 61,440 | 3.96 (3.88–4.15) | 108.16 (106.47–110.56) | 14.48 (14.40–14.91) | 15.08 |
| actix-tuned | 5/5 | 0 / 61,440 | 3.44 (3.40–3.47) | 90.45 (89.70–91.61) | 10.40 (9.32–11.03) | 12.29 |

### Same-block comparisons on equal completed work

- fiber. 5/5 jointly passing pairs. Wing minus peer, median (range), for p99 was -0.56 (-0.68–-0.32) ms, for CPU was -16.38 (-19.27–-14.79) percentage points, and for RSS was 6.21 (5.57–7.28) MiB. Wing lower in 5/5 p99, 5/5 CPU and 0/5 RSS pairs.
- actix-tuned. 5/5 jointly passing pairs. Wing minus peer, median (range), for p99 was -0.03 (-0.04–0.09) ms, for CPU was 0.00 (-0.82–5.33) percentage points, and for RSS was 10.84 (9.41–11.16) MiB. Wing lower in 3/5 p99, 2/5 CPU and 0/5 RSS pairs.

### Every scored arm

| Block | Stack | Pass | Complete | Bad | p99 ms | CPU % | RSS median / sampled peak MiB | Reuse / offered | Origin hits / expected incl. warm | Dispatch p99 ms | Origin / load CPU % | Max sampled CPU endpoint offset ms | Proxy max steal % |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| lower01 | wing-256 | True | 12,288 | 0 | 3.41 | 90.86 | 20.66 / 20.88 | 100.000% | 18,432 / 18,432 | 1.91 | 98.43 / 76.81 | 42.8 | 0.056 |
| lower01 | fiber | True | 12,288 | 0 | 3.96 | 107.24 | 14.45 / 14.80 | 99.984% | 18,432 / 18,432 | 1.97 | 99.84 / 86.71 | 40.9 | 0.056 |
| lower01 | actix-tuned | True | 12,288 | 0 | 3.44 | 89.70 | 10.40 / 12.29 | 99.984% | 18,432 / 18,432 | 1.89 | 93.36 / 72.49 | 39.1 | 0.055 |
| lower02 | fiber | True | 12,288 | 0 | 3.88 | 109.72 | 14.48 / 14.75 | 99.976% | 18,432 / 18,432 | 1.95 | 103.82 / 87.03 | 42.9 | 0.056 |
| lower02 | actix-tuned | True | 12,288 | 0 | 3.46 | 90.45 | 11.03 / 12.18 | 99.967% | 18,432 / 18,432 | 1.90 | 96.26 / 72.82 | 39.6 | 0.055 |
| lower02 | wing-256 | True | 12,288 | 0 | 3.42 | 90.45 | 20.44 / 21.24 | 100.000% | 18,432 / 18,432 | 1.91 | 95.35 / 78.39 | 39.0 | 0.056 |
| lower03 | actix-tuned | True | 12,288 | 0 | 3.40 | 91.61 | 10.09 / 11.26 | 99.984% | 18,432 / 18,432 | 1.89 | 93.19 / 72.16 | 40.5 | 0.054 |
| lower03 | wing-256 | True | 12,288 | 0 | 3.46 | 91.03 | 21.09 / 21.75 | 100.000% | 18,432 / 18,432 | 1.92 | 97.93 / 76.57 | 43.2 | 0.056 |
| lower03 | fiber | True | 12,288 | 0 | 4.15 | 108.16 | 14.70 / 15.08 | 100.000% | 18,432 / 18,432 | 1.97 | 99.59 / 89.04 | 39.6 | 0.056 |
| lower04 | actix-tuned | True | 12,288 | 0 | 3.47 | 90.44 | 10.84 / 11.56 | 100.000% | 18,432 / 18,432 | 1.90 | 98.42 / 74.48 | 40.1 | 0.055 |
| lower04 | fiber | True | 12,288 | 0 | 3.88 | 110.56 | 14.40 / 14.73 | 100.000% | 18,432 / 18,432 | 1.96 | 100.75 / 87.79 | 40.2 | 0.057 |
| lower04 | wing-256 | True | 12,288 | 0 | 3.56 | 95.77 | 21.68 / 21.76 | 100.000% | 18,432 / 18,432 | 1.90 | 101.51 / 78.81 | 43.0 | 0.056 |
| lower05 | fiber | True | 12,288 | 0 | 3.98 | 106.47 | 14.91 / 14.92 | 100.000% | 18,432 / 18,432 | 1.96 | 102.07 / 85.11 | 41.0 | 0.000 |
| lower05 | wing-256 | True | 12,288 | 0 | 3.40 | 90.12 | 20.48 / 20.75 | 99.984% | 18,432 / 18,432 | 1.92 | 97.43 / 76.73 | 40.6 | 0.000 |
| lower05 | actix-tuned | True | 12,288 | 0 | 3.43 | 90.94 | 9.32 / 10.72 | 100.000% | 18,432 / 18,432 | 1.88 | 94.69 / 74.15 | 39.2 | 0.055 |

### Scored request misses

- wing-256. Zero scored misses.
- fiber. Zero scored misses.
- actix-tuned. Zero scored misses.
