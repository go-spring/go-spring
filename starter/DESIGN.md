# Go-Spring Starter Design Guide

[English](DESIGN.md) | [中文](DESIGN_CN.md)

Design constraints that every official Go-Spring starter follows — to keep the
family consistent and guide anyone adding a new starter. For the domain-based
catalog of what exists today, see [README.md](README.md).

A starter is an *integration module*: it wires one third-party service or
framework into the Go-Spring IoC container and server lifecycle, and nothing
more. Business logic, deployment scaffolding, and cross-starter abstractions do
not belong here.

## 1. Module Layout

- **One starter, one Go module.** Each starter owns its own module and dependency
  graph, so a Redis app never pulls in Kafka's transitive dependencies.
- **Fixed file skeleton.** A starter directory contains `starter.go` (bean
  registration + lifecycle), `config.go` (the bound `Config` struct and any
  driver registry), `README.md` / `README_CN.md`, and an `example/` module that
  exists for smoke tests and integration only — no `build.sh` / `bootstrap.sh` /
  deployment scaffolding, only `check.sh` / `gen.sh` / source. Config-provider
  starters (§2.5) vary this: they carry `provider.go` instead of `config.go`
  (no bound `Config` — connection parameters are parsed from the import source
  string), and their smoke module is `example-config/`.
- **Apache License header** on every source file (see
  [../LICENSE_HEADER](../LICENSE_HEADER)).
- **`example/` owns its own `go.mod`.** Every starter example is a standalone Go
  module (added to `go.work`), not a package within the starter module. Its module
  path follows the directory structure: `go-spring.org/<starter-name>/example`.
  Internal deps resolve through `go.work`, never `require`.
- **Repo-wide module rules apply** (no root `go.mod`; one module per subproject;
  internal deps resolve through `go.work`, never `require`). These are owned by
  [../ARCHITECTURE.md §1](../ARCHITECTURE.md); adding a `require` on an in-workspace
  module sends `go mod tidy` to the proxy and 404s.

## 2. The Five Archetypes

Every starter falls into exactly one of five shapes. Its shape dictates its
lifecycle, its port behavior, and how the application consumes it.

### 2.1 Server starters (own a listener)

Web (`gin`, `echo`, `hertz`, ...) and RPC (`grpc`, `kitex`, `thrift`,
`dubbo`, ...) starters own a network listener and plug into the Go-Spring server
lifecycle by exporting a `gs.Server` bean.

- **Each server binds its own port, and the port must be explicitly configured.** A server starter reads a distinct address from its own `Config` (e.g. `${spring.grpc.server}` → `addr`). No default value is set — the `addr` tag is written as `value:"${addr}"` without `:=`. The port configuration itself is the server bean's startup gate: the bean is only created when `OnProperty` detects the address is configured (`Condition(gs.OnProperty("spring.<x>.server.addr"))`). Two server starters in one process must not share a port; the application assigns non-conflicting addresses. Contributor starters (§2.3) deliberately do *not* open a port — they mount onto a server the app already runs. The pprof server is the exception and uses a unified default port `:9981`.
- **Listen early, serve on the ready signal.** `Run(ctx, sig)` binds the
  listener immediately so a port conflict fails startup, then blocks on
  `<-sig.TriggerAndWait()` before `Serve`. This guarantees the socket is bound
  before Go-Spring reports readiness, but no traffic is served until all beans
  are wired.
- **Graceful shutdown in `Stop()`.** HTTP servers call `Shutdown`; RPC servers
  call `GracefulStop`.
- **The app owns routes, the starter owns the server.** The application supplies
  a register function bean (`RouterRegister`, `ServiceRegister`,
  `HandlerRegister`, ...); the starter creates and configures the engine and its
  transport. Registration is the seam.
- **No `gs.Module` wrapper.** Server bean registration is a direct `gs.Provide(...)` call in `init()`. The port configuration itself is the startup gate — no `enabled` toggle or `OnBean[ServiceRegister]` condition is needed.

### 2.2 Client starters (driver mode + multi-instance)

Database, cache, and message-queue clients (`go-redis`, `gorm-*`, `mongodb`,
`kafka`, `nats`, ...) connect *out* to an external service.

