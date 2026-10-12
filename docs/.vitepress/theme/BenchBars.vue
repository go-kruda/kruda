<script setup lang="ts">
interface Row { name: string; value: number; label: string }
interface Group { title: string; rows: Row[] }

defineProps<{ groups: Group[]; unit: string; direction: string }>()
</script>

<template>
  <figure class="bench-bars">
    <div class="groups">
      <div v-for="group in groups" :key="group.title">
        <figcaption>{{ group.title }} <small>{{ direction }}</small></figcaption>
        <div v-for="row in group.rows" :key="row.name" class="row" :class="{ kruda: row.name.startsWith('Kruda') }">
          <div class="label"><span>{{ row.name }}</span><strong>{{ row.label }} <small>{{ unit }}</small></strong></div>
          <div class="track" aria-hidden="true">
            <div class="bar" :style="{ width: `${row.value / Math.max(...group.rows.map((r) => r.value)) * 100}%` }" />
          </div>
        </div>
      </div>
    </div>
  </figure>
</template>

<style scoped>
.bench-bars { margin: 20px 0 28px; }
.groups { display: grid; grid-template-columns: 1fr 1fr; gap: 40px; }
figcaption { font-family: var(--kruda-display); font-size: 1rem; font-weight: 650; letter-spacing: -.01em; color: var(--vp-c-text-1); }
figcaption small { display: block; margin-top: 2px; font-family: var(--vp-font-family-base); font-size: 12.5px; font-weight: 400; color: var(--vp-c-text-2); }
.row { margin-top: 16px; }
.label { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 6px; font-size: 14px; color: var(--vp-c-text-2); }
.label strong { font-weight: 600; font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--vp-c-text-1); }
.label small { font-size: 12px; font-weight: 400; color: var(--vp-c-text-2); }
.row.kruda .label > span { font-weight: 650; color: var(--vp-c-text-1); }
.track { height: 8px; border-radius: 2px; background: var(--vp-c-bg-soft); overflow: hidden; }
.bar { height: 100%; border-radius: 2px; background: var(--kruda-bar-other); }
.row.kruda .bar { background: var(--kruda-bar); }
@media (max-width: 639px) { .groups { grid-template-columns: 1fr; gap: 28px; } }
</style>
