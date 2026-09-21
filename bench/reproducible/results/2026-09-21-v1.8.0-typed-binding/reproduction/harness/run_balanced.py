"""Controlled native HTTP comparison; isolation and restoration are externally owned."""
import csv
import fcntl
import hashlib
import http.client
import itertools
import json
import math
import os
from pathlib import Path
import shutil
import signal
import socket
import statistics
import subprocess
import sys
import time

from wrk_helpers import parse_wrk

ROOT = Path(__file__).resolve().parent.parent
MANIFEST = ROOT / "manifest.json"
SHARED_LOCK = "/home/tiger/kruda-typed-comparison-20260912/benchmark.lock"
BINARIES = {"kruda": "bin/kruda.test", "fiber": "bin/fiber", "actix": "bin/actix"}
PORT = 18373
ARGUMENTS = ["-test.run=^TestCompositionBenchmarkServer$", "-test.timeout=60s"]
STOP_SECONDS = 12
RUNNERS = ["actions.runner.tiinno-learn-platform.dev-server.service", "forgejo-runner.service"]
REMOVED_ENV = {"GOFLAGS", "GOEXPERIMENT", "GOGC", "GOMEMLIMIT", "GOMAXPROCS", "GODEBUG",
               "LD_PRELOAD", "LD_LIBRARY_PATH", "PORT", "RUST_LOG", "RUST_BACKTRACE"}
SCOPE = "Controlled DEV closed-loop 30-field validated HTTP comparison at exact Kruda v1.8.0 commit 5ff81f1c7d961f7aa71d74d426310987b5a96a19; profile-specific capacity evidence."
REQUIRED_SOURCES = {"kruda": {"sources/kruda/source.tar.gz", "sources/kruda/go.mod", "sources/kruda/commit.txt"}}
REQUIRED_SOURCES.update(fiber={"sources/fiber/main.go", "sources/fiber/go.mod", "sources/fiber/go.sum"},
    actix={"sources/actix/src/main.rs", "sources/actix/Cargo.toml", "sources/actix/Cargo.lock", "sources/actix/rust-version.txt", "sources/actix/build.log"})


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, allow_nan=False) + "\n")
    temporary.replace(path)


