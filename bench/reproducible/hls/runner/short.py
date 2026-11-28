import argparse
import hashlib
import http.client
import signal
import json
import os
import platform
import socket
import statistics
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path


ACTIX_MODES = {
    "actix": ("./actix-proxy",),
    "worker": ("./actix-lab", "worker", "0", "65536"),
    "split": ("./actix-lab", "shared", "65536", "65536"),
    "write32": ("./actix-lab", "shared", "0", "32768"),
    "actix-tuned": ("./actix-lab", "worker", "0", "32768"),
    "actix-ram": ("./actix-ram",),
}


def tcp_queues(proxy_port, origin_port):
    output = subprocess.check_output(
        ["ss", "-Htn", "state", "established",
         f"( sport = :{proxy_port} or dport = :{origin_port} )"], text=True)
    rows = {"upstream": [], "downstream": []}
    for line in output.splitlines():
        receive, send, local, remote = line.split()[-4:]
        if local.rsplit(":", 1)[-1] == proxy_port:
            rows["downstream"].append((int(receive), int(send)))
        elif remote.rsplit(":", 1)[-1] == origin_port:
            rows["upstream"].append((int(receive), int(send)))
    return rows


def worker_ticks(pid):
    return {t.name: {"comm": (t/"comm").read_text().strip(),
                     "cpu_ticks": sum(map(int,(t/"stat").read_text().rsplit(")",1)[1].split()[11:13]))}
            for t in Path(f"/proc/{pid}/task").iterdir() if (t/"stat").exists()}


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def fetch(url, method="GET", headers=None):
    request = urllib.request.Request(url, method=method, headers=headers or {})
    with urllib.request.urlopen(request, timeout=3) as response:
        return response.status, response.headers, response.read()


def ready(url, attempts=60):
    for _ in range(attempts):
        try:
            return fetch(url)
        except (OSError, urllib.error.URLError):
            time.sleep(0.1)
    raise RuntimeError(f"not ready: {url}")


def tcp_counters():
    result = {}
    for path, prefix, keys in [
        ("/proc/net/netstat", "TcpExt:", ["ListenDrops", "ListenOverflows", "TCPAbortOnMemory", "TCPTimeouts", "TCPMemoryPressures"]),
        ("/proc/net/snmp", "Tcp:", ["RetransSegs"]),
    ]:
        rows = [line.split()[1:] for line in Path(path).read_text().splitlines() if line.startswith(prefix)]
        counters = dict(zip(rows[0], map(int, rows[1])))
        result.update({key: counters[key] for key in keys if key in counters})
    return result


def smoke(origin, proxy):
    full = "/media/0.ts"
    origin_status, origin_headers, origin_body = fetch(origin + full)
    status, headers, body = ready(proxy + full)
    assert origin_status == status == 200 and len(body) == 3_000_000
    assert headers.get("X-Origin-Request") and headers.get("Content-Type") == origin_headers.get("Content-Type")
    assert hashlib.sha256(body).digest() == hashlib.sha256(origin_body).digest()
    status, headers, body = fetch(proxy + full, headers={"Range": "bytes=100-1099"})
    assert status == 206 and headers["Content-Range"] == "bytes 100-1099/3000000"
    assert body == origin_body[100:1100] and headers.get("Content-Type") == origin_headers.get("Content-Type")
    status, headers, body = fetch(proxy + full, method="HEAD")
    assert status == 200 and headers.get("Content-Length") == "3000000" and body == b"" and headers.get("Content-Type") == origin_headers.get("Content-Type")


def sample(pid):
    raw = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
    status = Path(f"/proc/{pid}/status").read_text().splitlines()
    rss = next((int(row.split()[1]) for row in status if row.startswith("VmRSS:")), 0)
    return {
        "user_ticks": int(raw[11]),
        "sys_ticks": int(raw[12]),
        "rss_mib": rss / 1024,
    }


def stat_report(samples, begin, end, name):
    valid = [(stamp, stats[name]) for stamp, stats in samples if name in stats]
    a_stamp, a = min(valid, key=lambda r: abs(r[0] - begin))
    b_stamp, b = min(valid, key=lambda r: abs(r[0] - end))
    seconds = (b_stamp - a_stamp) / 1e9
    scale = 100 / os.sysconf("SC_CLK_TCK") / seconds
    rss = [record["rss_mib"] for stamp, record in valid if begin <= stamp <= end]
    return {
        "cpu_pct": round((b["user_ticks"] + b["sys_ticks"] - a["user_ticks"] - a["sys_ticks"]) * scale, 2),
        "cpu_system_pct": round((b["sys_ticks"] - a["sys_ticks"]) * scale, 2),
        "rss_mib_median": round(statistics.median(rss), 2),
        "rss_mib_peak_sampled": round(max(rss), 2),
        "samples": len(rss),
        "cpu_window_seconds": round(seconds, 3),
    }


def launch(cpus, program, *args, **options):
    env = {**os.environ, "GOMAXPROCS": str(len(cpus.split(",")))}
    return subprocess.Popen(["taskset", "-c", cpus, program, *args], env=env, **options)


