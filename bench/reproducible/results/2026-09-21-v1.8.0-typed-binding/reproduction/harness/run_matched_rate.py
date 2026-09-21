"""Corrected-latency comparison at a common post-calibration offered rate."""
import csv, fcntl, hashlib, itertools, json, math, os, re, signal, statistics, subprocess, sys, time
from pathlib import Path
import run_balanced as capacity
from wrk_helpers import latency_ms

ROOT = capacity.ROOT
MANIFEST = ROOT / "matched-rate-manifest.json"
WRK2 = ROOT / "tools/wrk2/wrk"
WRK2_SHA = "9c25f03760d80ae11439badd5d57eb2d985c2e7329df4490b4a1e06a8102bc5e"
WRK2_COMMIT = "44a94c17d8e6a0bac8559b53da76848e430cb7a7"
FILES = ["harness/run_matched_rate.py", "harness/MATCHED-RATE.md", "tools/wrk2/wrk"]
WINDOW_FIELDS = {"thread", "calibrations", "valid", "start_complete", "end_complete",
    "start_wall_us", "end_wall_us", "start_mono_ns", "end_mono_ns",
    "start_clock_valid", "end_clock_valid"}

def matched_schedule():
    arms = list(capacity.BINARIES)
    base = [(0,1,2), (0,2,1), (1,2,0), (2,1,0)]
    entries = []
    for profile, orders in [(4, base), (8, [tuple(reversed(x)) for x in base])]:
        for round_no, order in enumerate(orders, 1):
            for position, index in enumerate(order, 1):
                entries.append(dict(sequence=len(entries)+1, profile=profile, fields=30,
                    round=round_no, position=position, arm=arms[index]))
        selected = [e for e in entries if e["profile"] == profile]
        for a, b in itertools.combinations(arms, 2):
            pairs = [{e["arm"]: e["position"] for e in selected if e["round"] == n} for n in range(1,5)]
            if sum(p[a] < p[b] for p in pairs) != 2:
                raise RuntimeError("Matched-rate pair order is not balanced")
    return entries

