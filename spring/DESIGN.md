# spring Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`go-spring.org/spring` is the container/core layer of the four-layer
Go-Spring stack (stdlib → spring → starter → gs). It provides the IoC
container, dependency-injection wiring, layered configuration binding, and
the application lifecycle model. It depends on `stdlib` and the Go standard
library only; it never imports a third-party business SDK (Redis, GORM,
Kafka, etc.) — those live one layer up in `starter/`.

## 1. Responsibilities & Boundaries

- The container's job is: collect bean definitions during `init()`, resolve
  conditions, wire dependencies, run the `Init` / `Destroy` lifecycle, and
  drive `Runner` / `Server` roles from `gs.Run()`. The container refuses to
  own protocol logic or third-party clients — it exposes seams (`Provide`,
  `Module`, `Group`, `Condition`) that starters use to contribute those.
- `spring/conf` is the configuration engine: layered sources (command line,
  env, `app-<profile>.<ext>`, `app.<ext>`, in-memory, tag defaults) merged
  by priority, format readers under `spring/conf/reader/{yaml,toml,prop}`,
  and pluggable decryption under `spring/conf/decrypt`. The engine is
  independent of the container; the container drives it during boot.
- `spring/gs` is the public surface: `Provide`, `Configure`, `Module`,
  `Group`, `OnProperty`, `OnBean`, `Dync[T]`, `Runner`, `Server`,
  `ReadySignal`, `PropertiesRefresher`. Everything else in
  `spring/gs/internal/...` is implementation detail and off-limits to users.

### Subpackage families

Beyond `gs`/`conf`, the capability abstractions are grouped by **concern layer**
(a package is filed by the layer its *abstraction body* belongs to, not by its
strongest backend). This is the authoritative family map:

```
spring/
├─ gs/  conf/  aspect/         core: container, config engine, AOP primitive
├─ cloud/     distributed coordination (backends usually cross-process)
│    discovery loadbalance resilience lock messaging transaction event scheduling batch
│    tlsconf   (shared TLS-config builder consumed by cloud-facing starters)
├─ web/       request-handling plane + built-in HTTP
│    httpsvr httpclt httpx security session validation i18n
├─ data/      persistence
│    cache repository migration
└─ actuator/  ops / probe exposure
     endpoint health podinfo
```

Filing rules:
- **`aspect` is a root core primitive** (zero deps, depended on by cache/event/
  security/transaction) — same tier as `conf`, not a family member.
- **Body, not backend, decides the family.** `cache` → `data` (its body is a
  generic KV store; Memory is a first-class backend), `session` → `web` (its body
  is HTTP request-state management; a distributed store is just one backend),
  `transaction` → `cloud` (its body is cross-service coordination).
- **`event`/`scheduling`/`batch` → `cloud`** — all depend on `lock` for
  cross-replica coordination.
- Cross-family imports flow strictly downward (e.g. `web/httpx → cloud/*`,
  `data/cache → aspect`, `cloud/event → actuator/health`); the graph is acyclic.
  Go does not enforce this at compile time — it is a review-level invariant.

## 2. Key Abstractions & Seams

- **Bean registration.** `gs.Provide(objOrCtor, args...)` records a bean
  definition at `init()` time. Constructors are functions; their parameters
  are matched against the type index. Chainable builders configure `Name`,
  `Init` / `Destroy`, `Condition`, `DependsOn`, `Export`, `Configuration`.
- **Type-index-by-exported-interface.** The container indexes each bean
  twice: under its own concrete type, and under every interface passed to
  `.Export(gs.As[Iface]())`. Interfaces not listed in `Export` are **not**
  indexed — `[]Iface autowire:""` and `gs.OnBean[Iface]()` won't find the
  bean even though it structurally satisfies `Iface`. See
  `spring/gs/internal/gs_core/injecting/injecting.go` (`beansByType`,
  `GetExports`). A bean must also be a reference type (pointer / interface);
  value structs cannot be beans. Multiple beans of the same type must call
  `.Name()` or duplicate-bean resolution fails.
- **Dependency injection.** Struct fields tagged `autowire:""` /
  `autowire:"name?"` / `autowire:"a,*?,b"` and `value:"${key:=default}"`
  are populated during a single reflection pass. After boot, no reflection
  runs — matched functions and field offsets are cached.
- **Conditional auto-config.** `gs.Module(cond, fn)` groups a starter's
  beans behind a `PropertyCondition`; `gs.OnProperty` / `gs.OnBean` /
  `gs.OnMissingBean` / `gs.OnSingleBean` compose via `And` / `Or` / `Not` /
  `None`. This is the seam every starter uses to opt itself in from a
  configuration key.
- **Dynamic configuration.** `gs.Dync[T]` wraps a field so
  `PropertiesRefresher.RefreshProperties()` re-binds it in place without a
  container restart. This is the shared seam that config-center starters
  (`starter-config-{nacos,etcd,consul,vault,file}`) plug into.
- **Runtime model.** `Runner` (one-shot) and `Server` (long-running with
  `ReadySignal`) are the only two roles. All servers listen first, then
  block on `sig.TriggerAndWait()` so no server accepts traffic before every
  server is bound. The framework owns concurrent start, signal handling,
  and graceful `Stop()`.

## 3. Constraints

- No third-party business dependency in `spring/`. Anything past the Go
  standard library and `stdlib/` belongs in `starter/`.
- All registration happens at `init()` time. `Configure(func(app gs.App))`
  extends that phase; nothing may register beans after `Run` starts.
- The `internal/` subtree is not part of the public API surface, even
  though it is reachable through re-exports; downstreams must consume the
  `gs.` package.
- A bean's exported interfaces are exactly what `.Export(gs.As[Iface]())`
  declares — no automatic interface discovery. Missing an `Export` is the
  most common wiring bug and it fails silently at collection time.

## 4. Trade-offs & Alternatives Rejected

- **Runtime scanning (Spring Boot classpath scanning) rejected.** All bean
  metadata is registered by `init()`, so there is no classpath walk. Cost:
  every bean provider must be linked in (a blank import from
  `internal/init.go`). Benefit: predictable boot, no ordering surprises,
  zero reflection after wiring.
- **Compile-time DI (Wire-style codegen) rejected.** The container keeps a
  runtime graph so conditional modules, `Group` from configuration maps,
  and hot-refresh of `Dync[T]` can decide at boot / at runtime what to
  materialize. Reflection is confined to the boot pass.
- **Implicit interface indexing rejected.** Indexing every implemented
  interface would make `OnBean[Iface]` and `[]Iface autowire:""` non-local
  and hard to reason about. Requiring explicit `Export` keeps the type
  index a closed set the maintainer controls.
