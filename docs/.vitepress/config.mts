import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Treefmt",
  description: "one CLI to format your repo",

  head: [
    ['link', { rel: 'icon', href: '/logo.png' }],
  ],

  themeConfig: {

    logo: '/logo.svg',

    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Quick Start', link: '/quick-start' }
    ],

    sidebar: [
      { text: 'Quick Start', link: '/quick-start' },
      { text: 'Installation', link: '/installation' },
      { text: 'Overview', link: '/overview' },
      { text: 'Usage', link: '/usage' },
      { text: 'Formatter Specification', link: '/formatter-spec' },
      { text: 'Contributing', link: '/contributing' },
      { text: 'FAQ', link: '/faq' },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://https://github.com/numtide/treefmt.go' }
    ],

    footer: {
      message: 'Released under the <a href="https://https://github.com/numtide/treefmt.go/src/branch/main/LICENSE.md">MIT License</a>.',
      copyright: "Copyright © 2024-present Treefmt Contributors"
    }
  }
})
