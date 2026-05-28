# Syscall Profile Summary

Perf rows are the comparable wrk pass. Strace rows are intrusive syscall-count diagnostics; use their syscall counts, not their RPS or latency, for performance conclusions. A strace status of 130 is expected because the harness stops strace with SIGINT after the measured wrk run.

| Framework | Route | Kind | RPS | p99 | Socket errors | Non-2xx | read/recv | write/send | epoll wait | epoll_ctl | futex | cycles | instructions | context switches |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| actix | json-serialize | perf | 723721.51 | 3.17ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 792249 |
| actix | json-serialize | strace | 88519.72 | 4.14ms | 0 | 0 | 894546 | 894583 | 29300 | 509 |  |  |  |  |
| actix | json-static | perf | 739090.25 | 3.55ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 663028 |
| actix | json-static | strace | 89117.87 | 3.97ms | 0 | 0 | 892224 | 892259 | 28079 | 507 |  |  |  |  |
| actix | plaintext-handler | perf | 730025.29 | 3.27ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 905794 |
| actix | plaintext-handler | strace | 89463.41 | 4.01ms | 0 | 0 | 895616 | 895630 | 28171 | 514 |  |  |  |  |
| kruda | json-serialize | perf | 813073.61 | 1.05ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 190609 |
| kruda | json-serialize | strace | 68066.23 | 1.55s | 8 | 0 | 686172 | 682532 | 473 | 1262 | 15519 |  |  |  |
| kruda | json-static | perf | 837252.19 | 1.10ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 553729 |
| kruda | json-static | strace | 68551.56 | 1.51s | 22 | 0 | 689762 | 686638 | 444 | 1276 | 14141 |  |  |  |
| kruda | plaintext-handler | perf | 851279.50 | 0.87ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 623462 |
| kruda | plaintext-handler | strace | 69129.27 | 1.41s | 58 | 0 | 695908 | 692489 | 700 | 1285 | 15209 |  |  |  |
