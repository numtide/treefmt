import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Treefmt",
  description: "one CLI to format your repo",
  themeConfig: {

    logo: '/logo.svg',

    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Quick Start', link: '/quick-start' }
    ],

    sidebar: [
      { text: 'Quick Start', link: '/quick-start' },
      { text: 'Overview', link: '/overview' },
      { text: 'Usage', link: '/usage' },
      { text: 'Formatter Spec', link: '/formatter-spec' },
      { text: 'Contributing', link: '/contributing' },
      { text: 'FAQ', link: '/faq' },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://git.numtide.com/numtide/treefmt' }
    ],

    footer: {
      message: 'Released under the <a href="https://git.numtide.com/numtide/treefmt/src/branch/main/LICENSE.md">MIT License</a>.',
      copyright: "Copyright © 2024-present Treefmt Contributors"
    }
  }
})
