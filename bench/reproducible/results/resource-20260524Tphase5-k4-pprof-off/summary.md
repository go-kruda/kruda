# Resource Benchmark Summary

Environment: `/home/tiger/kruda-p99-rss-phase5/bench/reproducible/results/resource-20260524Tphase5-k4-pprof-off/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 794939.68 | 0.831 | 385.87 | 399.00 | 17.12 | 16.60 | 206012 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 818009.71 | 0.831 | 386.34 | 397.00 | 18.75 | 18.75 | 211733 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 806017.13 | 0.664 | 388.07 | 400.00 | 21.10 | 20.75 | 207699 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 833728.62 | 0.771 | 386.87 | 396.00 | 21.69 | 21.69 | 215506 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 836681.14 | 0.831 | 386.27 | 396.00 | 23.19 | 23.19 | 216605 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 798468.64 | 1.250 | 381.94 | 397.00 | 24.53 | 23.86 | 209056 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 650531.87 | 2.370 | 401.80 | 416.00 | 12.88 | 12.88 | 161904 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 642300.15 | 2.690 | 403.20 | 419.00 | 13.00 | 13.00 | 159301 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 615703.25 | 2.940 | 411.00 | 433.00 | 17.76 | 17.37 | 149806 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 654783.25 | 3.340 | 405.53 | 426.00 | 18.46 | 18.46 | 161464 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 651430.19 | 3.610 | 404.13 | 428.00 | 18.59 | 18.47 | 161193 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 630787.36 | 4.020 | 411.60 | 431.00 | 22.41 | 21.81 | 153253 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 697337.21 | 2.750 | 375.90 | 439.00 | 8.07 | 8.07 | 185511 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 709351.46 | 3.060 | 391.73 | 442.00 | 8.20 | 8.18 | 181082 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 699779.93 | 2.740 | 386.87 | 438.00 | 8.20 | 8.20 | 180882 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 744944.17 | 3.650 | 401.27 | 440.00 | 10.32 | 10.32 | 185647 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 736451.44 | 3.100 | 386.54 | 432.00 | 10.57 | 10.57 | 190524 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 738918.95 | 3.580 | 404.93 | 437.00 | 10.57 | 10.57 | 182481 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
