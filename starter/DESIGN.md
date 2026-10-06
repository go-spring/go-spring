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
- **Every `init()` lives in `starter.go`.** Registration is the starter's entry
  point, so it must be readable in one place: no other file in the package
  declares an `init()`. The per-capability files (`config.go`, `client.go`,
  `observe.go`, ...) hold types, constructors and helpers — the implementation —
  never a registration. A module with nothing to register (a pure library such
  as `starter-http-server`, or `starter-gorm`, whose registration runs from each
  dialect's own `starter.go`) simply has no `init()` at all.
- **Sub-packages follow the same rule.** A helper package the starter imports
  for its side effect — an exporter factory under `starter-otel/metric`, a
  framework log bridge under `internal/logger` — exposes a plain `Register` /
  `Install` and `starter.go`'s `init()` calls it, instead of declaring an
  `init()` of its own. Importing such a package alone then does nothing, which
  is the point: the starter's registration is one readable list. The same holds
  for defaults a public sub-package owns: `starter-otel/trace` keeps its W3C
  propagator pair behind `RegisterDefaults`, which the starter calls — a caller
  that links the package without the starter root (the luohua umbrella, that
  package's tests) calls it explicitly rather than getting the registration from
  a hidden `init()`.
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
- **A non-listener long-running unit is still a `gs.Server`.** A message consumer, a
  callback server, or a poller that runs for the life of the process registers as a
  `gs.Server` so its `Stop` runs on shutdown — never a `gs.Runner`, whose `Run` must
  return quickly. When the wrapped library's own `Run` installs a signal handler,
  drive its `Start` and block on the context instead, so it cannot race Go-Spring's
  own shutdown.
- **Operator-facing servers serve during startup.** An operator or diagnostic server
  (a dashboard, a bean/config inspector) answers before the ready signal rather than
  waiting on it, so an operator can watch the boot. A dev-facing docs UI instead
  mounts on the existing app/actuator mux rather than owning a port of its own.

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
- **Config namespace: two buckets per family.** A client family prefix holds
  exactly two children and nothing else:

  ```
  spring.<family>.default.*             family-wide values, each overridable per instance
  spring.<family>.instances.<name>.*    one instance; <name> is the bean name
  ```

  - **The bean name carries an implementation qualifier when the returned type
    is a replaceable seam.** The criterion is "**will this type have more than
    one implementation**", not "how many does it have now".

    A seam (a type under `cloud/` that is a neutral, wholesale-replaceable
    contract: `session.SessionStore`, `batch.JobRepository`,
    `discovery.Discovery`, `resilience.Driver`, `loadbalance.Factory`, ...; or a
    framework interface such as `gs.Server`) → **every implementation's bean
    name must carry the implementation name**, as `<impl>.<instance>` when the
    family has instances. Beans are keyed by (name, type), so a second
    implementation carrying an instance of the same name would register the same
    pair and the container would refuse to start. The gorm family's five dialect
    modules (`starter-gorm-mysql`, `-postgres`, `-sqlite`, `-sqlserver`,
    `-clickhouse`) all register `gormcore.DB`, so they qualify the bean name as
    `<dialect>.<name>` (each dialect starter hardcodes its own qualifier in the
    `gs.Module` block it registers); discovery's
    `etcd.<name>` / `nacos.<name>` and session-redis' / batch-redis' `redis.<name>`
    are the same shape.

    Not a seam (a type private to the starter, which no second implementation
    will claim, such as `*redis.Client`) → the bare instance name is fine. And
    an "one constructor, one bean" implementation need not call `Name` at all:
    gs derives the default bean name from the constructor function's name
    (`gs_bean/bean.go`), which is already a natural qualifier — which is how the
    kratos / goframe / hertz server beans live.

    The qualifier follows **what the implementation is**, not how many exist —
    gorm's bean was `mysql.default` back when mysql was its only dialect.
    Judging by count forces a rename of the first member the moment a second
    arrives, and that rename is triggered by someone else adding a starter,
    which is the hardest thing to anticipate. The config key stays
    `instances.<name>` in every case — only the bean name carries the qualifier.

  Two buckets because a family's settings have exactly two levels, and splitting
  them structurally is what makes an instance name unable to ever collide with a
  family-wide key. Flat namespaces (`spring.<family>.<name>.*` with family-wide
  keys as siblings) need a reserved-word list instead: an instance named `driver`
  turns `spring.<family>.driver` from a scalar into a subtree and startup fails
  with a message that blames the scalar. The bucket removes the whole failure
  class by construction — **no reserved words, any instance name is legal**.
  (Modeled on Spring Cloud Stream, which solves the same problem the same way:
  `spring.cloud.stream.default.*` + `spring.cloud.stream.bindings.<name>.*`.)
  - `default` holds only **overridable defaults**. Non-overridable policy does
    not belong there; process-wide policy lives in its own namespace
    (`spring.governance.*`).
  - The `instances` bucket is the sole activation signal, so gate registration on
    it (`gs.OnProperty("spring.X.instances")`) and bind with
    `conf.BindEach(p, "${spring.X.instances}", ...)`. A process that sets only
    `${spring.X.default}` configures no client and must not activate the starter.
  - **`default` is inherited through the binder.** Wrap the storage before
    binding — `p = flatten.WithFallback(p, "spring.X.instances",
    "spring.X.default")` — so a key an instance does not define reads the same
    key under `default` with the instance name dropped. One value is overridden
    at a time: an instance's leaf replaces that leaf, and the default still
    supplies the values around it (the other members of the same struct, the
    other entries of the same map). A list is inherited **whole** — as soon as
    an instance defines one element, the list is the instance's. A wiring-time
    selection cannot use this: `${...driver}` names a *bean*, not a value, so it
    keeps its explicit `${instances.<name>.driver:=${default.driver:=?}}` chain.
  - **Where it applies.** The buckets are for families whose direct children are
    *user-chosen* instance names. A family that owns its child namespace keeps its
    own shape: `spring.discovery.*` holds this process's registration identity
    (`service-name`, `addr`, `weight`, ...) beside its center blocks
    `spring.discovery.<backend>.<name>`, and no user-controlled name can collide
    there (the `<backend>` segment is the framework's, and the user's `<name>` is
    one level deeper). A family that adds a framework level *inside* `instances`
    keeps one `default` at the family prefix anyway — `spring.lock.instances.
    <backend>.<name>` inherits `spring.lock.default.*`, shared by every backend —
    and the wrap takes that backend's instances path as its first argument.
    Forces that are not per-instance-overridable do **not** belong in `default`
    either — same-family process policy like discovery's identity sits at the
    family prefix, process-wide policy in its own namespace
    (`spring.governance.*`).
  - The invariant either way: **never let a user-chosen name share a level with a
    framework key.** A single-instance family (`spring.http.server`) keeps its keys
    directly under the family prefix because it has no instance names at all.
- **Address is required — fail fast.** A client must never silently fall back to
  `localhost`. Fields default to empty (`${addr:=}`). Single-field validation
  uses the `expr` tag (e.g. `expr:"$ != ''"` on a string, `expr:"len($) > 0"`
  on a slice), which fails at config-binding time. Cross-field rules ("addr OR
  service-name") use `errutil.RequireAny` in the constructor, since `expr` cannot
  express cross-field constraints.
- **Driver pattern for pluggable backends.** A client exposes a `Driver`
  interface; `DefaultDriver` ships built in and the container is the driver
  directory — a company contributes its own with
  `gs.Provide(...).Name("corp").Export(gs.As[Driver]())` and selects it with
  `${driver:=...}` without forking the starter. There is no package-level
  registry: naming a bean that does not exist fails startup. This is also the
  seam through which service discovery is injected (the driver builds the
  dialer). Optional capabilities go on *separate* interfaces (e.g. go-redis's
  `ClusterDriver`) so existing custom drivers keep compiling.
- **A client starter whose component reaches out gets governance from its own imports.** No
  governance import is needed: each authority is registered by the package that
  **owns it** — `cloud/resilience` registers `*resilience.Manager`,
  `cloud/loadbalance` `*loadbalance.Manager`, `cloud/fault` `*fault.Injector` — and
  a client that injects one has necessarily imported that package for the type.
  So the three beans are always in the container; the client injects them as
  REQUIRED (`gs.IndexArg(N, gs.TagArg(""))`), never as nullable (`"?"`). The beans
  are part of the client's contract — it is governable, observable and routable by
  construction — and with governance switched off in the rules document they behave
  exactly as before, so having them changes no default behaviour. Turning
  governance off is `spring.governance.enabled=false`, **not the absence of a
  bean** — and not the absence of a source either: the center's Source parameter is
  required, so a process that contributes none fails startup. A nullable injection
  would turn "the user forgot the import" into a silent degradation (governance
  looks on, is not) — exactly the error class this removes.
  (Blank-import it in non-test code only; `gs.RunTest` forces every injection
  nullable via `spring.force-autowire-is-nullable`, which is gs's behaviour, not
  an exception to this rule.)
  The constructor wires them **at construction time**, as part of what the client
  *is* rather than as a step someone must remember: it assembles
  `cloud.ClientParams{Resilience, Fault, Loadbalance, Discovery}` and pins
  `exec = params.ExecutorFor(system, label)` before returning. A zero
  `ClientParams` — an application driving the constructor outside the container
  — degrades to `resilience.Unmanaged`: still observed, plus a one-time "no
  protection is in effect" warning. Outside the container is therefore never a
  silent hole.
- **A component with nothing to protect emits its own signals.** The rule above is
  for a client of an external service; a component whose dependency lives in the
  same process is the exception, and `starter-bigcache` is its only member. A rate
  limit, breaker or retry has nothing to act on there — and a breaker tripped by a
  transient local failure (an oversized entry) would reject calls that would have
  hit, since the "shared downstream is down" premise does not hold. So bigcache
  injects no governance bean, `Driver.CreateClient` takes no
  `cloud.ClientParams`, and it does not route through the executor chain at all:
  it opens the operation's span and records its own metrics
  (`bigcache.operation.total` / `.duration`), and writes no access log — a cache
  call is high-frequency, and it is not a call to anything outside the process.
  Its span still rides the span-attribute carrier, so a layer above it contributes
  its attributes with no cooperation from either side. This is the one exception
  to the one-emitter rule below; `scripts/check-observability.sh` carries it as a
  section of its own, so it neither fails every family's check nor drifts back
  onto the chain unnoticed.
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
  - `config.go` — the `Config` struct and the `Driver` interface.
    Connection-creation only.
  - `starter.go` — the `init()`-time `gs.Group` / `gs.Module` registration and
    the constructor (`newClient`) that assembles the bean.
  - `discovery.go` — the client-side service-discovery seam: the mesh-gated
    builder (`discovery.NewResolver` over the backend bean injected from the
    config's `discovery` label + `WithScheme`, returning `nil` when the backend
    is absent, `ServiceName` is empty, or mesh is on). A `Resolver` is a pure
    snapshot function — no resources, no `Stop`, freshness lives inside the
    discovery backend bean — so nothing is cached per client and nothing is torn
    down on `Destroy`. The driver's per-backend dialer (which wraps the resolver
    as the source of a round-robin `Pool` and `Pick`s per
    connection) stays in `config.go` / `starter.go`; only the resolver build
    lives here. Every discovery-mode pool is built the same way: with a
    suspension `Tracker`, `Pick` paired with `Complete` at the dial site, and
    `lbMgr.Bind(pool, entry label)` on the injected `*loadbalance.Manager` — so the entry's
    governance rule drives `balancer` / `outlier-threshold` / `outlier-suspend-for` in place,
    through the `loadbalance` manager rather than by importing `cloud/governance`.
  - `client.go` — the assembled client and the resilience side of it: the
    executor the constructor pins on the client, the per-call routing into it,
    the `Close` Destroy hook, and any per-client protection seam (e.g. mongodb's
    dial-level `resilience.NewDialer`).
  - `observe.go` — the operation declarations this starter hands the framework's
    single emitter (its `Metric` prefix, its bounded attrs, its unbounded detail,
    its access `LogTag`); the emitter itself lives in `cloud/resilience` (§3).
  - `health.go` — the health-indicator constructor, in the starter's root
    package (a single-function subpackage is not worth the import cost). Omitted
    by a starter whose component has nothing to probe (bigcache, §3).
  This split mirrors the concern boundary established across the ecosystem and is
  what every existing client starter (go-redis, gorm-*, mongodb, elasticsearch,
  neo4j, ...) now follows — new starters should copy it verbatim. §4 checklist
  item 4 references this skeleton.
- **`gs.Group` has no registry access — use `gs.Module` where it cannot reach.**
  `gs.Group` returns one bean per bucket entry and cannot `.Name(...)` /
  `.Export(...)` or inject other beans. A starter that must export a per-instance
  seam bean, or whose constructor must inject additional beans, hand-writes a
  `gs.Module` over the instances bucket instead.
- **A transport fixed at construction gets a mutable indirection, and only the
  overload-sensitive path is guarded.** When the wrapped SDK fixes its transport or
  connector at construction, the starter installs a swappable holder and points it
  at its declaring-plus-resilience transport *after* construction — the declaring
  layer wraps the resilience one, never its base. A path the SDK already retries on
  a background goroutine stays unguarded, so protection is not counted twice.
- **Drain a library's error channel into the log.** When a wrapped library exposes
  an error channel, the starter drains it into the framework log rather than
  re-exporting it through the wrapper: a channel nobody drains eventually blocks the
  writer.
- **Put the guard where the library's seam is.** A `database/sql`-based client guards
  at the driver connection, so every caller — an ORM on the same pool included — is
  covered without a `Guarded*` helper. When a library exposes no interceptor and its
  send takes no ctx, the starter instruments through call-site seams: wrap the
  producer, run each consumed record through a helper, and let trace context ride
  the record headers.
- **A client whose library delivers a concrete type is hollowed into a chain.**
  When the wrapped library offers no hook/plugin point *and* hands back a
  concrete struct (gomemcache, bigcache, gocql, nats.go, go-mail, amqp091,
  franz-go, rocketmq-client-go), wrapping alone cannot modify behavior — so the
  starter's client embeds an `Inner*` interface (the command surface with ctx,
  plus `Release(releaseRaw bool) error`) and exports the raw instance as a
  read-only handle. The default chain is three layers with one capability each:
  the identity layer (`Obs*`: declares the operation via
  `observability.WithOperation` — every client has one), the governance layer
  (`Guard*`: runs the call under the resilience executor, which it builds, uses
  and closes inside the layer — only governed, RPC-style clients have one;
  bigcache's in-process cache has none), and the raw adapter (`Raw*`: discards
  the context the library never takes, does the wire call). Three rules keep it
  honest: (1) `Release` passes the flag down unchanged and every layer releases
  its own resources — only the adapter acts on the flag to close the instance,
  and the shell's `Close` (the gs destroy method) is the head's
  `Release(true)`; (2) chain reorganization is wrap-head —
  `c.Inner = myLayer{c.Inner}` — the keys a layer rewrites are what the
  identity layer declares, and governance keeps protecting underneath; (3) the
  per-delivery consume path does NOT ride the chain: its pipeline (extract
  trace → declare → run under the executor) inverts the chain's outside-in
  composition, so it keeps a fused wrapper on the shell. Three proven shapes:
  a flat command surface (memcached, nats, mail), a builder whose chain rides
  each query (cassandra), and a factory/registry entity with per-producer
  chains (rocketmq); where the bean used to be the raw client guarded through a
  package-level `sync.Map` registry (kafka, rabbitmq), the wrapper replaces
  both. If the library delivers an interface or a hook (go-redis, gorm,
  mongodb, redigo, neo4j, pulsar, mqtt, sarama), do not hollow — wrapping the
  delivered interface already is this pattern's seam.
- **Messaging-driver convention.** One producer per publisher, one push consumer per
  subscriber, and a handler error triggers redelivery.
- **A process-wide driver bean serves every instance.** Because the `Driver` bean is
  process-wide, a custom driver delegates to the bundled default to keep per-instance
  behaviour. Wrapping a library that is itself process-wide (one client per process)
  makes the starter single-instance, overriding the multi-instance default.
- **Declarative clients ride code generation, not runtime reflection.**
- **Resilience wraps outside the load balancer.** Wrapping *inside* would let a retry
  reuse the endpoint the balancer already picked, so the round-tripper sits outside
  the `Pool` and every retry re-picks.
- **Client ownership splits by backend.** A backend whose store exposes no shared
  client owns and closes its own client on destroy; a backend that reuses the
  application's client closes nothing.
- **Each backend adapts the shared TTL knob to what its store accepts.**
- **A stateless client registers neither a health indicator nor a destroy hook.** A
  client that holds no connection has nothing to probe and nothing to close.

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
- **A filter or middleware supplies a wrapped mux.** The framework installs its
  default `*gs.HttpServeMux` under `OnMissingBean`, so a filter or middleware
  contributes by providing a mux that wraps it.
- **A JWT verifier never accepts HMAC for an asymmetric key source.**
- **A session cookie is always HttpOnly**, with no toggle.
- **Compiled Go has no refreshable-script-bean culture.** In-process dynamic logic
  belongs to middleware, CEL, or WASM, so a Lua filter starter is a gateway-edge
  tool rather than a general component.

### 2.4 Global / infrastructure starters

`otel` (observability core) and `pprof` (diagnostics) install process-wide
facilities.

- **`starter-otel`** builds shared Tracer/Meter providers and installs them as
  OTel globals; client starters instrument against those globals so that when
  otel is absent the hooks are no-ops (zero-config opt-in).
- **`starter-pprof`** runs a *dedicated* HTTP server on its own port for runtime
  profiles, kept off the application's main port on purpose.
- **`starter-governance-sentinel`** contributes one process-wide bean — the
  `sentinel`-named `resilience.Driver` — into the governance center's driver
  directory. Imported alongside `cloud/governance`, the governance document's
  `spring.governance.driver=sentinel` switches every executor at once — inbound admission
  included, since one `Driver` answers for both directions. No port, no keys of
  its own.
- **A global starter installs its globals during `gs.Module` setup and tears them
  down via `gs.RegisterStopper`.** Setup runs before any bean constructor; the
  stopper runs after every server has stopped and the container has closed.
- **Read-optimised snapshots: a single writer under an RWMutex.** Handlers copy the
  snapshot and never block on live work.
- **Cap each background sweep's context at its own interval**, so a wedged target
  cannot make consecutive sweeps overlap.

### 2.5 Config-provider starters (remote configuration center)

`starter-config-nacos`, `starter-config-etcd`, and `starter-config-consul`
integrate a remote configuration center (Nacos / etcd / Consul KV) so an
application can load configuration from it at startup and hot-reload at runtime.

- **Split by role, not by backend.** Nacos, Consul, and etcd are each *dual*
  backends — they serve both configuration and service discovery. These two are
  different integration points in Go-Spring, so they live in different starters:
  the **config** role is a config-provider starter (this archetype); the
  **discovery** role is client-side (`cloud/discovery`, §3) or framework-native
  (`contrib/discovery/`, §3). A config-provider starter does the config role and
  nothing else. The naming mirrors Spring Cloud Alibaba
  (`nacos-config` vs `nacos-discovery`).
- **It registers a provider, not a bean.** The seam is
  `conf.RegisterProvider(name, p)` called in `init()`, not `gs.Provide`. A
  config-provider starter produces no injectable bean; the application just
  blank-imports it. This is why it carries `provider.go` and no `config.go`.
  Register the controller object itself, never `ctrl.Load` as a method value:
  the runtime holds the registered provider and calls its `Close` once at
  shutdown, which is the only handle it has on the watchers and listeners.
- **The provider runs before the container exists.** `spring.config.import=`
  `[optional:]<name>:<host>:<port>/<key>?<query>` invokes the provider during
  `AppConfig.Refresh`, before any bean is wired. It therefore cannot inject a
  client bean — it builds its own client from the source string, and caches that
  client per connection tuple so repeated refreshes do not leak goroutines.
  Connection parameters (auth, namespace, format, ...) come from the source
  query string, not a bound `Config`.
- **Close stops what Load started, and a later Load re-arms.** `Load` runs at
  startup and on every refresh, `Close` runs once per application instance; a
  process can run several instances in sequence (`gs.RunTest`). So `Close`
  cancels the controller's watch generation, drops the cached clients and the
  dedup sets, and the next `Load` rebuilds them from scratch. Never let `Close`
  leave state that the next `Load` would trust.
- **Register the change listener unconditionally, before the fetch.** The
  provider must install its watch/listener *before* the fetch's
  `optional`-and-missing early return. Otherwise an app that starts before the
  key exists never registers a watch, and a later publish never triggers a
  reload. Dedup listeners per `(client, key)`.
- **Hot-reload reuses the framework refresh, via the process-level
  `gs.RefreshProperties(ctx)` facade.** The controller is no longer a bean: on a
  remote change the watch/listener calls `gs.RefreshProperties(ctx)` directly — a
  package-level facade mounted by the app from its most recent `Start`, meant
  exactly for out-of-container infrastructure (config providers, watch
  goroutines) that cannot use dependency injection. The call reloads every
  source (re-running the provider) and re-binds all `gs.Dync[T]` fields through
  the two-phase, atomic commit in `gs_dync`; before the app has started the
  facade returns an error, so a pre-start fire is a safe no-op. Bind live keys
  to `gs.Dync[T]`.
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
  `spring.kafka.instances.xxx` vs `spring.kafka-sarama.instances.xxx`. This avoids bean conflicts
  when both implementations happen to be imported, and makes the config file
  self-documenting.
- **Everything the framework reads lives under `spring.` — with one registered
  exception.** A key bound by a starter, a `cloud/` package, or the gs core
  carries the `spring.` root (`spring.kafka.instances.*`,
  `spring.actuator.podinfo.labels-path`, `spring.profiles.active`). An
  application's **own** fields do not: they use a prefix of the application's
  choosing, never `spring.` — `spring.` means "the framework defined this key",
  so a starter that writes into it must earn the name, and an app that writes
  into it claims authority it does not have. The single exception is
  `logging.*`, read by `gs_app` *before* the container is refreshed: a log
  system that only took effect after wiring could not report a wiring failure,
  so it keeps its own top-level root. It is the only entry in
  `scripts/check-config-namespace.sh`'s `ALLOW_ROOTS`; adding one must be
  deliberate and recorded there and here. Keys derived from environment
  variables (`GS_POD_NAME` -> `pod.name`) are not configuration keys at all and
  are out of scope.
- **Fail-fast over silent defaults.** Required inputs (addresses, credentials,
  mode-specific fields) are validated at startup with a clear `errutil.Explain`
  message rather than defaulted to something that half-works.
- **Production capabilities are part of the wrapper.** Health/readiness,
  startup connection validation, TLS, and destroy hooks are considered part of
  what a starter must provide, not optional extras. Two of them carry a
  per-instance switch: the startup connectivity probe (`ping`, off by default,
  so an unready backend cannot block boot) and the health indicator (`health`,
  on by default). The health indicator is for a component with an external
  dependency to probe; an in-process component with none (bigcache) contributes
  no indicator, because a probe that cannot fail is a constant term in an
  AND-aggregated readiness and carries no signal. TLS is a nested `TLSConfig`
  (`enabled` + cert/key/CA), off by default.
- **Shared helpers live in their natural homes, not in a starter-shared
  package.** The three concerns every starter touches - TLS config, health
  indicator construction, fail-fast validation - each have a single home
  decided by their nature, not by who consumes them:
  `cloud/security.TLSConfig` (config fields -> `*tls.Config`),
  `cloud/actuator/health.NewIndicator` (factory for that package's
  `Indicator` interface), `stdlib/errutil.RequireField`/`RequireAny`
  (formatting sugar over `errutil.Explain`). Starters import these directly.
  `cloud/governance.Parse` is the fourth: the rule-document parser is shared by
  the file/http/etc/nacos sources, so it sits with the model it produces rather
  than in one starter its siblings would have to reach into — the one place
  `cloud/` takes a `spring` import, which is accepted because the document
  format is the framework's.
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
  `discov.EtcdConf`, ...), so a Go-Spring `Registry` on top would just become a
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
  `contrib/discovery/`.
