from collections import Counter
import json
from pathlib import Path
from statistics import median
import sys

ROOT = Path(sys.argv[1])
VIEWERS = 11_776
SEGMENT_BYTES = 3_000_000
WARM_SECONDS = 6
MEASURE_SECONDS = 60
STACKS = ("wing-256", "fiber", "actix-tuned")
BLOCKS = ((0, 1, 2), (1, 2, 0), (2, 0, 1), (2, 1, 0), (1, 0, 2))


def spread(values):
    values = list(values)
    return f"{median(values):.2f} ({min(values):.2f}–{max(values):.2f})"


def verify_phase(phase, attempts):
    assert phase["attempts"] == attempts
    assert phase["ok"] + phase["bad"] == attempts
    assert sum(phase["error_kinds"].values()) == phase["bad"]
    assert sum(phase["status"].values()) == attempts
    assert phase["origin_ids"] + phase["duplicate_origin_ids"] == phase["ok"]
    assert 0 <= phase["connections_reused"] <= attempts
    assert phase["error_kinds"].get("client inflight guard", 0) <= phase["status"].get("0", 0)
    return (
        phase["ok"] == attempts
        and phase["bad"] == phase["duplicate_origin_ids"] == 0
        and phase["status"] == {"200": attempts}
        and phase["bytes"] == attempts * SEGMENT_BYTES
        and phase["origin_ids"] == attempts
    )


def verify(row):
    key = row["trial"], row["stack"]
    assert row["viewers"] == VIEWERS and row["max_inflight"] == 512, key
    assert row["warm_seconds"] == WARM_SECONDS and row["measure_seconds"] == MEASURE_SECONDS, key
    assert row["cpu_affinity"] == {"proxy": "12,13", "load": "4,5,6,7", "origin": "8,9,10,11"}, key
    assert row["proxy_affinity_observed"] == [12,13], key
    assert row["expected_attempts"] == VIEWERS * 10, key
    assert row["expected_hits_including_warm"] == VIEWERS * 11, key
    warm, begin, end = (row[n] for n in ("warm_start_ns", "measure_start_ns", "measure_end_ns"))
    assert (begin-warm, end-begin) == (WARM_SECONDS*1_000_000_000, MEASURE_SECONDS*1_000_000_000), key

    warm_pass = verify_phase(row["warm"], VIEWERS)
    measured_pass = (
        verify_phase(row, VIEWERS * 10)
        and row["connections_reused"] >= 0.99 * row["attempts"]
        and row["p99_dispatch_ms"] <= 20
        and row["p99_ms"] <= 1000
    )
    id_count = row["unique_origin_ids_all"]
    assert max(row["warm"]["origin_ids"], row["origin_ids"]) <= id_count <= row["warm"]["origin_ids"] + row["origin_ids"], key
    passed = (
        warm_pass and measured_pass
        and id_count == row["origin_hits_including_warm"] == row["expected_hits_including_warm"]
    )
    assert (row["warm_pass"], row["measured_pass"], row["pass"]) == (warm_pass, measured_pass, passed), key
    assert abs(row["actual_gbps"] - row["bytes"] * 8 / MEASURE_SECONDS / 1e9) < 1e-9, key

    samples = json.loads((ROOT / "score" / ("%s-%s-samples.json" % key)).read_text())
    assert set(row["processes"]) == {"proxy", "load", "origin"}, key
    offsets = []
    for name, metrics in row["processes"].items():
        readings = [(t, snapshot[name]) for t, snapshot in samples if name in snapshot]
        a_time, a = min(readings, key=lambda item: abs(item[0] - begin))
        b_time, b = min(readings, key=lambda item: abs(item[0] - end))
        offsets.append(max(abs(a_time - begin), abs(b_time - end)) / 1e6)
        interval = (b_time-a_time)/1e9
        assert abs(interval - metrics["cpu_window_seconds"]) <= 0.001, key
        user = b["user_ticks"] - a["user_ticks"]
        system = b["sys_ticks"] - a["sys_ticks"]
        # Host getconf CLK_TCK = 100, so ticks/seconds gives CPU percent.
        assert abs((user+system)/interval - metrics["cpu_pct"]) <= 0.011, key
        assert abs(system/interval - metrics["cpu_system_pct"]) <= 0.011, key
        rss = [reading["rss_mib"] for stamp, reading in readings if begin <= stamp <= end]
        assert metrics["samples"] == len(rss)
        assert abs(metrics["rss_mib_median"] - median(rss)) <= 0.011, key
        assert abs(metrics["rss_mib_peak_sampled"] - max(rss)) <= 0.011, key
    return max(offsets)


rows = [json.loads(line) for line in (ROOT / "score/results.jsonl").read_text().splitlines()]
assert (ROOT / "score.exit").read_text().strip() == "0"
expected_order = [(f"block{i+1:02}", STACKS[j]) for i, order in enumerate(BLOCKS) for j in order]
assert [(row["trial"], row["stack"]) for row in rows] == expected_order
offsets = {(row["trial"], row["stack"]): verify(row) for row in rows}
stacks = {stack: [row for row in rows if row["stack"] == stack] for stack in STACKS}

