# Syscall Profile Summary

Perf rows are the comparable wrk pass. Strace rows are intrusive syscall-count diagnostics; use their syscall counts, not their RPS or latency, for performance conclusions. A strace status of 130 is expected because the harness stops strace with SIGINT after the measured wrk run.

| Framework | Route | Kind | RPS | p99 | Socket errors | Non-2xx | read/recv | write/send | epoll wait | epoll_ctl | futex | cycles | instructions | context switches |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| actix | json-serialize | perf | 732692.88 | 3.27ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 658763 |
| actix | json-serialize | strace | 89592.54 | 3.94ms | 0 | 0 | 897802 | 897848 | 28360 | 505 |  |  |  |  |
| actix | json-static | perf | 729731.10 | 3.13ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 786449 |
| actix | json-static | strace | 89109.13 | 4.22ms | 0 | 0 | 900486 | 900541 | 28658 | 501 |  |  |  |  |
| actix | plaintext-handler | perf | 734089.81 | 3.14ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 874690 |
| actix | plaintext-handler | strace | 89412.77 | 4.08ms | 0 | 0 | 896172 | 896226 | 28423 | 507 |  |  |  |  |
| kruda | json-serialize | perf | 803952.49 | 1.60ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 253818 |
| kruda | json-serialize | strace | 67966.51 | 1.05s | 0 | 0 | 685338 | 681248 | 484 | 1256 | 15492 |  |  |  |
| kruda | json-static | perf | 830353.47 | 1.33ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 362954 |
| kruda | json-static | strace | 69018.36 | 1.54s | 121 | 0 | 694358 | 691422 | 432 | 1277 | 14158 |  |  |  |
| kruda | plaintext-handler | perf | 841268.52 | 1.21ms | 0 | 0 |  |  |  |  |  | <not supported> | <not supported> | 353529 |
| kruda | plaintext-handler | strace | 69483.72 | 1.47s | 42 | 0 | 698806 | 695786 | 419 | 1262 | 14069 |  |  |  |