def parse_wrk2(text, offered_rate):
    def one(pattern, source=text):
        found = re.findall(pattern, source, re.M)
        if len(found) != 1: raise RuntimeError(f"Expected one wrk2 record: {pattern}")
        return found[0]
    heading = "Latency Distribution (HdrHistogram - Recorded Latency)"
    if text.count(heading) != 1 or "Uncorrected Latency" in text:
        raise RuntimeError("Expected only corrected Recorded Latency histogram")
    section = text.split(heading,1)[1].split("----------------------------------------------------------",1)[0]
    if "Detailed Percentile spectrum:" not in section or "TotalCount" not in section:
        raise RuntimeError("Missing raw HDR percentile spectrum")
    p50 = latency_ms(one(r"^\s*50\.000%\s+(\S+)\s*$", section))
    p90 = latency_ms(one(r"^\s*90\.000%\s+(\S+)\s*$", section))
    p99 = latency_ms(one(r"^\s*99\.000%\s+(\S+)\s*$", section))
    histogram_count = int(one(r"Total count\s*=\s*(\d+)\]", section))
    count, duration = one(r"^\s*(\d+) requests in ([\d.]+s), .+B read\s*$")
    requests, elapsed = int(count), latency_ms(duration)/1000
    full_rps = float(one(r"^Requests/sec:\s+([\d.]+)\s*$"))
    calibrations = re.findall(r"^\s*Thread calibration: mean lat\.: ([\d.]+)ms, rate sampling interval: (\d+)ms\s*$", text, re.M)
    if len(calibrations) != 4 or any(float(a)<=0 or int(b)<=0 for a,b in calibrations):
        raise RuntimeError("Expected four completed thread calibrations")
    windows = []
    for raw in re.findall(r"^WRK2_THREAD_WINDOW (.+)$", text, re.M):
        values = {}
        for token in raw.split():
            key, sep, value = token.partition("=")
            if not sep or key not in WINDOW_FIELDS or key in values: raise RuntimeError("Malformed thread window")
            values[key] = int(value)
        if set(values) != WINDOW_FIELDS: raise RuntimeError("Incomplete thread window")
        if any(values[k] != 1 for k in ["calibrations","valid","start_clock_valid","end_clock_valid"]):
            raise RuntimeError("Invalid calibration/window clock state")
        completed = values["end_complete"] - values["start_complete"]
        mono = (values["end_mono_ns"] - values["start_mono_ns"])/1e9
        wall = (values["end_wall_us"] - values["start_wall_us"])/1e6
        if completed <= 0 or mono < 15 or wall <= 0: raise RuntimeError("Invalid post-calibration window")
        values.update(completed=completed, monotonic_seconds=mono, wall_seconds=wall,
            rate=completed/mono, clock_delta_seconds=wall-mono)
        windows.append(values)
    if len(windows) != 4 or sorted(x["thread"] for x in windows) != list(range(4)):
        raise RuntimeError("Expected four distinct zero-based thread windows")
    achieved, completed = sum(x["rate"] for x in windows), sum(x["completed"] for x in windows)
    sockets = re.findall(r"Socket errors: connect (\d+), read (\d+), write (\d+), timeout (\d+)", text)
    if len(sockets)>1 or ("Socket errors:" in text and not sockets): raise RuntimeError("Malformed socket errors")
    errors = dict(zip(["connect","read","write","timeout"], map(int,sockets[0]))) if sockets else dict.fromkeys(["connect","read","write","timeout"],0)
    statuses = re.findall(r"Non-2xx or 3xx responses:\s+(\d+)", text)
    if len(statuses)>1 or ("Non-2xx" in text and not statuses): raise RuntimeError("Malformed status errors")
    errors["non2xx"] = int(statuses[0]) if statuses else 0
    metrics = [p50,p90,p99,full_rps,achieved,elapsed]
    if any(errors.values()) or min(requests,histogram_count,*metrics)<=0 or not all(math.isfinite(x) for x in metrics):
        raise RuntimeError(f"Invalid counts/latency/errors: {errors}")
    if not 29.95<=elapsed<=30.5 or not .99<=achieved/offered_rate<=1.01:
        raise RuntimeError("Invalid duration or post-calibration achieved rate")
    if abs(histogram_count-completed)>256: raise RuntimeError("Histogram/window count exceeds connection bound")
    return dict(rps=achieved, full_invocation_rps=full_rps, p50_ms=p50, p90_ms=p90, p99_ms=p99,
        requests=requests, elapsed_seconds=elapsed, post_calibration_completions=completed,
        corrected_histogram_count=histogram_count, thread_windows=windows,
        achieved_rate_fraction=achieved/offered_rate, **errors)

def capacity_evidence():
    content, _, expected = capacity.read_manifest(); capacity.verify_hashes(expected)
    path = ROOT/"controlled-results"; complete=json.loads((path/"complete.json").read_text())
    if (path/"failure.json").exists() or complete.get("complete") is not True or complete.get("measurements") != 36:
        raise RuntimeError("Complete fresh capacity results required")
    if (path/"frozen-manifest.json").read_bytes()!=content: raise RuntimeError("Capacity manifest changed")
    for name in ["hashes-before.json","hashes-after.json"]:
        if json.loads((path/name).read_text()) != expected: raise RuntimeError("Capacity hashes differ")
    with (path/"summary.csv").open() as handle: rows=list(csv.DictReader(handle))
    wanted=capacity.schedule(True)
    if [{k:r[k] for k in wanted[0]} for r in rows] != [{k:str(v) for k,v in e.items()} for e in wanted]:
        raise RuntimeError("Capacity schedule differs")
    medians, rates = {}, {}
    for profile in [4,8]:
        medians[str(profile)]={}
        for arm in capacity.BINARIES:
            selected=[r for r in rows if r["profile"]==str(profile) and r["fields"]=="30" and r["arm"]==arm]
            values=[]
            for row in selected:
                if row["server_exit_code"]!="0" or row["server_forced_kill"]!="False" or (row["server_pass_required"]=="True" and row["server_final_pass"]!="True"):
                    raise RuntimeError("Invalid capacity exit")
                raw=path/"raw"/f"p{profile}-f30-r{int(row['round']):02d}-{arm}-measure.log"
                parsed=capacity.parse_wrk(raw.read_text())
                if parsed["rps"] != float(row["rps"]): raise RuntimeError("Capacity raw/CSV mismatch")
                values.append(parsed["rps"])
            if len(values)!=6: raise RuntimeError("Expected six capacity samples per arm/profile")
            medians[str(profile)][arm]=statistics.median(values)
        rates[str(profile)]=math.floor(min(medians[str(profile)].values())*.5/1000)*1000
        if rates[str(profile)]<=0: raise RuntimeError("Invalid derived rate")
    evidence={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(path.rglob("*")) if p.is_file()}
    return content,medians,rates,evidence

