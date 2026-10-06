<!-- # Go-Spring Architecture & Boundaries

[English](ARCHITECTURE.md) | [中文](ARCHITECTURE_CN.md)

This is the **authoritative map** of what lives in each top-level directory, what
must *not* live there, and how new code is routed to the right place. Its job is
to keep the repository from drifting: when in doubt about *where* something
belongs or *whether* it belongs at all, this document decides.

It is a map, not an encyclopedia. Per-module design rules live in their own
`DESIGN.md` files; this document links to them rather than repeating them (see
[CLAUDE.md](CLAUDE.md), "When to Record a Convention": link, don't repeat).

## 1. The Layered Model

The repo has **no `go.mod` at the root**. Every subproject owns its own module
and dependency graph. Modules are organized into layers, and **dependencies flow
one way only — never in reverse**:

```
  foundation        core          ecosystem        integration       tooling
 ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐   ┌─────────┐
 │ stdlib/  │──▶│          │──▶│  cloud/  │──▶│   starter/   │   │   gs/   │
 │ log/     │   │ spring/  │   │          │   │  starter-*   │   │  gs-*   │
 └──────────┘   └──────────┘   └──────────┘   └──────────────┘   └─────────┘
       │              │                │                │
       └──────────────┴────────────────┴────────────────┘
                            ▲
              consumed by demos & template (never depended on in reverse)
        contrib/   examples/   layout/            (+ website/ docs/ scripts/ skills/)
```

Verified dependency facts (do not violate):

- `stdlib/` has **zero third-party dependencies** — standard library only. It is a
  general-purpose utility library (a completion of the Go standard library:
  types, encoding, collections, ...); beyond utilities it also hosts **pure
  semantic pieces with no ecosystem dependency** (`httpsvr`, `httpclt`).
  Adding a subpackage needs **all three**: the pattern appears twice or more in
  the repo (an extraction, not a speculation), the helpers share one verb (one
  package, one concern — no grab-bag), and the standard library genuinely lacks
  it. Utility package names point at the standard-library package they wrap
  (`timeutil`→`time`, `md5util`→`md5`, `randutil`→`crypto/rand`).
- `log/` depends on `stdlib/` (plus an ANTLR parser for its config grammar); it is
  a foundation module, not part of `spring`.
- `spring/` depends on `log/` and `stdlib/` only, and contains **only the pure
  core**: `gs` (IoC container, lifecycle) and `conf` (layered configuration
  engine). No third-party business package, no capability families.
- `cloud/` (module `go-spring.org/cloud`) is the **ecosystem abstraction
  library**: governance (resilience/fault/traffic), discovery, loadbalance,
  cache, repository, migration, i18n, validation, event/scheduling/batch,
  lock, messaging, transaction, actuator, ... The **capability abstractions are
  container-free** — their types, drivers and seams work with or without a
  container — but the packages that own a default bean register it, so `spring`
  appears in four of them:

  - `cloud/resilience` registers `*resilience.Manager` (and the bundled driver),
    `cloud/loadbalance` registers `*loadbalance.Manager`, `cloud/fault` registers
    `*fault.Injector`. The rule is **the package that owns the type registers its
    default**: a client that injects one has already imported that package for
    the type, so there is no separate "governance import" to forget.
  - `cloud/governance` registers the center over those authorities and the one
    bean that wires it into gs, and its `rules.go` binds a rules document through
    `spring/conf`'s value-tag machinery (the document format is the framework's,
    and a per-backend parser would let the backends drift apart).

  Everything else in `cloud/` stays gs-free, and gs wiring for any other family
  still belongs in a starter (e.g. `starter-cache`).
- `starter-*` and `gs-*` sit on top and may pull third-party packages.
- Nothing in a lower layer may import a higher layer. A `starter` importing
  another `starter`, or `spring`/`cloud` importing a `starter`, is a layering
  violation. The rule of thumb: **abstractions go in `cloud`, third-party SDKs
  and gs wiring go in a starter, pure semantics go in `stdlib`**.

