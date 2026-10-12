<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { condense } from './benchmarkHistory.mjs'

interface Run { date: number; sha: string; message: string }
interface Point extends Run { ns: number }
interface Series { name: string; pkg: string; points: Point[]; bytes: number | null; allocs: number | null }
interface History { lastUpdate: number; repoUrl: string; runs: Run[]; series: { name: string; pkg: string; bytes: number | null; allocs: number | null; ns: (number | null)[] }[] }

const state = ref<'loading' | 'ready' | 'failed'>('loading')
const history = ref<History | null>(null)
const filter = ref('')
const hover = ref<{ series: string; index: number } | null>(null)

function loadScript(src: string) {
  return new Promise<any>((resolve, reject) => {
    const el = document.createElement('script')
    el.src = src
    el.onload = () => (window as any).BENCHMARK_DATA ? resolve((window as any).BENCHMARK_DATA) : reject(new Error('no data'))
    el.onerror = () => { el.remove(); reject(new Error(`failed to load ${src}`)) }
    document.head.appendChild(el)
  })
}

// history.json is condensed from data.js at deploy time. A local dev server
// has neither file, so it falls back to condensing the live data.js here.
const sources: (() => Promise<History>)[] = [
  () => fetch('/benchmarks/history.json').then((r) => (r.ok ? r.json() : Promise.reject(r.status))),
  () => fetch('https://kruda.dev/benchmarks/history.json').then((r) => (r.ok ? r.json() : Promise.reject(r.status))),
  () => loadScript('https://kruda.dev/benchmarks/data.js').then(condense),
]

onMounted(async () => {
  for (const load of sources) {
    try {
      history.value = await load()
      state.value = 'ready'
      return
    } catch {}
  }
  state.value = 'failed'
})

const series = computed<Series[]>(() => {
  const h = history.value
  if (!h) return []
  return h.series.map((s) => ({
    name: s.name,
    pkg: s.pkg,
    bytes: s.bytes,
    allocs: s.allocs,
    points: s.ns.flatMap((ns, i) => (ns === null ? [] : [{ ...h.runs[i], ns }])),
  }))
})

const commitUrl = (sha: string) => `${history.value?.repoUrl ?? 'https://github.com/go-kruda/kruda'}/commit/${sha}`

const groups = computed(() => {
  const q = filter.value.trim().toLowerCase()
  const map = new Map<string, Series[]>()
  for (const s of series.value) {
    if (q && !`${s.name} ${s.pkg}`.toLowerCase().includes(q)) continue
    const key = s.pkg === 'kruda' ? 'Core' : s.pkg
    map.set(key, [...(map.get(key) ?? []), s])
  }
  return [...map]
})

const W = 520
const H = 132
const PAD = { top: 10, right: 8, bottom: 22, left: 8 }

function geometry(s: Series) {
  const ys = s.points.map((p) => p.ns)
  const min = Math.min(...ys)
  const max = Math.max(...ys)
  const span = max - min || max || 1
  const x = (i: number) => PAD.left + (i / Math.max(1, s.points.length - 1)) * (W - PAD.left - PAD.right)
  const y = (v: number) => PAD.top + (1 - (v - min) / span) * (H - PAD.top - PAD.bottom)
  const line = s.points.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(p.ns).toFixed(1)}`).join('')
  const area = `${line}L${x(s.points.length - 1).toFixed(1)},${H - PAD.bottom}L${x(0).toFixed(1)},${H - PAD.bottom}Z`
  return { min, max, x, y, line, area }
}

function onMove(ev: PointerEvent, s: Series) {
  const svg = ev.currentTarget as SVGSVGElement
  const r = svg.getBoundingClientRect()
  const fx = ((ev.clientX - r.left) / r.width) * W
  const i = Math.round(((fx - PAD.left) / (W - PAD.left - PAD.right)) * (s.points.length - 1))
  hover.value = { series: s.name + s.pkg, index: Math.min(s.points.length - 1, Math.max(0, i)) }
}

const fmtNs = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(2)} µs` : `${v < 10 ? v.toFixed(2) : v.toFixed(1)} ns`)
const fmtDate = (d: number) => new Date(d).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })

