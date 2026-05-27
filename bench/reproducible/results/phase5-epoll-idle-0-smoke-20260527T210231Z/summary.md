# Benchmark Summary

Environment: `bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | plaintext-handler | 1 | 793696.29 | 0.098 | 0.215 | 1.240 | 7.810 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-latency-plaintext-handler-round-1.txt |
| latency | kruda | plaintext-handler | 2 | 790854.40 | 0.101 | 0.226 | 1.270 | 7.130 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-latency-plaintext-handler-round-2.txt |
| latency | kruda | plaintext-handler | 3 | 790034.83 | 0.104 | 0.225 | 1.150 | 8.190 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-latency-plaintext-handler-round-3.txt |
| throughput | kruda | plaintext-handler | 1 | 789926.34 | 0.227 | 0.473 | 1.220 | 6.680 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-throughput-plaintext-handler-round-1.txt |
| throughput | kruda | plaintext-handler | 2 | 797753.37 | 0.186 | 0.437 | 1.380 | 8.440 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-throughput-plaintext-handler-round-2.txt |
| throughput | kruda | plaintext-handler | 3 | 809279.46 | 0.197 | 0.422 | 1.210 | 8.110 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/kruda-throughput-plaintext-handler-round-3.txt |
| latency | fiber | plaintext-handler | 1 | 629621.75 | 0.142 | 0.630 | 2.810 | 16.530 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-latency-plaintext-handler-round-1.txt |
| latency | fiber | plaintext-handler | 2 | 628329.83 | 0.141 | 0.581 | 2.830 | 13.000 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-latency-plaintext-handler-round-2.txt |
| latency | fiber | plaintext-handler | 3 | 631878.00 | 0.141 | 0.632 | 2.790 | 12.360 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-latency-plaintext-handler-round-3.txt |
| throughput | fiber | plaintext-handler | 1 | 638037.53 | 0.265 | 1.110 | 3.490 | 16.270 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-throughput-plaintext-handler-round-1.txt |
| throughput | fiber | plaintext-handler | 2 | 641461.08 | 0.266 | 1.120 | 3.530 | 44.190 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-throughput-plaintext-handler-round-2.txt |
| throughput | fiber | plaintext-handler | 3 | 639119.58 | 0.268 | 1.180 | 3.570 | 17.310 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/fiber-throughput-plaintext-handler-round-3.txt |
| latency | actix | plaintext-handler | 1 | 706711.08 | 0.091 | 0.636 | 2.910 | 19.140 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-latency-plaintext-handler-round-1.txt |
| latency | actix | plaintext-handler | 2 | 712614.07 | 0.092 | 0.711 | 2.950 | 15.060 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-latency-plaintext-handler-round-2.txt |
| latency | actix | plaintext-handler | 3 | 680160.27 | 0.092 | 0.515 | 2.890 | 15.650 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-latency-plaintext-handler-round-3.txt |
| throughput | actix | plaintext-handler | 1 | 719649.79 | 0.164 | 0.729 | 3.100 | 19.220 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-throughput-plaintext-handler-round-1.txt |
| throughput | actix | plaintext-handler | 2 | 726693.43 | 0.172 | 1.250 | 3.440 | 12.630 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-throughput-plaintext-handler-round-2.txt |
| throughput | actix | plaintext-handler | 3 | 709762.08 | 0.166 | 0.930 | 3.220 | 15.550 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-0-smoke-20260527T210231Z/raw/actix-throughput-plaintext-handler-round-3.txt |
