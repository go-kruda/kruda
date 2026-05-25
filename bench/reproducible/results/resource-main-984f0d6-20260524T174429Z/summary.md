# Resource Benchmark Summary

Environment: `/home/tiger/kruda-main-bench-20260524T171254Z/bench/reproducible/results/resource-main-984f0d6-20260524T174429Z/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 833707.69 | 0.741 | 384.87 | 396.00 | 15.88 | 15.82 | 216621 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 814287.16 | 0.690 | 384.13 | 395.00 | 17.50 | 17.50 | 211982 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 794918.28 | 0.733 | 386.94 | 397.00 | 22.26 | 20.92 | 205437 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 821877.29 | 0.821 | 386.13 | 395.00 | 20.92 | 20.92 | 212850 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 800915.57 | 0.736 | 387.39 | 400.00 | 21.92 | 21.92 | 206747 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 797616.20 | 0.960 | 386.13 | 398.00 | 24.25 | 23.72 | 206567 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 638388.90 | 2.670 | 404.20 | 418.00 | 12.88 | 12.88 | 157939 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 614925.10 | 2.860 | 408.40 | 433.00 | 13.00 | 13.00 | 150569 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 604076.58 | 3.000 | 414.05 | 437.00 | 18.33 | 17.62 | 145895 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 648306.45 | 3.350 | 410.53 | 444.00 | 18.14 | 18.14 | 157919 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 644459.09 | 3.270 | 408.20 | 424.00 | 18.14 | 18.14 | 157878 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 621569.91 | 3.510 | 414.19 | 426.00 | 22.57 | 21.93 | 150069 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 693306.46 | 2.740 | 381.93 | 430.00 | 7.82 | 7.82 | 181527 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 707049.66 | 3.020 | 409.19 | 440.00 | 7.95 | 7.95 | 172793 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 691357.88 | 2.870 | 396.33 | 448.00 | 7.95 | 7.95 | 174440 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 720928.80 | 3.460 | 397.78 | 431.00 | 10.07 | 10.07 | 181238 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 703111.48 | 2.900 | 381.54 | 432.00 | 10.20 | 10.20 | 184283 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 700305.01 | 3.320 | 395.07 | 427.00 | 10.20 | 10.20 | 177261 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