def contract(fields):
    query, body = [], {}
    for group in range(1, fields // 5 + 1):
        suffix = "" if group == 1 else str(group)
        for name, value in [("id", "9"), ("page", "3"), ("active", "false"), ("score", "2.5"), ("name", "alice")]:
            query.append(f"{name}{suffix}={value}")
        for name, value in [("ID", 42 if group == 1 else 9), ("Page", 3), ("Active", False), ("Score", 2.5), ("Name", "alice")]:
            body[name + suffix] = value
    return "/users/42?" + "&".join(query), json.dumps(body, separators=(",", ":")).encode()


def validation_contract(fields):
    path, _ = contract(fields)
    body = {"code": 422, "message": "Validation failed", "errors": [
        {"field": "page", "rule": "min", "param": "1", "message": "page must be at least 1", "value": "0"}]}
    return path.replace("page=3", "page=0", 1), json.dumps(body, separators=(",", ":")).encode()


def verify_hashes(expected):
    got = {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in expected}
    if got != expected:
        raise RuntimeError("Frozen source/binary/harness hashes changed")
    return got


def require_alive(server):
    if server.poll() is not None:
        raise RuntimeError(f"Server exited early: {server.returncode}")


def smoke(fields, record_path, invalid=False):
    path, expected = validation_contract(fields) if invalid else contract(fields)
    status = 422 if invalid else 200
    conn = http.client.HTTPConnection("127.0.0.1", PORT, timeout=2)
    try:
        conn.request("GET", path)
        response = conn.getresponse()
        body, pairs = response.read(), response.getheaders()
        headers = {k.lower(): v for k, v in pairs}
        wanted = {"content-type": "application/json; charset=utf-8", "content-length": str(len(expected)), "x-benchmark": "bindgen"}
        record = dict(path=path, status=response.status, body=body.decode(errors="replace"), body_hex=body.hex(), headers=pairs,
                      expected=dict(status=status, body=expected.decode(), headers=wanted,
                                    errors=json.loads(expected)["errors"] if invalid else []))
        write_json(record_path, record)
        if response.status != status or body != expected:
            raise RuntimeError(f"Unexpected {fields}-field response: {response.status}, {body!r}")
        for key, value in wanted.items():
            if sum(k.lower() == key for k, _ in pairs) != 1 or headers.get(key) != value:
                raise RuntimeError(f"Unexpected or duplicate {key}: {pairs}")
        return record
    finally:
        conn.close()


def stop_owned(process):
    if process is None or process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        try:
            process.wait(timeout=3)
        except subprocess.TimeoutExpired as error:
            raise RuntimeError(f"Owned process {process.pid} remained unreaped after SIGKILL") from error


def affinity(pid, expected):
    observed = sorted(os.sched_getaffinity(pid))
    if observed != expected:
        raise RuntimeError(f"Process {pid} CPU affinity changed: {observed}")
    return observed


def cpu_snapshot(pid):
    fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
    return dict(user_ticks=int(fields[11]), system_ticks=int(fields[12]), start_ticks=int(fields[19]),
                observed_monotonic=time.monotonic())


def cpu_usage(before, after, ticks, result):
    if before["start_ticks"] != after["start_ticks"]:
        raise RuntimeError("Server process identity changed")
    user = (after["user_ticks"] - before["user_ticks"]) / ticks
    system = (after["system_ticks"] - before["system_ticks"]) / ticks
    elapsed = after["observed_monotonic"] - before["observed_monotonic"]
    if min(user, system) < 0 or user + system <= 0:
        raise RuntimeError("Invalid CPU deltas")
    if not result["elapsed_seconds"] - 0.05 <= elapsed <= result["elapsed_seconds"] + 0.5:
        raise RuntimeError("CPU sample window does not align with measured wrk interval")
    return dict(server_user_cpu_seconds=user, server_system_cpu_seconds=system, server_cpu_seconds=user + system,
                cpu_observed_elapsed_seconds=elapsed, average_server_cpu_cores=(user + system) / elapsed,
                server_cpu_microseconds_per_request=(user + system) * 1e6 / result["requests"],
                server_system_cpu_microseconds_per_request=system * 1e6 / result["requests"])


def schedule(measure):
    arms = list(BINARIES)
    orders = list(itertools.permutations(range(3)))
    for arm in range(3):
        positions = [order.index(arm) for order in orders]
        if any(positions.count(position) != 2 for position in range(3)):
            raise RuntimeError("Invalid six-round position balance")
    for a, b in itertools.combinations(range(3), 2):
        if sum(order.index(a) < order.index(b) for order in orders) != 3:
            raise RuntimeError("Invalid Williams pair-order balance")
    entries = []
    for profile in [4, 8]:
        for fields in [30]:
            for round_no, order in enumerate(orders if measure else [list(range(3))], 1):
                for position, index in enumerate(order, 1):
                    entries.append(dict(sequence=len(entries) + 1, profile=profile, fields=fields,
                                        round=round_no, position=position, arm=arms[index]))
    return entries


def read_manifest():
    content = MANIFEST.read_bytes()
    manifest = json.loads(content)
    if manifest["benchmark_lock"] != SHARED_LOCK or set(manifest["binaries"]) != set(BINARIES):
        raise RuntimeError("Expected original shared lock and three exact arms")
    expected = dict(manifest["files"])
    if not {"harness/run_balanced.py", "harness/wrk_helpers.py", "harness/PROTOCOL.md"}.issubset(expected):
        raise RuntimeError("Missing frozen harness/protocol files")
    for arm, item in manifest["binaries"].items():
        toolchain = {"name": "rustc", "version": "rustc 1.98.1"} if arm == "actix" else {"name": "go", "version": "go1.27.1"}
        if item["path"] != BINARIES[arm] or item["toolchain"] != toolchain:
            raise RuntimeError(f"Unexpected binary path/toolchain: {arm}")
        if not REQUIRED_SOURCES[arm].issubset(item["sources"]) or not set(item["sources"]).issubset(expected):
            raise RuntimeError(f"Missing frozen source provenance: {arm}")
        if item["path"] in expected:
            raise RuntimeError("Duplicate binary manifest entry")
        expected[item["path"]] = item["sha256"]
    for name, sha in expected.items():
        if Path(name).is_absolute() or ".." in Path(name).parts or not (ROOT / name).resolve().is_relative_to(ROOT):
            raise RuntimeError("Manifest paths must remain inside the controlled evidence directory")
        if len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha):
            raise RuntimeError(f"Invalid SHA256: {name}")
    return content, manifest, expected