- **Multi-instance only, via `gs.Group` / `gs.Module`.** Client starters do
  **not** register a default singleton bean. They bind a
  `map[string]Config` under the prefix and register one named bean per entry.
  Reason: the default-singleton + multi-instance dual registration was
  error-prone and the conditional singleton semantics were opaque. The
  application selects an instance by name (`autowire:"a"`), and adding a second
  instance is a pure-config change.
- **Address is required — fail fast.** A client must never silently fall back to
  `localhost`. Fields default to empty (`${addr:=}`). Single-field validation
  uses the `expr` tag (e.g. `expr:"$ != ''"` on a string, `expr:"len($) > 0"`
  on a slice), which fails at config-binding time. Cross-field rules ("addr OR
  service-name") use `errutil.RequireAny` in the constructor, since `expr` cannot
  express cross-field constraints.
- **Driver pattern for pluggable backends.** A client exposes a `Driver`
  interface plus a package-level registry (`RegisterDriver`, panic on
  dup/empty/nil). `DefaultDriver` ships built in; a company can register its own
  driver and select it with `${driver:=...}` without forking the starter. This
  is also the seam through which service discovery is injected (the driver builds
  the dialer). Optional capabilities go on *separate* interfaces (e.g. go-redis's
  `ClusterDriver`) so existing custom drivers keep compiling.
- **Startup connection check.** Where the client library allows it, the
  constructor performs a bounded probe (e.g. Redis `PING` with `DialTimeout`) so
  a misconfiguration surfaces at boot, not on first request.
- **Every instance has a `Destroy`.** Each bean registers a destructor that
  `Close()`s the connection and stops any background goroutine or discovery
  watch behind it. Missing destroy hooks were a known gap and are now required.
- **One concern, one file — a standard client-starter skeleton.** A client
  starter splits its cross-cutting concerns across dedicated files rather than
  piling them into `config.go` / `starter.go`, so the wiring for each capability
  lives where a maintainer expects it:
  - `config.go` — the `Config` struct, the `Driver` interface, and the
    driver-registry (`RegisterDriver`, `init()`). Connection-creation only.
  - `starter.go` — the `init()`-time `gs.Group` / `gs.Module` registration and
    the constructor (`newClient`) that assembles the bean.
  - `discovery.go` — the client-side service-discovery seam: the mesh-gated
    builder (`GetDiscovery` + `discovery.NewLoader` + `WithScheme`, returning
    `nil` when `ServiceName` is empty or mesh is on). A `Loader` is a pure
    snapshot function — no resources, no `Stop`, freshness lives inside the
    discovery backend — so nothing is cached per client and nothing is torn down
    on `Destroy`. The driver's per-backend dialer (which wraps the loader via
    `loadbalance.SourceFunc` in a round-robin `Pool` and `Pick`s per connection)
    stays in `config.go` / `starter.go`; only the loader build lives here.
  - `resilience.go` — the wrapper bean's `ApplyResilience` InitMethod, the
    executor, and its `Close`/`CloseDriver` Destroy hook.
  - `observability.go` — the observe kit bridge (trace/metric/access-log hooks).
  - `health/` — the health-indicator constructor, as a subpackage so it can be
    imported without pulling the whole starter.
  This split mirrors the concern boundary established across the ecosystem and is
  what every existing client starter (go-redis, gorm-*, mongodb, elasticsearch,
  neo4j, ...) now follows — new starters should copy it verbatim. §4 checklist
  item 4 references this skeleton.

### 2.3 Contributor starters (no port of their own)

WebSocket (`websocket`, `websocket-coder`), middleware (`lua-filter`), and
authorization (`casbin`, `oauth2-client`) starters contribute a configured bean
that the application mounts onto infrastructure it already runs.

- **They open no listener.** A WebSocket starter contributes a
  `*websocket.Upgrader` / `*websocket.AcceptOptions`; the app upgrades
  connections on its existing HTTP server. This is why WebSocket lives apart from
  the server archetype.
- **The bean type is the seam.** Switching between two implementations of the
  same capability is a one-line blank-import change; see the shared-prefix rule
  in §3.

### 2.4 Global / infrastructure starters

`otel` (observability core) and `pprof` (diagnostics) install process-wide
facilities.

