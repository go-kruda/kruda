from collections import Counter
import json
from pathlib import Path
from statistics import median
import sys

root = Path(sys.argv[1])
levels = sys.argv[2:] or ["high"]
names = ("wing-256", "fiber", "actix-tuned")
order = ((0, 1, 2), (1, 2, 0), (2, 0, 1), (2, 1, 0), (1, 0, 2))

def summarize(values):
    values = list(values)
    return f"{median(values):.2f} ({min(values):.2f}–{max(values):.2f})"

def verify_arm(row, viewers, path):
    attempts = viewers * 2
    assert row["viewers"] == viewers
    assert row["attempts"] == row["expected_attempts"] == attempts
    assert row["ok"] + row["bad"] == attempts
    assert sum(row["error_kinds"].values()) == row["bad"]
    assert row["expected_hits_including_warm"] == viewers * 3
    assert row["proxy_affinity_observed"] == [12, 13]
    assert row["cpu_affinity"] == {"proxy":"12,13", "origin":"8,9,10,11", "load":"4,5,6,7"}
    assert row["warm_seconds"] == 6 and row["measure_seconds"] == 12
    expected_pass = (
        row["ok"] == attempts and row["bad"] == 0
        and row["status"] == {"200":attempts} and row["bytes"] == attempts * 3_000_000
        and row["duplicate_origin_ids"] == 0 and row["origin_ids"] == attempts
        and row["origin_hits_including_warm"] == viewers * 3
        and row["connections_reused"] >= 0.99 * attempts
        and row["p99_dispatch_ms"] <= 20 and row["p99_ms"] <= 1000
    )
    assert row["pass"] == expected_pass
    begin, end = row["measure_start_ns"], row["measure_end_ns"]
    assert abs((end - begin)/1e9 - 12) <= 0.05
    samples = json.loads(path.read_text())
    assert set(row["processes"]) == {"proxy", "origin", "load"}
    # Verify the reported values and record the largest endpoint offset.
    max_offset_ms = 0.0
    for name, reported in row["processes"].items():
        seq = [(t, raw[name]) for t, raw in samples if name in raw]
        start, a = min(seq, key=lambda pair: abs(pair[0] - begin))
        stop, b = min(seq, key=lambda pair: abs(pair[0] - end))
        dt = (stop - start) / 1e9
        offset_ms = max(abs(start - begin), abs(stop - end)) / 1e6
        max_offset_ms = max(max_offset_ms, offset_ms)
        assert abs(dt - reported["cpu_window_seconds"]) <= 0.001
        assert abs((b["user_ticks"] + b["sys_ticks"] - a["user_ticks"] - a["sys_ticks"]) / dt
                   - reported["cpu_pct"]) <= 0.011
        rss = [r["rss_mib"] for t, r in seq if begin <= t <= end]
        assert len(rss) == reported["samples"]
        assert abs(median(rss) - reported["rss_mib_median"]) <= 0.011
        assert abs(max(rss) - reported["rss_mib_peak_sampled"]) <= 0.011
    return max_offset_ms

print("# Linux DEV HLS matched comparison")
print("\nAll arms had one 6 s warm cycle and two scored 6 s cycles. "
      "CPU 100% equals one vCPU. RSS is sampled about every 0.2 s. "
      "The p99 column includes passing runs only, so the driver's 1,001 ms penalty "
      "for missed requests stays in raw results. Resource statistics across all arms "
      "describe behavior at the offered load. See same-block comparisons for equal-delivery results.")
