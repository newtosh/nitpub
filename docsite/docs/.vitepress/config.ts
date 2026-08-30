import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'nitpub',
  description: 'Self-hosted ActivityPub blog — install and operate nitpub',
  cleanUrls: true,
  head: [['link', { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' }]],
  themeConfig: {
    logo: {
      light: '/wordmark-light.svg',
      dark: '/wordmark-dark.svg',
    },
    siteTitle: false,
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Install', link: '/guide/install' },
      { text: 'One-click deploy', link: '/guide/one-click-deploy' },
      { text: 'Changelog', link: '/changelog' },
      { text: 'Product site', link: 'https://www.nitpub.com/' },
    ],
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'Quick install', link: '/guide/install' },
          { text: 'One-click deploy', link: '/guide/one-click-deploy' },
          { text: 'Manual download', link: '/guide/manual-download' },
          { text: 'Updates', link: '/guide/updates' },
          { text: 'Federation', link: '/guide/federation' },
          { text: 'Analytics', link: '/guide/analytics' },
        ],
      },
      {
        text: 'Reference',
        items: [{ text: 'Changelog', link: '/changelog' }],
      },
    ],
    socialLinks: [{ icon: 'github', link: 'https://github.com/newtosh/nitpub' }],
    footer: {
      message: 'MIT Licensed',
      copyright: 'Demo @nit@nitpub.com',
    },
  },
})