**Internal deps resolve through `go.work`, never `require`.** Adding a `require`
on an in-workspace module sends `go mod tidy` to the proxy and 404s. See the
[go.work](go.work) `use` list for the full module set.

## 2. Directory Responsibility Matrix

| Directory | Layer | Purpose (one line) | Belongs here | Does **not** belong here | Deep dive |
|---|---|---|---|---|---|
| `stdlib/` | foundation | Zero-dependency general-purpose utilities (a completion of the Go standard library) | Pure Go helpers — types, encoding, collections, hashing, text, ... | Any third-party import; capability abstractions / driver registries (those live in `spring/`); container/DI logic | [stdlib/README.md](stdlib/README.md) |
| `log/` | foundation | Structured logging model, config grammar, adapters | The logging model, appenders, field encoding, log config parser | Business logging; hard deps on `spring` | [log/DESIGN.md](log/README.md) |
| `spring/` | core | IoC container, dependency injection, app lifecycle, layered config engine — the pure core (`gs` + `conf`) | Bean model, injection, start/stop state machine, config binding/refresh | Third-party business packages; capability abstractions (those live in `cloud/`); integration code that wires a real backend | [spring/DESIGN.md](spring/README.md) |
| `cloud/` | ecosystem | Capability abstractions: governance, discovery, cache, repository, i18n/validation, ... | Ecosystem interfaces + driver seams usable with or without the container; the four packages that own a default bean register it (`resilience`/`loadbalance`/`fault`/`governance`) | Spring imports beyond those four; third-party SDKs; gs wiring for any other family (that belongs in a starter) | [cloud/](cloud/) per-family docs |
| `starter/` | integration | One module per third-party service/framework, wired into the IoC container | `starter-*` modules following the five archetypes; the family design guide | Business logic; deployment scaffolding; a *new* shared-helper package living only to serve starters (use an existing natural home instead - see trap below) | [starter/DESIGN.md](starter/DESIGN.md) |
| `gs/` | tooling | Dev tools: scaffolding (`gs`), GUI, code generation (`gs-http-gen`), mocking (`gs-mock`) | CLI/codegen/tooling that operates *on* projects | Runtime framework code; anything imported by a running app | [gs/README.md](gs/README.md) |
| `contrib/` | demo | Runnable examples showing how third-party frameworks are wired the Go-Spring way | Per-framework runnable variants; smoke tests | Reusable modules (those become `starter-*`); deployment scaffolding | [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) |
| `examples/` | demo | End-to-end sample applications built only from published starters | Reference apps (fullstack, bookman, ...) that *consume* the framework | New framework capabilities; code an app shouldn't need to copy | [examples/examples.md](examples/examples.md) |
| `layout/` | template | The project skeleton `gs init` stamps out | Template files, agent rules, per-protocol IDL layout | Framework implementation; anything not meant to be copied into a user project | [layout/DESIGN.en.md](layout/DESIGN.en.md) |
| `website/` | site | Documentation site **source** (Node.js) | Markdown content, site config, assets | Build output (that is `docs/`) | — |
| `docs/` | site | **Published** site output (GitHub Pages; has `CNAME`) | Generated HTML/assets | Hand-authored source (edit `website/` instead) | — |
| `scripts/` | ops | Repo-maintenance scripts | Module checks, release, history audit | App runtime code; per-project build scripts | — |
| `skills/` | agent | Agent skills shipped with the repo (e.g. `gs`) | Skill definitions | Runtime framework code | — |

## 3. Where Does New Code Go? (decision guide)

Answer top-down; the first match wins.

1. **Is it a runnable demo or reference app, not meant to be imported?**
   - Demonstrates a third-party framework wiring → `contrib/<framework>/<variant>/`
   - End-to-end app built from existing starters → `examples/`
2. **Does it integrate a specific third-party service/framework** (Redis, GORM,
   Kafka, a web/RPC framework, a config center, ...)?
   → a `starter-*` module under `starter/`. Pick the archetype from
   [starter/DESIGN.md §2](starter/DESIGN.md) — it fixes lifecycle, port, and
   config-prefix behavior.
