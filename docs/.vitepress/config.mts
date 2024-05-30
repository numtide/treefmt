import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  base: '/',

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
      { text: 'Motivation', link: '/about' },
      { text: 'Quick Start', link: '/quick-start' },
      { text: 'Install Treefmt', link: '/install' },
      { text: 'Configure Treefmt', link: '/configure' },
      { text: 'Run Treefmt', link: '/usage' },
      { text: 'Formatter Spec', link: '/formatter-spec' },
      { text: 'Contributing', link: '/contributing' },
      { text: 'FAQ', link: '/faq' },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/numtide/treefmt' }
    ],

    footer: {
      message: 'Released under the <a href="https://github.com/numtide/treefmt/blob/main/LICENSE">MIT License</a>.',
      copyright: "Copyright © Numtide & Contributors"
    }
  }
})