def stop(proc):
    if proc and proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=4)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=4)


def case(args, origin_proc, origin, mode, viewers, trial, out):
    proxy_proc = load_proc = None
    address = f"127.0.0.1:{free_port()}"
    url = f"http://{address}"
    log_path = out / f"{trial}-{mode}-{viewers}.log"
    with log_path.open("wb") as log:
        try:
            if mode == "wing-256":
                env = {**os.environ, "GOMAXPROCS": str(len(args.proxy_cpu.split(","))),
                       "TIINNO_BENCH_WING_COPY_256": "1"}
                proxy_proc = subprocess.Popen(
                    ["taskset", "-c", args.proxy_cpu, "./go-bench-size", "wing", address, origin],
                    env=env, stdout=log, stderr=log)
            elif mode in ("fiber", "pooled"):
                proxy_proc = launch(args.proxy_cpu, "./go-proxy", mode, address, origin, stdout=log, stderr=log)
            elif mode in ACTIX_MODES:
                executable, *opts = ACTIX_MODES[mode]
                proxy_proc = launch(args.proxy_cpu, executable, address, origin, *opts, stdout=log, stderr=log)
            else:
                raise RuntimeError(f"unknown backend: {mode}")
            smoke(origin, url)
            if args.preflight:
                return verify_reuse(address, origin)
            counter_url = url if mode == "actix-ram" else origin
            before = int(fetch(counter_url + "/ready")[2])
            cores_before = core_times()
            affinity = sorted(os.sched_getaffinity(proxy_proc.pid))
            assert affinity == sorted(map(int, args.proxy_cpu.split(",")))
            tcp_before = tcp_counters()
            load_args = ["load", url, str(viewers), str(args.warm), str(args.cycles)]
            load_proc = launch(args.load_cpu, "./go-bench", *load_args, stdout=subprocess.PIPE, stderr=log)
            processes = {"origin": origin_proc, "load": load_proc}
            if proxy_proc:
                processes["proxy"] = proxy_proc
            samples = []
            diagnostic = []
            start_load = os.getloadavg()
            while load_proc.poll() is None:
                stats = {}
                stamp = time.time_ns()
                for name, proc in processes.items():
                    try:
                        stats[name] = sample(proc.pid)
                    except (FileNotFoundError, ProcessLookupError, IndexError):
                        pass
                samples.append((stamp, stats))
                if args.diagnostic and proxy_proc and len(samples) % 10 == 1:
                    try:
                        diagnostic.append((stamp, worker_ticks(proxy_proc.pid), tcp_queues(address.rsplit(":",1)[1],origin.rsplit(":",1)[1])))
                    except (FileNotFoundError, ProcessLookupError, subprocess.CalledProcessError, ValueError):
                        pass
                time.sleep(0.2)
            if args.diagnostic:
                (out/f"{trial}-{mode}-diagnostics.json").write_text(json.dumps(diagnostic))
            result = json.loads(load_proc.communicate(timeout=3)[0])
            if load_proc.returncode != 0:
                raise RuntimeError(f"load exited with {load_proc.returncode}")
            tcp_after = tcp_counters()
            cores_after = core_times()
            with (out / f"{trial}-{mode}-samples.json").open("w") as raw:
                json.dump(samples, raw)
            if mode != "actix-ram":
                stop(proxy_proc)
            try:
                after = int(ready(counter_url + "/ready", attempts=3)[2])
            except RuntimeError:
                after = None
            begin, end = result.pop("measure_start_ns"), result.pop("measure_end_ns")
            result.update({
                "trial": trial, "stack": mode,
                "origin_hits_including_warm": after - before if after is not None else None,
                "guest_tcp_delta_including_warm": {key: tcp_after[key] - tcp_before[key] for key in tcp_before},
                "expected_hits_including_warm": viewers * (args.warm + args.cycles),
                "target_gbps": viewers * 4 / 1000,
                "actual_gbps": result["bytes"] * 8 / (6 * args.cycles) / 1e9,
                "processes": {name: stat_report(samples, begin, end, name) for name in processes},
                "warm_seconds": 6 * args.warm, "measure_seconds": 6 * args.cycles,
                "cpu_affinity": {"proxy": args.proxy_cpu, "load": args.load_cpu, "origin": args.origin_cpu},
                "load_average_start": start_load, "load_average_end": os.getloadavg(),
                "measure_start_ns": begin, "measure_end_ns": end,
                "proxy_affinity_observed": affinity,
                "core_ticks_including_warm": {k: [b - a for a, b in zip(v, cores_after[k])]
                                               for k, v in cores_before.items()},
                "expected_attempts": viewers * args.cycles,
            })
            result["pass"] = (
                result["bad"] == 0 and result["p99_ms"] <= 1000
                and result["duplicate_origin_ids"] == 0 and result["origin_ids"] == result["attempts"]
                and result["origin_hits_including_warm"] == result["expected_hits_including_warm"]
                and result["p99_dispatch_ms"] <= 20
                and result["attempts"] == result["ok"] == result["expected_attempts"]
                and result["status"] == {"200":result["expected_attempts"]}
                and result["bytes"] == result["attempts"] * 3_000_000
                and result["connections_reused"] >= 0.99 * result["attempts"]
            )
            with (out / "results.jsonl").open("a") as results:
                print(json.dumps(result), file=results)
            stats = result["processes"].get("proxy", result["processes"]["origin"])
            print(
                f"{trial} {mode:10} {viewers:5} | pass={result['pass']:5} "
                f"p99={result['p99_ms']:7.1f}ms bad={result['bad']:5} "
                f"cpu={stats['cpu_pct']:6.1f}% rss={stats['rss_mib_median']:7.1f}MiB "
                f"origin={result['processes']['origin']['cpu_pct']:6.1f}% "
                f"load={result['processes']['load']['cpu_pct']:6.1f}% "
                f"actual={result['actual_gbps']:5.2f}Gbps "
                f"dispatch-p99={result['p99_dispatch_ms']:5.1f}ms "
                f"upstream={result['origin_hits_including_warm']}/{result['expected_hits_including_warm']}",
                flush=True,
            )
            return result
        finally:
            stop(load_proc)
            stop(proxy_proc)


