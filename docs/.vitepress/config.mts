import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Kruda',
  description: 'Fast by default, type-safe by design — Go web framework',
  lang: 'en-US',
  base: '/kruda/',

  head: [
    ['meta', { name: 'theme-color', content: '#e44d26' }],
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'Kruda — Go Web Framework' }],
    ['meta', { name: 'og:description', content: 'Fast by default, type-safe by design. High-performance Go web framework with typed handlers, auto CRUD, and custom async I/O transport.' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'API', link: '/api/app' },
      { text: 'Benchmarks', link: 'https://go-kruda.github.io/kruda/benchmarks/' },
      { text: 'GitHub', link: 'https://github.com/go-kruda/kruda' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
          ],
        },
        {
          text: 'Core Concepts',
          items: [
            { text: 'Routing', link: '/guide/routing' },
            { text: 'Typed Handlers', link: '/guide/handlers' },
            { text: 'Middleware', link: '/guide/middleware' },
            { text: 'Error Handling', link: '/guide/error-handling' },
          ],
        },
        {
          text: 'Features',
          items: [
            { text: 'Auto CRUD', link: '/guide/auto-crud' },
            { text: 'DI Container', link: '/guide/di-container' },
            { text: 'Transport', link: '/guide/transport' },
            { text: 'Security', link: '/guide/security' },
            { text: 'Performance', link: '/guide/performance' },
          ],
        },
        {
          text: 'Ecosystem',
          items: [
            { text: 'Contrib Modules', link: '/guide/contrib' },
            { text: 'AI Integration', link: '/guide/ai-integration' },
          ],
        },
        {
          text: 'Migration',
          items: [
            { text: 'From Gin', link: '/guide/coming-from-gin' },
            { text: 'From Fiber', link: '/guide/coming-from-fiber' },
            { text: 'From Echo', link: '/guide/coming-from-echo' },
            { text: 'From stdlib', link: '/guide/coming-from-stdlib' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'API Reference',
          items: [
            { text: 'App', link: '/api/app' },
            { text: 'Context', link: '/api/context' },
            { text: 'Handler', link: '/api/handler' },
            { text: 'Container', link: '/api/container' },
            { text: 'Resource', link: '/api/resource' },
            { text: 'Config', link: '/api/config' },
            { text: 'Error', link: '/api/error' },
            { text: 'Test Client', link: '/api/test-client' },
            { text: 'Health', link: '/api/health' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/go-kruda/kruda' },
    ],

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright 2026 Kruda Contributors',
    },
  },
})
