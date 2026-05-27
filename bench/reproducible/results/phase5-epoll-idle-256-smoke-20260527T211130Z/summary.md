# Benchmark Summary

Environment: `bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | plaintext-handler | 1 | 815311.02 | 0.112 | 0.215 | 1.000 | 16.080 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-latency-plaintext-handler-round-1.txt |
| latency | kruda | plaintext-handler | 2 | 813545.14 | 0.113 | 0.201 | 0.980 | 10.660 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-latency-plaintext-handler-round-2.txt |
| latency | kruda | plaintext-handler | 3 | 830388.45 | 0.101 | 0.192 | 0.712 | 12.110 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-latency-plaintext-handler-round-3.txt |
| throughput | kruda | plaintext-handler | 1 | 778118.64 | 0.165 | 0.335 | 1.050 | 18.830 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-throughput-plaintext-handler-round-1.txt |
| throughput | kruda | plaintext-handler | 2 | 829346.32 | 0.191 | 0.406 | 0.739 | 12.700 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-throughput-plaintext-handler-round-2.txt |
| throughput | kruda | plaintext-handler | 3 | 810179.70 | 0.230 | 0.436 | 0.841 | 11.650 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/kruda-throughput-plaintext-handler-round-3.txt |
| latency | fiber | plaintext-handler | 1 | 636681.13 | 0.141 | 0.593 | 2.780 | 14.640 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-latency-plaintext-handler-round-1.txt |
| latency | fiber | plaintext-handler | 2 | 631918.37 | 0.141 | 0.635 | 2.760 | 11.990 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-latency-plaintext-handler-round-2.txt |
| latency | fiber | plaintext-handler | 3 | 630886.99 | 0.139 | 0.527 | 2.530 | 12.100 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-latency-plaintext-handler-round-3.txt |
| throughput | fiber | plaintext-handler | 1 | 652372.28 | 0.257 | 0.890 | 2.990 | 12.900 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-throughput-plaintext-handler-round-1.txt |
| throughput | fiber | plaintext-handler | 2 | 648069.20 | 0.259 | 0.940 | 3.160 | 15.700 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-throughput-plaintext-handler-round-2.txt |
| throughput | fiber | plaintext-handler | 3 | 650259.25 | 0.263 | 0.990 | 3.330 | 15.280 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/fiber-throughput-plaintext-handler-round-3.txt |
| latency | actix | plaintext-handler | 1 | 705580.76 | 0.089 | 0.950 | 3.230 | 15.660 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-latency-plaintext-handler-round-1.txt |
| latency | actix | plaintext-handler | 2 | 717933.70 | 0.090 | 0.822 | 3.150 | 16.430 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-latency-plaintext-handler-round-2.txt |
| latency | actix | plaintext-handler | 3 | 706160.35 | 0.091 | 0.623 | 2.940 | 10.880 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-latency-plaintext-handler-round-3.txt |
| throughput | actix | plaintext-handler | 1 | 717212.58 | 0.160 | 0.638 | 3.220 | 16.020 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-throughput-plaintext-handler-round-1.txt |
| throughput | actix | plaintext-handler | 2 | 749627.68 | 0.172 | 1.450 | 3.510 | 10.740 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-throughput-plaintext-handler-round-2.txt |
| throughput | actix | plaintext-handler | 3 | 733006.35 | 0.186 | 1.150 | 3.370 | 20.530 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle-256-smoke-20260527T211130Z/raw/actix-throughput-plaintext-handler-round-3.txt |
