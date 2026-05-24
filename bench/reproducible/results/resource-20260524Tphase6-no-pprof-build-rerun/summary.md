# Resource Benchmark Summary

Environment: `/home/tiger/kruda-wing-rss-phase6/bench/reproducible/results/resource-20260524Tphase6-no-pprof-build-rerun/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 834242.28 | 1.030 | 385.00 | 397.00 | 16.88 | 16.64 | 216686 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 816610.11 | 0.930 | 384.60 | 396.00 | 19.00 | 18.70 | 212327 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 812548.68 | 0.950 | 383.55 | 397.00 | 20.85 | 20.45 | 211849 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 835187.06 | 1.030 | 385.07 | 397.00 | 21.07 | 20.93 | 216892 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 825018.49 | 1.070 | 384.13 | 396.00 | 23.57 | 23.57 | 214776 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 804931.49 | 1.080 | 384.87 | 397.00 | 23.50 | 23.22 | 209144 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 635994.45 | 2.930 | 409.53 | 444.00 | 11.75 | 11.70 | 155299 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 632721.32 | 2.900 | 403.46 | 422.00 | 12.12 | 12.12 | 156824 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 609879.53 | 3.030 | 408.05 | 440.00 | 17.21 | 16.75 | 149462 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 655729.24 | 3.510 | 408.13 | 447.00 | 16.98 | 16.98 | 160667 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 644125.32 | 3.480 | 409.72 | 433.00 | 16.98 | 16.98 | 157211 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 636513.73 | 3.640 | 411.27 | 429.00 | 21.93 | 21.29 | 154768 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 715684.03 | 2.900 | 392.73 | 441.00 | 8.20 | 8.20 | 182233 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 705639.58 | 2.760 | 383.53 | 443.00 | 8.20 | 8.20 | 183985 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 704716.38 | 2.780 | 396.73 | 443.00 | 8.20 | 8.20 | 177631 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 752886.69 | 3.270 | 405.51 | 447.00 | 10.45 | 10.45 | 185664 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 747466.82 | 3.250 | 393.67 | 436.00 | 10.45 | 10.45 | 189871 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 737094.27 | 3.960 | 418.27 | 435.00 | 10.45 | 10.45 | 176225 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
