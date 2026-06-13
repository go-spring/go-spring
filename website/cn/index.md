---
layout: home

hero:
  name: Go-Spring
  text: 类 Spring 的 Go 应用框架
  tagline: 为 Go 服务提供配置管理、依赖注入、组件装配和工程化工具链。

actions:
  - theme: brand
    text: 快速开始
    link: /cn/docs/1.getting-started/getting-started
  - theme: alt
    text: 阅读指南
    link: /cn/docs/2.guides/01-configuration

features:
  - title: IoC 容器
    details: 用熟悉的组件模型管理 Bean、依赖注入和应用生命周期。
  - title: 配置管理
    details: 支持配置加载、绑定、默认值和属性引用，适合配置驱动的服务开发。
  - title: HTTP 服务
    details: 提供 HTTP 服务开发能力，并配合工具链生成重复样板代码。
---

<section class="gs-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Core Features</p>
    <h2>面向 Go 服务工程化的核心能力</h2>
    <p>Go-Spring 关注服务开发中反复出现的基础设施问题，让业务代码保持清晰、直接、可组合。</p>
  </div>
  <div class="gs-grid gs-grid-3">
    <div class="gs-card">
      <h3>配置绑定</h3>
      <p>将配置文件、环境变量和默认值绑定到结构体，降低配置解析和校验的样板代码。</p>
    </div>
    <div class="gs-card">
      <h3>依赖注入</h3>
      <p>通过 IoC 容器组织组件依赖，让服务装配、测试替换和生命周期管理更加统一。</p>
    </div>
    <div class="gs-card">
      <h3>组件 Starter</h3>
      <p>围绕 Redis、GORM、PProf 等常见组件提供集成入口，减少重复初始化逻辑。</p>
    </div>
    <div class="gs-card">
      <h3>HTTP 开发</h3>
      <p>支持 HTTP 服务、路由和接口生成，让接口开发更贴近工程实践。</p>
    </div>
    <div class="gs-card">
      <h3>测试支持</h3>
      <p>围绕组件和应用装配提供测试能力，帮助验证真实配置下的服务行为。</p>
    </div>
    <div class="gs-card">
      <h3>工具链</h3>
      <p>通过 gs 相关工具支持项目初始化、代码生成和开发辅助，提升日常开发效率。</p>
    </div>
  </div>
</section>

<section class="gs-section gs-split">
  <div class="gs-section-heading gs-section-heading-left">
    <p class="gs-eyebrow">Why Go-Spring</p>
    <h2>为什么选择 Go-Spring</h2>
    <p>它不是把 Java Spring 原样搬到 Go，而是在保留 Go 简单直接风格的同时，补齐服务工程化所需的组织能力。</p>
  </div>
  <div class="gs-list-card">
    <div>
      <h3>熟悉的编程模型</h3>
      <p>对熟悉 Spring 的团队更友好，降低从传统服务框架迁移到 Go 的工程化成本。</p>
    </div>
    <div>
      <h3>显式且轻量</h3>
      <p>优先保持 Go 的直接表达，不为了框架抽象引入过度魔法。</p>
    </div>
    <div>
      <h3>面向生产实践</h3>
      <p>围绕配置、组件、日志、HTTP 和常见中间件集成构建能力，而不只是提供示例代码。</p>
    </div>
  </div>
</section>

<section class="gs-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Ecosystem</p>
    <h2>探索 Go-Spring 生态</h2>
  </div>
  <div class="gs-grid gs-grid-4">
    <a class="gs-link-card" href="/spring/">spring<span>核心框架</span></a>
    <a class="gs-link-card" href="/log/">log<span>日志模块</span></a>
    <a class="gs-link-card" href="/gs/">gs<span>命令行工具</span></a>
    <a class="gs-link-card" href="/starter-gorm-mysql/">starters<span>组件集成</span></a>
  </div>
</section>

<section class="gs-section gs-muted-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Who is it for</p>
    <h2>适合这些团队和场景</h2>
    <p>如果你的 Go 服务已经开始面对配置膨胀、组件装配、重复初始化和接口样板代码，Go-Spring 可以提供更统一的开发体验。</p>
  </div>
  <div class="gs-pill-list">
    <span>配置驱动的 Go 服务</span>
    <span>组件化应用装配</span>
    <span>类 Spring 开发体验</span>
    <span>HTTP 服务代码生成</span>
    <span>中间件 Starter 集成</span>
  </div>
</section>

<section class="gs-section gs-cta">
  <p class="gs-eyebrow">Get Started</p>
  <h2>开始构建你的 Go-Spring 应用</h2>
  <p>从快速开始了解基础用法，再通过指南和示例逐步接入配置、IoC、HTTP 和组件能力。</p>
  <div class="gs-actions">
    <a href="/cn/docs/1.getting-started/getting-started">快速开始</a>
    <a href="/cn/docs/3.examples/examples">查看示例</a>
  </div>
</section>
