"""Add the two missing matched-rate permutations per profile."""
import fcntl
import hashlib
import itertools
import json
import os
from pathlib import Path
import signal
import statistics
import subprocess
import sys
import time

import run_balanced as capacity
import run_matched_rate as matched

ROOT = capacity.ROOT
OUTPUT = ROOT / "matched-extension-results"
MANIFEST = ROOT / "matched-extension-manifest.json"
FILES = ["harness/run_matched_extension.py", "harness/MATCHED-EXTENSION.md", "tools/wrk2/wrk"]

def extension_schedule():
    arms = list(capacity.BINARIES)
    entries = []
    for profile in [4, 8]:
        for round_no, order in [(5, (1, 0, 2)), (6, (2, 0, 1))]:
            for position, index in enumerate(order, 1):
                entries.append(dict(sequence=len(entries)+1, profile=profile, fields=30,
                    round=round_no, position=position, arm=arms[index]))
    return entries

def file_hashes(path):
    return {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(path.rglob("*")) if p.is_file()}

def freeze_or_read(freeze):
    fixed = matched.freeze_or_read(False)
    original = ROOT / "matched-rate-results"
    complete = json.loads((original / "complete.json").read_text())
    if (original / "failure.json").exists() or complete.get("complete") is not True or complete.get("measurements") != 24:
        raise RuntimeError("Complete original matched-rate evidence required")
    files = {name: hashlib.sha256((ROOT/name).read_bytes()).hexdigest() for name in FILES}
    value = dict(original_matched_manifest_sha256=hashlib.sha256(matched.MANIFEST.read_bytes()).hexdigest(),
        original_evidence_sha256=file_hashes(original), files=files,
        offered_rates=fixed["offered_rates"], added_orders=["fiber,kruda,actix", "actix,kruda,fiber"],
        profiles=[4,8], fields=30, added_measurements=12,
        resulting_design="all six permutations per profile; each position twice and each pair direction 3/3")
    if freeze:
        with MANIFEST.open("x") as handle: handle.write(json.dumps(value,indent=2,allow_nan=False)+"\n")
    elif json.loads(MANIFEST.read_text()) != value:
        raise RuntimeError("Extension manifest or original evidence changed")
    return value

def combined_summary(extension_rows):
    original = json.loads((ROOT/"matched-rate-results/summary.json").read_text())["rows"]
    rows = original + extension_rows
    arms = list(capacity.BINARIES)
    for profile in [4,8]:
        selected = [r for r in rows if r["profile"] == profile]
        if len(selected) != 18: raise RuntimeError("Expected 18 combined samples per profile")
        orders=[]
        for round_no in range(1,7):
            ordered=sorted((r for r in selected if r["round"]==round_no),key=lambda r:r["position"])
            if len(ordered)!=3: raise RuntimeError("Incomplete combined round")
            orders.append(tuple(arms.index(r["arm"]) for r in ordered))
        if set(orders) != set(itertools.permutations(range(3))): raise RuntimeError("Combined orders are not all permutations")
        for arm in arms:
            positions=[r["position"] for r in selected if r["arm"]==arm]
            if any(positions.count(p)!=2 for p in [1,2,3]): raise RuntimeError("Combined position imbalance")
        for a,b in itertools.combinations(arms,2):
            pairs=[{r["arm"]:r["position"] for r in selected if r["round"]==n} for n in range(1,7)]
            if sum(p[a]<p[b] for p in pairs)!=3: raise RuntimeError("Combined pair-order imbalance")
    profiles=[]
    for profile in [4,8]:
        selected=[r for r in rows if r["profile"]==profile]
        medians={arm:{metric:statistics.median(r[metric] for r in selected if r["arm"]==arm)
            for metric in ["p50_ms","p90_ms","p99_ms","rps"]} for arm in arms}
        comparisons=[]
        for a,b in itertools.combinations(arms,2):
            pairs=[{r["arm"]:r for r in selected if r["round"]==n} for n in range(1,7)]
            comparisons.append(dict(a=a,b=b,b_p99_wins=sum(p[b]["p99_ms"]<p[a]["p99_ms"] for p in pairs),
                p99_ties=sum(p[b]["p99_ms"]==p[a]["p99_ms"] for p in pairs),
                paired_p99_change_ms=[p[b]["p99_ms"]-p[a]["p99_ms"] for p in pairs]))
        profiles.append(dict(profile=profile,offered_rate=selected[0]["offered_rate"],medians=medians,comparisons=comparisons))
    return dict(rows=rows,profiles=profiles,measurements=36,
        scope="Balanced six-permutation P30 common-rate corrected latency; about 1ms wrk2 granularity")