3. **Is it a dev-time tool** (scaffolding, codegen, mocking, GUI) that operates
   *on* projects rather than running inside them? → `gs/gs-*`.
4. **Is it container / DI / lifecycle / config logic?** → `spring/` (a
   subpackage of `gs`/`conf`). **Is it a capability abstraction** (interface +
   driver seam, e.g. cache, governance, discovery) with no third-party business
   dependency? → `cloud/`. If it needs a third-party import, the abstraction
   stays in `cloud` but the concrete backend — and the gs wiring — belong in a
   `starter`.
5. **Is it a reusable general-purpose utility** (types, encoding, collections,
   ...) with **zero third-party dependencies** and no framework/capability
   concern? → `stdlib/` (or `log/` if it is logging).
6. **Is it documentation?** Author in `website/`; never hand-edit `docs/`.

Two recurring traps:

- *"I'll just add a small helper shared by two starters."* No — cross-starter
  shared helper packages are disallowed when no natural home exists
  ([starter/DESIGN.md §3](starter/DESIGN.md)). First check whether one of the
  existing natural homes covers it - `cloud/security` (TLS config),
  `cloud/actuator/health.NewIndicator` (health indicator factory),
  `stdlib/errutil.RequireField`/`RequireAny` (fail-fast validation). If yes,
  import it. If no, inline per-starter; a shared package materializes only
  when its nature points at a real home, not when two starters happen to
  share a snippet.
- *"This abstraction needs a Redis client, I'll put it in stdlib."* No — two
  things are wrong. It is not a pure utility (it is a capability abstraction, so
  its home is `spring/`, not `stdlib/`), and the moment a third-party import is
  required it cannot live in either foundation layer. The pattern is:
  **abstraction + driver registry in `spring/`, concrete backend in a `starter`**
  (see `cloud/cache`, `cloud/lock`, `cloud/discovery`).

## 4. Scope Red Lines (non-goals)

These are deliberate limits. Expanding past them is drift, not progress.

- **`stdlib/` stays dependency-free.** The value of the foundation layer is that
  any module can use it without inheriting a dependency graph. A single
  third-party import defeats the purpose.
- **`spring/` is not a web framework.** The built-in HTTP server intentionally
  does **not** provide a framework-level context object, parameter binding /
  response auto-serialization, route grouping or priority, or template rendering.
  Those belong in a web-framework `starter` (gin/echo/hertz/...). See
  [OUTLINE.md](OUTLINE.md) §五 "内置 HTTP Server".
- **`starter-*` is integration only.** A starter wires *one* third-party
  service/framework into the container and lifecycle — no business logic, no
  deployment scaffolding, no cross-starter abstractions.
- **`contrib/` and `examples/` are demos, not products.** They exist for smoke
  testing and integration demonstration. Do not add deployment scaffolding
  (`build.sh`, `bootstrap.sh`, extra `script/` dirs); keep to source +
  `smoke-test.sh` / `check.sh` / `gen.sh`.
- **Prefer framework-native mechanisms; unify only where none exists.** Do not
  layer a Go-Spring abstraction over a capability each framework already ships
  (e.g. RPC provider registration). The reasoning and the current
  have-native-vs-candidate breakdown are in
  [starter/DESIGN.md §3](starter/DESIGN.md).

The following are decided **not to be built** — do not propose them again:

- Annotation caching (`@Cacheable`) with L1/L2 multi-level cache, and
  region/zone multi-level affinity (2026-08-27).
- Not done for ecosystem reasons: sharding, a generalized Cloud Bus, batch
  skip/flow, cluster rate limiting / hot-parameter limiting, actuator
  `/shutdown` and `/mappings`, a canary-release platform, persistent scheduling
  / misfire, and an HTTP-pull config source.
- The filter: skip it if the ecosystem is outdated, if another ecosystem
  already does it, or if it is unrelated to the Go ecosystem.
- Cloud-audit verdict **"leave as-is"** (do not re-raise): `health.Indicator`'s
  four fields (`Name` / `Probe` / `Groups` / `Optional`), the podinfo directory
  ownership, `traffic.Propagator`'s customizable key, resilience's dual registry
  not merging types, and contract's pure-HTTP shape (2026-08-26).