def build_info():
    result = {}
    for arm in ["kruda", "fiber"]:
        binary = ROOT / BINARIES[arm]
        info = subprocess.check_output(["go", "version", "-m", str(binary)], text=True, timeout=10)
        lines = {line.strip() for line in info.splitlines()}
        if info.splitlines()[0] != f"{binary}: go1.27.1" or not {"build\tGOOS=linux", "build\tGOARCH=amd64"}.issubset(lines):
            raise RuntimeError(f"Unexpected embedded Go toolchain/target: {arm}")
        if arm == "fiber" and not any(line.startswith("dep\tgithub.com/gofiber/fiber/v3\tv3.5.0\t") for line in lines):
            raise RuntimeError("Missing exact embedded Fiber dependency")
        result[arm] = info
    rust = (ROOT / "sources/actix/rust-version.txt").read_text()
    if not rust.startswith("rustc 1.98.1 "):
        raise RuntimeError("Unexpected frozen Rust compiler record")
    result["actix"] = dict(compiler_record=rust, provenance="Frozen compiler/build/source records; runtime separately attests Actix version")
    return result


def environment(entry):
    arm, profile = entry["arm"], entry["profile"]
    controlled = dict(LC_ALL="C")
    if arm != "actix":
        controlled["GOMAXPROCS"] = str(profile)
    if arm == "kruda":
        controlled.update(KRUDA_COMPOSITION_SERVER="1", KRUDA_COMPOSITION_FIELDS=str(entry["fields"]),
                          KRUDA_COMPOSITION_WORKERS=str(profile), KRUDA_COMPOSITION_ADDR=f"127.0.0.1:{PORT}")
    else:
        controlled.update(PORT=str(PORT), BENCH_FIELDS=str(entry["fields"]), BENCH_WORKERS=str(profile))
    env = {k: v for k, v in os.environ.items() if k not in REMOVED_ENV and not k.startswith(("KRUDA_", "BENCH_"))}
    env.update(controlled)
    return env, controlled


def foreign_benchmarks(owned_pids=()):
    found, unreadable = [], 0
    binary_paths = {str((ROOT / name).resolve()) for name in BINARIES.values()}
    for proc in Path("/proc").iterdir():
        if not proc.name.isdigit() or int(proc.name) in {os.getpid(), *owned_pids}:
            continue
        try:
            args = [s.decode(errors="replace") for s in (proc / "cmdline").read_bytes().split(b"\0") if s]
        except (FileNotFoundError, ProcessLookupError):
            continue
        except PermissionError:
            unreadable += 1
            continue
        if not args:
            continue
        name = Path(args[0]).name
        bench = any(a.startswith(("-test.bench=", "-bench=")) and a.split("=", 1)[1] not in {"", "^$"} for a in args)
        bench |= "-bench" in args or "-test.bench" in args
        server = any("TestBindgenBenchmarkServer" in a or "TestCompositionBenchmarkServer" in a for a in args)
        if name in {"wrk", "wrk2", "vegeta", "oha", "io_probe", "io_probe_partial", "perf", "strace"} or args[0] in binary_paths or bench or server:
            found.append(dict(pid=int(proc.name), executable=name))
    return dict(known_foreign_benchmarks=found, unreadable_cmdlines=unreadable)


