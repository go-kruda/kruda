# Benchmark Summary

Environment: `bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | plaintext-handler | 1 | 845498.66 | 0.100 | 0.182 | 0.589 | 12.390 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-latency-plaintext-handler-round-1.txt |
| latency | kruda | plaintext-handler | 2 | 833482.85 | 0.103 | 0.162 | 1.120 | 9.300 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-latency-plaintext-handler-round-2.txt |
| latency | kruda | plaintext-handler | 3 | 830144.17 | 0.109 | 0.194 | 0.567 | 8.520 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-latency-plaintext-handler-round-3.txt |
| throughput | kruda | plaintext-handler | 1 | 806429.69 | 0.220 | 0.473 | 1.650 | 13.030 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-throughput-plaintext-handler-round-1.txt |
| throughput | kruda | plaintext-handler | 2 | 805757.05 | 0.230 | 0.451 | 0.960 | 9.160 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-throughput-plaintext-handler-round-2.txt |
| throughput | kruda | plaintext-handler | 3 | 813098.89 | 0.230 | 0.431 | 0.880 | 9.930 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/kruda-throughput-plaintext-handler-round-3.txt |
| latency | fiber | plaintext-handler | 1 | 628963.80 | 0.142 | 0.633 | 2.780 | 12.160 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-latency-plaintext-handler-round-1.txt |
| latency | fiber | plaintext-handler | 2 | 629111.91 | 0.142 | 0.626 | 2.810 | 12.560 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-latency-plaintext-handler-round-2.txt |
| latency | fiber | plaintext-handler | 3 | 623307.13 | 0.143 | 0.695 | 2.950 | 21.600 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-latency-plaintext-handler-round-3.txt |
| throughput | fiber | plaintext-handler | 1 | 643966.38 | 0.265 | 1.080 | 3.430 | 21.690 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-throughput-plaintext-handler-round-1.txt |
| throughput | fiber | plaintext-handler | 2 | 644925.78 | 0.262 | 1.050 | 3.340 | 15.140 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-throughput-plaintext-handler-round-2.txt |
| throughput | fiber | plaintext-handler | 3 | 653151.26 | 0.261 | 0.940 | 3.200 | 12.370 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/fiber-throughput-plaintext-handler-round-3.txt |
| latency | actix | plaintext-handler | 1 | 711126.68 | 0.090 | 0.801 | 3.120 | 20.160 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-latency-plaintext-handler-round-1.txt |
| latency | actix | plaintext-handler | 2 | 711791.62 | 0.088 | 0.835 | 3.170 | 12.160 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-latency-plaintext-handler-round-2.txt |
| latency | actix | plaintext-handler | 3 | 706453.34 | 0.090 | 0.516 | 2.900 | 13.160 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-latency-plaintext-handler-round-3.txt |
| throughput | actix | plaintext-handler | 1 | 746479.87 | 0.172 | 1.450 | 3.550 | 12.620 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-throughput-plaintext-handler-round-1.txt |
| throughput | actix | plaintext-handler | 2 | 745947.17 | 0.166 | 1.540 | 3.680 | 13.350 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-throughput-plaintext-handler-round-2.txt |
| throughput | actix | plaintext-handler | 3 | 732955.14 | 0.167 | 1.070 | 3.320 | 12.000 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-default-smoke-20260527T205813Z/raw/actix-throughput-plaintext-handler-round-3.txt |
