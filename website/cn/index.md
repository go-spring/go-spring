---
layout: home

hero:
  name: Go-Spring
  text: 把工程能力装进 Go
  tagline: 配置、依赖、生命周期与基础设施统一装配，让业务代码继续保持 Go 应有的简单、清晰和直接。
  image:
    src: /xiake.jpg
    alt: Go-Spring 水墨侠客
  actions:
    - theme: brand
      text: 5 分钟快速开始
      link: /cn/docs/1.getting-started/getting-started
    - theme: alt
      text: 了解设计理念
      link: /cn/docs/0.overview/overview
---

<div class="gs-home-cn">

<div class="gs-capability-strip" aria-label="Go-Spring 核心能力">
  <span>配置管理</span>
  <i></i>
  <span>IoC 容器</span>
  <i></i>
  <span>生命周期</span>
  <i></i>
  <span>HTTP 服务</span>
  <i></i>
  <span>结构化日志</span>
  <i></i>
  <span>Starter 生态</span>
</div>

<section class="gs-story">
  <div class="gs-story-copy">
    <p class="gs-kicker">Less plumbing, more product</p>
    <h2><span>服务变复杂，</span><strong>业务代码不必跟着变乱</strong></h2>
    <p class="gs-lead">真实的 Go 服务不只有 Handler。配置加载、组件初始化、依赖装配、启动顺序和优雅退出，会一点点挤进每个项目。</p>
    <p>Go-Spring 把这些重复问题收拢到一致的应用模型中。你仍然编写普通 Go 代码，只是不再手工编排每一块基础设施。</p>
    <a class="gs-text-link" href="/cn/docs/0.overview/overview">为什么需要 Go-Spring <span>→</span></a>
  </div>

  <div class="gs-code-stage">
    <div class="gs-code-window">
      <div class="gs-code-titlebar">
        <span class="gs-window-dots"><i></i><i></i><i></i></span>
        <span>main.go</span>
        <span>Go</span>
      </div>
      <pre><code><span class="gs-token-keyword">package</span> main
<span class="gs-code-blank"> </span>
<span class="gs-token-keyword">import</span> <span class="gs-token-string">"go-spring.org/spring/gs"</span>
<span class="gs-code-blank"> </span>
<span class="gs-token-keyword">func</span> <span class="gs-token-fn">init</span>() {
    gs.<span class="gs-token-fn">Provide</span>(&amp;UserService{})
    gs.<span class="gs-token-fn">Provide</span>(&amp;UserHandler{})
}
<span class="gs-code-blank"> </span>
<span class="gs-token-keyword">func</span> <span class="gs-token-fn">main</span>() {
    gs.<span class="gs-token-fn">Run</span>()
}</code></pre>
      <div class="gs-code-status">
        <span><i></i> application started</span>
        <span>:9090</span>
      </div>
    </div>
    <div class="gs-float-note gs-float-note-top">
      <span>01</span>
      <p><strong>启动时装配</strong>运行时保持直接</p>
    </div>
    <div class="gs-float-note gs-float-note-bottom">
      <span>02</span>
      <p><strong>统一生命周期</strong>启动与退出有序可控</p>
    </div>
  </div>
</section>