def verify_server(server, entry, controlled, log_path):
    require_alive(server)
    observed = {}
    for item in Path(f"/proc/{server.pid}/environ").read_bytes().split(b"\0"):
        key, sep, value = item.partition(b"=")
        key = key.decode(errors="replace")
        if sep and (key in REMOVED_ENV or key in controlled or key.startswith(("KRUDA_", "BENCH_"))):
            observed[key] = value.decode()
    if observed != controlled:
        raise RuntimeError(f"Live environment mismatch: {observed}")
    required = {f"fields={entry['fields']}", f"workers={entry['profile']}", f"addr=127.0.0.1:{PORT}"}
    if entry["arm"] == "kruda":
        required.update({"composition", "typed=true",
                         "go=go1.27.1", "engine=sonic", "read_buffer=8192", f"gomaxprocs={entry['profile']}"})
    elif entry["arm"] == "fiber":
        required.update({"fiber=3.5.0", f"gomaxprocs={entry['profile']}"})
    else:
        required.add("actix=4.15.0")
    if not any(required.issubset(set(line.split())) for line in log_path.read_text().splitlines()):
        raise RuntimeError(f"Missing startup markers: {sorted(required)}")
    return observed


def service_state():
    state = dict(observed_unix=time.time(), observed_monotonic=time.monotonic(), runners={})
    command = ["docker", "ps", "--format", "{{.ID}}\t{{.Names}}"]
    result = subprocess.run(command, capture_output=True, text=True, timeout=5)
    state["containers"] = dict(command=command, exit_code=result.returncode, stdout=result.stdout, stderr=result.stderr)
    for unit in RUNNERS:
        command = ["systemctl", "show", unit, "--property=LoadState,ActiveState,SubState"]
        result = subprocess.run(command, capture_output=True, text=True, timeout=5)
        state["runners"][unit] = dict(command=command, exit_code=result.returncode, stdout=result.stdout, stderr=result.stderr)
    return state


def require_service_state(state, isolated):
    containers = state["containers"]
    if containers["exit_code"] or bool(containers["stdout"].strip()) == isolated:
        raise RuntimeError("Container state does not match preflight/isolated phase")
    for unit, result in state["runners"].items():
        props = dict(line.split("=", 1) for line in result["stdout"].splitlines() if "=" in line)
        if result["exit_code"] or props.get("LoadState") != "loaded" or props.get("ActiveState") != ("inactive" if isolated else "active"):
            raise RuntimeError(f"Unexpected runner state: {unit}: {result}")


def interval_snapshot():
    result = dict(observed_unix=time.time(), observed_monotonic=time.monotonic())
    for name in ["stat", "loadavg", "meminfo", "pressure/cpu", "pressure/io", "pressure/memory"]:
        path = Path("/proc") / name
        result[name] = path.read_text() if path.exists() else None
    return result


def finish_server(server, log_path, exit_path, require_pass):
    record = dict(pid=server.pid, terminate_requested=False, forced_kill=False, returncode=None,
                  pass_required=require_pass, final_pass=False)
    try:
        require_alive(server)
        record["terminate_requested"] = True
        server.terminate()
        try:
            server.wait(timeout=STOP_SECONDS)
        except subprocess.TimeoutExpired:
            record["forced_kill"] = True
            server.kill()
            server.wait(timeout=3)
            raise RuntimeError("Server required forced kill")
    finally:
        record["returncode"] = server.poll()
        lines = log_path.read_text().strip().splitlines()
        record["final_pass"] = bool(lines) and lines[-1] == "PASS"
        write_json(exit_path, record)
    if record["returncode"] != 0 or (require_pass and not record["final_pass"]):
        raise RuntimeError(f"Invalid graceful server exit: {record}")
    return record


