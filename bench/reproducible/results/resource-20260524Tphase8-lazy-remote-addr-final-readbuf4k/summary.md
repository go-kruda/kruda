# Resource Benchmark Summary

Environment: `/home/tiger/kruda-lazy-remote-addr-phase8/bench/reproducible/results/resource-20260524Tphase8-lazy-remote-addr-final-readbuf4k/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 840174.94 | 0.496 | 386.53 | 398.00 | 15.75 | 15.71 | 217363 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 814729.38 | 0.698 | 386.27 | 398.00 | 16.62 | 16.59 | 210922 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 777569.04 | 0.668 | 386.20 | 399.00 | 19.36 | 18.71 | 201338 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 840234.65 | 0.890 | 386.14 | 398.00 | 19.53 | 19.53 | 217598 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 828715.84 | 0.741 | 386.26 | 396.00 | 19.78 | 19.78 | 214549 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 799022.77 | 0.970 | 382.48 | 397.00 | 20.96 | 20.62 | 208906 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 636017.81 | 2.680 | 409.80 | 431.00 | 11.88 | 11.78 | 155202 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 633112.21 | 2.730 | 404.46 | 416.00 | 12.25 | 12.25 | 156533 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 613584.05 | 2.890 | 408.80 | 424.00 | 16.99 | 16.63 | 150094 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 645619.82 | 3.530 | 412.87 | 438.00 | 17.11 | 17.11 | 156374 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 642131.19 | 3.570 | 410.91 | 441.00 | 17.11 | 17.11 | 156271 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 632952.92 | 3.730 | 414.45 | 430.00 | 22.03 | 21.27 | 152721 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 696402.54 | 2.500 | 378.73 | 449.00 | 8.07 | 8.07 | 183878 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 716514.74 | 2.880 | 397.11 | 441.58 | 8.20 | 8.20 | 180432 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 717171.51 | 3.050 | 414.00 | 442.00 | 8.20 | 8.20 | 173230 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 748302.49 | 3.300 | 408.28 | 445.00 | 10.32 | 10.32 | 183282 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 737413.14 | 3.240 | 393.00 | 445.00 | 10.70 | 10.61 | 187637 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 734178.97 | 3.120 | 394.93 | 441.00 | 10.70 | 10.70 | 185901 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