## 5. Extensibility Is the Framework's Contract

Go-Spring exists to serve every team's full range of scenarios; it cannot ship a
fixed feature set and hope it fits. So in the framework layers — `stdlib/`,
`spring/`, `starter/` — **extension points are not optional**:

- **Every capability leaves a seam.** A capability abstraction defines the
  interface + driver registry; concrete behavior plugs in behind it. A scenario
  the framework didn't anticipate must still have a way in — otherwise the
  architecture fails by omission, not by a visible bug.
- **Built-ins ride the same seams they expose.** Go-Spring's own built-in
  implementations must go through the very extension points offered to users,
  never a privileged private path — `cloud/cache`'s Memory backend,
  `cloud/resilience`'s built-in strategies, and the starter archetypes all
  consume their own registries/interfaces. If a built-in can't be expressed
  through the public seam, the seam is wrong, not the built-in.
- **An extension is a layer, not a table entry.** Prefer the onion: wrap the
  thing you extend (`WrapClientExecutor(inner, ...)`, `Observe(inner Driver, ...)`,
  a middleware chain) instead of registering a name in a global map, and instead
  of every call site writing into one package-level state object. Layers compose
  by nesting — retry → circuit breaker → bulkhead → observe — each knowing only
  the interface it wraps, and each owning the state it accumulates: that state's
  lifetime is the wrapped object's lifetime, so there is nothing global to reset
  and no way for two instances in one process to contaminate each other. A
  registry keeps its place as the *selection* mechanism (a config value picks an
  implementation), but what it hands back has to be a layer carrying the behavior
  and the state, not a global that the behavior writes into. The client-side
  embodiment of this rule is the hollowed client chain — a client whose library
  delivers a concrete, hook-less type embeds an `Inner*` interface (identity
  layer over governance layer over raw adapter) instead of holding the raw
  instance privately; see [starter/DESIGN.md §2.2](starter/DESIGN.md).
- **A layer owns its state; it may still read through a shared instrument set.**
  The rule above is about the state a layer accumulates — that state's lifetime is
  the wrapped object's. Its *instruments* are a different thing: the OTel SDK
  already keys an instrument by name/description/unit/kind process-wide, so a
  per-instance copy is invisible at best and, for an observable gauge, actively
  wrong. A layer therefore resolves one shared, immutable instrument set — once
  per process — while still owning every piece of state it accumulates. The rules
  that go with it (how a gauge is registered, who unregisters it) are in
  [starter/DESIGN.md §3](starter/DESIGN.md).
- **A layer only sees what crosses the seam it wraps, so wrap the lowest thing
  every path crosses.** If a path bypasses the interface — a library's internal
  retry, a self-healing re-registration that never re-enters `Register` — a layer
  around that interface silently misses it, and silent missing coverage is worse
  than having no layer at all. Wrap the primitive both paths funnel through, or
  read the bypass as evidence that the seam sits in the wrong place — the same
  verdict as "if a built-in can't be expressed through the public seam, the seam
  is wrong, not the built-in".
- **This is a framework-layer duty, not a universal one.** Downstream business
  code (apps stamped from `layout/`, and `examples/` / `contrib/`) follows YAGNI
  instead: leave a seam only when a real second case crosses the line (see the
  coding-style guide, "Extensibility and Extension Points"). The framework's line
  is crossed almost by definition; a business app's rarely is.
- **Aggregator starters are the consumers that keep this honest.** A company
  baseline starter (`starter-luohua`, DESIGN §2.6) re-bases a whole fleet onto one
  organization's conventions by composing other starters *through these same
  seams* — it is the first external consumer of every seam, so a seam that only
  its own built-in can use is caught the moment an aggregator needs it.

The concrete extension-point shapes (driver registry, seam interface,
Provider/Contributor, functional hook) and the "abstraction in `spring`, backend
in `starter`" rule are catalogued in §2–§3 above and in
[starter/DESIGN.md §2](starter/DESIGN.md).

## 6. Design Principles