def require_preflight(content, expected):
    path = ROOT / "preflight-results"
    complete = json.loads((path / "complete.json").read_text())
    actual = json.loads((path / "actual-order.json").read_text())
    if (path / "failure.json").exists() or complete.get("complete") is not True or complete.get("cases") != 6:
        raise RuntimeError("A complete native preflight is required before isolation timing")
    if (path / "frozen-manifest.json").read_bytes() != content:
        raise RuntimeError("Preflight manifest differs from the measurement manifest")
    for name in ["hashes-before.json", "hashes-after.json"]:
        if json.loads((path / name).read_text()) != expected:
            raise RuntimeError("Preflight source/binary hashes differ")
    if [{k: r[k] for k in schedule(False)[0]} for r in actual] != schedule(False):
        raise RuntimeError("Preflight cases differ")
    for item in actual:
        record = item["server_exit"]
        if item["status"] != "passed" or record["returncode"] != 0 or record["forced_kill"] or (record["pass_required"] and not record["final_pass"]):
            raise RuntimeError("Invalid preflight completion record")


def summarize(rows, actual):
    if len(rows) != 36 or [{k: r[k] for k in schedule(True)[0]} for r in rows] != schedule(True):
        raise RuntimeError("Incomplete or reordered measurements")
    cells = []
    metrics = ["rps", "p99_ms", "server_cpu_microseconds_per_request", "server_system_cpu_microseconds_per_request"]
    for profile in [4, 8]:
        for fields in [30]:
            selected = [r for r in rows if r["profile"] == profile and r["fields"] == fields]
            medians = {arm: {m: statistics.median(r[m] for r in selected if r["arm"] == arm) for m in metrics} for arm in BINARIES}
            comparisons = []
            for a, b in itertools.combinations(BINARIES, 2):
                pairs = []
                for n in range(1, 7):
                    pair = {r["arm"]: r for r in selected if r["round"] == n}
                    x, y = pair[a], pair[b]
                    pairs.append(dict(round=n, a_rps=x["rps"], b_rps=y["rps"], b_over_a_rps=y["rps"] / x["rps"],
                                      a_p99_ms=x["p99_ms"], b_p99_ms=y["p99_ms"]))
                comparisons.append(dict(a=a, b=b, pairs=pairs,
                    median_rps_change_pct=(medians[b]["rps"] / medians[a]["rps"] - 1) * 100,
                    median_p99_change_pct=(medians[b]["p99_ms"] / medians[a]["p99_ms"] - 1) * 100,
                    b_rps_wins=sum(p["b_over_a_rps"] > 1 for p in pairs), rps_ties=sum(p["b_over_a_rps"] == 1 for p in pairs),
                    b_p99_wins=sum(p["b_p99_ms"] < p["a_p99_ms"] for p in pairs), p99_ties=sum(p["b_p99_ms"] == p["a_p99_ms"] for p in pairs)))
            cells.append(dict(profile=profile, fields=fields, medians=medians, comparisons=comparisons))
    return dict(complete=True, measurements=36, cells=cells, scope=SCOPE,
        measured_requests=sum(r["requests"] for r in rows), warmup_requests=sum(r["warmup"]["requests"] for r in actual),
        recorded_errors={k: sum(r[p][k] for r in actual for p in ["warmup", "measure"]) for k in ["connect", "read", "write", "timeout", "non2xx"]})