for level in levels:
    viewers = {"high": 11_776, "lower": 6_144}[level]
    rows = [json.loads(line) for line in (root / level / "results.jsonl").read_text().splitlines()]
    assert int((root / f"{level}.exit").read_text()) == 0
    expected = [(f"{level}{i + 1:02}", names[j]) for i, group in enumerate(order) for j in group]
    assert [(x["trial"], x["stack"]) for x in rows] == expected
    offsets = {}
    for row in rows:
        key = row["trial"], row["stack"]
        offsets[key] = verify_arm(row, viewers, root / level / ("%s-%s-samples.json" % key))
    stacks = {name: [x for x in rows if x["stack"] == name] for name in names}
    print(f"\n## {viewers:,} simulated viewers, {viewers * .004:.3f} Gbps offered payload")
    print("\n| Stack | Passed | Bad / attempts | p99 ms, passing median (range) | CPU %, median (range) | RSS MiB, median (range) | Peak sampled RSS MiB, max |")
    print("|---|---:|---:|---:|---:|---:|---:|")
    for name, seq in stacks.items():
        passed = [x for x in seq if x['pass']]
        latency = summarize(x["p99_ms"] for x in passed) if passed else "—"
        stats = [x["processes"]["proxy"] for x in seq]
        print(f"| {name} | {len(passed)}/5 | {sum(x['bad'] for x in seq):,} / {sum(x['attempts'] for x in seq):,} | {latency} | "
              f"{summarize(x['cpu_pct'] for x in stats)} | {summarize(x['rss_mib_median'] for x in stats)} | "
              f"{max(x['rss_mib_peak_sampled'] for x in stats):.2f} |")
    print("\n### Same-block comparisons on equal completed work\n")
    for peer in names[1:]:
        pairs = [(a, b) for a, b in zip(stacks[names[0]], stacks[peer]) if a["pass"] and b["pass"]]
        if not pairs:
            print(f"- {peer}. 0/5 jointly passing pairs.")
            continue
        dp99 = [a["p99_ms"] - b["p99_ms"] for a,b in pairs]
        dcpu = [a["processes"]["proxy"]["cpu_pct"] - b["processes"]["proxy"]["cpu_pct"] for a,b in pairs]
        drss = [a["processes"]["proxy"]["rss_mib_median"] - b["processes"]["proxy"]["rss_mib_median"] for a,b in pairs]
        print(f"- {peer}. {len(pairs)}/5 jointly passing pairs. Wing minus peer, median (range), "
              f"for p99 was {summarize(dp99)} ms, for CPU was {summarize(dcpu)} percentage points, and for RSS was {summarize(drss)} MiB. "
              f"Wing lower in {sum(d < 0 for d in dp99)}/{len(pairs)} p99, {sum(d < 0 for d in dcpu)}/{len(pairs)} CPU and "
              f"{sum(d < 0 for d in drss)}/{len(pairs)} RSS pairs.")
    print("\n### Every scored arm\n")
    print("| Block | Stack | Pass | Complete | Bad | p99 ms | CPU % | RSS median / sampled peak MiB | Reuse / offered | Origin hits / expected incl. warm | Dispatch p99 ms | Origin / load CPU % | Max sampled CPU endpoint offset ms | Proxy max steal % |")
    print("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
    for x in rows:
        y = x["processes"]["proxy"]
        cores = x["core_ticks_including_warm"]
        steal = max(100 * cores[f"cpu{i}"][7] / sum(cores[f"cpu{i}"]) for i in (12, 13))
        print(f"| {x['trial']} | {x['stack']} | {x['pass']} | {x['ok']:,} | {x['bad']:,} | {x['p99_ms']:.2f} | {y['cpu_pct']:.2f} | "
              f"{y['rss_mib_median']:.2f} / {y['rss_mib_peak_sampled']:.2f} | {100*x['connections_reused']/x['attempts']:.3f}% | "
              f"{x['origin_hits_including_warm']:,} / {x['expected_hits_including_warm']:,} | {x['p99_dispatch_ms']:.2f} | "
              f"{x['processes']['origin']['cpu_pct']:.2f} / {x['processes']['load']['cpu_pct']:.2f} | "
              f"{offsets[x['trial'],x['stack']]:.1f} | {steal:.3f} |")
    print("\n### Scored request misses\n")
    for mode, seq in stacks.items():
        failures = Counter()
        for x in seq:
            failures.update(x["error_kinds"])
        print(f"- {mode}. {dict(failures) if failures else 'Zero scored misses.'}")
