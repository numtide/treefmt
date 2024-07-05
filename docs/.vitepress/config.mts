import { defineConfig } from 'vitepress'
import defineVersionedConfig from "vitepress-versioning-plugin";
import DefaultTheme from "vitepress/theme";


// manually specifying the versioned sidebars to preserve isActive linking, I just couldn't seem to get it to
// work properly with the plugin

const sidebar = {
  "/": [
    { text: 'Motivation', link: '/about' },
    { text: 'Quick Start', link: '/quick-start' },
    { text: 'Install Treefmt', link: '/install' },
    { text: 'Configure Treefmt', link: '/configure' },
    { text: 'Run Treefmt', link: '/usage' },
    { text: 'Formatter Spec', link: '/formatter-spec' },
    { text: 'Contributing', link: '/contributing' },
    { text: 'FAQ', link: '/faq' },
  ]
}

// static version list, KISS
const versions = ["v2.0.1", "v2.0.2"]

versions.forEach(version => {
  sidebar[`/${version}/`] = sidebar["/"].map(({ text, link }) => ({ text, link: `/${version}${link}`}))
})

// https://vitepress.dev/reference/site-config
export default defineVersionedConfig({
  base: '/',

  title: "Treefmt",
  description: "one CLI to format your repo",

  head: [
    ['link', { rel: 'icon', href: '/logo.png' }],
  ],

  cleanUrls: true,

  versioning: {
    latestVersion: "main",
  },

  themeConfig: {

    logo: '/logo.svg',

    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: './' },
      { text: 'Quick Start', link: './quick-start' }
    ],

    // manually specifying the versioned sidebars to preserve isActive linking, and I just couldn't seem to get it to
    // work properly
    sidebar,

    socialLinks: [
      { icon: 'github', link: 'https://github.com/numtide/treefmt' }
    ],

    footer: {
      message: 'Released under the <a href="https://github.com/numtide/treefmt/blob/main/LICENSE">MIT License</a>.',
      copyright: "Copyright © Numtide & Contributors"
    }
  }
}, __dirname)
