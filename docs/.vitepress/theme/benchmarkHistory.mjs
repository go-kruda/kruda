// Condenses github-action-benchmark's data.js into the small history the
// /benchmarks/ page draws. Used at deploy time by scripts/condense-benchmarks.mjs
// and, when history.json is missing (local dev), by the page itself.

const median = (xs) => {
  const s = [...xs].sort((a, b) => a - b)
  const m = s.length >> 1
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2
}

// A run repeats each benchmark several times (-count), so a run contributes
// the median of its samples. Only benchmarks present in the latest run are
// kept; retired ones would end mid-chart.
export function condense(data) {
  const entries = Object.values(data.entries)[0] ?? []
  const last = entries.at(-1)?.benches ?? []
  const current = [...new Set(last.filter((b) => b.name.endsWith(' - ns/op')).map((b) => b.name.slice(0, -8)))]
  const pickLatest = (full, suffix) => {
    const xs = last.filter((b) => b.name === `${full} - ${suffix}`).map((b) => b.value)
    return xs.length ? median(xs) : null
  }
  return {
    lastUpdate: data.lastUpdate,
    repoUrl: data.repoUrl,
    runs: entries.map((e) => ({ date: e.date, sha: e.commit.id.slice(0, 7), message: e.commit.message.split('\n')[0] })),
    series: current.map((full) => {
      const match = full.match(/^(\S+) \((.+)\)$/)
      return {
        name: (match?.[1] ?? full).replace(/^Benchmark/, ''),
        pkg: (match?.[2] ?? '').replace('github.com/go-kruda/kruda', 'kruda').replace(/^kruda\//, ''),
        bytes: pickLatest(full, 'B/op'),
        allocs: pickLatest(full, 'allocs/op'),
        ns: entries.map((e) => {
          const xs = e.benches.filter((b) => b.name === `${full} - ns/op`).map((b) => b.value)
          return xs.length ? +median(xs).toFixed(3) : null
        }),
      }
    }),
  }
}
