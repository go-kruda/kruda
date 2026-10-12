import DefaultTheme from 'vitepress/theme'
import { h } from 'vue'
import HomeBenchmark from './HomeBenchmark.vue'
import HomeCode from './HomeCode.vue'
import HomeInstall from './HomeInstall.vue'
import HeroArt from './HeroArt.vue'
import '@fontsource-variable/bricolage-grotesque'
import './custom.css'

export default {
  extends: DefaultTheme,
  Layout: () => h(DefaultTheme.Layout, null, {
    'home-hero-image': () => h(HeroArt),
    'home-hero-actions-after': () => h(HomeInstall),
    'home-hero-after': () => [h(HomeCode), h(HomeBenchmark)],
  }),
}