The rules below decide *what the framework is allowed to do*, independent of any
single module. Per-module mechanism detail lives in that module's doc; this
section holds the cross-cutting verdicts and the decisions already made.

### 6.1 The container only assembles

The IoC container assembles; it does not adjudicate. To evaluate an extension
point, ask: does it solve a real problem for the *user*, or a problem an earlier
design decision created? A three-level cache to break circular dependencies
solves a self-inflicted problem (injection by type) — not legitimate. A family
collection point answers a real need — legitimate.

- **No `@Primary` / priority markers.** Multiple candidates are expressed with
  "inject by name wins" plus a condition (`OnMissingBean` / `OnSingleBean` /
  `OnProperty`), which decides *whether a bean exists*, not *which of several
  wins*. Do not propose a container-level `@Primary` or "auto-pick one when
  several beans match a type" — the answer is: name the injection point, or add
  a condition.
- **Rejected outright, on the same ground:** runtime scanning, compile-time DI,
  and implicit interface indexing. These are instances of "assemble, don't
  adjudicate".
- **No `autoconfig.exclude`.** Starters register under a configuration-key
  condition and are off by default (no config = no assembly). There is no
  exclude / blacklist / disable-list mechanism; a new starter must gate
  registration on a config key, `init()` must never `Provide` unconditionally,
  and a component that should be on by default needs explicit user sign-off plus
  an explicit `enabled` switch (the pprof precedent).
- **A starter that offers a default bean must gate it on `OnMissingBean` /
  `OnProperty`.**

### 6.2 The onion, applied

§5 states the onion as the framework's extensibility contract. The families that
embody it: `cloud/lock.Observe`, `observability.WrapClientExecutor` /
`WrapServerExecutor`, `fault.WrapClientExecutor`.

- **Killed by this principle (do not revive):** the `actuator/endpoint` `serving`
  global signal (`MarkServing()` / `IsServing()`) was deleted wholesale; do not
  add it back under a "startup detection" name. An `*endpoint.Endpoint` bean
  nobody collects is simply not assembled and stays silent (2026-10-03).

### 6.3 Method-level cross-cutting concerns

Method-level concerns that wrap a business method (transactions, security) do
**not** get a shared interceptor-protocol package. Each family carries its own
minimal decorator shape, composed by ordinary function nesting (the middleware
pattern) — `cloud/experimental/aspect` was deleted for this reason.

- Do not propose any "unified interceptor chain / AOP protocol" package,
  including for future method-level governance / idempotency / method-level
  locks — when such a case appears, that family carries its own decorator.
- New method-level cross-cutting code copies the shape and documentation wording
  of `GlobalTransactional` (whose doc states explicitly "deliberately no shared
  interceptor-chain protocol").
- Before judging whether an abstraction should exist, check its actual usage
  (call-site count, parameter count) — do not trust a documented composition
  story.

### 6.4 Ecosystem mainline: batteries included + specializable

go-spring is a complete, batteries-included ecosystem (usable standalone with no
company extension) *and* is designed to be specialized wholesale into a
company's own ecosystem through extension — without forking, and without
hard-coding any one company's defaults. Everything is designed and implemented
around this principle (2026-09-06).

- The invariant that holds at every layer (recursively across framework →
  company umbrella → app): everyone has a default; every default enters through
  a public seam; every default can be explicitly overridden by a higher layer
  (defaults always yield via `OnMissingBean` and are gated by `OnProperty`; an
  app's explicit bean/key overrides). Contracts live in the neutral layer;
  defaults live in the replaceable layer.
- In one line: there may be many layers, but each layer does exactly one thing —
  provide a default and leave a seam.
- A standard component must always leave a customization slot; a default
  implementation must go through the exposed seam, never a private privileged
  path.
- Every default / every seam is annotated with "which is the standard component
  + where the company's customization slot is" (three states: framework-owned /
  company-mandatory / config-declared).
- Coexistence transition: any capability that crosses a process boundary and has
  an existing company convention must be able to "read the old on the way in and
  write the old on the way out", with business code reading only the neutral
  in-process contract, unchanged.
- Abstraction criterion (all three required): the capability (a) has an existing
  company convention ∧ (b) will coexist old-and-new ∧ (c) has a real neutral
  contract — otherwise it is premature abstraction (this is why the repo rejects
  `aspect` and a shared interceptor).
- Gold-standard shape: a neutral contract (what business reads) + a
  wholesale-replaceable implementation (registry / `LastWriteWins` /
  `OnMissingBean` / config-selected); the boundary does only old↔new translation.
- Every default / extension must carry actual, observable, verifiable behavior;
  a pure alias / empty shell is inert, so problems go undiscovered.

### 6.5 Wrap mature components

Prefer wrapping a mature component over building your own. Maintain an
abstraction layer only where a domain has multiple interchangeable components
(2026-09-01).

- An abstraction layer is justified only by at least two interchangeable
  implementations (or a clear multi-backend roadmap); with a single backend,
  wrap that component directly.
- "go-spring builds its own X" needs an argument that no mature component exists
  in that domain; a mature component's long-tested edges (dialects, concurrency,
  locking) mean rebuilding pays the tuition again.