def main():
    fixed=freeze_or_read(False); entries=extension_schedule(); actual=[]; rows=[]
    server=load=None; log_path=exit_path=None; require_pass=False; allowed=sorted(os.sched_getaffinity(0))
    signal.signal(signal.SIGTERM,lambda a,b:(_ for _ in ()).throw(KeyboardInterrupt(a)))
    signal.signal(signal.SIGINT,lambda a,b:(_ for _ in ()).throw(KeyboardInterrupt(a)))
    with Path(capacity.SHARED_LOCK).open("r+") as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB); OUTPUT.mkdir()
        try:
            (OUTPUT/"frozen-manifest.json").write_bytes(MANIFEST.read_bytes())
            capacity.write_json(OUTPUT/"expected-schedule.json",entries)
            for entry in entries:
                key=f"p{entry['profile']}-r{entry['round']:02d}-{entry['arm']}"
                capacity.require_service_state(capacity.service_state(),True)
                scan=capacity.foreign_benchmarks()
                if scan["known_foreign_benchmarks"] or scan["unreadable_cmdlines"]: raise RuntimeError("Foreign benchmark found")
                env,controlled=capacity.environment(entry); require_pass=entry["arm"]=="kruda"
                command=[str(ROOT/capacity.BINARIES[entry["arm"]]),*(capacity.ARGUMENTS if require_pass else [])]
                current=dict(entry,status="starting",command=command,environment=controlled); actual.append(current)
                capacity.write_json(OUTPUT/"actual-order.json",actual)
                log_path,exit_path=OUTPUT/f"{key}-server.log",OUTPUT/f"{key}-exit.json"
                with log_path.open("w") as log: server=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,env=env)
                deadline=time.monotonic()+7
                while time.monotonic()<deadline:
                    capacity.require_alive(server)
                    try: current["smoke"]=capacity.smoke(30,OUTPUT/f"{key}-smoke.json")
                    except (OSError,capacity.http.client.HTTPException): time.sleep(.05); continue
                    if not any(line.startswith(("composition ","fiber=","actix=")) for line in log_path.read_text().splitlines()): time.sleep(.05); continue
                    current["observed_environment"]=capacity.verify_server(server,entry,controlled,log_path); break
                else: raise RuntimeError("Server readiness timeout")
                current["validation_smoke"]=capacity.smoke(30,OUTPUT/f"{key}-invalid-smoke.json",invalid=True)
                current["server_affinity"]=capacity.affinity(server.pid,allowed); offered=fixed["offered_rates"][str(entry["profile"])]
                path,_=capacity.contract(30); command=[str(matched.WRK2),"--latency","-t4","-c256","-d30s","-R",str(offered),f"http://127.0.0.1:{capacity.PORT}{path}"]
                current.update(status="load",load_command=command); current["server_cpu_before"]=capacity.cpu_snapshot(server.pid)
                with (OUTPUT/f"{key}-wrk2.log").open("w") as log:
                    load=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,env=env)
                    current["client_affinity"]=capacity.affinity(load.pid,allowed); current["client_cpu"]=matched.wait_client(load)
                current["server_cpu_after"]=capacity.cpu_snapshot(server.pid); load=None
                if current["client_cpu"]["returncode"]: raise RuntimeError("wrk2 failed")
                parsed=matched.parse_wrk2((OUTPUT/f"{key}-wrk2.log").read_text(),offered); current["result"]=parsed
                current["server_cpu"]=capacity.cpu_usage(current["server_cpu_before"],current["server_cpu_after"],os.sysconf("SC_CLK_TCK"),parsed)
                capacity.verify_server(server,entry,controlled,log_path); capacity.require_service_state(capacity.service_state(),True)
                current["server_exit"]=capacity.finish_server(server,log_path,exit_path,require_pass); server=None; current["status"]="passed"
                rows.append(dict(entry,offered_rate=offered,**parsed,client_cpu=current["client_cpu"])); capacity.write_json(OUTPUT/"actual-order.json",actual)
                print(json.dumps(rows[-1]),flush=True)
            if len(rows)!=12: raise RuntimeError("Incomplete extension schedule")
            capacity.write_json(OUTPUT/"summary.json",combined_summary(rows))
            capacity.write_json(OUTPUT/"hashes-after.json",freeze_or_read(False))
        except BaseException as error:
            if actual and actual[-1]["status"]!="passed": actual[-1].update(status="failed",error=f"{type(error).__name__}: {error}")
            capacity.write_json(OUTPUT/"failure.json",dict(complete=False,error=f"{type(error).__name__}: {error}")); raise
        finally:
            signal.signal(signal.SIGTERM,signal.SIG_IGN); signal.signal(signal.SIGINT,signal.SIG_IGN); capacity.stop_owned(load)
            if server is not None:
                try:
                    if exit_path is not None and not exit_path.exists(): capacity.finish_server(server,log_path,exit_path,require_pass)
                finally: capacity.stop_owned(server)
            capacity.write_json(OUTPUT/"actual-order.json",actual)
        capacity.write_json(OUTPUT/"complete.json",dict(complete=True,added_measurements=12,combined_measurements=36,
            extension_manifest_sha256=hashlib.sha256(MANIFEST.read_bytes()).hexdigest(),services_restored=False))

if __name__=="__main__":
    if sys.argv[1:]==["freeze"]:
        print(json.dumps(freeze_or_read(True),sort_keys=True))
    elif not sys.argv[1:]:
        main()
    else:
        raise RuntimeError("Usage: run_matched_extension.py [freeze]")
