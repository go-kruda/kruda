import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Kruda',
  description: 'Type-safe Go web framework with auto-everything',
  lang: 'en-US',

  head: [
    ['meta', { name: 'theme-color', content: '#e44d26' }],
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'Kruda — Type-safe Go Web Framework' }],
    ['meta', { name: 'og:description', content: 'High-performance Go web framework combining speed with type-safety through Go generics.' }],
    ['meta', { name: 'og:url', content: 'https://kruda.dev' }],
  ],

  sitemap: {
    hostname: 'https://kruda.dev',
  },

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'API', link: '/api/app' },
      { text: 'Examples', link: '/guide/getting-started#next-steps' },
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
            { text: 'Handlers', link: '/guide/handlers' },
            { text: 'Middleware', link: '/guide/middleware' },
            { text: 'Error Handling', link: '/guide/error-handling' },
          ],
        },
        {
          text: 'Advanced',
          items: [
            { text: 'DI Container', link: '/guide/di-container' },
            { text: 'Auto CRUD', link: '/guide/auto-crud' },
            { text: 'Security', link: '/guide/security' },
            { text: 'Performance', link: '/guide/performance' },
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
      copyright: 'Copyright © 2024 Kruda Contributors',
    },
  },
})
