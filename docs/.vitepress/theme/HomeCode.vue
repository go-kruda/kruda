<script setup lang="ts">
import Samples from './HomeCodeSamples.md'
</script>

<template>
  <section class="home-code" aria-labelledby="home-code-title">
    <div class="copy">
      <h2 id="home-code-title">Declare the input. Kruda does the rest.</h2>
      <p>Path, query and JSON body bind into one struct. Validation tags run before your handler does, and the return type becomes the response and the OpenAPI schema.</p>
      <p>Each tab is a complete program you can paste into <code>main.go</code> and run.</p>
      <a href="/guide/handlers">Read the typed handler guide</a>
    </div>
    <div class="samples vp-doc">
      <Samples />
    </div>
  </section>
</template>

<style scoped>
.home-code { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 56px; align-items: start; max-width: 1152px; margin: 0 auto; padding: 48px 0 64px; border-top: 1px solid var(--vp-c-divider); }
h2 { font-family: var(--kruda-display); font-size: clamp(1.75rem, 3.4vw, 2.5rem); font-weight: 750; letter-spacing: -.035em; line-height: 1.1; text-wrap: balance; }
p { max-width: 46ch; margin-top: 16px; font-size: 1.0625rem; line-height: 1.6; color: var(--vp-c-text-2); }
a { display: inline-block; margin-top: 20px; color: var(--vp-c-brand-1); font-weight: 600; text-decoration: underline; text-underline-offset: 4px; }
a:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 4px; }
.samples :deep(.vp-code-group) { margin: 0; }
p code { font-family: var(--vp-font-family-mono); font-size: .9em; color: var(--vp-c-text-1); }
.samples :deep(.vp-code-group .tabs) { border-radius: 12px 12px 0 0; }
.samples :deep(.vp-code-group div[class*='language-']) { margin: 0; border-radius: 0 0 12px 12px; }
.samples :deep(div[class*='language-'] code) { font-size: 13.5px; }
@media (max-width: 1279px) { .home-code { margin-left: 64px; margin-right: 64px; } }
@media (max-width: 959px) { .home-code { grid-template-columns: minmax(0, 1fr); gap: 32px; } }
@media (max-width: 767px) { .home-code { margin: 0 24px; padding: 36px 0 48px; } .samples :deep(.vp-code-group .tabs), .samples :deep(.vp-code-group div[class*='language-']) { border-radius: 0; } }
</style>
