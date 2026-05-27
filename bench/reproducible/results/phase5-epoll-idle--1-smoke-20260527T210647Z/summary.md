# Benchmark Summary

Environment: `bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | plaintext-handler | 1 | 790447.16 | 0.096 | 0.220 | 1.380 | 8.910 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-latency-plaintext-handler-round-1.txt |
| latency | kruda | plaintext-handler | 2 | 770320.01 | 0.096 | 0.209 | 1.410 | 9.820 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-latency-plaintext-handler-round-2.txt |
| latency | kruda | plaintext-handler | 3 | 793046.63 | 0.098 | 0.218 | 1.170 | 7.560 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-latency-plaintext-handler-round-3.txt |
| throughput | kruda | plaintext-handler | 1 | 794591.89 | 0.197 | 0.444 | 1.540 | 8.250 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-throughput-plaintext-handler-round-1.txt |
| throughput | kruda | plaintext-handler | 2 | 809075.08 | 0.206 | 0.422 | 1.200 | 10.770 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-throughput-plaintext-handler-round-2.txt |
| throughput | kruda | plaintext-handler | 3 | 786996.00 | 0.200 | 0.447 | 1.500 | 7.570 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/kruda-throughput-plaintext-handler-round-3.txt |
| latency | fiber | plaintext-handler | 1 | 629603.24 | 0.143 | 0.618 | 2.850 | 13.970 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-latency-plaintext-handler-round-1.txt |
| latency | fiber | plaintext-handler | 2 | 621824.79 | 0.144 | 0.652 | 2.840 | 20.270 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-latency-plaintext-handler-round-2.txt |
| latency | fiber | plaintext-handler | 3 | 620120.57 | 0.144 | 0.654 | 2.880 | 13.210 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-latency-plaintext-handler-round-3.txt |
| throughput | fiber | plaintext-handler | 1 | 646855.24 | 0.264 | 1.040 | 3.340 | 12.420 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-throughput-plaintext-handler-round-1.txt |
| throughput | fiber | plaintext-handler | 2 | 639394.58 | 0.265 | 1.110 | 3.450 | 14.130 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-throughput-plaintext-handler-round-2.txt |
| throughput | fiber | plaintext-handler | 3 | 644348.80 | 0.266 | 1.070 | 3.410 | 13.190 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/fiber-throughput-plaintext-handler-round-3.txt |
| latency | actix | plaintext-handler | 1 | 704985.40 | 0.092 | 0.711 | 2.970 | 15.650 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-latency-plaintext-handler-round-1.txt |
| latency | actix | plaintext-handler | 2 | 692412.68 | 0.093 | 0.566 | 2.720 | 11.130 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-latency-plaintext-handler-round-2.txt |
| latency | actix | plaintext-handler | 3 | 695643.85 | 0.093 | 0.529 | 2.870 | 14.390 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-latency-plaintext-handler-round-3.txt |
| throughput | actix | plaintext-handler | 1 | 716382.21 | 0.172 | 0.920 | 3.160 | 18.190 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-throughput-plaintext-handler-round-1.txt |
| throughput | actix | plaintext-handler | 2 | 721466.60 | 0.167 | 0.980 | 3.270 | 14.710 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-throughput-plaintext-handler-round-2.txt |
| throughput | actix | plaintext-handler | 3 | 723701.19 | 0.167 | 0.850 | 3.100 | 10.450 | 0 | 0 | bench/reproducible/results/phase5-epoll-idle--1-smoke-20260527T210647Z/raw/actix-throughput-plaintext-handler-round-3.txt |
