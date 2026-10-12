<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useData } from 'vitepress'
import { VPImage } from 'vitepress/theme'
import { startFeathers2D } from './heroArt/feathers2d'
import { landFeather } from './heroArt/landing'

const { frontmatter } = useData()
const canvas = ref<HTMLCanvasElement | null>(null)
const stops: (() => void)[] = []

onMounted(() => {
  const el = canvas.value
  if (!el) return
  const card = el.parentElement?.querySelector<HTMLElement>('.image-src') ?? null
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches
  stops.push(startFeathers2D(el, card, { count: matchMedia('(max-width: 959px)').matches ? 8 : 14, still }))
  const slot = document.querySelector<HTMLElement>('.VPHero .name .kruda-feather')
  if (slot) stops.push(landFeather(slot, card, { still }))
})

onBeforeUnmount(() => stops.forEach((stop) => stop()))
</script>

<template>
  <canvas ref="canvas" class="hero-art" aria-hidden="true" />
  <VPImage v-if="frontmatter.hero?.image" class="image-src" :image="frontmatter.hero.image" />
</template>

<style scoped>
.hero-art {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 640px;
  height: 520px;
  translate: -50% -50%;
  pointer-events: none;
  -webkit-mask-image: radial-gradient(closest-side, #000 62%, transparent);
  mask-image: radial-gradient(closest-side, #000 62%, transparent);
}

.image-src {
  z-index: 1;
}

@media (max-width: 959px) {
  .hero-art {
    width: 380px;
    height: 340px;
  }
}
</style>
