# Kruda Worker Sweep

Environment: `/home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/environment.txt`

| Workers | Route | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---|
| 2 | plaintext-handler | 244406.70 | 0.443 | 0.461 | 0.511 | 2.030 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k2-plaintext-handler.txt |
| 2 | json-static | 244789.66 | 0.492 | 0.510 | 0.541 | 3.100 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k2-json-static.txt |
| 2 | json-serialize | 231421.28 | 0.523 | 0.550 | 0.652 | 3.670 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k2-json-serialize.txt |
| 3 | plaintext-handler | 783139.30 | 0.276 | 0.410 | 0.551 | 6.090 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k3-plaintext-handler.txt |
| 3 | json-static | 762224.38 | 0.281 | 0.410 | 0.617 | 7.600 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k3-json-static.txt |
| 3 | json-serialize | 718792.22 | 0.310 | 0.444 | 0.835 | 5.620 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k3-json-serialize.txt |
| 4 | plaintext-handler | 802088.20 | 0.227 | 0.477 | 1.200 | 11.380 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k4-plaintext-handler.txt |
| 4 | json-static | 821874.10 | 0.203 | 0.441 | 0.910 | 12.980 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k4-json-static.txt |
| 4 | json-serialize | 807711.76 | 0.198 | 0.417 | 0.940 | 12.980 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k4-json-serialize.txt |
| 5 | plaintext-handler | 820561.39 | 0.196 | 0.501 | 2.270 | 13.750 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k5-plaintext-handler.txt |
| 5 | json-static | 822854.99 | 0.191 | 0.497 | 2.150 | 17.620 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k5-json-static.txt |
| 5 | json-serialize | 787711.01 | 0.191 | 0.733 | 3.220 | 16.340 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k5-json-serialize.txt |
| 6 | plaintext-handler | 807055.28 | 0.185 | 0.880 | 3.180 | 13.090 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k6-plaintext-handler.txt |
| 6 | json-static | 798515.09 | 0.184 | 1.060 | 3.450 | 27.100 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k6-json-static.txt |
| 6 | json-serialize | 788145.75 | 0.175 | 1.270 | 3.590 | 16.770 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k6-json-serialize.txt |
| 8 | plaintext-handler | 800031.66 | 0.178 | 1.320 | 3.510 | 15.060 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k8-plaintext-handler.txt |
| 8 | json-static | 803263.09 | 0.190 | 1.260 | 3.660 | 19.710 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k8-json-static.txt |
| 8 | json-serialize | 771410.37 | 0.180 | 1.420 | 3.710 | 22.580 | 0 | 0 | /home/tiger/kruda-phase5-default-profile-144e1f6/bench/reproducible/results/phase5-worker-sweep-raw-20260528T035902Z/raw/k8-json-serialize.txt |