- Cloud-native trade-off: a registry that only registers and does not resolve is
  acceptable (do not add Discovery backends for etcd/consul/nacos/zk);
  gRPC / thrift protocol work need not be complete — lean on mature frameworks
  (dubbo-go, kitex, ...) that ship a full RPC ecosystem (2026-08-11).
- **Do not connect go-spring governance to components that ship their own
  governance ecosystem** (2026-09-12). Criterion: does the component ship its
  own governance / observability ecosystem (connecting would duplicate or fight
  it)? Check whether its starter exposes the framework's own
  observer / registry / middleware / filter config blocks.
  - **Not connected** (have their own): dubbo, trpc, kitex, hertz, kratos,
    go-zero, goframe.
  - **Connected** (thin wrapper; no framework-side governance layer): gin, echo,
    grpc, thrift, http-server, websocket.
  - Do not propose adding governance to dubbo's inbound side, or to
    trpc / kitex / hertz / kratos / go-zero / goframe.
  - Boundary: this decision only constrains "should we add more" — it does
    **not** retroactively remove what exists (dubbo outbound timeout/retries
    bridging, and `fault` wiring on these components, are done facts with tests)
    (2026-09-12).
  - `starter-websocket` only provides a configured `*websocket.Upgrader` bean
    and owns no server; the upgrade request goes through the app-chosen HTTP
    server.

### 6.6 The stability quintuplet

Every component in the ecosystem must meet a stability baseline of five:
① observability ② service discovery ③ load balancing ④ fault injection
⑤ service governance (2026-10-01).

- Shape = two primitives + three services: ① and ④ never change the normal-path
  behavior (① is the eyes, ④ is the hand — and without ① you cannot verify ④);
  ②③⑤ are the addressing and protection of every hop.
- To review a component, ask three questions: can it be seen? can it be
  fault-injected? can it be governed? — ① requires exposing observation points,
  ④ requires exposing injection points (together = must be verifiable); ②③⑤ =
  must be governable.
- Naming caution: "service governance" is used at two levels (the whole bundle's
  nickname / block ⑤); in docs call block ⑤ **resilience** (the code name), to
  avoid ambiguity.
- ③ vs ⑤ divide by *output*, not input: both consume runtime feedback; ③'s
  output is one target address (where), ⑤'s output is whether/how to make this
  call. Wiring ≠ conceptual classification (`Governance` carrying ③④⑤ is the
  container's authoritative wiring fact, not a claim they are one concept).
- Security is not in the quintuplet, on the premise of "internal trust";
  criterion = does it have a per-hop decision — cross-tenant / cross-BU /
  externally exposed / compliance requiring per-hop identity ⇒ either answer
  "config" or promote it to a sixth block.
- Configuration is not in the quintuplet (delivery is not a capability), but
  every block's thresholds / policies / fallbacks are config-driven (the
  governance center's policy source + hot reload); the docs must say this.
- In one line: the quintuplet is a capability, entirely driven by configuration
  (the governance center).

### 6.7 Dependency direction and abstraction trimming

- **User code must not use types from a starter.** If a third-party package
  wrapped by gs has a corresponding abstraction in `cloud/`, the user always
  uses the `cloud` type (triggers, options, worker functions); only when the
  third-party package has no `cloud` counterpart may the user touch starter
  types (2026-09-23).
- A starter's own job = provide the Server / bean wiring, bridge beans by name,
  and manage lifecycle.
- When auditing a starter's user-facing API, ask of each type "does `cloud` have
  this?" — if so, take the `cloud` type as the parameter; the starter does not
  rename-re-export.
- An API should not make the user memorize one more intermediate type: take the
  dependency package's own type directly (e.g.
  `scheduling.WithLock(l lock.Locker, key string, ttl time.Duration)`), never an
  adapter / minimal interface invented to "keep this package zero-dependency".