<section class="gs-home-section gs-system-section">
  <div class="gs-section-intro">
    <p class="gs-kicker">One application model</p>
    <h2>一套模型，覆盖服务的工程骨架</h2>
    <p>能力彼此协作，而不是一组互不相关的工具。配置决定装配，容器管理依赖，应用上下文协调完整生命周期。</p>
  </div>

  <div class="gs-system-grid">
    <a class="gs-system-card gs-system-card-config" href="/cn/docs/2.guides/01-configuration">
      <div class="gs-card-topline"><span>01 / CONFIG</span><b>→</b></div>
      <h3>配置成为应用的一部分</h3>
      <p>统一加载文件、环境变量、默认值与属性引用，并直接绑定到类型安全的结构体。</p>
      <code>server.port: ${PORT:=9090}</code>
    </a>
    <a class="gs-system-card gs-system-card-ioc" href="/cn/docs/2.guides/02-ioc-container">
      <div class="gs-card-topline"><span>02 / COMPOSE</span><b>→</b></div>
      <h3>依赖关系清楚可见</h3>
      <p>在启动阶段完成 Bean 注册、条件装配和依赖注入，避免把服务定位逻辑散落到业务代码。</p>
      <div class="gs-node-map" aria-hidden="true">
        <span>Handler</span><i></i><span>Service</span><i></i><span>Repo</span>
      </div>
    </a>
    <a class="gs-system-card gs-system-card-runtime" href="/cn/docs/2.guides/03-app-start-stop">
      <div class="gs-card-topline"><span>03 / RUNTIME</span><b>→</b></div>
      <h3>从启动到退出，全程有序</h3>
      <p>Runner、Server 与资源清理遵循统一生命周期，服务启动和优雅关闭不再依赖零散约定。</p>
      <div class="gs-runtime-flow" aria-hidden="true">
        <span>Load</span><i></i><span>Wire</span><i></i><span>Run</span><i></i><span>Stop</span>
      </div>
    </a>
  </div>

  <div class="gs-tool-row">
    <a href="/cn/docs/2.guides/04-logging"><span>LOG</span><strong>结构化日志</strong><small>标签路由与上下文提取</small></a>
    <a href="/cn/docs/2.guides/05-http-server"><span>HTTP</span><strong>标准库兼容</strong><small>中间件与多 Server 管理</small></a>
    <a href="/cn/docs/2.guides/08-http-gen"><span>GEN</span><strong>接口代码生成</strong><small>减少服务端与客户端样板</small></a>
    <a href="/cn/docs/4.integrations/starter-go-redis"><span>START</span><strong>组件快速接入</strong><small>Redis、GORM、PProf</small></a>
  </div>
</section>

<section class="gs-home-section gs-principle-section">
  <div class="gs-principle-heading">
    <p class="gs-kicker">Spring inspired, Go designed</p>
    <h2><span>借鉴 Spring 的工程经验，</span><span>但不把 Java 搬进 Go</span></h2>
  </div>
  <div class="gs-principle-grid">
    <div class="gs-principle-card gs-principle-keep">
      <span class="gs-principle-label">保留</span>
      <h3>成熟的服务组织方式</h3>
      <ul>
        <li><span>01</span>配置驱动的应用装配</li>
        <li><span>02</span>清晰的组件边界与依赖</li>
        <li><span>03</span>可管理的完整生命周期</li>
        <li><span>04</span>可复用的基础设施集成</li>
      </ul>
    </div>
    <div class="gs-principle-card gs-principle-drop">
      <span class="gs-principle-label">舍弃</span>
      <h3>不适合 Go 的复杂度</h3>
      <ul>
        <li><span>×</span>运行时动态代理与扫描</li>
        <li><span>×</span>层层包装的抽象体系</li>
        <li><span>×</span>隐藏执行路径的黑盒魔法</li>
        <li><span>×</span>为框架而框架的设计</li>
      </ul>
    </div>
  </div>
</section>

<section class="gs-home-section gs-ecosystem-section">
  <div class="gs-section-intro gs-section-intro-left">
    <p class="gs-kicker">Ecosystem</p>
    <h2><span>从核心框架开始，</span><br><span>按需扩展</span></h2>
    <p>核心保持克制，日志、工具链和基础设施集成独立演进。只引入当前服务真正需要的部分。</p>
  </div>
  <div class="gs-ecosystem-list">
    <a href="/spring/"><span class="gs-eco-index">01</span><strong>spring</strong><small>应用上下文、IoC 与生命周期</small><b>核心框架</b><i>↗</i></a>
    <a href="/log/"><span class="gs-eco-index">02</span><strong>log</strong><small>配置驱动的结构化日志</small><b>日志模块</b><i>↗</i></a>
    <a href="/gs/"><span class="gs-eco-index">03</span><strong>gs</strong><small>项目初始化与代码生成工具</small><b>工具链</b><i>↗</i></a>
    <a href="/starter-gorm-mysql/"><span class="gs-eco-index">04</span><strong>starters</strong><small>Redis、GORM、PProf 等常用集成</small><b>组件生态</b><i>↗</i></a>
  </div>
</section>

<section class="gs-home-section gs-final-cta">
  <div>
    <p class="gs-kicker">Ready to build</p>
    <h2>让下一个 Go 服务，<br>从清晰的工程骨架开始</h2>
  </div>
  <div class="gs-final-actions">
    <p>从一个可运行的最小应用开始，再逐步接入配置、IoC、HTTP 与常用组件。</p>
    <div>
      <a class="gs-final-primary" href="/cn/docs/1.getting-started/getting-started">开始构建 <span>→</span></a>
      <a href="/cn/docs/3.examples/examples">浏览示例</a>
    </div>
  </div>
</section>

</div>