def verify_reuse(address, origin):
    origin_status, origin_headers, origin_body = fetch(origin + "/media/0.ts")
    assert origin_status == 200
    conn = http.client.HTTPConnection(address, timeout=8)
    rows = []
    original_socket = None
    try:
        for method, range_, status, expected in [
            ("GET", None, 200, origin_body),
            ("GET", "bytes=100-1099", 206, origin_body[100:1100]),
            ("GET", None, 200, origin_body),
        ]:
            if not rows:
                conn.connect()
                original_socket = conn.sock
            else:
                # Replay the fixture cadence across the same downstream socket.
                time.sleep(6.05 if len(rows) == 1 else 0)
            conn.request(method, "/media/0.ts", headers={"Range": range_} if range_ else {})
            response = conn.getresponse()
            body = response.read()
            observed = {
                "method": method, "status": response.status,
                "content_length": response.getheader("Content-Length"),
                "content_range": response.getheader("Content-Range"),
                "content_type": response.getheader("Content-Type"),
                "body_bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
                "same_socket": conn.sock is original_socket, "will_close": response.will_close,
            }
            print("preflight", observed, flush=True)
            assert response.status == status and body == expected, observed
            assert observed["content_length"] == ("1000" if range_ else "3000000"), observed
            assert observed["content_range"] == ("bytes 100-1099/3000000" if range_ else None), observed
            assert observed["content_type"] == origin_headers["Content-Type"], observed
            assert response.getheader("X-Origin-Request"), observed
            assert observed["same_socket"] and not response.will_close, observed
            rows.append(observed)
    finally:
        conn.close()
    return rows


def core_times():
    return {parts[0]: list(map(int, parts[1:9])) for line in Path("/proc/stat").read_text().splitlines()
            if (parts := line.split()) and parts[0].startswith("cpu") and parts[0][3:].isdigit()}


def terminate_handler(signum, frame):
    raise SystemExit(128 + signum)


def main():
    signal.signal(signal.SIGTERM, terminate_handler)
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--pair", action="append", required=True, help="stack:viewer_count")
    parser.add_argument("--rounds", type=int, default=1)
    parser.add_argument("--preflight", action="store_true")
    parser.add_argument("--diagnostic", action="store_true")
    parser.add_argument("--cycles", type=int, default=3)
    parser.add_argument("--warm", type=int, default=1)
    parser.add_argument("--proxy-cpu", default="2,4")
    parser.add_argument("--origin-cpu", default="3,5,7")
    parser.add_argument("--load-cpu", default="0,1,6")
    parser.add_argument("--label", default="trial")
    args = parser.parse_args()
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=False)
    print(platform.platform(), "idle load:", os.getloadavg(), flush=True)
    cases = [(mode, int(viewers)) for mode, viewers in (pair.split(":") for pair in args.pair)]
    for i in range(args.rounds):
        if len(cases) == 3 and args.rounds == 5:
            order = ((0, 1, 2), (1, 2, 0), (2, 0, 1), (2, 1, 0), (1, 0, 2))
            ordered = [cases[j] for j in order[i]]
        else:
            ordered = cases
        for mode, viewers in ordered:
            trial = f"{args.label}{i + 1:02}"
            origin_address = f"127.0.0.1:{free_port()}"
            origin = "http://" + origin_address
            with (out / f"{trial}-{mode}-{viewers}-origin.log").open("wb") as log:
                origin_proc = launch(args.origin_cpu, "./go-bench", "origin", origin_address, stdout=log, stderr=log)
                try:
                    ready(origin + "/ready")
                    result = case(args, origin_proc, origin, mode, viewers, trial, out)
                    if args.preflight:
                        (out / f"{trial}-{mode}-preflight.json").write_text(json.dumps(result))
                finally:
                    stop(origin_proc)
            time.sleep(3)


if __name__ == "__main__":
    main()