- Delete public abstractions with zero consumers: single-implementation
  interfaces, byte-identical pass-through adapters, inputs with no consumer.
  "No consumer" must be counted down to bean uniqueness, not just business reads
  (a multi-instance starter registering `health.Indicator` with `.Name(...)`
  has a real consumer — the container's dedup — do not delete it) (2026-09-23).
- Audited and kept: `cache.Codec` (a deliberate user extension point),
  `messaging.Driver` (multi-backend contract), each starter's Driver interface
  (multi-implementation convention).

### 6.8 Global registries and bean escape

- The real anti-pattern: a container-created instance escaping into a
  package-level global (a bean ctor self-registering + a global `Get` service
  locator). On this ground `cloud/messaging`'s `RegisterDriver`/`GetDriver` and
  `cloud/security`'s `RegisterValidator`/`GetValidator` were deleted (moved to
  beans), and `starter-casbin`'s `RegisterAdapter`/`Watcher` became config
  `adapter`/`watcher` = bean names (2026-09-08).
- **Kept, unchanged** (legitimate static plugin directories, not bean escape):
  `starter-gateway`'s `RegisterFilter(name, FilterFactory)`, `starter-otel`'s
  `RegisterSpanExporter`/`RegisterMeterExporter`/`RegisterPropagator` — all
  package-level registrations of type-level constructors, not
  container-instance escape. Do not propose deleting them (2026-09-08).
- Deleting public API has a cost: `cloud/messaging` / `cloud/security` are
  released `go-spring.org/cloud` modules, so removing public API needs a version
  bump.
