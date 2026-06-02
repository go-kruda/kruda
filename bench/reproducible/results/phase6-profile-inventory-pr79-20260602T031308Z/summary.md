# Kruda DB Dispatch Sweep

Environment: `results/phase6-profile-inventory-pr79-20260602T031308Z/environment.txt`

| Mode | Route | Profile | Median RPS | Median p99 ms | Max socket errors | Max non-2xx | Run dir |
|---|---|---|---:|---:|---:|---:|---|
| takeover | db | latency | 100842.54 | 7.780 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-takeover` |
| takeover | db | throughput | 98171.23 | 8.070 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-takeover` |
| inline | db | latency | 42880.63 | 4.100 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-inline` |
| inline | db | throughput | 42862.96 | 7.360 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-inline` |
| pool | db | latency | 918.50 | 2.210 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-pool` |
| pool | db | throughput | 3897.65 | 3.220 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-pool` |
| spawn | db | latency | 1301.42 | 2.160 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-spawn` |
| spawn | db | throughput | 2599.58 | 3.680 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/db-spawn` |
| takeover | queries | latency | 97717.34 | 4.230 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-takeover` |
| takeover | queries | throughput | 95789.13 | 6.040 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-takeover` |
| inline | queries | latency | 42253.14 | 4.190 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-inline` |
| inline | queries | throughput | 42076.97 | 7.630 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-inline` |
| pool | queries | latency | 3264.79 | 1.790 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-pool` |
| pool | queries | throughput | 1309.96 | 3.690 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-pool` |
| spawn | queries | latency | 1392.51 | 2.790 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-spawn` |
| spawn | queries | throughput | 1840.24 | 4.080 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/queries-spawn` |
| takeover | fortunes | latency | 83464.75 | 4.340 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-takeover` |
| takeover | fortunes | throughput | 82493.44 | 5.760 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-takeover` |
| inline | fortunes | latency | 40139.25 | 4.630 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-inline` |
| inline | fortunes | throughput | 40438.29 | 8.310 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-inline` |
| pool | fortunes | latency | 1533.69 | 2.570 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-pool` |
| pool | fortunes | throughput | 1890.77 | 4.500 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-pool` |
| spawn | fortunes | latency | 2450.87 | 2.410 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-spawn` |
| spawn | fortunes | throughput | 1771.41 | 4.220 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/fortunes-spawn` |
| takeover | updates | latency | 2816.72 | 283.180 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-takeover` |
| takeover | updates | throughput | 2649.12 | 535.120 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-takeover` |
| inline | updates | latency | 163.63 | 1540.000 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-inline` |
| inline | updates | throughput | 131.16 | 1960.000 | 450 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-inline` |
| pool | updates | latency | 66.22 | 69.500 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-pool` |
| pool | updates | throughput | 190.71 | 143.770 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-pool` |
| spawn | updates | latency | 65.72 | 122.260 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-spawn` |
| spawn | updates | throughput | 175.65 | 206.150 | 0 | 0 | `results/phase6-profile-inventory-pr79-20260602T031308Z/runs/updates-spawn` |
