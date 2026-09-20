# KRUDA-29 Linux gates (prototype)

One-command native-Linux verification for the combined prototype branch.
Runs everything the Mac cannot: the full race suites on Linux, the
Linux-only Wing short-read tests, and short fuzz smokes.

This proves **correctness only**. Any timing numbers produced inside this
container are not benchmark evidence: final throughput/latency claims need
the controlled bare-metal window described in Plane KRUDA-60.

## Run

From the repo root (prototype branch checked out):

```bash
docker build -f bench/kruda29-gates/Dockerfile -t kruda29-gates .
docker run --rm kruda29-gates
```

Results and the environment record are printed to stdout. To keep them:

```bash
docker run --rm -v "$PWD/bench/kruda29-gates/results:/out" -e RESULTS_DIR=/out kruda29-gates
```

Options (build args / env):

- `GO_IMAGE` (build arg, default `golang:1.25.13-bookworm`, the repo floor —
  same as CI): toolchain image. Rebuild with e.g.
  `--build-arg GO_IMAGE=golang:1.27-bookworm` to repeat the gates on Go 1.27.
- `FUZZTIME` (env, default `30s`): per-target fuzz time for the six fuzz
  suites. `0` skips fuzzing (seed corpus still runs as unit tests).

## Gates

1. Environment record: Go version, kernel, CPU/RAM, git SHA, package hashes.
2. `go build ./...`
3. `go vet ./...` (default and `kruda_stdjson`).
4. Full race suite, default engine.
5. Full race suite, `kruda_stdjson` engine.
6. Linux-only `TestWingShortRead*` tests, explicitly (also inside 4–5).
7. Six fuzz smokes: `FuzzValidateString`, `FuzzParserDifferential`,
   `FuzzBindJSON`, `FuzzParseHTTPRequest`, `FuzzRouterPattern`,
   `FuzzRouterMatch`.

Any failure stops the run (`set -e`) and the container exits non-zero.