print("# Linux DEV 60-second HLS proxy comparison\n")
print("11,776 simulated viewers at 4 Mbps each, 2 pinned vCPUs per proxy, "
      "HTTP/1.1 loopback with the same RAM origin and load driver. Five interleaved blocks of three arms, "
      "each with 6 seconds of warm-up and 60 scored seconds. Outcomes follow scheduled phase, "
      "even if the response completes across a boundary. Process CPU 100% equals 1 vCPU.\n")
print("| Stack | Full / measured passes | Warm bad/attempts | Measured bad/attempts | "
      "p99 ms across passing measured phases | CPU %, median (range) | "
      "RSS median MiB, median (range) | Maximum sampled RSS MiB |")
print("|---|---:|---:|---:|---:|---:|---:|---:|")
for stack, seq in stacks.items():
    proxy = [row["processes"]["proxy"] for row in seq]
    measured_p99 = [row["p99_ms"] for row in seq if row["measured_pass"]]
    latency = spread(measured_p99) if measured_p99 else "—"
    print(f"| {stack} | {sum(row['pass'] for row in seq)}/5 / {len(measured_p99)}/5 | "
          f"{sum(row['warm']['bad'] for row in seq):,}/{sum(row['warm']['attempts'] for row in seq):,} | "
          f"{sum(row['bad'] for row in seq):,}/{sum(row['attempts'] for row in seq):,} | {latency} | "
          f"{spread(row['cpu_pct'] for row in proxy)} | "
          f"{spread(row['rss_mib_median'] for row in proxy)} | "
          f"{max(row['rss_mib_peak_sampled'] for row in proxy):.2f} |")

print("\nResource summaries describe the same offered load. Delivered volume can differ. "
      "For substantive p99 and CPU comparisons use same-block joint full passes only. "
      "Per-arm p99 for any failed measurement can include the driver's cap-miss penalty.\n")
print("## Joint full passes\n")
for peer in STACKS[1:]:
    pairs = [(a, b) for a, b in zip(stacks[STACKS[0]], stacks[peer]) if a["pass"] and b["pass"]]
    if not pairs:
        print(f"- {peer} had no jointly passing blocks with Wing.")
        continue
    dp99 = [a["p99_ms"] - b["p99_ms"] for a,b in pairs]
    dcpu = [a["processes"]["proxy"]["cpu_pct"] - b["processes"]["proxy"]["cpu_pct"] for a,b in pairs]
    print(f"- {peer} had {len(pairs)}/5 jointly passing blocks with Wing. "
          f"Wing minus peer median (range) {spread(dp99)} ms for p99 and {spread(dcpu)} points for CPU.")

print("\n## Measured rounds\n")
print("| Block | Stack | Full / measured pass | Warm dispatched bad / unsent | Measured dispatched bad / unsent | "
      "p99 ms | Proxy CPU % | Median / sampled peak RSS MiB | Measured reuse | "
      "Origin hits incl. warm | Dispatch p99 ms | Origin / driver CPU % | Max CPU endpoint offset ms | Proxy max steal % |")
print("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
for row in rows:
    proxy = row["processes"]["proxy"]
    wguard = row["warm"]["error_kinds"].get("client inflight guard", 0)
    mguard = row["error_kinds"].get("client inflight guard", 0)
    ticks = row["core_ticks_including_warm"]
    max_steal = max(100*ticks[f"cpu{i}"][7]/sum(ticks[f"cpu{i}"]) for i in (12,13))
    print(f"| {row['trial']} | {row['stack']} | {row['pass']} / {row['measured_pass']} | "
          f"{row['warm']['bad']-wguard:,} / {wguard:,} | {row['bad']-mguard:,} / {mguard:,} | "
          f"{row['p99_ms']:.2f} | {proxy['cpu_pct']:.2f} | "
          f"{proxy['rss_mib_median']:.2f} / {proxy['rss_mib_peak_sampled']:.2f} | "
          f"{100*row['connections_reused']/row['attempts']:.3f}% | "
          f"{row['origin_hits_including_warm']:,}/{row['expected_hits_including_warm']:,} | "
          f"{row['p99_dispatch_ms']:.2f} | "
          f"{row['processes']['origin']['cpu_pct']:.2f} / {row['processes']['load']['cpu_pct']:.2f} | "
          f"{offsets[row['trial'],row['stack']]:.1f} | {max_steal:.3f} |")

print("\n## Error categories by scheduled phase\n")
for stack, seq in stacks.items():
    warm = Counter()
    measured = Counter()
    for row in seq:
        warm.update(row["warm"]["error_kinds"])
        measured.update(row["error_kinds"])
    print(f"- {stack} warm {dict(warm) if warm else 'none'}, measured {dict(measured) if measured else 'none'}.")

print("\nSee PROTOCOL.md and BUILD.md for the build identities, scope and run settings. "
      "The five loopback blocks describe the tested configuration; production capacity requires a separate measurement.")
