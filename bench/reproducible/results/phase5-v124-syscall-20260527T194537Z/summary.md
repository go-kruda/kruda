# Syscall Profile Summary

Perf rows are the comparable wrk pass. Strace rows are intrusive syscall-count diagnostics; use their syscall counts, not their RPS or latency, for performance conclusions. A strace status of 130 is expected because the harness stops strace with SIGINT after the measured wrk run.

| Framework | Route | Kind | RPS | p99 | Socket errors | Non-2xx | read/recv | write/send | epoll wait | epoll_ctl | futex | cycles | instructions | context switches |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| actix | json-serialize | perf | 721038.24 | 3.21ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 719473 |
| actix | json-serialize | strace | 89394.75 | 4.00ms | 0 | 0 | 903305 | 903432 | 28507 | 405 |  |  |  |  |
| actix | json-static | perf | 733955.91 | 3.27ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 693335 |
| actix | json-static | strace | 89256.99 | 4.04ms | 0 | 0 | 894795 | 894829 | 28278 | 514 |  |  |  |  |
| actix | plaintext-handler | perf | 744213.92 | 3.36ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 572216 |
| actix | plaintext-handler | strace | 89507.26 | 3.96ms | 0 | 0 | 904479 | 904549 | 28693 | 479 |  |  |  |  |
| kruda | json-serialize | perf | 794603.28 | 1.36ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 207811 |
| kruda | json-serialize | strace | 68313.69 | 1.34s | 0 | 0 | 687405 | 684133 | 491 | 1283 | 15316 |  |  |  |
| kruda | json-static | perf | 817824.53 | 1.53ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 358534 |
| kruda | json-static | strace | 69095.24 | 1.32s | 0 | 0 | 695462 | 691965 | 520 | 1285 | 14409 |  |  |  |
| kruda | plaintext-handler | perf | 830630.04 | 1.41ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 374509 |
| kruda | plaintext-handler | strace | 69094.46 | 1.51s | 0 | 0 | 695406 | 692132 | 471 | 1281 | 14215 |  |  |  |
