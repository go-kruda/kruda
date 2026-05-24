# Resource Benchmark Summary

Environment: `/home/tiger/kruda-rss-footprint-phase7/bench/reproducible/results/resource-20260524Tphase7-readbuf4k/environment.txt`

CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed.

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| latency | kruda | plaintext-handler | 854526.31 | 0.351 | 388.80 | 399.00 | 15.75 | 15.63 | 219786 | 0 | 0 | raw/kruda-latency-plaintext-handler-wrk.txt | raw/kruda-latency-plaintext-handler-pidstat.txt |
| latency | kruda | json-static | 847808.53 | 0.363 | 386.61 | 396.00 | 16.62 | 16.62 | 219293 | 0 | 0 | raw/kruda-latency-json-static-wrk.txt | raw/kruda-latency-json-static-pidstat.txt |
| latency | kruda | json-serialize | 818137.86 | 0.593 | 386.47 | 399.00 | 19.88 | 19.06 | 211695 | 0 | 0 | raw/kruda-latency-json-serialize-wrk.txt | raw/kruda-latency-json-serialize-pidstat.txt |
| throughput | kruda | plaintext-handler | 848169.94 | 0.632 | 387.87 | 396.00 | 18.80 | 18.71 | 218674 | 0 | 0 | raw/kruda-throughput-plaintext-handler-wrk.txt | raw/kruda-throughput-plaintext-handler-pidstat.txt |
| throughput | kruda | json-static | 841818.81 | 0.652 | 388.00 | 397.00 | 19.42 | 19.42 | 216964 | 0 | 0 | raw/kruda-throughput-json-static-wrk.txt | raw/kruda-throughput-json-static-pidstat.txt |
| throughput | kruda | json-serialize | 796066.33 | 0.736 | 385.40 | 398.00 | 20.95 | 20.67 | 206556 | 0 | 0 | raw/kruda-throughput-json-serialize-wrk.txt | raw/kruda-throughput-json-serialize-pidstat.txt |
| latency | fiber | plaintext-handler | 649055.40 | 2.910 | 409.65 | 448.00 | 11.75 | 11.75 | 158441 | 0 | 0 | raw/fiber-latency-plaintext-handler-wrk.txt | raw/fiber-latency-plaintext-handler-pidstat.txt |
| latency | fiber | json-static | 648553.59 | 2.590 | 405.93 | 420.00 | 11.88 | 11.88 | 159770 | 0 | 0 | raw/fiber-latency-json-static-wrk.txt | raw/fiber-latency-json-static-pidstat.txt |
| latency | fiber | json-serialize | 624436.89 | 2.690 | 413.06 | 439.00 | 17.45 | 16.77 | 151173 | 0 | 0 | raw/fiber-latency-json-serialize-wrk.txt | raw/fiber-latency-json-serialize-pidstat.txt |
| throughput | fiber | plaintext-handler | 667040.96 | 3.150 | 409.92 | 424.00 | 17.30 | 17.30 | 162725 | 0 | 0 | raw/fiber-throughput-plaintext-handler-wrk.txt | raw/fiber-throughput-plaintext-handler-pidstat.txt |
| throughput | fiber | json-static | 664412.76 | 2.940 | 408.66 | 422.00 | 17.30 | 17.30 | 162583 | 0 | 0 | raw/fiber-throughput-json-static-wrk.txt | raw/fiber-throughput-json-static-pidstat.txt |
| throughput | fiber | json-serialize | 644553.15 | 3.120 | 415.80 | 429.00 | 22.12 | 21.37 | 155015 | 0 | 0 | raw/fiber-throughput-json-serialize-wrk.txt | raw/fiber-throughput-json-serialize-pidstat.txt |
| latency | actix | plaintext-handler | 730716.75 | 2.870 | 397.13 | 442.00 | 7.82 | 7.82 | 183999 | 0 | 0 | raw/actix-latency-plaintext-handler-wrk.txt | raw/actix-latency-plaintext-handler-pidstat.txt |
| latency | actix | json-static | 722331.87 | 2.790 | 392.33 | 449.00 | 7.95 | 7.89 | 184113 | 0 | 0 | raw/actix-latency-json-static-wrk.txt | raw/actix-latency-json-static-pidstat.txt |
| latency | actix | json-serialize | 713235.81 | 2.630 | 384.56 | 422.00 | 8.07 | 8.07 | 185468 | 0 | 0 | raw/actix-latency-json-serialize-wrk.txt | raw/actix-latency-json-serialize-pidstat.txt |
| throughput | actix | plaintext-handler | 743301.11 | 3.150 | 384.28 | 431.00 | 10.20 | 10.20 | 193427 | 0 | 0 | raw/actix-throughput-plaintext-handler-wrk.txt | raw/actix-throughput-plaintext-handler-pidstat.txt |
| throughput | actix | json-static | 747995.33 | 3.220 | 397.13 | 445.00 | 10.32 | 10.32 | 188350 | 0 | 0 | raw/actix-throughput-json-static-wrk.txt | raw/actix-throughput-json-static-pidstat.txt |
| throughput | actix | json-serialize | 727273.77 | 2.970 | 387.78 | 436.00 | 10.32 | 10.32 | 187548 | 0 | 0 | raw/actix-throughput-json-serialize-wrk.txt | raw/actix-throughput-json-serialize-pidstat.txt |
