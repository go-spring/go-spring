import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Go-Spring',
  description: 'Documentation for Go-Spring',
  outDir: '../docs',
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: 'Overview', link: '/docs/0.overview/overview' },
      { text: 'Getting Started', link: '/docs/1.getting-started/getting-started' },
      { text: 'Guides', link: '/docs/2.guides/01-configuration' },
      { text: 'Examples', link: '/docs/3.examples/examples' }
    ],
    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'Overview', link: '/docs/0.overview/overview' },
          { text: 'Getting Started', link: '/docs/1.getting-started/getting-started' }
        ]
      },
      {
        text: 'Guides',
        items: [
          { text: 'Configuration', link: '/docs/2.guides/01-configuration' },
          { text: 'IoC Container', link: '/docs/2.guides/02-ioc-container' },
          { text: 'App Start/Stop', link: '/docs/2.guides/03-app-start-stop' },
          { text: 'Logging', link: '/docs/2.guides/04-logging' },
          { text: 'HTTP Server', link: '/docs/2.guides/05-http-server' },
          { text: 'Components', link: '/docs/2.guides/06-components' },
          { text: 'Testing', link: '/docs/2.guides/07-testing' },
          { text: 'HTTP Gen', link: '/docs/2.guides/08-http-gen' }
        ]
      },
      {
        text: 'Examples',
        items: [
          { text: 'Examples', link: '/docs/3.examples/examples' }
        ]
      },
      {
        text: 'Integrations',
        items: [
          { text: 'GORM MySQL', link: '/docs/4.integrations/starter-gorm-mysql' },
          { text: 'Go Redis', link: '/docs/4.integrations/starter-go-redis' },
          { text: 'Redigo', link: '/docs/4.integrations/starter-redigo' },
          { text: 'PProf', link: '/docs/4.integrations/starter-pprof' }
        ]
      },
      {
        text: 'More',
        items: [
          { text: 'FAQ', link: '/docs/5.faq' },
          { text: 'Contributing', link: '/docs/6.contributing' },
          { text: 'Changelog', link: '/docs/7.changelog' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/go-spring/go-spring' }
    ]
  }
})
