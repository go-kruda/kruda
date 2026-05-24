# Resource Benchmark Summary

Environment: `/home/tiger/kruda-resource-phase4/bench/reproducible/results/resource-20260524Tphase4-current/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 805459.36 | 3.580 | 417.13 | 442.00 | 17.50 | 17.41 | 193096 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 794672.61 | 3.450 | 413.27 | 446.00 | 19.38 | 19.20 | 192289 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 770009.42 | 3.720 | 430.04 | 453.00 | 24.47 | 23.19 | 179055 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 813896.06 | 3.620 | 419.24 | 446.00 | 23.33 | 23.33 | 194136 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 808414.35 | 3.530 | 418.67 | 448.00 | 25.33 | 25.33 | 193091 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 796820.54 | 4.110 | 419.18 | 449.00 | 26.71 | 26.05 | 190090 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 656057.04 | 2.730 | 403.40 | 417.00 | 12.62 | 12.62 | 162632 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 637220.70 | 2.850 | 411.99 | 434.00 | 12.88 | 12.88 | 154669 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 618372.21 | 2.940 | 415.00 | 445.00 | 17.88 | 17.56 | 149005 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 669162.26 | 3.320 | 405.19 | 418.00 | 18.00 | 18.00 | 165148 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 655574.20 | 3.210 | 412.53 | 434.00 | 18.00 | 18.00 | 158916 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 638775.03 | 3.510 | 412.07 | 430.00 | 22.89 | 21.85 | 155016 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 717406.41 | 2.920 | 392.15 | 442.00 | 8.07 | 8.07 | 182942 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 702854.09 | 2.720 | 378.27 | 427.00 | 8.07 | 8.07 | 185808 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 708835.20 | 2.920 | 397.35 | 434.00 | 8.07 | 8.07 | 178391 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 734983.54 | 3.110 | 381.03 | 435.00 | 10.45 | 10.45 | 192894 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 736223.04 | 3.160 | 387.35 | 438.00 | 10.57 | 10.57 | 190067 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 735724.38 | 3.180 | 401.80 | 451.00 | 10.57 | 10.57 | 183107 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