- **`starter-otel`** builds shared Tracer/Meter providers and installs them as
  OTel globals; client starters instrument against those globals so that when
  otel is absent the hooks are no-ops (zero-config opt-in).
- **`starter-pprof`** runs a *dedicated* HTTP server on its own port for runtime
  profiles, kept off the application's main port on purpose.

### 2.5 Config-provider starters (remote configuration center)

`starter-config-nacos`, `starter-config-etcd`, and `starter-config-consul`
integrate a remote configuration center (Nacos / etcd / Consul KV) so an
application can load configuration from it at startup and hot-reload at runtime.

- **Split by role, not by backend.** Nacos, Consul, and etcd are each *dual*
  backends — they serve both configuration and service discovery. These two are
  different integration points in Go-Spring, so they live in different starters:
  the **config** role is a config-provider starter (this archetype); the
  **discovery** role is client-side (`cloud/discovery`, §3) or framework-native
  (`contrib/registry/`, §3). A config-provider starter does the config role and
  nothing else. The naming mirrors Spring Cloud Alibaba
  (`nacos-config` vs `nacos-discovery`).
- **It registers a provider, not a bean.** The seam is
  `conf.RegisterProvider(name, fn)` called in `init()`, not `gs.Provide`. A
  config-provider starter produces no injectable bean; the application just
  blank-imports it. This is why it carries `provider.go` and no `config.go`.
- **The provider runs before the container exists.** `spring.config.import=`
  `[optional:]<name>:<host>:<port>/<key>?<query>` invokes the provider during
  `AppConfig.Refresh`, before any bean is wired. It therefore cannot inject a
  client bean — it builds its own client from the source string, and caches that
  client per connection tuple so repeated refreshes do not leak goroutines.
  Connection parameters (auth, namespace, format, ...) come from the source
  query string, not a bound `Config`.
- **Register the change listener unconditionally, before the fetch.** The
  provider must install its watch/listener *before* the fetch's
  `optional`-and-missing early return. Otherwise an app that starts before the
  key exists never registers a watch, and a later publish never triggers a
  reload. Dedup listeners per `(client, key)`.
- **Hot-reload reuses the framework refresh, via a `Rooter` bridge.** The
  provider's controller singleton is exported both as a `conf.RegisterProvider`
  target and as a `gs.Rooter` bean (`gs.Provide(ctrl).Export(gs.As[gs.Rooter]())`),
  which makes the IoC container collect it into `app.Rooters` and autowire its
  `*gs.PropertiesRefresher` field. On a remote change the watch/listener calls
  the controller's `TriggerRefresh`, which calls `RefreshProperties` — this
  reloads every source (re-running the provider) and re-binds all `gs.Dync[T]`
  fields through the two-phase, atomic commit in `gs_dync`. Bind live keys to
  `gs.Dync[T]`. The autowired refresher field is a plain pointer, not an
  `atomic.Pointer`: the container autowires by field assignment (no setter), and
  the cross-goroutine read from the watch callback is safe because
  `TriggerRefresh` nil-checks the field and `RefreshProperties` is itself gated
  by the app's `started` atomic flag (set only after wiring completes), so a
  pre-wiring fire is a no-op.
- **Content parsing reuses core readers.** Decode remote bytes with the
  `spring/conf/reader/{prop,yaml,toml,json}` `Read` functions keyed by a
  `format` query param, then `flatten.Flatten` before returning
  `map[string]string`.

### 2.6 Aggregator / profile starters (company baseline)

An aggregator starter re-bases go-spring onto one organization's conventions by
*composing* other starters and supplying defaults, not by re-implementing
anything. `starter-luohua` is the reference. It is a distinct archetype because
it imports and wires several existing starters rather than one third party —
single-concern still holds: the "third party" it integrates is the company
baseline (its identity, wire vocabulary, error catalog, standard drivers).

- **It wires seams, never private paths.** Every default it provides — a
  `security.TokenValidator` bean, an `i18n.MessageSource` catalog, a
  registered driver, a log context hook — goes through the public seam that
  capability already exposes (ARCHITECTURE §5: built-ins ride the same seams).
  If an aggregator default needs a private path to work, the seam — not the
  default — is wrong.
- **Defaults step aside.** A company default is provided with `gs.Provide` +
  `gs.OnMissingBean` / config gating so an application that brings its own
  identity / catalog / driver wins; the aggregator never adjudicates.
