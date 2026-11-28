# Kruda HLS segment proxy benchmark

This download contains a Linux example pinned to Kruda v1.9.0, the source for the experimental comparison, and data from the original runs. The origin serves synthetic bytes in RAM. The example checks HTTP segment delivery and TCP connection reuse without a video player.

## Try the released version

Use Linux, Go 1.26.11 or newer with toolchain downloads enabled, Bash, and Python 3.10 or newer. The build selects Go 1.26.11. Go downloads the published Kruda module and verifies it with its module checksum.

```sh
bash build.sh release
bash run.sh release
```

The check starts an origin and a Wing proxy on IPv4 loopback and stops them when it exits. It checks a 3,000,000-byte GET, a 1,000-byte Range response, HEAD metadata, and one TCP socket reused for GET, Range GET, and GET after a six-second idle interval. The route uses `kruda.Stream.With(kruda.StreamKeepAlive())` in [fixture/main.go](fixture/main.go).

To inspect the responses manually after building, start the two processes in separate terminals:

```sh
./bin/go-bench origin 127.0.0.1:8701
./bin/go-bench wing 127.0.0.1:8702 http://127.0.0.1:8701
```

In a third terminal:

```sh
curl -fsS -o /dev/null -w 'GET %{http_code}, %{size_download} bytes\n' http://127.0.0.1:8702/media/0.ts
curl -fsS -o /dev/null -w 'Range %{http_code}, %{size_download} bytes\n' -H 'Range: bytes=100-1099' http://127.0.0.1:8702/media/0.ts
curl -fsSI http://127.0.0.1:8702/media/0.ts
```

Expect GET 200 with 3,000,000 bytes, Range 206 with 1,000 bytes, and HEAD 200 with a Content-Length of 3,000,000. Stop both processes with Ctrl+C. See the [released performance qualification](https://github.com/go-kruda/kruda/releases/tag/v1.9.0#hls-and-byte-range-streaming-on-wing-streamkeepalive) and its retained [round log](evidence/release/gate11.txt). The small check above verifies the feature. The qualification used a separately built release-tree fixture at 5,120 simulated viewers.

## Build and smoke-test the experimental comparison

Use Linux with Python, Bash, GNU patch, unzip, coreutils, util-linux for `taskset`, Go, Rustup, and a C toolchain for linking. Install Rust 1.98.1 with `rustup toolchain install 1.98.1 --profile minimal` if needed. The Go and Rust dependency versions are pinned in the package.

```sh
bash build.sh experimental
bash run.sh smoke
```

The build keeps a stock v1.9.0 binary alongside the experimental one. It extracts the checksum-verified Go module archive under `bin/`, applies [wing-256.patch](wing-256.patch), and uses a separate Go modfile for the experimental binary. The patch reconstructs the measured copy path from the original build record. Rebuilt binaries can have different SHA-256 digests because the build paths and the original overlay differ. The historical executable digests are in [evidence/long/original-sha256.txt](evidence/long/original-sha256.txt). The smoke test checks the same HTTP contract for all three stacks and runs each at 64 simulated viewers for one warm cycle and one measured cycle.

## Run the original workloads

Use a dedicated Linux host. The measured profiles used 2 pinned proxy vCPUs, 4 each for the driver and origin, and a separate coordinator. The scripts default to the historical CPU numbering. Set `HLS_PROXY_CPUS`, `HLS_DRIVER_CPUS`, `HLS_ORIGIN_CPUS`, and `HLS_COORDINATOR_CPU` to disjoint CPU sets of the same size if your numbering differs. The scored modes require a file descriptor limit of at least 65,535 and read Linux procfs counters. Only loopback sockets are opened.

```sh
bash run.sh short
bash run.sh long
```

- `short` replays the 6,144-viewer profile with one 6-second warm-up and two scored cycles. It uses the original driver that records measurement outcomes and combined origin hits.
- `long` replays the 11,776-viewer profile with one 6-second warm-up and ten scored cycles. It uses the phase-aware driver to distinguish warm-up misses from measured misses.
- Each mode first checks full and partial body bytes, HEAD, and socket reuse for every stack. Then five interleaved blocks run in the original order. The runner writes a new directory under `runs/` by default. Exit status covers execution of the prescribed arms. Per-arm `pass` and, for `long`, `measured_pass` in `score/results.jsonl` describe the measurement gates.

These profiles use a 512-request in-flight cap in the load driver, one 3,000,000-byte segment every 6 seconds per simulated viewer, a RAM origin, and HTTP/1.1 loopback. Read the [short protocol](evidence/short/PROTOCOL.md) and [long protocol](evidence/long/PROTOCOL.md) for the exact checks. Hardware and host load affect the results.

## Audit the retained data

```sh
sha256sum -c SHA256SUMS
python3 evidence/long/summarize.py evidence/long | diff -u evidence/long/REPORT.md -
python3 evidence/short/summarize.py evidence/short high lower | diff -u evidence/short/REPORT.md -
```

The scripts read the original JSONL and sampled process counters, then reconstruct the checked-in reports. `evidence/short/high` is the earlier 12-second high-load screen. The 60-second results in `evidence/long/score` are the results used for 11,776 viewers on the benchmark page. The driver retained p99 aggregates, so the replay does not rederive latency percentiles from individual observations.
