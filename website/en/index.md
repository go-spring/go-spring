---
layout: home

hero:
  name: Go-Spring
  text: Spring-style Go application framework
  tagline: Configuration, dependency injection, component composition, and tooling for Go services.

actions:
  - theme: brand
    text: Get Started
    link: /en/docs/1.getting-started/getting-started
  - theme: alt
    text: Read Guides
    link: /en/docs/2.guides/01-configuration

features:
  - title: IoC Container
    details: Manage beans, dependency injection, and application lifecycle with a familiar component model.
  - title: Configuration
    details: Load, bind, default, and reference properties for configuration-driven service development.
  - title: HTTP Server
    details: Build HTTP services and generate repetitive boilerplate with the Go-Spring toolchain.
---

<section class="gs-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Core Features</p>
    <h2>Core capabilities for production Go services</h2>
    <p>Go-Spring focuses on infrastructure concerns that appear repeatedly in service development, keeping business code clear, direct, and composable.</p>
  </div>
  <div class="gs-grid gs-grid-3">
    <div class="gs-card">
      <h3>Configuration Binding</h3>
      <p>Bind files, environment variables, and defaults to structs to reduce configuration parsing boilerplate.</p>
    </div>
    <div class="gs-card">
      <h3>Dependency Injection</h3>
      <p>Use the IoC container to organize component dependencies, replacements in tests, and lifecycle management.</p>
    </div>
    <div class="gs-card">
      <h3>Component Starters</h3>
      <p>Integrate common components such as Redis, GORM, and PProf with less repeated initialization code.</p>
    </div>
    <div class="gs-card">
      <h3>HTTP Development</h3>
      <p>Build HTTP services, routes, and generated interfaces that fit real service engineering workflows.</p>
    </div>
    <div class="gs-card">
      <h3>Testing Support</h3>
      <p>Test components and application assembly against realistic configuration and service behavior.</p>
    </div>
    <div class="gs-card">
      <h3>Toolchain</h3>
      <p>Use gs tools for project initialization, code generation, and daily development assistance.</p>
    </div>
  </div>
</section>

<section class="gs-section gs-split">
  <div class="gs-section-heading gs-section-heading-left">
    <p class="gs-eyebrow">Why Go-Spring</p>
    <h2>Why choose Go-Spring</h2>
    <p>Go-Spring does not copy Java Spring directly. It keeps Go explicit and lightweight while adding the structure needed for service engineering.</p>
  </div>
  <div class="gs-list-card">
    <div>
      <h3>Familiar programming model</h3>
      <p>Teams with Spring experience can adopt Go service development with a lower engineering learning curve.</p>
    </div>
    <div>
      <h3>Explicit and lightweight</h3>
      <p>The framework favors direct Go code and avoids unnecessary magic around simple application logic.</p>
    </div>
    <div>
      <h3>Production-oriented components</h3>
      <p>Capabilities are built around configuration, components, logging, HTTP, and common middleware integrations.</p>
    </div>
  </div>
</section>

<section class="gs-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Ecosystem</p>
    <h2>Explore the Go-Spring ecosystem</h2>
  </div>
  <div class="gs-grid gs-grid-4">
    <a class="gs-link-card" href="/spring/">spring<span>Core framework</span></a>
    <a class="gs-link-card" href="/log/">log<span>Logging module</span></a>
    <a class="gs-link-card" href="/gs/">gs<span>Command-line tools</span></a>
    <a class="gs-link-card" href="/starter-gorm-mysql/">starters<span>Component integrations</span></a>
  </div>
</section>

<section class="gs-section gs-muted-section">
  <div class="gs-section-heading">
    <p class="gs-eyebrow">Who is it for</p>
    <h2>Built for these teams and use cases</h2>
    <p>If your Go services are facing growing configuration, component assembly, repeated initialization, or HTTP boilerplate, Go-Spring provides a more consistent development experience.</p>
  </div>
  <div class="gs-pill-list">
    <span>Configuration-driven Go services</span>
    <span>Component-based application assembly</span>
    <span>Spring-style development experience</span>
    <span>Generated HTTP service boilerplate</span>
    <span>Middleware starter integrations</span>
  </div>
</section>

<section class="gs-section gs-cta">
  <p class="gs-eyebrow">Get Started</p>
  <h2>Start building with Go-Spring</h2>
  <p>Begin with the getting started guide, then explore configuration, IoC, HTTP, components, and examples.</p>
  <div class="gs-actions">
    <a href="/en/docs/1.getting-started/getting-started">Get Started</a>
    <a href="/en/docs/3.examples/examples">View Examples</a>
  </div>
</section>
