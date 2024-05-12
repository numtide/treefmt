import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  base: '/treefmt/',

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
      { text: 'Install Treefmt', link: '/install' },
      { text: 'Configure Treefmt', link: '/configure' },
      { text: 'Run Treefmt', link: '/usage' },
      { text: 'Motivation', link: '/about' },
      { text: 'Formatter Spec', link: '/formatter-spec' },
      { text: 'Contributing', link: '/contributing' },
      { text: 'FAQ', link: '/faq' },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/numtide/treefmt' }
    ],

    footer: {
      message: 'Released under the <a href="https://https://github.com/numtide/treefmt/src/branch/main/LICENSE.md">MIT License</a>.',
      copyright: "Copyright © 2024-present Treefmt Contributors"
    }
  }
})