def main():
    if sys.platform != "linux" or sys.argv[1:] not in [["preflight"], ["measure"]]:
        raise RuntimeError("Usage on native Linux: python3 -B harness/run_balanced.py preflight|measure")
    measure = sys.argv[1] == "measure"
    content, manifest, expected = read_manifest()
    output = ROOT / ("controlled-results" if measure else "preflight-results")
    entries = schedule(measure)
    actual, rows = [], []
    server = load = vmstat = None
    log_path = exit_path = None
    require_pass = False
    allowed_cpus = sorted(os.sched_getaffinity(0))
    ticks = os.sysconf("SC_CLK_TCK")
    wrk = shutil.which("wrk") if measure else None
    if measure and wrk is None:
        raise RuntimeError("wrk is required")

    def interrupted(signum, frame):
        raise KeyboardInterrupt(f"signal {signum}")

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    with Path(SHARED_LOCK).open("r+") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        output.mkdir()
        raw = output / "raw"
        raw.mkdir()
        try:
            write_json(output / "hashes-before.json", verify_hashes(expected))
            (output / "frozen-manifest.json").write_bytes(content)
            if measure:
                require_preflight(content, expected)
            state = service_state()
            write_json(output / "services-before.json", state)
            require_service_state(state, isolated=measure)
            write_json(output / "host-before.json", interval_snapshot())
            write_json(output / "build-info.json", build_info())
            write_json(output / "expected-schedule.json", entries)
            write_json(output / "actual-order.json", actual)
            wrk_hash = hashlib.sha256(Path(wrk).read_bytes()).hexdigest() if measure else None
            write_json(output / "protocol.json", dict(stage=sys.argv[1], scope=SCOPE,
                cases=len(entries), warmup_seconds=2 if measure else 0, measure_seconds=8 if measure else 0,
                wrk_threads=4 if measure else None, connections=256 if measure else None,
                wrk_binary=str(Path(wrk).resolve()) if measure else None, wrk_sha256=wrk_hash,
                profiles={"4": "Kruda W4/P4, Fiber P4, Actix workers4", "8": "Kruda W8/P8, Fiber P8, Actix workers8"},
                kruda_arguments=ARGUMENTS, competitor_arguments=[], read_buffer_kruda=8192,
                http_version="HTTP/1.1", pipelining=False, load_model="closed-loop" if measure else "correctness smoke only",
                query_scope="Unique unescaped keys only; native competitor decoding/duplicate behavior is not a Wing parity claim",
                contracts={str(f): dict(path=contract(f)[0], body=contract(f)[1].decode(),
                    invalid_path=validation_contract(f)[0], invalid_body=validation_contract(f)[1].decode(),
                    content_type="application/json; charset=utf-8", benchmark_header="bindgen") for f in [30]},
                arms=manifest["binaries"], controlled_environment=[dict(entry=e, values=environment(e)[1]) for e in schedule(False)],
                removed_environment=sorted(REMOVED_ENV) + ["KRUDA_*", "BENCH_*"], benchmark_lock=SHARED_LOCK,
                source_manifest_sha256=hashlib.sha256(content).hexdigest(), parent_affinity=allowed_cpus,
                cpu_scope="Approximate own-server thread-aggregate /proc/PID/stat deltas around wrk; excludes client/host services",
                affinity_scope="Main-thread allowed sets observed only; no pinning or every-worker-thread proof",
                clock_ticks_per_second=ticks, cpu_window_tolerance_seconds=[-0.05, 0.5],
                duration_tolerance_seconds=[-0.05, 0.5], readiness_deadline_seconds=7,
                graceful_stop_timeout_seconds=STOP_SECONDS, cleanup_term_timeout_seconds=5, cleanup_kill_timeout_seconds=3,
                ordering="All six permutations per profile; each arm occupies each position twice and every pair order is 3/3",
                restoration="Externally owned; measurement complete.json does not attest service restoration"))
            with (output / "vmstat.txt").open("w") as stats, (output / "summary.csv").open("w", buffering=1) as summary:
                if measure:
                    vmstat = subprocess.Popen(["vmstat", "-w", "1"], stdout=stats, stderr=subprocess.STDOUT)
                writer = None
                for entry in entries:
                    key = f"p{entry['profile']}-f{entry['fields']}-r{entry['round']:02d}-{entry['arm']}"
                    scan = foreign_benchmarks()
                    write_json(raw / f"{key}-benchmark-scan.json", scan)
                    if scan["known_foreign_benchmarks"] or scan["unreadable_cmdlines"]:
                        raise RuntimeError("Foreign benchmark found or process inspection incomplete")
                    env, controlled = environment(entry)
                    require_pass = entry["arm"] == "kruda"
                    command = [str(ROOT / BINARIES[entry["arm"]]), *(ARGUMENTS if require_pass else [])]
                    current = dict(entry, status="starting", started_unix=time.time(), command=command, environment=controlled)
                    actual.append(current)
                    write_json(output / "actual-order.json", actual)
                    with socket.socket() as check:
                        check.bind(("127.0.0.1", PORT))
                    log_path, exit_path = raw / f"{key}-server.log", raw / f"{key}-exit.json"
                    with log_path.open("w") as log:
                        server = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, env=env)
                    current["server_pid"] = server.pid
                    deadline = time.monotonic() + 7
                    while time.monotonic() < deadline:
                        require_alive(server)
                        try:
                            current["smoke"] = smoke(entry["fields"], raw / f"{key}-smoke.json")
                        except (OSError, http.client.HTTPException):
                            time.sleep(0.05)
                            continue
                        if not any(line.startswith(("composition ", "fiber=", "actix=")) for line in log_path.read_text().splitlines()):
                            time.sleep(0.05)
                            continue
                        current["observed_environment"] = verify_server(server, entry, controlled, log_path)
                        break
                    else:
                        raise RuntimeError("Server readiness timeout")
                    current["validation_smoke"] = smoke(entry["fields"], raw / f"{key}-invalid-smoke.json", invalid=True)
                    require_alive(server)
                    current["server_affinity_before"] = affinity(server.pid, allowed_cpus)
                    for phase, seconds in ([("warmup", 2), ("measure", 8)] if measure else []):
                        state = service_state()
                        write_json(raw / f"{key}-{phase}-services-before.json", state)
                        require_service_state(state, isolated=True)
                        scan = foreign_benchmarks([server.pid])
                        write_json(raw / f"{key}-{phase}-benchmark-scan.json", scan)
                        if scan["known_foreign_benchmarks"] or scan["unreadable_cmdlines"]:
                            raise RuntimeError("Foreign benchmark before load phase")
                        write_json(raw / f"{key}-{phase}-host-before.json", interval_snapshot())
                        current.update(status=phase, phase_started_unix=time.time())
                        path, _ = contract(entry["fields"])
                        command = [wrk, "--latency", "-t4", "-c256", f"-d{seconds}s", f"http://127.0.0.1:{PORT}{path}"]
                        current[f"{phase}_command"] = command
                        write_json(output / "actual-order.json", actual)
                        require_alive(server)
                        if phase == "measure":
                            current["cpu_before"] = cpu_snapshot(server.pid)
                        with (raw / f"{key}-{phase}.log").open("w") as log:
                            load = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, env=env)
                            current[f"{phase}_load_affinity"] = affinity(load.pid, allowed_cpus)
                            code = load.wait(timeout=seconds + 10)
                        if phase == "measure":
                            current["cpu_after"] = cpu_snapshot(server.pid)
                        load = None
                        current[f"{phase}_load_returncode"] = code
                        write_json(raw / f"{key}-{phase}-host-after.json", interval_snapshot())
                        require_alive(server)
                        current[f"{phase}_server_alive"] = True
                        if code:
                            raise RuntimeError(f"wrk exited {code} during {phase}")
                        result = parse_wrk((raw / f"{key}-{phase}.log").read_text())
                        if not all(math.isfinite(v) for v in result.values()) or min(result["requests"], result["rps"], result["p99_ms"]) <= 0 or not seconds - 0.05 <= result["elapsed_seconds"] <= seconds + 0.5:
                            raise RuntimeError(f"Invalid {phase} count/duration/metrics")
                        current[phase] = result
                        state = service_state()
                        write_json(raw / f"{key}-{phase}-services-after.json", state)
                        require_service_state(state, isolated=True)
                    verify_server(server, entry, controlled, log_path)
                    current["server_affinity_after"] = affinity(server.pid, allowed_cpus)
                    affinity(0, allowed_cpus)
                    if measure:
                        current["cpu_usage"] = cpu_usage(current["cpu_before"], current["cpu_after"], ticks, current["measure"])
                        if vmstat.poll() is not None:
                            raise RuntimeError("vmstat exited early")
                    current["server_exit"] = finish_server(server, log_path, exit_path, require_pass)
                    server = None
                    current.update(status="passed", completed_unix=time.time())
                    write_json(output / "actual-order.json", actual)
                    if measure:
                        record = current["server_exit"]
                        row = dict(entry, workers=entry["profile"], gomaxprocs=entry["profile"] if entry["arm"] != "actix" else None,
                            **current["measure"], **current["cpu_usage"], server_exit_code=record["returncode"],
                            server_pass_required=require_pass, server_final_pass=record["final_pass"], server_forced_kill=record["forced_kill"])
                        if writer is None:
                            writer = csv.DictWriter(summary, fieldnames=row)
                            writer.writeheader()
                        writer.writerow(row)
                        rows.append(row)
                        print(json.dumps(row), flush=True)
                    else:
                        print(json.dumps(dict(entry, status="passed")), flush=True)
            if [{k: item[k] for k in entries[0]} for item in actual] != entries or any(item["status"] != "passed" for item in actual):
                raise RuntimeError("Incomplete or reordered cases")
            state = service_state()
            write_json(output / "services-after.json", state)
            require_service_state(state, isolated=measure)
            write_json(output / "hashes-after.json", verify_hashes(expected))
            if MANIFEST.read_bytes() != content:
                raise RuntimeError("Manifest changed")
            if measure:
                after_wrk = hashlib.sha256(Path(wrk).read_bytes()).hexdigest()
                write_json(output / "wrk-hashes.json", dict(before=wrk_hash, after=after_wrk))
                if after_wrk != wrk_hash:
                    raise RuntimeError("wrk changed")
                write_json(output / "summary.json", summarize(rows, actual))
        except BaseException as error:
            if actual and actual[-1]["status"] != "passed":
                actual[-1].update(status="failed", error=f"{type(error).__name__}: {error}")
            write_json(output / "failure.json", dict(complete=False, error=f"{type(error).__name__}: {error}"))
            raise
        finally:
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
            signal.signal(signal.SIGINT, signal.SIG_IGN)
            cleanup_errors = []
            try:
                stop_owned(load)
            except BaseException as error:
                cleanup_errors.append(f"load: {type(error).__name__}: {error}")
            if server is not None:
                try:
                    if not exit_path.exists():
                        finish_server(server, log_path, exit_path, require_pass)
                except BaseException as error:
                    cleanup_errors.append(f"server graceful exit: {type(error).__name__}: {error}")
                try:
                    stop_owned(server)
                except BaseException as error:
                    cleanup_errors.append(f"server cleanup: {type(error).__name__}: {error}")
            try:
                stop_owned(vmstat)
            except BaseException as error:
                cleanup_errors.append(f"vmstat: {type(error).__name__}: {error}")
            if cleanup_errors:
                failure_path = output / "failure.json"
                failure = json.loads(failure_path.read_text()) if failure_path.exists() else dict(complete=False)
                failure["cleanup_errors"] = cleanup_errors
                write_json(failure_path, failure)
            write_json(output / "actual-order.json", actual)
            write_json(output / "host-after.json", interval_snapshot())
            if cleanup_errors:
                raise RuntimeError("Owned-process cleanup failed: " + "; ".join(cleanup_errors))
        write_json(output / "complete.json", dict(complete=True, cases=len(entries), measurements=len(rows), scope=SCOPE,
            manifest_sha256=hashlib.sha256(content).hexdigest(), services_restored=False,
            restoration="Root supervisor must separately restore and verify the original service inventory"))


if __name__ == "__main__":
    main()
