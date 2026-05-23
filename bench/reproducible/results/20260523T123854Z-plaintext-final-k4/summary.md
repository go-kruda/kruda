# Benchmark Summary

Environment: `results/20260523T123854Z-plaintext-final-k4/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | plaintext-handler | 1 | 835408.64 | 0.104 | 0.205 | 0.516 | 10.220 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-latency-plaintext-handler-round-1.txt |
| latency | kruda | plaintext-handler | 2 | 802676.21 | 0.112 | 0.230 | 0.870 | 10.150 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-latency-plaintext-handler-round-2.txt |
| latency | kruda | plaintext-handler | 3 | 807032.72 | 0.110 | 0.217 | 0.469 | 12.680 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-latency-plaintext-handler-round-3.txt |
| latency | kruda | plaintext-handler | 4 | 800288.61 | 0.113 | 0.233 | 0.832 | 14.800 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-latency-plaintext-handler-round-4.txt |
| latency | kruda | plaintext-handler | 5 | 807259.66 | 0.114 | 0.208 | 0.551 | 12.250 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-latency-plaintext-handler-round-5.txt |
| throughput | kruda | plaintext-handler | 1 | 824043.68 | 0.180 | 0.394 | 0.727 | 8.590 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-throughput-plaintext-handler-round-1.txt |
| throughput | kruda | plaintext-handler | 2 | 807490.41 | 0.200 | 0.404 | 1.010 | 13.310 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-throughput-plaintext-handler-round-2.txt |
| throughput | kruda | plaintext-handler | 3 | 830035.02 | 0.151 | 0.349 | 0.794 | 10.610 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-throughput-plaintext-handler-round-3.txt |
| throughput | kruda | plaintext-handler | 4 | 809719.57 | 0.209 | 0.421 | 0.940 | 10.650 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-throughput-plaintext-handler-round-4.txt |
| throughput | kruda | plaintext-handler | 5 | 812782.42 | 0.217 | 0.434 | 0.734 | 9.530 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/kruda-throughput-plaintext-handler-round-5.txt |
| latency | fiber | plaintext-handler | 1 | 641736.21 | 0.135 | 0.473 | 2.280 | 10.960 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-latency-plaintext-handler-round-1.txt |
| latency | fiber | plaintext-handler | 2 | 638545.65 | 0.137 | 0.540 | 2.500 | 13.040 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-latency-plaintext-handler-round-2.txt |
| latency | fiber | plaintext-handler | 3 | 641549.49 | 0.138 | 0.525 | 2.560 | 11.980 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-latency-plaintext-handler-round-3.txt |
| latency | fiber | plaintext-handler | 4 | 639856.69 | 0.137 | 0.506 | 2.520 | 16.010 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-latency-plaintext-handler-round-4.txt |
| latency | fiber | plaintext-handler | 5 | 639669.06 | 0.139 | 0.554 | 2.660 | 11.010 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-latency-plaintext-handler-round-5.txt |
| throughput | fiber | plaintext-handler | 1 | 657833.86 | 0.260 | 0.980 | 3.220 | 13.340 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-throughput-plaintext-handler-round-1.txt |
| throughput | fiber | plaintext-handler | 2 | 646722.35 | 0.265 | 1.070 | 3.320 | 13.160 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-throughput-plaintext-handler-round-2.txt |
| throughput | fiber | plaintext-handler | 3 | 657243.74 | 0.262 | 1.000 | 3.270 | 13.890 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-throughput-plaintext-handler-round-3.txt |
| throughput | fiber | plaintext-handler | 4 | 650680.91 | 0.261 | 0.980 | 3.220 | 16.580 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-throughput-plaintext-handler-round-4.txt |
| throughput | fiber | plaintext-handler | 5 | 654458.99 | 0.259 | 0.990 | 3.180 | 14.320 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/fiber-throughput-plaintext-handler-round-5.txt |
| latency | actix | plaintext-handler | 1 | 709835.81 | 0.091 | 0.980 | 3.240 | 15.550 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-latency-plaintext-handler-round-1.txt |
| latency | actix | plaintext-handler | 2 | 703483.48 | 0.092 | 0.940 | 3.230 | 15.080 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-latency-plaintext-handler-round-2.txt |
| latency | actix | plaintext-handler | 3 | 711347.07 | 0.091 | 1.030 | 3.170 | 13.950 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-latency-plaintext-handler-round-3.txt |
| latency | actix | plaintext-handler | 4 | 695246.40 | 0.089 | 0.950 | 3.270 | 14.870 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-latency-plaintext-handler-round-4.txt |
| latency | actix | plaintext-handler | 5 | 719051.65 | 0.088 | 1.390 | 3.700 | 16.100 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-latency-plaintext-handler-round-5.txt |
| throughput | actix | plaintext-handler | 1 | 738607.46 | 0.172 | 1.410 | 3.450 | 21.430 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-throughput-plaintext-handler-round-1.txt |
| throughput | actix | plaintext-handler | 2 | 734484.61 | 0.175 | 1.570 | 3.790 | 22.880 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-throughput-plaintext-handler-round-2.txt |
| throughput | actix | plaintext-handler | 3 | 734378.34 | 0.167 | 1.380 | 3.450 | 13.830 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-throughput-plaintext-handler-round-3.txt |
| throughput | actix | plaintext-handler | 4 | 725454.40 | 0.164 | 1.110 | 3.450 | 22.300 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-throughput-plaintext-handler-round-4.txt |
| throughput | actix | plaintext-handler | 5 | 732961.87 | 0.169 | 1.510 | 3.590 | 11.680 | 0 | 0 | results/20260523T123854Z-plaintext-final-k4/raw/actix-throughput-plaintext-handler-round-5.txt |
