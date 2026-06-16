import { defineConfig } from 'vitepress'

const repoRoot = 'https://github.com/go-spring/go-spring'

const goImportModules: Record<string, string> = {
  spring: 'spring',
  log: 'log',
  stdlib: 'stdlib',
  gs: 'gs/gs',
  'gs-http-gen': 'gs/gs-http-gen',
  'gs-gen': 'gs/gs-gen',
  'gs-init': 'gs/gs-init',
  'gs-mock': 'gs/gs-mock',
  'starter-gorm-mysql': 'starter/starter-gorm-mysql',
  'starter-go-redis': 'starter/starter-go-redis',
  'starter-redigo': 'starter/starter-redigo',
  'starter-pprof': 'starter/starter-pprof'
}

const goImportHeadTags = (relativePath: string) => {
  const moduleName = relativePath
    .replace(/^go-import\//, '')
    .replace(/\/index\.md$/, '')
    .replace(/\.md$/, '')
  const moduleSubdir = goImportModules[moduleName]

  if (!moduleSubdir) {
    return []
  }

  const importPrefix = `go-spring.org/${moduleName}`
  const sourceDir = `${repoRoot}/tree/master/${moduleSubdir}`
  const sourceFile = `${repoRoot}/blob/master/${moduleSubdir}{/dir}/{file}#L{line}`

  return [
    ['meta', { name: 'go-import', content: `${importPrefix} git ${repoRoot} ${moduleSubdir}` }],
    ['meta', {
      name: 'go-source',
      content: `${importPrefix} ${sourceDir} ${sourceDir}{/dir} ${sourceFile}`
    }]
  ]
}

const zhNav = [
  { text: '概览', link: '/cn/docs/0.overview/overview' },
  { text: '快速开始', link: '/cn/docs/1.getting-started/getting-started' },
  { text: '指南', link: '/cn/docs/2.guides/01-configuration' },
  { text: '示例', link: '/cn/docs/3.examples/examples' },
  { text: '参与贡献', link: '/cn/docs/6.contributing' }
]

const enNav = [
  { text: 'Overview', link: '/en/docs/0.overview/overview' },
  { text: 'Getting Started', link: '/en/docs/1.getting-started/getting-started' },
  { text: 'Guides', link: '/en/docs/2.guides/01-configuration' },
  { text: 'Examples', link: '/en/docs/3.examples/examples' },
  { text: 'Contributing', link: '/en/docs/6.contributing' }
]

const zhSidebar = [
  {
    text: '介绍',
    items: [
      { text: '项目概览', link: '/cn/docs/0.overview/overview' },
      { text: '快速开始', link: '/cn/docs/1.getting-started/getting-started' }
    ]
  },
  {
    text: '指南',
    items: [
      { text: '配置管理', link: '/cn/docs/2.guides/01-configuration' },
      { text: 'IoC 容器', link: '/cn/docs/2.guides/02-ioc-container' },
      { text: '应用启动与停止', link: '/cn/docs/2.guides/03-app-start-stop' },
      { text: '日志系统', link: '/cn/docs/2.guides/04-logging' },
      { text: 'HTTP 服务', link: '/cn/docs/2.guides/05-http-server' },
      { text: '组件机制', link: '/cn/docs/2.guides/06-components' },
      { text: '测试支持', link: '/cn/docs/2.guides/07-testing' },
      { text: 'HTTP 代码生成', link: '/cn/docs/2.guides/08-http-gen' }
    ]
  },
  {
    text: '示例',
    items: [
      { text: '示例总览', link: '/cn/docs/3.examples/examples' }
    ]
  },
  {
    text: '集成',
    items: [
      { text: 'GORM MySQL', link: '/cn/docs/4.integrations/starter-gorm-mysql' },
      { text: 'Go Redis', link: '/cn/docs/4.integrations/starter-go-redis' },
      { text: 'Redigo', link: '/cn/docs/4.integrations/starter-redigo' },
      { text: 'PProf', link: '/cn/docs/4.integrations/starter-pprof' }
    ]
  },
  {
    text: '更多',
    items: [
      { text: 'FAQ', link: '/cn/docs/5.faq' },
      { text: '参与贡献', link: '/cn/docs/6.contributing' },
      { text: '更新日志', link: '/cn/docs/7.changelog' }
    ]
  }
]

const enSidebar = [
  {
    text: 'Introduction',
    items: [
      { text: 'Overview', link: '/en/docs/0.overview/overview' },
      { text: 'Getting Started', link: '/en/docs/1.getting-started/getting-started' }
    ]
  },
  {
    text: 'Guides',
    items: [
      { text: 'Configuration', link: '/en/docs/2.guides/01-configuration' },
      { text: 'IoC Container', link: '/en/docs/2.guides/02-ioc-container' },
      { text: 'App Start/Stop', link: '/en/docs/2.guides/03-app-start-stop' },
      { text: 'Logging', link: '/en/docs/2.guides/04-logging' },
      { text: 'HTTP Server', link: '/en/docs/2.guides/05-http-server' },
      { text: 'Components', link: '/en/docs/2.guides/06-components' },
      { text: 'Testing', link: '/en/docs/2.guides/07-testing' },
      { text: 'HTTP Gen', link: '/en/docs/2.guides/08-http-gen' }
    ]
  },
  {
    text: 'Examples',
    items: [
      { text: 'Examples', link: '/en/docs/3.examples/examples' }
    ]
  },
  {
    text: 'Integrations',
    items: [
      { text: 'GORM MySQL', link: '/en/docs/4.integrations/starter-gorm-mysql' },
      { text: 'Go Redis', link: '/en/docs/4.integrations/starter-go-redis' },
      { text: 'Redigo', link: '/en/docs/4.integrations/starter-redigo' },
      { text: 'PProf', link: '/en/docs/4.integrations/starter-pprof' }
    ]
  },
  {
    text: 'More',
    items: [
      { text: 'FAQ', link: '/en/docs/5.faq' },
      { text: 'Contributing', link: '/en/docs/6.contributing' },
      { text: 'Changelog', link: '/en/docs/7.changelog' }
    ]
  }
]

export default defineConfig({
  title: 'Go-Spring',
  description: 'Documentation for Go-Spring',
  head: [['link', { rel: 'icon', href: '/logo.png' }]],
  transformPageData(pageData) {
    pageData.frontmatter.head ??= []
    pageData.frontmatter.head.push(...goImportHeadTags(pageData.relativePath))
  },
  outDir: '../docs',
  cleanUrls: true,
  rewrites: {
    'cn/index.md': 'index.md',
    'go-import/spring.md': 'spring/index.md',
    'go-import/log.md': 'log/index.md',
    'go-import/stdlib.md': 'stdlib/index.md',
    'go-import/gs.md': 'gs/index.md',
    'go-import/gs-http-gen.md': 'gs-http-gen/index.md',
    'go-import/gs-gen.md': 'gs-gen/index.md',
    'go-import/gs-init.md': 'gs-init/index.md',
    'go-import/gs-mock.md': 'gs-mock/index.md',
    'go-import/starter-gorm-mysql.md': 'starter-gorm-mysql/index.md',
    'go-import/starter-go-redis.md': 'starter-go-redis/index.md',
    'go-import/starter-redigo.md': 'starter-redigo/index.md',
    'go-import/starter-pprof.md': 'starter-pprof/index.md'
  },
  themeConfig: {
    logo: '/logo.png',
    socialLinks: [
      { icon: 'github', link: 'https://github.com/go-spring/go-spring' }
    ]
  },
  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      title: 'Go-Spring',
      description: 'Go-Spring 中文文档',
      themeConfig: {
        nav: zhNav,
        sidebar: zhSidebar
      }
    },
    en: {
      label: 'English',
      lang: 'en-US',
      title: 'Go-Spring',
      description: 'Documentation for Go-Spring',
      themeConfig: {
        nav: enNav,
        sidebar: enSidebar
      }
    }
  }
})