- **Service-mesh mode degrades the client-side stack centrally, not per
  starter.** When a sidecar (Istio/Envoy, Linkerd) is injected it already does
  discovery and load balancing, so running the app's own on top double-balances
  and confuses locality/outlier logic. A single process-global switch
  (`mesh.Enabled`, auto-detected from sidecar env vars by default; the `GS_MESH_MODE`
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
  direction - is a generic, transport-agnostic capability. Its publication
  lifecycle lives in `cloud/discovery`: `Server` is the one exported
  `gs.Server` that drives every configured center, and each
  `starter-discovery-<backend>` (etcd/nacos/consul/zookeeper) contributes a
  `Registry` - the protocol adapter for its own center. Swapping backends means
  swapping the starter.
  The one shared rule every discovery starter follows: **Register must
  self-renew** (TTL, heartbeat, or an ephemeral node) so that correctness never
  depends on `Deregister` being called - a process that crashes (SIGKILL, OOM)
  without deregistering must still be removed by the registry once its
  keep-alive goes silent; `Deregister` is only the prompt, graceful path for
  clean shutdown. (2) Registering an RPC framework's *services* stays
  framework-native per the bullet above. Neither is needed in pure Kubernetes,
  where the platform registers every Pod behind a Service (discover with
  `starter-discovery-k8s`); instance-level registration exists for VM /
  bare-metal / hybrid deployments. The registration core is a
  global/infrastructure archetype (§2.4): it exports a `gs.Server` that
  registers once the app is ready and deregisters on `PreStop`, so a rolling
  restart is lossless.
- **Observability is central-define, edge-bridge.** A client-side starter does
  not emit at all: it **declares** its operations for the framework's single
  emitter (the next bullet). A component that *does* emit writes through the OTel
  globals; a component that bridges a library's internal logs into go-spring
  `log` does it via a `SetLogger` hook, and must also add a go-spring
  `FileLogger` sink or the console output is lost.
- **The instrument set is shared; the observation is not.** A component resolves
  its instruments **once per process**, not once per instance: the OTel SDK keys
  an instrument by name/description/unit/kind and hands every later creation the
  first one, so a duplicate creation is a no-op that silently keeps the first
  description. Duplicating is therefore not the risk — **registering** is.
  - *A registration belongs to whoever holds the value.* An instance registers
    its own observation of a shared observable gauge and unregisters it from the
    same destructor that tears the instance down; a process-level value is
    registered once, by the component itself. A registration held anywhere else
    outlives its value — it reports a dead instance, and keeps it reachable.
  - *Register a gauge; never create it with a callback.*
    `metric.WithInt64Callback` applies a callback to the **first** creation of a
    descriptor only and drops every later one without an error. It is correct
    only while exactly one creation happens per process — a condition no call
    site can see, and whose violation shows up as a missing series rather than a
    failed boot. `RegisterCallback` is additive and reversible.
  - *Same name means the same descriptor.* Name, description and unit must match
    on every creation, or the later one is ignored together with its description.
    The mismatches that still exist are **registered, not licensed** — see
    `KNOWN_MULTI_DESC` in `scripts/check-observability.sh`: the HTTP/RPC server
    families (unmigrated, §5) and `messaging.client.connection.state_changes`
    (its three backends write the system name into the description).
  - *A tracer is never cached* in a struct field or a package variable: a
    captured `otel.Tracer` stops forwarding once the global provider is set
    again. Resolve it at the use site.
  - *State in an instrument set is membership at most* — which instances are
    live — never measurements. A shared observer that buffers data is a global
    data store with no owner to bound it.
- **Observation is two layers — a data granularity, not a second emission
  point.** One logical call yields a **call** record (total time, attempts plus
  backoff, final status, attempt count) and an **attempt** record per downstream
  try (what the downstream itself took). They measure different things: a
  downstream's latency is the downstream's property and a backoff is our
  policy's, so merging them yields the absurd "raising the backoff made the
  downstream slower". A call that succeeds on the first try reports equal numbers
  on both layers — two meanings, not a duplicate. An error a retry erased is
  never recorded as an error (OTel's rule): the call reports its final status.
- **Component observability follows one rule: same kind → same name, same
  instrument type, same completeness, and a name states the capability, not the
  implementation** (`db.client.operation.duration`, never
  `redis.command.duration`). The framework now has **one emitter**, on the
  resilience chain, and client-side starters feed it by **declaring** what each
  operation is — with the one exception of the in-process component above, which
  emits its own. A client starter declares
  `observability.WithOperation(ctx, observability.Operation{...})` — the
  operation's `Name`, its `Metric` prefix (`db.client`, `messaging.client`), its
  bounded `Attrs`, its unbounded `Detail` and its `LogTag` — and routes the call
  through the resilience executor. The resilience observe layer
  (`cloud/resilience/observe.go`, in `wrappedClientExecutor.Execute`)
  emits from that declaration: the call span, `<prefix>.operation.duration`
  (call level, retries and backoff included), `<prefix>.attempt.duration`
  (attempt level, what the downstream itself took), `<prefix>.active_requests`
  (in-flight calls), the always-on `resilience.client.calls{status}`, and one
  access-log line per call.
  - **The emitter sits outside the retry loop**, which is why the attempt layer
    carries metrics only and no span: at that position a per-attempt span cannot
    be opened. Deliberate, not a gap.
  - **Turning governance off does not turn observation off.** The manager wraps
    the pass-through executor too, so `spring.governance.enabled=false` keeps
    every span, metric and access log and drops only the protection. The emitter
    lives in `resilience` rather than `cloud/observability` because its fallback
    path carries `resilience.*` names and the resilience log tag; a neutral
    package would have to borrow that vocabulary.
  - **`Attrs` must be bounded**: every attribute there becomes a metric label,
    a span attribute and a log field, so an unbounded value (a cache key, a
    subject) would multiply the series without bound. **`Detail` may be
    unbounded** — a key, a statement, a URL path, a subject — and reaches the
    span and the log but never a metric label.
  - **The emitter reads the operation at `Execute` entry**, so a declaration
    made *inside* the executor is read by nobody: a transport whose hook sits
    below the resilience round tripper (see `starter-s3`) declares outside it.
  - **Log levelling is the emitter's**: failure at Warn, a success carrying
    `Detail` at Debug (lazy), a success carrying none at Info.
  - **A non-idempotent operation is never retried**: a client whose repeat is a
    second side effect rather than a second attempt — sending mail, publishing a
    message, handling a consumed record — declares `NonIdempotent: true`, and the
    executor runs the call **once** whatever retry the policy configures (it
    warns once per service when a configured rule is dropped that way). The
    policy cannot know this; only the client does, which is why the declaration
    is the client's to make.
  Three principles bound the rule, and dropping any one bends it:
  - *Completeness is the emitter's obligation, discharged once.* The signals —
    a span, a duration metric, an in-flight gauge, and one access-log line per
    call — used to be a per-component checklist; they are now asserted once, at
    the one emitter, and hold for every client that declares. A client-side
    starter's completeness is therefore its **declaration**: a client that fails
    to declare, or that re-builds a signal the emitter already owns (its own
    `otel.Tracer`, its own duration/in-flight instrument), is the defect. A
    result dimension that cannot tell success from failure is still a defect; the
    emitter's is the `status` label.
  - *Commonality moved with completeness.* Same-kind-same-name still holds, but
    the names are now produced by the emitter from the declared prefix, so the
    members of a family (interchangeable backends behind one capability) agree by
    **declaring the same `Metric` prefix and the same bounded attribute keys**.
    That is what lets a backend be swapped without rewriting every dashboard,
    alert and query against it.
  - *Flexibility* — a component-specific key is allowed. Nothing is a violation
    merely for differing from its neighbours.
  - *Two tiers, because a shared name is only meaningful for a shared meaning.*
    `status` (`ok`/`error`), `rpc.method` and `duration_ms` mean exactly the same
    thing in every RPC backend, so they must share one name — they are the only
    thing a cross-framework query can join on. A transport-specific code
    (`rpc.grpc.status_code`, `rpc.thrift.status_code`) carries a different value
    vocabulary per backend, so forcing one name there would put two meanings
    under one key — worse than two names. The same split applies to HTTP: the
    shared axis is `status`, the detail is `http.response.status_code`.
    The client emitter follows the same rule: `status` is `ok`/`error` like
    everywhere else, and **`resilience.outcome`** carries what the protection
    stages did to the call (`rate_limited`, `circuit_open`, `bulkhead_full`,
    `retry_budget_exceeded`, `timeout`) — absent when they did nothing. Putting
    those words in `status` would have made the one key two vocabularies, which
    is exactly the failure this tier exists to prevent.
  - *Logs must join metrics.* An identity key in a log line must exist under the
    same name as an attribute (span attribute or metric label), or a failing
    metric cannot be traced to the line that explains it. Duration keys (ending
    in `_ms`) and `error` are the line's own payload, not identity. The rule is
    about the line that explains the component's *own* signals; the component's
    ordinary application log (written under `log.TagAppDef`) is not one of these.
  The per-family key lists are mechanical and live in
  `scripts/check-observability.sh`, which is the enforcement — a family rule
  lists what every member must have, and anything outside that list is left
  alone. Its sections, and the gaps that are registered rather than closed:
  - *families* — DB and messaging. A member **declares** (its family's `Metric`
    prefix and the family's bounded attribute keys, through
    `observability.WithOperation`) and routes the call through the executor; the
    rule checks that declaration and that the member has not gone back to
    building its own span (`otel.Tracer`) or its own duration/in-flight
    instrument. The HTTP server and RPC families are **not migrated** — their
    members still self-build their spans and instruments and are checked as
    before. A member whose metric or span is emitted by a third-party library is
    registered with an access-point evidence regex (checked against
    comment-stripped source, so deleting the wiring but keeping the comment still
    fails); the per-call third-party spans that go-redis
    (`redisotel.InstrumentTracing`) and elasticsearch
    (`ElasticsearchOpenTelemetry`) used to enable are gone — they duplicated the
    call-level span the emitter now opens. Third-party *non* per-call telemetry
    (go-redis pool metrics, kotel's client/connection metrics) is still left
    alone.
  - *emitter* — the single emission point is checked once for the completeness
    above: the call span, `<prefix>.operation.duration`,
    `<prefix>.attempt.duration`, `<prefix>.active_requests`, the always-on
    `resilience.client.calls{status}`, one access-log line per call at the right
    level, and the `Detail` kept out of the metric labels.
  - *baseline* — self-built instrumentation with no interchangeable siblings
    (`config-bus`, `gateway`): completeness and joinability only; commonality has
    nothing to bind them to.
  - *delegation* — signals come wholly from a shared layer (`http-client`,
    `oauth2-client`, the four `lock` backends, the three `transaction` backends,
    `scheduler` — whose signals live in `cloud/scheduling` — the config
    providers, the discovery backends); what is checked is that the wiring is
    still there.
  - *forward list* — components that must be instrumented. The model discovers
    members by the signals they already carry, so a component that was never
    instrumented is invisible to it; this list is what makes "should have been
    done and was not" visible.
  - *accepted gaps* — `starter-mongodb` emits at the **command** layer instead of
    declaring an operation, because the mongo driver v2 offers no per-command
    execute hook (only a `CommandMonitor`, an observer that cannot gate), so its
    resilience/executor seam is the **dialer**. Per-command signals must therefore
    ride the driver's command monitor while protection rides the dial. It keeps
    the emitter's vocabulary — bounded `db.system` / `db.operation` labels, the
    statement as span-and-log-only detail, the same metric names, its own access
    tag — and its protection stays dial-level. This is a driver constraint,
    deliberately accepted.
    `starter-oauth2-client` logs every business call through the
    same resilience layer as `starter-http-client` (it reaches that layer via the injected
    `resilience.Manager` instead of composing the wrapper itself), but the
    token-endpoint exchange inside the oauth2 library bypasses that round tripper,
    so it is traced and not logged;
    kitex's and kratos's duration metrics carry no `status` dimension because the
    libraries emit them; `cloud/experimental/transaction` emits spans only (its
    `Observer` is the whole-path-replacement seam that §1.5 keeps as-is, so the
    framework does not prescribe the other two signals for it);
    the property-refresh funnel (`observability.RefreshConf`) is the one place
    that both emits metrics and logs the refresh outcome — callers log only
    their backend events, so the funnel owns the "was the fleet refreshed"
    log line (the module that knows the subject owns the log).
  - *connection state* — `messaging.client.connection.state_changes` is emitted
    only by the members whose client library exposes connection-state callbacks
    (mqtt, nats, rabbitmq). It is deliberately not part of the messaging family's
    shared list: requiring it of a library that has no such callback could only
    produce fabricated data. The checker holds the registered list and fails if a
    registered member stops emitting it, or if an unregistered member starts.
- **Discovery backends report at their own seams, through their own observer.**
  Each configured block builds one `discovery.Observer` at construction — from
  its backend name and its block name — and reports through its methods
  (`RegisterAttempt`, `DeregisterAttempt`, `WeightChange`, `Synced`), passing
  `discovery.ReasonSelfHeal` for a background re-registration; the block's
  destructor closes the observer, which drops its gauges. The observer is a
  layer the block owns, so the state behind those gauges lives exactly as long
  as the block: two blocks in one process cannot contaminate each other, and
  nothing is left to reset between tests. Report where BOTH the initial publish
  and the self-healing path funnel through (etcd `publish`, zookeeper
  `createNode`, consul `upsert`), not around the `discovery.Registry`
  interface: the self-heal path never crosses that interface, and it is the one
  that leaves an instance serving while no longer discoverable. The metric and
  span definitions live once in `cloud/discovery/observe.go` — backends never
  name an instrument themselves.
  A **discovery-only** backend of this family (`starter-discovery-k8s`, where the
  platform registers Pods for you) has no such seam to report at: it builds the
  same observer and reports `Synced` on both sync outcomes, and emits none of the
  three registrar operations. Registered as an exception in
  `scripts/check-observability.sh` (its discovery section).
  The log lines around those operations belong to the backend, so each one
  writes its fields out. **A line that explains a discovery metric or span must
  carry all of these, under exactly these names, or it does not join it:**

  | Field | Value |
  |---|---|
  | `system` | the backend name (`etcd`, `nacos`, ...) — take it from `Observer.System()`, the same value the metric's own `system` attribute carries |
  | `center` | the configured block (`${spring.discovery.<backend>.<name>}`) — from `Observer.Center()`. One process publishes the same service into every configured center, so without this the readings of two clusters are one indistinguishable series |
  | `service` | the name being registered or synced |
  | `operation` | `register` / `deregister` / `update_weight` / `sync` |
  | `reason` | `initial` / `self_heal` — registration only |
  | `status` | `ok` / `failed`, taken from `discovery.StatusOf(err)` — the vocabulary is not the `error` other families use, which is exactly why the value comes from the function rather than from hand |
  | `error` | the error itself, via `log.Err` |

  The caller supplies its own message and its own detail (the key, the action
  taken); the fields above cover only what identifies the operation, which is
  what the join needs.
- **No package-global registry for a live implementation.** `cloud/lock`
  deliberately keeps no string driver registry (unlike discovery and resilience): a
  lock needs a live backend handle, so the backend is chosen by which starter is
  blank-imported. More generally, a live, config-derived implementation is never
  registered into a package-global map — globals are wrong across tests and
  restarts.
- **Starters are independently versioned and never depend on each other.** A second
  starter of the same capability re-implements the proven pattern rather than
  importing its sibling, and distributed-transaction patterns ship as separate
  starters, never one merged abstraction.
- **More than two wire behaviours means a TLS mode enum.** When a protocol has more
  than two wire behaviours (SMTP: STARTTLS / implicit TLS / plaintext), TLS config
  is a mode enum, not the ecosystem's `enabled` plus cert pair.
- **No startup probe when the only universal probe would have a side effect.** When
  a real POST is the only probe available, ship no probe at all — a boot-time junk
  call is worse than a first-send error.
- **Bind an aggregate struct with field-level `value:"..."` tags**, never
  `gs.TagArg("${prefix}")`.
- **A second `gs.Server` bean must be `.Name(...)`d** — the container already holds
  a default web-server bean.
- **A `gs.Dync[map]` default must be empty**, never a non-empty map literal.
- **Prefer owning a small protocol with the stdlib over depending on a dormant
  SDK**, and ship a self-contained mock-server example when the official counterpart
  service needs heavy orchestration.
- **A zero-dependency capability package exposes a plain Observer/callback seam.**
  A `cloud/` capability package must not depend on a concrete backend; the OTel span
  helpers live in the starter layer, not the neutral foundation.
- **A capability's default in-memory store rides `gs.OnMissingBean`**, so a
  durable-store starter displaces it for every consumer with no business-code
  change.

## 4. Adding a New Starter — Checklist

1. Pick the archetype (§2); it fixes your lifecycle and port behavior.
2. Own module, standard file skeleton, license headers.
3. Choose the config prefix — use a unique `${spring.<name>}` prefix that
   identifies *this* implementation. If you are a second implementation of an
   existing capability, pick `<capability>-<impl>` (e.g. `spring.kafka-sarama`),
   do not reuse the existing prefix. The `spring.` root is not optional (§3);
   `scripts/check-config-namespace.sh` enforces it at every binding site.
4. Client? → `gs.Group` multi-instance, driver registry, required address with
   fail-fast, opt-in startup probe (`ping`, off by default), per-instance
   `Destroy`, and the one-concern-one-file
   skeleton (§2.2): `config.go` / `starter.go` / `discovery.go` /
   `client.go` / `observe.go` / `health.go`. Config goes in the two
   buckets: `conf.BindEach(p, "${spring.<family>.instances}", ...)`, gate the
   module on `gs.OnProperty("spring.<family>.instances")`, and read family-wide
   values from `${spring.<family>.default.*}` inside each entry's tags. Never bind
   directly on the family prefix — `scripts/check-config-namespace.sh` enforces it.
   **Returned type a replaceable seam?** → qualify the bean name with the
   implementation (`<impl>.<instance>`, see §2.2); otherwise a second
   implementation's same-named instance registers the same (name, type) key.
   **Dials through discovery?** → build the pool as the same three-piece set every
   time: a `loadbalance.Tracker` attached, `Pool.Pick` paired with `Pool.Complete`
   at the dial site (feeding the dial outcome — without it the tracker is blind),
   and `lbMgr.Bind(pool, <entry label>)` on the injected `*loadbalance.Manager` with the
   *same* `resilience.ServiceLabel` the entry's executor uses. That is what makes
   `balancer` / `outlier-threshold` / `outlier-suspend-for` configurable through the
   governance rule instead of hardcoded — no `balancer` key of your own, and no import of
   `cloud/governance`. A client that cannot re-pick (one-shot resolution, or a
   library that owns its own selector) stays out of this on purpose; say so in its
   USAGE.
5. Server? → own port, listen-early/serve-on-ready, graceful `Stop`,
   app-supplied register bean, port-as-startup-gate (no `enabled` toggle — see §2.1).
6. Config-provider? → `provider.go` with `conf.RegisterProvider` (no `config.go`,
   no bean), parse params from the source string, cache the client, register the
   listener unconditionally before the fetch, call the `gs.RefreshProperties(ctx)`
   facade from the change callback, ship `example-config/`.
7. Discovery starter? → define `obsSystem` and report at the seams the initial
   publish and the self-healing path share (`discovery.RegisterAttempt` with
   `ReasonInitial`/`ReasonSelfHeal`, `DeregisterAttempt`, `WeightChange`, and
   `discovery.Synced` on both sync outcomes); never wrap the `Registry`
   interface instead — the self-heal path does not cross it. A discovery-only
   backend of the family (`starter-discovery-k8s`) has no registrar: it defines
   `obsSystem` and reports `discovery.Synced` only, and is registered as an
   exception in `scripts/check-observability.sh` (its discovery section).
   `scripts/check-observability.sh` (its discovery section) enforces this.
8. Add health (`health` switch, on by default) where the component has an
   external dependency to probe, TLS, and destroy where the underlying library
   supports them.
9. Ship a bilingual README pair and an `example/` with `check.sh` only (no
   deployment scaffolding).
10. Resolve internal deps through `go.work`, never `require`.
