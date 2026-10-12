<script setup lang="ts">
import { ref } from 'vue'

const command = 'go get github.com/go-kruda/kruda'
const copied = ref(false)

async function copy() {
  await navigator.clipboard.writeText(command)
  copied.value = true
  setTimeout(() => { copied.value = false }, 1600)
}
</script>

<template>
  <div class="install">
    <code><span aria-hidden="true">$</span> {{ command }}</code>
    <button type="button" @click="copy" :aria-label="copied ? 'Copied' : 'Copy install command'">{{ copied ? 'Copied' : 'Copy' }}</button>
  </div>
</template>

<style scoped>
.install { display: inline-flex; align-items: center; gap: 4px; max-width: 100%; margin-top: 20px; padding: 4px 4px 4px 14px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); }
code { overflow-x: auto; scrollbar-width: none; font-family: var(--vp-font-family-mono); font-size: 13.5px; white-space: nowrap; color: var(--vp-c-text-1); }
code span { color: var(--vp-c-text-3); user-select: none; }
button { flex: none; padding: 6px 10px; border-radius: 6px; font-size: 13px; font-weight: 600; color: var(--vp-c-text-2); transition: color .15s, background-color .15s; }
button:hover { background: var(--vp-c-bg-elv); color: var(--vp-c-text-1); }
button:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
@media (max-width: 420px) { .install { padding-left: 12px; } code { font-size: 12px; } }
</style>