</script>

<template>
  <div class="bench-history">
    <header>
      <h1>Continuous benchmarks</h1>
      <p class="lede">Go microbenchmarks run in CI on every push to <code>main</code>. Each point is the median of one run's samples, in nanoseconds per operation, so lower is better.</p>
      <p class="note">These run on shared CI machines and catch regressions between commits. For comparisons with other frameworks, see the <a href="/benchmarks/2026-09-21-v1.8.0-typed-binding.html">v1.8.0 benchmark report</a>.</p>
    </header>

    <p v-if="state === 'loading'" class="status">Loading benchmark history…</p>
    <p v-else-if="state === 'failed'" class="status">The benchmark history could not be loaded. The raw data is on the <a href="https://github.com/go-kruda/kruda/tree/gh-pages/benchmarks">gh-pages branch</a>.</p>

    <template v-else>
      <div class="toolbar">
        <label class="search">
          <span class="visually-hidden">Filter benchmarks</span>
          <input v-model="filter" type="search" placeholder="Filter benchmarks" />
        </label>
        <span class="meta">{{ series.length }} benchmarks across {{ history!.runs.length }} runs, last updated {{ fmtDate(history!.lastUpdate) }}</span>
        <a class="dl" href="https://kruda.dev/benchmarks/data.js" download>Download raw data</a>
      </div>

      <section v-for="[group, list] in groups" :key="group" class="group">
        <h2>{{ group }}</h2>
        <div class="grid">
          <article v-for="s in list" :key="s.name + s.pkg" class="card">
            <div class="card-head">
              <h3>{{ s.name }}</h3>
              <strong>{{ fmtNs(s.points.at(-1)!.ns) }}</strong>
            </div>
            <p class="allocs">
              <span v-if="s.bytes !== undefined">{{ s.bytes }} B/op</span>
              <span v-if="s.allocs !== undefined">{{ s.allocs }} allocs/op</span>
            </p>
            <svg :viewBox="`0 0 ${W} ${H}`" class="chart" role="img"
              :aria-label="`${s.name}: ${fmtNs(s.points[0].ns)} to ${fmtNs(s.points.at(-1)!.ns)} over ${s.points.length} runs`"
              @pointermove="onMove($event, s)" @pointerleave="hover = null">
              <template v-for="g in [geometry(s)]" :key="'g'">
                <path :d="g.area" class="area" />
                <path :d="g.line" class="line" />
                <text :x="W - PAD.right" :y="PAD.top + 8" text-anchor="end" class="tick">{{ fmtNs(g.max) }}</text>
                <text :x="W - PAD.right" :y="H - PAD.bottom - 4" text-anchor="end" class="tick">{{ fmtNs(g.min) }}</text>
                <text :x="PAD.left" :y="H - 6" class="tick">{{ fmtDate(s.points[0].date) }}</text>
                <text :x="W - PAD.right" :y="H - 6" text-anchor="end" class="tick">{{ fmtDate(s.points.at(-1)!.date) }}</text>
                <template v-if="hover && hover.series === s.name + s.pkg">
                  <line :x1="g.x(hover.index)" :x2="g.x(hover.index)" :y1="PAD.top" :y2="H - PAD.bottom" class="cursor" />
                  <circle :cx="g.x(hover.index)" :cy="g.y(s.points[hover.index].ns)" r="4" class="dot" />
                </template>
              </template>
            </svg>
            <p class="detail">
              <template v-if="hover && hover.series === s.name + s.pkg">
                <strong>{{ fmtNs(s.points[hover.index].ns) }}</strong> on {{ fmtDate(s.points[hover.index].date) }},
                <a :href="commitUrl(s.points[hover.index].sha)">{{ s.points[hover.index].sha }}</a>
                {{ s.points[hover.index].message }}
              </template>
              <template v-else>Axis is scaled to this benchmark's own range. Hover for each run.</template>
            </p>
          </article>
        </div>
      </section>
      <p v-if="!groups.length" class="status">No benchmark matches “{{ filter }}”.</p>
    </template>
  </div>
