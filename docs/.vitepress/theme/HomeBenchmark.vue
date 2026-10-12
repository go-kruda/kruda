<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const frameworks = ['Kruda v1.8.0', 'Fiber v3.5.0', 'Actix v4.15.0']
const metrics = [
  { title: 'Throughput', direction: 'Higher is better', unit: 'req/s', values: [533209.91, 259169.96, 521983.89], labels: ['533,210', '259,170', '521,984'] },
  { title: 'Corrected p99 at 129k req/s', direction: 'Lower is better', unit: 'ms', values: [1.970, 3.090, 1.880], labels: ['1.970', '3.090', '1.880'] },
]

// Bars grow once, on a fixed timer, when the charts scroll into view.
const charts = ref<HTMLElement | null>(null)
const pending = ref(false)
let observer: IntersectionObserver | undefined

onMounted(() => {
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return
  if (!('IntersectionObserver' in window) || !charts.value) return
  pending.value = true
  observer = new IntersectionObserver((entries) => {
    if (!entries.some((e) => e.isIntersecting)) return
    requestAnimationFrame(() => { pending.value = false })
    observer?.disconnect()
  }, { threshold: 0.35 })
  observer.observe(charts.value)
})

onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <section class="benchmark" aria-labelledby="benchmark-title">
    <h2 id="benchmark-title">2.06x Fiber throughput on validated typed input.</h2>
    <p class="intro">Bind 30 query fields, validate the typed input, and return 30-field JSON — with 36.2% lower p99 than Fiber at equal load.</p>
    <div ref="charts" class="charts" :class="{ pending }">
      <div v-for="metric in metrics" :key="metric.title">
        <h3>{{ metric.title }} <small>{{ metric.direction }}</small></h3>
        <div v-for="(name, index) in frameworks" :key="name" class="chart-row" :class="{ kruda: index === 0 }">
          <div class="chart-label"><span>{{ name }}</span><strong>{{ metric.labels[index] }} <small>{{ metric.unit }}</small></strong></div>
          <div class="bar-track" aria-hidden="true"><div class="bar"
            :style="{ width: `${metric.values[index] / Math.max(...metric.values) * 100}%`, '--i': index }"></div></div>
        </div>
      </div>
    </div>
    <footer>
      <p>21 Sep 2026 · 30-field typed binding + compiled validation · Linux / Wing / HTTP/1.1 loopback · 256 connections</p>
      <p>Six balanced permutations per profile. Kruda and Actix were in the same P4 throughput range; Actix led at P8.</p>
      <p>Bars start at zero. Capacity and common-load latency are separate measurements. Scoped lab results, not production, TLS or HTTP/2 guarantees.</p>
      <a href="/benchmarks/2026-09-21-v1.8.0-typed-binding.html">See all results and test conditions</a>
    </footer>
  </section>
</template>

<style scoped>
.benchmark { max-width: 1152px; margin: 0 auto 56px; padding: 48px 0 8px; border-top: 1px solid var(--vp-c-divider); }
h2 { max-width: 24ch; font-family: var(--kruda-display); font-size: clamp(1.75rem, 4vw, 2.75rem); font-weight: 750; letter-spacing: -.035em; line-height: 1.1; text-wrap: balance; }
.intro { max-width: 62ch; margin-top: 16px; font-size: 1.0625rem; line-height: 1.6; color: var(--vp-c-text-2); }
.charts { display: grid; grid-template-columns: 1fr 1fr; gap: 64px; margin: 40px 0 36px; }
h3 { font-family: var(--kruda-display); font-size: 1.0625rem; font-weight: 650; letter-spacing: -.01em; }
h3 small { display: block; margin-top: 2px; font-family: var(--vp-font-family-base); font-size: 13px; font-weight: 400; letter-spacing: 0; color: var(--vp-c-text-2); }
.chart-row { margin-top: 22px; }
.chart-label { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 8px; font-size: 15px; color: var(--vp-c-text-2); }
.chart-label strong { font-weight: 600; font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--vp-c-text-1); }
.chart-label small { font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.chart-row.kruda .chart-label > span { font-weight: 650; color: var(--vp-c-text-1); }
.bar-track { height: 10px; background: var(--vp-c-bg-soft); border-radius: 2px; overflow: hidden; }
.bar { height: 100%; background: var(--kruda-bar-other); border-radius: 2px; }
.chart-row.kruda .bar { background: var(--kruda-bar); }
footer { max-width: 80ch; font-size: 13px; line-height: 1.8; color: var(--vp-c-text-2); }
a { display: inline-block; margin-top: 12px; color: var(--vp-c-brand-1); font-weight: 600; text-decoration: underline; text-underline-offset: 4px; }
a:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 4px; }
@media (prefers-reduced-motion: no-preference) {
  .bar { clip-path: inset(0 0 0 0); transition: clip-path 1.1s cubic-bezier(.2, .7, .2, 1) calc(var(--i) * 140ms); }
  .charts.pending .bar { clip-path: inset(0 100% 0 0); }
}
@media (max-width: 1279px) { .benchmark { margin-left: 64px; margin-right: 64px; } }
@media (max-width: 959px) { .charts { gap: 48px; } }
@media (max-width: 767px) { .benchmark { margin: 0 24px 40px; padding-top: 36px; } .charts { grid-template-columns: 1fr; gap: 36px; margin: 32px 0 28px; } }
</style>