def freeze_or_read(freeze):
    content,medians,rates,evidence=capacity_evidence()
    files={name:hashlib.sha256((ROOT/name).read_bytes()).hexdigest() for name in FILES}
    if files["tools/wrk2/wrk"]!=WRK2_SHA: raise RuntimeError("Unexpected patched wrk2 binary")
    value=dict(capacity_manifest_sha256=hashlib.sha256(content).hexdigest(),capacity_evidence_sha256=evidence,
        files=files,wrk2_base_commit=WRK2_COMMIT,wrk2_binary_sha256=WRK2_SHA,
        wrk2_provenance="Upstream base plus per-thread post-calibration window metadata patch",
        capacity_median_rps=medians,offered_rates=rates,
        rate_rule="per profile: floor(0.5 * slowest fresh P30 median RPS / 1000) * 1000",
        fields=30,profiles=[4,8],duration_seconds=30,wrk_threads=4,connections=256,rounds=4,
        achieved_rate_fraction_bounds=[.99,1.01],minimum_post_calibration_window_seconds=15,
        histogram_window_count_tolerance=256)
    if freeze:
        with MANIFEST.open("x") as handle: handle.write(json.dumps(value,indent=2,allow_nan=False)+"\n")
    elif json.loads(MANIFEST.read_text())!=value: raise RuntimeError("Matched manifest differs")
    return value

def wait_client(process):
    started=time.monotonic()
    while time.monotonic()-started<40:
        pid,status,usage=os.wait4(process.pid,os.WNOHANG)
        if pid:
            process.returncode=os.waitstatus_to_exitcode(status); elapsed=time.monotonic()-started
            total=usage.ru_utime+usage.ru_stime
            return dict(returncode=process.returncode,user_cpu_seconds=usage.ru_utime,system_cpu_seconds=usage.ru_stime,
                cpu_seconds=total,observed_elapsed_seconds=elapsed,average_cpu_cores=total/elapsed,
                fraction_of_four_worker_cores=total/elapsed/4,max_resident_kib=usage.ru_maxrss)
        time.sleep(.05)
    raise RuntimeError("wrk2 exceeded 40-second deadline")