- New criterion (superseding "a package init registering a type-level
  constructor is legitimate"): when the same contract already has an
  in-container precedent (`map[string]discovery.Discovery`, a named Driver bean,
  a `governance.Source` injected into the same wiring bean), a global registry is
  a leftover, not a legitimate form (2026-09-13).
- The config-source family (`conf.RegisterProvider` in init) is not a
  to-be-governed escape but a structural seam: `spring.config.import` is parsed
  before the container exists (the container is built from this config — chicken
  and egg), so the cross-phase stitch can only be process-level. Do not propose
  making it per-container. Package-level mutable `xxxController` variables have
  all been deleted (8 sites became an init closure calling `(&xxxCtrl{}).Load`, a
  method-value closure holding state).
- `dubbo`'s mapconfig `singleton` stays: it adapts to dubbo-go's process-level
  global world (gs's write and the dubbo extension factory's read must be the
  same instance).
- By-name optional bean injection: `TagArg(name)` injects by name; to pass nil
  for an empty key use `ValueArg((persist.Adapter)(nil))` (a typed nil
  interface), not `TagArg("?")` (by-type nullable will grab an unrelated bean).
- `ModuleFunc`'s parameter is fixed as `(r,p)`; `BeanProvider` only has
  `Provide` — there is no imperative by-name bean lookup; every by-name
  injection goes through the ctor's `TagArg`.

### 6.9 The `experimental/` directory

- `experimental/` means "the maintainer has not yet reviewed it", not
  "immature / substandard". Completing a capability does not target "moving out
  of experimental".
- To promote an experimental module, add tests, run the example smoke, and
  finish the docs — do not change the directory or do a "GA relocation"
  refactor; whether it moves out is the user's review call.
- Moving is two-way: the experimental area is neither exit-only nor a promotion
  queue; whether something moves is decided by the user family by family — do
  not infer it.
- Do not infer from "the core is in `cloud/`, not experimental ⇒ the starter
  belongs in `starter/`" — that rule does not hold.
- The experimental area holds both "unreviewed" and "non-production tool"; that
  ambiguity is not yet adjudicated — do not write a contract for it or move
  things on your own.
- Moving a module carries: `go.work` paths, `replace` relative depth, and every
  module's USAGE/README self-reference paths and imports repo-wide.

### 6.10 Maintainer tooling vs. the user surface

- go-spring's own maintainer tooling and the tooling for user projects must be
  separate: do not register maintainer / in-repo commands in the `gs` builtins
  map (`gs/gs/main.go`, which feeds `gs --help`).
- Maintainer tooling lives in `scripts/` (when real Go logic is needed, write a
  `package main` under `scripts/<tool>/` and run it with `go run`, e.g.
  `scripts/bomtool`). Note `gs/gs/cmd/` is `package cmd`, the user subcommand
  library, not a maintainer slot. Mark a maintainer tool in its cobra Short/Long:
  "maintainer-only; not part of the gs user toolkit".
- BOM version governance (`versions.yaml` baseline, `scripts/versions.sh
  {check|diff|apply <module>}` wrapping `scripts/bomtool`) is maintainer-only and
  does not enter the user-visible `gs` binary.
- If maintainer tooling is later shipped to external adopters, use the
  `gs-<name>` external-tool model (`main.go` `tool.Call`, discovering `gs-*` on
  PATH), not a built-in subcommand.
- Before adding any command to builtins, ask: is this go-spring's own
  maintenance, or the user's project?

### 6.11 Standing decisions

Cross-module decisions already made, recorded here so they are not re-litigated;
the mechanism detail lives in the owning module's doc.

- **MQ binder paths go through the same guard/executor seam as the direct API**
  (`Manager.ClientExecutorFor`); governance applies by target resource, not by
  call path; the per-instance switch is on by default and can be turned off
  (2026-08-28).
- **Registration self-healing:** after an etcd lease keepalive death / zk session
  expiry, re-register automatically (backoff 1s doubling to 1min, exit only on
  `Close`); consul self-recovers via `UpdateTTL` and needs no change (2026-08-28).
  Skeleton preference: prefer a family-shared skeleton; if starters implement it
  independently, write a small local helper rather than a new cross-module
  dependency.
- **casbin:** `policy` and `adapter` are mutually exclusive, fail fast — *unless*
  the docs explicitly support a "file load + library persist" mode, in which case
  do three states (2026-08-28).
- **General rule:** delete dead config, fail loudly on semantic conflicts, turn
  silent failures into WARN / ERROR (2026-08-28).
- **Observability pipeline ownership:** protocol starters always connect to the
  `starter-otel` global pipeline and never build their own; applies to any future
  protocol starter (2026-08-28).
- **Admin-plane auth = a shared guard** (`cloud/security.Guard`, no JWT); the
  admin-ui activation semantic = configuring `addr` activates it.
- The "governance is call-path-independent" extension: the guard covers the
  unified "transparent protection per request" posture.

## 7. Related Documents

- [CLAUDE.md](CLAUDE.md) — when to record a convention; output & coding rules.
- [starter/DESIGN.md](starter/DESIGN.md) — the five starter archetypes and every
  cross-cutting constraint (the deepest ruleset in the repo).
- [contrib/DIRECTORY_CONVENTIONS.md](contrib/DIRECTORY_CONVENTIONS.md) — contrib
  example layout and naming.
- [spring/DESIGN.md](spring/README.md), [log/DESIGN.md](log/README.md),
  [layout/DESIGN.en.md](layout/DESIGN.en.md) — per-module internal design.
- [layout/docs/agent-rules/common-rules.en.md](layout/docs/agent-rules/common-rules.en.md)
  — shared design/coding/testing rules for projects built on Go-Spring.
- [MANIFESTO.md](MANIFESTO.md) — the long-term "Process as Code" direction. -->
