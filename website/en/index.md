---
layout: home

hero:
  name: Go-Spring
  text: Put engineering capabilities into Go
  tagline: Configuration, dependencies, lifecycle, and infrastructure are assembled through one model, while your business code stays simple, clear, and direct.
  image:
    src: /xiake.jpg
    alt: Go-Spring ink hero
  actions:
    - theme: brand
      text: Start in 5 minutes
      link: /en/docs/1.getting-started/getting-started
    - theme: alt
      text: Explore the design
      link: /en/docs/0.overview/overview
---

<div class="gs-home-cn">

<div class="gs-capability-strip" aria-label="Go-Spring core capabilities">
  <span>Configuration</span>
  <i></i>
  <span>IoC Container</span>
  <i></i>
  <span>Lifecycle</span>
  <i></i>
  <span>HTTP Server</span>
  <i></i>
  <span>Structured Logging</span>
  <i></i>
  <span>Starter Ecosystem</span>
</div>

<section class="gs-story">
  <div class="gs-story-copy">
    <p class="gs-kicker">Less plumbing, more product</p>
    <h2><span>Services get complex, </span><strong>business code does not have to</strong></h2>
    <p class="gs-lead">Real Go services are more than handlers. Configuration loading, component initialization, dependency wiring, startup order, and graceful shutdown gradually leak into every project.</p>
    <p>Go-Spring gathers those repeated concerns into one consistent application model. You still write plain Go code, but you no longer hand-wire every piece of infrastructure.</p>
    <a class="gs-text-link" href="/en/docs/0.overview/overview">Why Go-Spring exists <span>→</span></a>
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
      <p><strong>Startup wiring</strong>runtime stays direct</p>
    </div>
    <div class="gs-float-note gs-float-note-bottom">
      <span>02</span>
      <p><strong>Unified lifecycle</strong>startup and shutdown stay ordered</p>
    </div>
  </div>
</section>

<section class="gs-home-section gs-system-section">
  <div class="gs-section-intro">
    <p class="gs-kicker">One application model</p>
    <h2>One model for the engineering skeleton of a service</h2>
    <p>The capabilities work together, instead of living as unrelated tools. Configuration drives assembly, the container manages dependencies, and the application context coordinates the full lifecycle.</p>
  </div>

  <div class="gs-system-grid">
    <a class="gs-system-card gs-system-card-config" href="/en/docs/2.guides/01-configuration">
      <div class="gs-card-topline"><span>01 / CONFIG</span><b>→</b></div>
      <h3>Configuration becomes part of the application</h3>
      <p>Load files, environment variables, defaults, and property references in one place, then bind them directly to type-safe structs.</p>
      <code>server.port: ${PORT:=9090}</code>
    </a>
    <a class="gs-system-card gs-system-card-ioc" href="/en/docs/2.guides/02-ioc-container">
      <div class="gs-card-topline"><span>02 / COMPOSE</span><b>→</b></div>
      <h3>Dependencies stay visible</h3>
      <p>Register beans, apply conditions, and inject dependencies at startup, instead of scattering service lookup logic through business code.</p>
      <div class="gs-node-map" aria-hidden="true">
        <span>Handler</span><i></i><span>Service</span><i></i><span>Repo</span>
      </div>
    </a>
    <a class="gs-system-card gs-system-card-runtime" href="/en/docs/2.guides/03-app-start-stop">
      <div class="gs-card-topline"><span>03 / RUNTIME</span><b>→</b></div>
      <h3>Ordered from startup to shutdown</h3>
      <p>Runners, servers, and resource cleanup follow one lifecycle, so service startup and graceful shutdown no longer depend on scattered conventions.</p>
      <div class="gs-runtime-flow" aria-hidden="true">
        <span>Load</span><i></i><span>Wire</span><i></i><span>Run</span><i></i><span>Stop</span>
      </div>
    </a>
  </div>

  <div class="gs-tool-row">
    <a href="/en/docs/2.guides/04-logging"><span>LOG</span><strong>Structured logging</strong><small>Tag routing and context extraction</small></a>
    <a href="/en/docs/2.guides/05-http-server"><span>HTTP</span><strong>Standard library compatible</strong><small>Middleware and multiple servers</small></a>
    <a href="/en/docs/2.guides/08-http-gen"><span>GEN</span><strong>HTTP code generation</strong><small>Less server and client boilerplate</small></a>
    <a href="/en/docs/4.integrations/starter-go-redis"><span>START</span><strong>Fast component adoption</strong><small>Redis, GORM, PProf</small></a>
  </div>
</section>

<section class="gs-home-section gs-principle-section">
  <div class="gs-principle-heading">
    <p class="gs-kicker">Spring inspired, Go designed</p>
    <h2><span>Borrow Spring's engineering experience, </span><span>but do not bring Java into Go</span></h2>
  </div>
  <div class="gs-principle-grid">
    <div class="gs-principle-card gs-principle-keep">
      <span class="gs-principle-label">Keep</span>
      <h3>Mature service organization</h3>
      <ul>
        <li><span>01</span>Configuration-driven application assembly</li>
        <li><span>02</span>Clear component boundaries and dependencies</li>
        <li><span>03</span>A manageable full lifecycle</li>
        <li><span>04</span>Reusable infrastructure integrations</li>
      </ul>
    </div>
    <div class="gs-principle-card gs-principle-drop">
      <span class="gs-principle-label">Drop</span>
      <h3>Complexity that does not fit Go</h3>
      <ul>
        <li><span>×</span>Runtime dynamic proxies and scanning</li>
        <li><span>×</span>Layers of wrapped abstractions</li>
        <li><span>×</span>Black-box magic that hides execution paths</li>
        <li><span>×</span>Framework design for its own sake</li>
      </ul>
    </div>
  </div>
</section>

<section class="gs-home-section gs-ecosystem-section">
  <div class="gs-section-intro gs-section-intro-left">
    <p class="gs-kicker">Ecosystem</p>
    <h2><span>Start with the core framework, </span><br><span>extend only when needed</span></h2>
    <p>The core stays restrained, while logging, tooling, and infrastructure integrations evolve independently. Bring in only what the current service actually needs.</p>
  </div>
  <div class="gs-ecosystem-list">
    <a href="/spring/"><span class="gs-eco-index">01</span><strong>spring</strong><small>Application context, IoC, and lifecycle</small><b>Core framework</b><i>↗</i></a>
    <a href="/log/"><span class="gs-eco-index">02</span><strong>log</strong><small>Configuration-driven structured logging</small><b>Logging module</b><i>↗</i></a>
    <a href="/gs/"><span class="gs-eco-index">03</span><strong>gs</strong><small>Project initialization and code generation</small><b>Toolchain</b><i>↗</i></a>
    <a href="/starter-gorm-mysql/"><span class="gs-eco-index">04</span><strong>starters</strong><small>Redis, GORM, PProf, and common integrations</small><b>Component ecosystem</b><i>↗</i></a>
  </div>
</section>

<section class="gs-home-section gs-final-cta">
  <div>
    <p class="gs-kicker">Ready to build</p>
    <h2>Let your next Go service<br>start from a clear engineering skeleton</h2>
  </div>
  <div class="gs-final-actions">
    <p>Start with a minimal runnable application, then gradually add configuration, IoC, HTTP, and common components.</p>
    <div>
      <a class="gs-final-primary" href="/en/docs/1.getting-started/getting-started">Start building <span>→</span></a>
      <a href="/en/docs/3.examples/examples">Browse examples</a>
    </div>
  </div>
</section>

</div>