</template>

<style scoped>
.bench-history { max-width: 1152px; margin: 0 auto; padding: 48px 24px 96px; }
h1 { font-family: var(--kruda-display); font-size: clamp(2rem, 5vw, 3rem); font-weight: 800; letter-spacing: -.04em; line-height: 1.05; }
.lede { max-width: 62ch; margin-top: 16px; font-size: 1.0625rem; line-height: 1.6; color: var(--vp-c-text-2); }
.note { max-width: 62ch; margin-top: 10px; font-size: .9375rem; line-height: 1.6; color: var(--vp-c-text-3); }
.note a, .status a, .detail a { color: var(--vp-c-brand-1); font-weight: 600; text-decoration: underline; text-underline-offset: 3px; }
code { padding: 2px 6px; border-radius: 4px; background: var(--vp-c-bg-soft); font-family: var(--vp-font-family-mono); font-size: .875em; color: var(--vp-code-color); }
.status { margin-top: 48px; color: var(--vp-c-text-2); }
.toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 20px; margin: 40px 0 8px; }
.search input { width: min(280px, 70vw); padding: 8px 12px; border: 1px solid var(--vp-c-border); border-radius: 8px; background: var(--vp-c-bg); font-size: 14px; color: var(--vp-c-text-1); }
.search input:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 1px; }
.meta { font-size: 13px; color: var(--vp-c-text-3); }
.dl { margin-left: auto; padding: 8px 14px; border: 1px solid var(--vp-c-border); border-radius: 8px; font-size: 13px; font-weight: 600; color: var(--vp-c-text-2); }
.dl:hover { border-color: var(--vp-c-text-2); color: var(--vp-c-text-1); }
.dl:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.group { margin-top: 40px; }
h2 { padding-top: 20px; border-top: 1px solid var(--vp-c-divider); font-family: var(--kruda-display); font-size: 1.375rem; font-weight: 700; letter-spacing: -.02em; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px 32px; margin-top: 16px; }
.card { padding: 16px 0 8px; }
.card-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
h3 { font-family: var(--kruda-display); font-size: 1.0625rem; font-weight: 650; letter-spacing: -.01em; overflow-wrap: anywhere; }
.card-head strong { font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--vp-c-text-1); }
.allocs { display: flex; gap: 14px; margin-top: 2px; font-size: 13px; color: var(--vp-c-text-3); font-variant-numeric: tabular-nums; }
.chart { display: block; width: 100%; height: auto; margin-top: 10px; touch-action: pan-y; cursor: crosshair; }
.area { fill: var(--vp-c-brand-soft); }
.line { fill: none; stroke: var(--vp-c-brand-1); stroke-width: 1.6; stroke-linejoin: round; vector-effect: non-scaling-stroke; }
.tick { fill: var(--vp-c-text-3); font-size: 11px; font-variant-numeric: tabular-nums; }
.cursor { stroke: var(--vp-c-text-3); stroke-dasharray: 3 3; vector-effect: non-scaling-stroke; }
.dot { fill: var(--vp-c-bg); stroke: var(--vp-c-brand-1); stroke-width: 2; vector-effect: non-scaling-stroke; }
.detail { min-height: 2.6em; margin-top: 6px; font-size: 12.5px; line-height: 1.5; color: var(--vp-c-text-2); overflow-wrap: anywhere; }
.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
@media (max-width: 767px) { .grid { grid-template-columns: minmax(0, 1fr); } .bench-history { padding-top: 32px; } }
</style>
