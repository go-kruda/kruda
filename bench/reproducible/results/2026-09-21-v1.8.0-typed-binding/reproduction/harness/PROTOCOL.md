# Balanced capacity protocol

- Compare Kruda v1.8.0 at commit `5ff81f1c7d961f7aa71d74d426310987b5a96a19`, Fiber 3.5.0, and Actix Web 4.15.0 on the same native Linux host.
- Exercise the same 30-field query binding and validated JSON response contract, including a valid request and an invalid-value smoke check.
- Test profiles 4 and 8. Use every permutation of the three servers once per profile: six samples per server and 36 measured samples total.
- Run a 2-second warmup followed by an 8-second closed-loop capacity measurement with `wrk -t4 -c256 --latency`.
- Record exact binaries, sources, commands, toolchains, environment, process affinity, errors, p99, throughput, and approximate server CPU cost.
- Stop benchmark-owned processes on every exit. Service isolation and exact restoration are owned by the controller.

Capacity p99 is saturation/closed-loop evidence. It is not used as a common-load latency claim.