def main():
    if sys.platform!="linux" or sys.argv[1:] not in [["freeze"],["run"]]:
        raise RuntimeError("Usage: run_matched_rate.py freeze|run")
    with Path(capacity.SHARED_LOCK).open("r+") as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB); fixed=freeze_or_read(sys.argv[1]=="freeze")
        if sys.argv[1]=="freeze": print(json.dumps({"offered_rates":fixed["offered_rates"]}),flush=True); return
        fixed_bytes=MANIFEST.read_bytes(); output=ROOT/"matched-rate-results"; output.mkdir()
        entries=matched_schedule(); actual=[]; rows=[]; server=load=None; log_path=exit_path=None; require_pass=False
        allowed=sorted(os.sched_getaffinity(0))
        signal.signal(signal.SIGTERM,lambda a,b:(_ for _ in ()).throw(KeyboardInterrupt(a)))
        signal.signal(signal.SIGINT,lambda a,b:(_ for _ in ()).throw(KeyboardInterrupt(a)))
        try:
            (output/"frozen-manifest.json").write_bytes(fixed_bytes); capacity.write_json(output/"expected-schedule.json",entries)
            capacity.write_json(output/"hashes-before.json",fixed)
            for entry in entries:
                key=f"p{entry['profile']}-r{entry['round']:02d}-{entry['arm']}"
                state=capacity.service_state(); capacity.require_service_state(state,True)
                scan=capacity.foreign_benchmarks()
                if scan["known_foreign_benchmarks"] or scan["unreadable_cmdlines"]: raise RuntimeError("Foreign benchmark found")
                env,controlled=capacity.environment(entry); require_pass=entry["arm"]=="kruda"
                command=[str(ROOT/capacity.BINARIES[entry["arm"]]),*(capacity.ARGUMENTS if require_pass else [])]
                current=dict(entry,status="starting",command=command,environment=controlled); actual.append(current)
                capacity.write_json(output/"actual-order.json",actual)
                log_path,exit_path=output/f"{key}-server.log",output/f"{key}-exit.json"
                with log_path.open("w") as log: server=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,env=env)
                deadline=time.monotonic()+7
                while time.monotonic()<deadline:
                    capacity.require_alive(server)
                    try: current["smoke"]=capacity.smoke(30,output/f"{key}-smoke.json")
                    except (OSError,capacity.http.client.HTTPException): time.sleep(.05); continue
                    if not any(line.startswith(("composition ","fiber=","actix=")) for line in log_path.read_text().splitlines()): time.sleep(.05); continue
                    current["observed_environment"]=capacity.verify_server(server,entry,controlled,log_path); break
                else: raise RuntimeError("Server readiness timeout")
                current["validation_smoke"]=capacity.smoke(30,output/f"{key}-invalid-smoke.json",invalid=True)
                current["server_affinity"]=capacity.affinity(server.pid,allowed); offered=fixed["offered_rates"][str(entry["profile"])]
                path,_=capacity.contract(30); command=[str(WRK2),"--latency","-t4","-c256","-d30s","-R",str(offered),f"http://127.0.0.1:{capacity.PORT}{path}"]
                current.update(status="load",load_command=command); current["server_cpu_before"]=capacity.cpu_snapshot(server.pid)
                with (output/f"{key}-wrk2.log").open("w") as log:
                    load=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,env=env)
                    current["client_affinity"]=capacity.affinity(load.pid,allowed); current["client_cpu"]=wait_client(load)
                current["server_cpu_after"]=capacity.cpu_snapshot(server.pid); load=None
                if current["client_cpu"]["returncode"]: raise RuntimeError("wrk2 failed")
                parsed=parse_wrk2((output/f"{key}-wrk2.log").read_text(),offered); current["result"]=parsed
                current["server_cpu"]=capacity.cpu_usage(current["server_cpu_before"],current["server_cpu_after"],os.sysconf("SC_CLK_TCK"),parsed)
                capacity.verify_server(server,entry,controlled,log_path); capacity.require_service_state(capacity.service_state(),True)
                current["server_exit"]=capacity.finish_server(server,log_path,exit_path,require_pass); server=None; current["status"]="passed"
                rows.append(dict(entry,offered_rate=offered,**parsed,client_cpu=current["client_cpu"])); capacity.write_json(output/"actual-order.json",actual)
                print(json.dumps(rows[-1]),flush=True)
            if len(rows)!=24 or [{k:r[k] for k in entries[0]} for r in rows]!=entries: raise RuntimeError("Incomplete matched schedule")
            profiles=[]
            for profile in [4,8]:
                selected=[r for r in rows if r["profile"]==profile]
                medians={arm:{m:statistics.median(r[m] for r in selected if r["arm"]==arm) for m in ["p50_ms","p90_ms","p99_ms","rps"]} for arm in capacity.BINARIES}
                profiles.append(dict(profile=profile,offered_rate=selected[0]["offered_rate"],medians=medians))
            capacity.write_json(output/"summary.json",dict(rows=rows,profiles=profiles,
                scope="P30 common-rate corrected latency at 50% of slowest fresh capacity median per profile; about 1ms wrk2 granularity"))
            capacity.write_json(output/"hashes-after.json",freeze_or_read(False))
        except BaseException as error:
            if actual and actual[-1]["status"]!="passed": actual[-1].update(status="failed",error=f"{type(error).__name__}: {error}")
            capacity.write_json(output/"failure.json",dict(complete=False,error=f"{type(error).__name__}: {error}")); raise
        finally:
            signal.signal(signal.SIGTERM,signal.SIG_IGN); signal.signal(signal.SIGINT,signal.SIG_IGN); capacity.stop_owned(load)
            if server is not None:
                try:
                    if exit_path is not None and not exit_path.exists(): capacity.finish_server(server,log_path,exit_path,require_pass)
                finally: capacity.stop_owned(server)
            capacity.write_json(output/"actual-order.json",actual)
        capacity.write_json(output/"complete.json",dict(complete=True,measurements=24,
            matched_manifest_sha256=hashlib.sha256(fixed_bytes).hexdigest(),services_restored=False))

if __name__=="__main__": main()
