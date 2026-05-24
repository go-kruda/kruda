# Benchmark Summary

Environment: `results/20260524Tphase3-json-final/environment.txt`

| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| latency | kruda | json-static | 1 | 816182.67 | 0.105 | 0.214 | 1.060 | 18.710 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-static-round-1.txt |
| latency | kruda | json-static | 2 | 810368.07 | 0.106 | 0.231 | 0.505 | 10.150 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-static-round-2.txt |
| latency | kruda | json-static | 3 | 807664.24 | 0.105 | 0.215 | 0.920 | 12.030 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-static-round-3.txt |
| latency | kruda | json-static | 4 | 802750.45 | 0.110 | 0.217 | 0.526 | 10.140 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-static-round-4.txt |
| latency | kruda | json-static | 5 | 766197.40 | 0.131 | 0.246 | 0.860 | 14.650 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-static-round-5.txt |
| latency | kruda | json-serialize | 1 | 784098.80 | 0.120 | 0.245 | 0.812 | 17.600 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-serialize-round-1.txt |
| latency | kruda | json-serialize | 2 | 793534.67 | 0.110 | 0.236 | 0.695 | 7.860 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-serialize-round-2.txt |
| latency | kruda | json-serialize | 3 | 780020.19 | 0.114 | 0.259 | 1.120 | 21.240 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-serialize-round-3.txt |
| latency | kruda | json-serialize | 4 | 793358.63 | 0.106 | 0.236 | 0.821 | 11.150 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-serialize-round-4.txt |
| latency | kruda | json-serialize | 5 | 790803.39 | 0.114 | 0.229 | 0.960 | 10.380 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-latency-json-serialize-round-5.txt |
| throughput | kruda | json-static | 1 | 787133.72 | 0.232 | 0.465 | 1.150 | 11.020 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-static-round-1.txt |
| throughput | kruda | json-static | 2 | 801623.90 | 0.218 | 0.477 | 0.800 | 9.720 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-static-round-2.txt |
| throughput | kruda | json-static | 3 | 817079.53 | 0.178 | 0.406 | 1.000 | 12.500 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-static-round-3.txt |
| throughput | kruda | json-static | 4 | 815121.58 | 0.207 | 0.403 | 0.719 | 13.330 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-static-round-4.txt |
| throughput | kruda | json-static | 5 | 811740.53 | 0.155 | 0.426 | 1.090 | 11.020 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-static-round-5.txt |
| throughput | kruda | json-serialize | 1 | 791778.92 | 0.225 | 0.481 | 1.030 | 9.790 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-serialize-round-1.txt |
| throughput | kruda | json-serialize | 2 | 803072.40 | 0.220 | 0.457 | 0.836 | 7.540 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-serialize-round-2.txt |
| throughput | kruda | json-serialize | 3 | 782438.26 | 0.213 | 0.463 | 1.110 | 13.620 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-serialize-round-3.txt |
| throughput | kruda | json-serialize | 4 | 796074.44 | 0.208 | 0.442 | 0.950 | 7.520 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-serialize-round-4.txt |
| throughput | kruda | json-serialize | 5 | 791812.23 | 0.215 | 0.484 | 1.080 | 9.580 | 0 | 0 | results/20260524Tphase3-json-final/raw/kruda-throughput-json-serialize-round-5.txt |
| latency | fiber | json-static | 1 | 640533.94 | 0.139 | 0.501 | 2.570 | 16.440 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-static-round-1.txt |
| latency | fiber | json-static | 2 | 641954.93 | 0.138 | 0.509 | 2.580 | 12.150 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-static-round-2.txt |
| latency | fiber | json-static | 3 | 632775.59 | 0.141 | 0.561 | 2.620 | 12.020 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-static-round-3.txt |
| latency | fiber | json-static | 4 | 639788.06 | 0.139 | 0.536 | 2.680 | 12.780 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-static-round-4.txt |
| latency | fiber | json-static | 5 | 630367.82 | 0.141 | 0.639 | 2.770 | 11.830 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-static-round-5.txt |
| latency | fiber | json-serialize | 1 | 612847.06 | 0.144 | 0.677 | 2.850 | 12.010 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-serialize-round-1.txt |
| latency | fiber | json-serialize | 2 | 604493.46 | 0.147 | 0.778 | 3.020 | 12.380 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-serialize-round-2.txt |
| latency | fiber | json-serialize | 3 | 598711.13 | 0.147 | 0.797 | 3.130 | 14.300 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-serialize-round-3.txt |
| latency | fiber | json-serialize | 4 | 607745.74 | 0.145 | 0.736 | 3.060 | 14.710 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-serialize-round-4.txt |
| latency | fiber | json-serialize | 5 | 601885.59 | 0.147 | 0.769 | 3.110 | 15.970 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-latency-json-serialize-round-5.txt |
| throughput | fiber | json-static | 1 | 641124.37 | 0.268 | 1.060 | 3.400 | 12.260 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-static-round-1.txt |
| throughput | fiber | json-static | 2 | 645609.26 | 0.264 | 1.000 | 3.340 | 15.210 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-static-round-2.txt |
| throughput | fiber | json-static | 3 | 636589.52 | 0.268 | 1.090 | 3.330 | 12.190 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-static-round-3.txt |
| throughput | fiber | json-static | 4 | 642816.49 | 0.266 | 1.080 | 3.490 | 14.820 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-static-round-4.txt |
| throughput | fiber | json-static | 5 | 637246.92 | 0.269 | 1.130 | 3.510 | 16.480 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-static-round-5.txt |
| throughput | fiber | json-serialize | 1 | 618003.22 | 0.275 | 1.290 | 3.900 | 19.750 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-serialize-round-1.txt |
| throughput | fiber | json-serialize | 2 | 624155.14 | 0.275 | 1.240 | 3.800 | 17.980 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-serialize-round-2.txt |
| throughput | fiber | json-serialize | 3 | 615130.70 | 0.277 | 1.310 | 3.740 | 16.340 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-serialize-round-3.txt |
| throughput | fiber | json-serialize | 4 | 627086.05 | 0.274 | 1.190 | 3.650 | 17.350 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-serialize-round-4.txt |
| throughput | fiber | json-serialize | 5 | 624986.72 | 0.272 | 1.200 | 3.710 | 15.330 | 0 | 0 | results/20260524Tphase3-json-final/raw/fiber-throughput-json-serialize-round-5.txt |
| latency | actix | json-static | 1 | 680199.11 | 0.094 | 0.431 | 2.550 | 15.930 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-static-round-1.txt |
| latency | actix | json-static | 2 | 706626.78 | 0.093 | 0.792 | 3.060 | 13.170 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-static-round-2.txt |
| latency | actix | json-static | 3 | 690469.34 | 0.093 | 0.517 | 2.840 | 13.250 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-static-round-3.txt |
| latency | actix | json-static | 4 | 687231.50 | 0.092 | 0.470 | 2.810 | 14.540 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-static-round-4.txt |
| latency | actix | json-static | 5 | 693124.92 | 0.093 | 0.507 | 2.780 | 13.470 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-static-round-5.txt |
| latency | actix | json-serialize | 1 | 691881.12 | 0.099 | 0.633 | 2.930 | 14.200 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-serialize-round-1.txt |
| latency | actix | json-serialize | 2 | 677736.35 | 0.097 | 0.288 | 2.310 | 13.580 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-serialize-round-2.txt |
| latency | actix | json-serialize | 3 | 691178.32 | 0.097 | 0.695 | 2.860 | 15.080 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-serialize-round-3.txt |
| latency | actix | json-serialize | 4 | 684823.12 | 0.098 | 0.300 | 2.360 | 15.070 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-serialize-round-4.txt |
| latency | actix | json-serialize | 5 | 689458.20 | 0.094 | 0.960 | 3.310 | 15.260 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-latency-json-serialize-round-5.txt |
| throughput | actix | json-static | 1 | 710112.35 | 0.170 | 0.900 | 3.260 | 17.800 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-static-round-1.txt |
| throughput | actix | json-static | 2 | 712263.97 | 0.172 | 0.970 | 3.210 | 13.360 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-static-round-2.txt |
| throughput | actix | json-static | 3 | 712204.09 | 0.168 | 0.870 | 3.220 | 12.570 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-static-round-3.txt |
| throughput | actix | json-static | 4 | 719638.86 | 0.169 | 1.100 | 3.340 | 12.170 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-static-round-4.txt |
| throughput | actix | json-static | 5 | 714434.92 | 0.170 | 0.960 | 3.270 | 26.420 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-static-round-5.txt |
| throughput | actix | json-serialize | 1 | 701322.95 | 0.166 | 0.556 | 2.670 | 24.800 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-serialize-round-1.txt |
| throughput | actix | json-serialize | 2 | 706101.18 | 0.172 | 0.880 | 3.220 | 18.550 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-serialize-round-2.txt |
| throughput | actix | json-serialize | 3 | 706973.54 | 0.174 | 0.920 | 3.150 | 14.280 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-serialize-round-3.txt |
| throughput | actix | json-serialize | 4 | 704128.66 | 0.173 | 1.250 | 3.630 | 24.600 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-serialize-round-4.txt |
| throughput | actix | json-serialize | 5 | 714633.73 | 0.173 | 1.710 | 4.140 | 19.140 | 0 | 0 | results/20260524Tphase3-json-final/raw/actix-throughput-json-serialize-round-5.txt |