- **Own config prefix, armed by config.** Bind under its own
  `${spring.<name>}` prefix (e.g. `spring.luohua`); arming is `gs.OnProperty`
  on the prefix, per-capability toggles inside. Importing it is inert until
  configured.
- **Body is a set of `gs.Module` blocks + `Register*` calls.** Each capability
  gets its own `gs.Module` (bound config → provide default / override) or a
  driver-registry registration in `init()`.
- **Layout mirrors one-concern-one-file.** Identity, propagation,
  observability, driver, i18n each live in their own file, named after the
  concern (§2.2's client skeleton convention).

## 3. Cross-Cutting Constraints

- **Config prefix is per implementation, not per capability.** Every starter
  binds under its own unique `${spring.<name>}` prefix — client starters via
  `gs.Group`, server starters via `gs.Provide` with `TagArg`. Two starters that
  implement the *same* capability use distinct prefixes (`spring.kafka` for
  franz-go, `spring.kafka-sarama` for sarama; `spring.redis` for go-redis,
  `spring.redigo` for redigo). The configuration key itself is an explicit
  declaration of the technology choice: the user decides by writing
  `spring.kafka.xxx` vs `spring.kafka-sarama.xxx`. This avoids bean conflicts
  when both implementations happen to be imported, and makes the config file
  self-documenting.
- **Fail-fast over silent defaults.** Required inputs (addresses, credentials,
  mode-specific fields) are validated at startup with a clear `errutil.Explain`
  message rather than defaulted to something that half-works.
- **Production capabilities are part of the wrapper.** Health/readiness,
  startup connection validation, TLS, and destroy hooks are considered part of
  what a starter must provide, not optional extras. TLS is a nested
  `TLSConfig` (`enabled` + cert/key/CA), off by default.
- **Shared helpers live in their natural homes, not in a starter-shared
  package.** The three concerns every starter touches - TLS config, health
  indicator construction, fail-fast validation - each have a single home
  decided by their nature, not by who consumes them:
  `cloud/tlsconf.TLSConfig` (config fields -> `*tls.Config`),
  `cloud/actuator/health.NewIndicator` (factory for that package's
  `Indicator` interface), `stdlib/errutil.RequireField`/`RequireAny`
  (formatting sugar over `errutil.Explain`). Starters import these directly.
  A *new* cross-cutting concern that no existing package currently owns is
  still inlined per-starter until a second home materializes - do not
  pre-emptively create a shared package for it.
- **Prefer framework-native registration and discovery; unify only where none
  exists.** The default is to use each framework's *own* registration and
  discovery mechanism rather than force a Go-Spring abstraction on top of it. A
  Go-Spring-provided unified capability is considered *only* for transports that
  have no native mechanism of their own. The reasoning: the RPC frameworks each
  ship an incompatible registration abstraction (kitex's `registry.Registry`,
  kratos's `registry.Registrar`, dubbo-go's config-only registries, go-zero's
  `discov.EtcdConf`, ...), so a Go-Spring `Registrar` on top would just become a
  second translation layer bridging our abstraction into each framework's — the
  very coupling that makes "unify it" a net loss. Evaluation as of 2026-07-18:
  - *Have native registration + discovery — use theirs (opt-in):* `kitex`
    (`kitex-contrib/registry-etcd`), `kratos` (`kratos.Registrar`), `go-zero`
    (`discov.EtcdConf`), `goframe` (`gsvc`), `dubbo` (config registries), `trpc`
    (naming plugins). Each starter already wires this behind an empty-means-
    direct-connect toggle.
  - *No native provider registration — candidates if a real need appears:* plain
    gRPC (`starter-grpc`), Apache Thrift (`starter-thrift`), and plain HTTP web
    servers (gin/echo/hertz). Only these would justify a Go-Spring registration
    seam, and only when a concrete requirement lands.
- **Client-side discovery is already unified; provider registration is not.**
  Client starters resolve a `ServiceName` to live endpoints through
  `cloud/discovery` (a `Loader` built via the driver's dialer hook); this
  is generic across infrastructure clients. RPC *provider* registration stays
  framework-native per the principle above. When `ServiceName` is empty the
  client dials the address directly, unchanged. For examples of framework-native
  provider registration into consul/etcd/nacos/zookeeper/polaris, see
  `contrib/registry/`.
- **Service-mesh mode degrades the client-side stack centrally, not per
  starter.** When a sidecar (Istio/Envoy, Linkerd) is injected it already does
  discovery and load balancing, so running the app's own on top double-balances
  and confuses locality/outlier logic. A single process-global switch
  (`mesh.Enabled`, auto-detected from sidecar env vars by default; the `GS_MESH`
  env var — on/off/auto — overrides) is read at the discovery and load-balancing
  factory points — each client starter's mesh-gated loader builder (§2.2) and
  `loadbalance.Pool` — and degrades both to a
  pass-through: names resolve to one stable Service address (ClusterIP) the
  sidecar intercepts, and the balancer stops selecting and ejecting. Because
  the loader builder reads `mesh.Enabled()` internally and returns `nil` when
  mesh is on, a client starter honors the switch without per-starter branching
  at the call site — the driver just skips installing its discovery-backed
  dialer and dials the configured
  address directly. The code is not removed — flipping the switch off restores
  full client-side behavior.
- **Instance-level registration is per-starter; RPC-framework provider
  registration is not.** Do not conflate two different "registration" concerns.
  (1) Registering *this process* into an external registry
  (Nacos/Consul/Eureka/ZooKeeper) - the Spring Cloud `@EnableDiscoveryClient`
  direction - is a generic, transport-agnostic capability. It is **not** a shared
  abstraction in `cloud/discovery`: each `starter-registry-<backend>`
  (etcd/nacos/consul/zookeeper) owns its full register/deregister lifecycle -
  the registrar is a local value wired into the exported `gs.Server`, not a
  globally registered backend, and swapping backends means swapping the starter.
  The one shared rule every registry starter follows: **Register must
  self-renew** (TTL, heartbeat, or an ephemeral node) so that correctness never
  depends on `Deregister` being called - a process that crashes (SIGKILL, OOM)
  without deregistering must still be removed by the registry once its
  keep-alive goes silent; `Deregister` is only the prompt, graceful path for
  clean shutdown. (2) Registering an RPC framework's *services* stays
  framework-native per the bullet above. Neither is needed in pure Kubernetes,
  where the platform registers every Pod behind a Service (discover with
  `starter-discovery-k8s`); instance-level registration exists for VM /
  bare-metal / hybrid deployments. Each registry starter is a
  global/infrastructure archetype (§2.4): it exports a `gs.Server` that
  registers once the app is ready and deregisters on `PreStop`, so a rolling
  restart is lossless.
- **Observability is central-define, edge-bridge.** The starter emits through
  the OTel globals or bridges the library's internal logs into go-spring `log`
  via a `SetLogger` hook; it must also add a go-spring `FileLogger` sink or the
  console output is lost.

## 4. Adding a New Starter — Checklist

1. Pick the archetype (§2); it fixes your lifecycle and port behavior.
2. Own module, standard file skeleton, license headers.
3. Choose the config prefix — use a unique `${spring.<name>}` prefix that
   identifies *this* implementation. If you are a second implementation of an
   existing capability, pick `<capability>-<impl>` (e.g. `spring.kafka-sarama`),
   do not reuse the existing prefix.
4. Client? → `gs.Group` multi-instance, driver registry, required address with
   fail-fast, startup probe, per-instance `Destroy`, and the one-concern-one-file
   skeleton (§2.2): `config.go` / `starter.go` / `discovery.go` /
   `resilience.go` / `observability.go` (+ `health/`).
5. Server? → own port, listen-early/serve-on-ready, graceful `Stop`,
   app-supplied register bean, port-as-startup-gate (no `enabled` toggle — see §2.1).
6. Config-provider? → `provider.go` with `conf.RegisterProvider` (no `config.go`,
   no bean), parse params from the source string, cache the client, register the
   listener unconditionally before the fetch, bridge `PropertiesRefresher` into
   `refreshHook` via a `Rooter` bean, ship `example-config/`.
7. Add health, TLS, and destroy where the underlying library supports them.
8. Ship a bilingual README pair and an `example/` with `check.sh` only (no
   deployment scaffolding).
9. Resolve internal deps through `go.work`, never `require`.
