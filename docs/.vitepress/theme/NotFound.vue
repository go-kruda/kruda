<script setup lang="ts">
import { withBase } from 'vitepress'
</script>

<template>
  <div class="not-found">
    <div class="art" aria-hidden="true">
      <img class="mascot" :src="withBase('/hero/mascot.webp')" alt="" />
      <img class="feather" :src="withBase('/feathers/feather-2.webp')" alt="" />
    </div>
    <p class="code">404</p>
    <h1>This page flew away.</h1>
    <p class="lede">The link may be out of date, or the page moved when the docs were reorganised. Try the search box at the top, or start from one of these.</p>
    <div class="actions">
      <a class="btn brand" :href="withBase('/')">Back to home</a>
      <a class="btn alt" :href="withBase('/guide/getting-started')">Getting started</a>
    </div>
  </div>
</template>

<style scoped>
.not-found { max-width: 640px; margin: 0 auto; padding: 56px 24px 96px; text-align: center; }
.art { position: relative; width: 220px; height: 220px; margin: 0 auto 8px; }
.mascot { width: 100%; rotate: -8deg; filter: drop-shadow(0 18px 24px rgba(120, 20, 18, .2)); }
.feather { position: absolute; left: -14px; bottom: -6px; height: 64px; rotate: 72deg; opacity: .95; }
.code { font-family: var(--kruda-display); font-size: 15px; font-weight: 700; letter-spacing: .02em; color: var(--vp-c-brand-1); }
h1 { margin-top: 6px; font-family: var(--kruda-display); font-size: clamp(2rem, 6vw, 2.75rem); font-weight: 800; letter-spacing: -.035em; line-height: 1.1; text-wrap: balance; }
.lede { margin: 16px auto 0; max-width: 46ch; font-size: 1.0625rem; line-height: 1.6; color: var(--vp-c-text-2); }
.actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 12px; margin-top: 28px; }
.btn { padding: 10px 20px; border: 1px solid transparent; border-radius: 8px; font-weight: 600; font-size: 15px; transition: background-color .2s, border-color .2s, color .2s; }
.btn.brand { background: var(--vp-button-brand-bg); color: var(--vp-button-brand-text); }
.btn.brand:hover { background: var(--vp-button-brand-hover-bg); }
.btn.alt { border-color: var(--vp-c-border); color: var(--vp-c-text-1); }
.btn.alt:hover { border-color: var(--vp-c-text-2); }
.btn:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
@media (prefers-reduced-motion: no-preference) {
  .feather { animation: drop 1.6s cubic-bezier(.3, .6, .4, 1) .3s both; }
  @keyframes drop {
    0% { translate: 150px -170px; rotate: -30deg; opacity: 0; }
    15% { opacity: 1; }
    40% { translate: 60px -90px; rotate: 30deg; }
    70% { translate: 10px -30px; rotate: 50deg; }
    100% { translate: 0 0; rotate: 72deg; }
  }
}
</style>
