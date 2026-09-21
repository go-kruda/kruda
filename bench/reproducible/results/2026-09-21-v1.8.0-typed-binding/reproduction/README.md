# Kruda v1.8.0 typed-binding reproduction bundle

This directory preserves the exact source inputs, workload controller, execution order, and manifests behind the published 21 September 2026 results. It is evidence from the completed run rather than a portable one-command benchmark: the controller contains the original DEV host paths and service isolation policy so reviewers can audit exactly what ran.

## Frozen implementations

| Server | Source | Measured binary SHA-256 |
|---|---|---|
| Kruda v1.8.0 | `sources/kruda/source.tar.gz`, generated with `git archive` from commit `5ff81f1c7d961f7aa71d74d426310987b5a96a19` | `2c50a205ebaf76c3f4cc7605204dc7e02e22de6ff29b74160a057fcb7cdc2232` |
| Fiber v3.5.0 | `sources/fiber/` including `go.mod` and `go.sum` | `e1212e3aedffdaf1f8b8c14a1df705e15918478f1784535244a9b227754d5101` |
| Actix Web v4.15.0 | `sources/actix/` including `Cargo.toml` and `Cargo.lock` | `6c81a0c70ebfa78e00a618803d23635b94456ef1a894027ae0642f84550e2815` |

`manifests/capacity/manifest.json` binds every source file and measured binary to its hash. `manifests/capacity/build-info.json` records the Go build information embedded in the Kruda and Fiber binaries and the Actix compiler record.

Using the pinned Go and Rust toolchains, rebuild the server inputs with:

```text
mkdir -p bin
(cd sources/fiber && GOTOOLCHAIN=local GOWORK=off go build -o ../../bin/fiber .)
(cd sources/actix && cargo build --locked --release && cp target/release/actix-bench ../../bin/actix)
python3 harness/prepare.py
```

The final command extracts and builds the frozen Kruda archive, then writes a new manifest for the rebuilt binaries. Binary hashes are environment-sensitive; compare source hashes and embedded build information when rebuilding on a different host.

## Exact execution

- `harness/prepare.py` extracts and builds the frozen Kruda source, then freezes source and binary hashes.
- `harness/run_balanced.py` constructs the 30-field valid and invalid contracts, verifies response bytes and headers, records affinity and host state, and runs capacity measurements.
- `harness/run_matched_rate.py` runs the first four matched-rate permutations and applies the achieved-rate, error, calibration, and histogram reconciliation gates.
- `harness/run_matched_extension.py` runs the two missing permutations without rewriting the original matched-rate evidence.
- `harness/run_full.py` is the exact top-level sequence used for the v1.8.0 run.
- `harness/remote_controller.py` is the exact isolation and restoration controller used on the DEV host.

The three `actual-order.json` files under `manifests/` record every server command, load command, environment, position, and result in execution order. The capacity schedule and protocol are preserved beside them. The two matched-rate manifests hash the evidence inputs and outputs.

The controller was invoked as:

```text
python3 harness/remote_controller.py
```

It launched the measurement sequence as:

```text
python3 harness/run_full.py
```

## Patched wrk2

Matched-rate measurements used wrk2 commit `44a94c17d8e6a0bac8559b53da76848e430cb7a7` plus `wrk2/window-metadata.patch`. The patch adds per-thread post-calibration completion windows and no per-request work. Apply it to that commit and build with:

```text
make -j2 CC=/usr/bin/gcc CFLAGS="-std=c99 -Wall -O2 -D_REENTRANT -D_POSIX_C_SOURCE=200809L -D_BSD_SOURCE -Ideps/luajit/src -I<openssl-dev>/usr/include -I<openssl-dev>/usr/include/x86_64-linux-gnu" LIBS="-lluajit -lpthread -lm <openssl-dev>/usr/lib/x86_64-linux-gnu/libssl.a <openssl-dev>/usr/lib/x86_64-linux-gnu/libcrypto.a -ldl"
```

The benchmark binary SHA-256 was `9c25f03760d80ae11439badd5d57eb2d985c2e7329df4490b4a1e06a8102bc5e`. The patched `src/wrk.c` and `src/wrk.h` hashes were `a5a6380320c8a90270e6a34ae433ec52ce973136b44c9112116da61bbb1cedfd` and `14571456938cdf85127e3538cd7e053e84b6052c0aa4b66a4f1bd5c6892867f0`.

## Scope

The checked-in summaries remain the compact result interface. This bundle supplies reviewable provenance and exact commands; it intentionally omits compiled binaries, raw service snapshots, and repetitive per-sample logs.
