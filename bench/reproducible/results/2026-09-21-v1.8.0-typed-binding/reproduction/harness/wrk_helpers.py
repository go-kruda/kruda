import re
import subprocess
def stop(process):
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def latency_ms(value):
    match = re.fullmatch(r"([\d.]+)(us|ms|s)", value)
    if not match:
        raise ValueError(f"Invalid latency: {value}")
    return float(match[1]) * {"us": 0.001, "ms": 1, "s": 1000}[match[2]]


def parse_wrk(text):
    rps = re.search(r"Requests/sec:\s+([\d.]+)", text)
    p99 = re.search(r"^\s+99%\s+(\S+)", text, re.M)
    requests = re.search(r"(\d+) requests in ([\d.]+)s", text)
    if not (rps and p99 and requests):
        raise ValueError("Missing wrk measurement")
    socket_errors = re.search(r"Socket errors: connect (\d+), read (\d+), write (\d+), timeout (\d+)", text)
    errors = dict(zip(["connect", "read", "write", "timeout"], map(int, socket_errors.groups()))) if socket_errors else dict.fromkeys(["connect", "read", "write", "timeout"], 0)
    non2xx = re.search(r"Non-2xx or 3xx responses:\s+(\d+)", text)
    errors["non2xx"] = int(non2xx[1]) if non2xx else 0
    if any(errors.values()):
        raise ValueError(f"Invalid benchmark response/errors: {errors}")
    return {"rps": float(rps[1]), "p99_ms": latency_ms(p99[1]), "requests": int(requests[1]), "elapsed_seconds": float(requests[2]), **errors}


