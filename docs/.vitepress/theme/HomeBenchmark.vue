<script setup lang="ts">
const frameworks = ['Kruda v1.7.2', 'Fiber v3.5.0', 'Actix v4.15.0']
const metrics = [
  { title: 'Throughput', direction: 'Higher is better', unit: 'req/s', values: [816186.87, 816411.19, 783652.58], labels: ['816,187', '816,411', '783,653'] },
  { title: 'p99 latency', direction: 'Lower is better', unit: 'ms', values: [0.642, 0.910, 0.692], labels: ['0.642', '0.910', '0.692'] },
]
</script>

<template>
  <section class="benchmark" aria-labelledby="benchmark-title">
    <h2 id="benchmark-title">High throughput. Low tail latency.</h2>
    <p class="intro">JSON throughput on par with Fiber, with 29.5% lower p99 in this run.</p>
    <div class="charts">
      <div v-for="metric in metrics" :key="metric.title">
        <h3>{{ metric.title }} <small>{{ metric.direction }}</small></h3>
        <div v-for="(name, index) in frameworks" :key="name" class="chart-row">
          <div class="chart-label"><span>{{ name }}</span><strong>{{ metric.labels[index] }} <small>{{ metric.unit }}</small></strong></div>
          <div class="bar-track" aria-hidden="true"><div class="bar" :class="{ kruda: index === 0 }"
            :style="{ width: `${metric.values[index] / Math.max(...metric.values) * 100}%` }"></div></div>
        </div>
      </div>
    </div>
    <footer>
      <p>12 Sep 2026 · JSON serialization · Linux / Wing / HTTP/1.1 loopback · 256 connections · 5-round medians</p>
      <p>Kruda throughput was 4.2% higher than Actix. Fiber led throughput on plaintext and static JSON.</p>
      <p>Bars start at zero. p99 is the median of five per-round 99th percentiles. Lab results, not production, TLS or HTTP/2 guarantees.</p>
      <a href="/benchmarks/2026-09-12-stable.html">See all results and test conditions →</a>
    </footer>
  </section>
</template>

<style scoped>
.benchmark { max-width: 1152px; margin: 0 auto 64px; padding: 40px 0 24px; border-top: 1px solid var(--vp-c-divider); border-bottom: 1px solid var(--vp-c-divider); }
h2 { font-size: clamp(24px, 3vw, 32px); font-weight: 700; letter-spacing: -.035em; line-height: 1.2; }
.intro { margin-top: 12px; color: var(--vp-c-text-2); }
.charts { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; margin: 32px 0; }
h3 { font-size: 18px; font-weight: 600; }
h3 small { display: block; font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.chart-row { margin-top: 24px; }
.chart-label { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 10px; font-size: 15px; }
.chart-label strong { font-variant-numeric: tabular-nums; white-space: nowrap; }
.chart-label small { font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.bar-track { height: 14px; background: var(--vp-c-bg-soft); border-radius: 3px; overflow: hidden; }
.bar { height: 100%; background: var(--vp-c-text-3); border-radius: 3px; }
.bar.kruda { background: var(--vp-c-brand-1); }
footer { color: var(--vp-c-text-2); font-size: 12px; line-height: 1.8; }
a { display: inline-block; margin-top: 12px; color: var(--vp-c-brand-1); font-weight: 600; text-decoration: underline; text-underline-offset: 4px; }
a:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 4px; }
@media (max-width: 1279px) { .benchmark { margin-left: 64px; margin-right: 64px; } }
@media (max-width: 767px) { .benchmark { margin: 0 24px 40px; padding-top: 28px; } .charts { grid-template-columns: 1fr; gap: 32px; } }
</style>
