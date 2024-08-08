---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
    name: "Treefmt"
    tagline: Code formatting multiplexer
    image:
        src: /treefmt.gif
        alt: Treefmt
    actions:
        - theme: brand
          text: Quick Start
          link: ./quick-start
        - theme: alt
          text: More Info
          link: ./about

features:
    - icon: ⚡
      title: Fast
      details: Run all code formatters in parallel
    - icon: ⛁
      title: Cached
      details: Only format files that have changed
    - icon: ☷
      title: Unified
      details: Compatible with at least 74 code formatters
---
