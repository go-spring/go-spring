# discovery Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`discovery` is the zero-dependency, framework-agnostic foundation for
client-side service discovery. It defines the narrow contract every naming
adapter satisfies and the shared plumbing every infrastructure client reuses,
so a company's naming service is adapted once and works across Redis, MySQL,
MongoDB, Kafka, gRPC.

## 1. Responsibilities & Boundaries

- **Does:**
  - Read side — `Discovery` (`Resolve` + `Watch`), `Endpoint`, `WatchResult`,
    the optional `Catalog`, and `Resolver` (the stateful, watch-refreshed
    endpoint picker).
  - Write side — `Registrar` (`Register` + `Deregister`) and `Instance`, the
    traffic-agnostic "publish this process" counterpart.
  - Package-level backend registries (`RegisterDiscovery` / `GetDiscovery`,
    `RegisterRegistrar` / `GetRegistrar`).
- **Refuses:**
  - **No selection policy, no traffic feedback.** `Resolver.Pick` is minimal
    round-robin. Strategies (weighted, least-conn, consistent-hash, zone-aware)
    and failure ejection belong in `go-spring.org/spring/cloud/loadbalance`,
    which sits *above* discovery. Naming is shared and changes on the slow
    topology timescale; selection and feedback are per-consumer and per-request.
    Mixing them couples a shared registry reader to one noisy consumer's
    failures.
  - **No mesh knowledge.** "Am I in a mesh?" is a deployment question, orthogonal
    to name resolution; the switch lives in `go-spring.org/spring/cloud/mesh`.
  - **No tracing.** Trace propagation across hops lives in `starter-otel`.
  - **No provider registration of RPC frameworks** (kitex, kratos, dubbo-go).
    Each already has its own registry model; a unifying wrapper is just a
    translator per framework — a net negative.
  - **No SDK backends.** Nacos / Consul / etcd / DNS / Kubernetes adapters drag
    in their SDK clients, so they live in their own starters and register
    themselves under a name. The one exception is [NewStaticDiscovery]: a
    zero-dependency, fixed-snapshot reference backend kept in this package as the
    minimal implementation demo and a shared helper for examples/tests.

## 2. Key Abstractions

- **Two-method `Discovery`** (`Resolve` + `Watch`) keeps the adapter surface
  minimal. `Watch` returns a `<-chan WatchResult`; each delivered value is a
  *full* snapshot (not an incremental delta), the first one is the current
  state, and the context is the sole lifecycle — cancelling it closes the
  channel. No `Watcher` handle, no `Stop`. Backends that cannot stream poll
  internally; the contract stays uniform.
- **`Endpoint` three-state eligibility.** `Disabled` (an operator or provider
  decree — drain, maintenance, Nacos `enabled=false`) is separate from
  `Healthy` (a probe result). The eligible set is `!Disabled && Healthy`,
  degrading to `!Disabled` when none are healthy; `Disabled` instances never
  enter either set. This prevents the classic bug where an operator-disabled
  instance is resurrected by the "no healthy → use all" fallback.
- **`Resolver` is a pure decision layer.** It seeds from one explicit `Resolve`
  (synchronous, fail-fast), refreshes via a background `Watch`, and picks
  round-robin among eligible endpoints. It deliberately does **not** dial — the
  client owns the socket, its pool, retries, and dead-connection eviction.
  Dial-time failover, when wanted, belongs in the client (or a helper), not in
  the resolver. This mirrors `loadbalance.Pick`, which also decides without
  connecting.
- **`Catalog` is optional, as a separate interface.** Some backends cannot
  enumerate service names (DNS, a static adapter, a Kubernetes headless Service
  reached by name). Forcing enumeration onto `Discovery` would make those
  backends lie with empty lists or panics. Consumers type-assert
  `d.(Catalog)` when they need it.
- **`Registrar` is the write-side counterpart.** `Register` MUST be
  self-renewing (heartbeat / TTL / "ephemeral"): if the process dies without
  `Deregister`, the registry expires the instance on its own. Correctness never
  depends on `Deregister` being called; `Deregister` is just the prompt,
  graceful path. There is intentionally **no `Update` method** — re-`Register`
  with the same ID is idempotent refresh, which is how Consul and Nacos natively
  implement "modify an instance" (and kratos / kitex expose only
  Register/Deregister).
- **`Instance` mirrors `Endpoint`.** `Scheme`, `Weight`, `Disabled`, `Metadata`
  carry over; `Endpoint` adds probe-driven `Healthy`, `Instance` carries the
  publisher-only `ID`.
- **Package-level registries with init-time panics** mirror the driver-registry
  idiom (e.g. starter-go-redis `RegisterDriver`). Empty name, nil backend, or a
  duplicate is a wiring bug, not a runtime condition. `GetDiscovery` /
  `GetRegistrar` return a descriptive error listing the registered names, so a
  typo or a missing starter is obvious at construction time.

## 3. Constraints

- Backends and resolvers must be safe for concurrent use; `Resolver` stores the
  snapshot in an `atomic.Pointer[[]Endpoint]` and guards `Stop` with
  `sync.Once`, so `Stop` is safe to call concurrently with `Pick` and from a
  bean destructor.
- `Resolver.Pick` prefers `!Disabled && Healthy`; when none are healthy it
  degrades to `!Disabled`; `Disabled` instances are never eligible. Discovery
  must not black-hole traffic just because a backend omits health reporting.
- A `Watch` channel closes when the context is cancelled or the backend emits a
  terminal `WatchResult.Err`; the consumer then stops ranging and keeps serving
  from the last snapshot, since stale addresses are safer than none. Reconnect
  with backoff, when wanted, is the caller's responsibility, not `Watch`'s.
- The two registries (`discoveries`, `registrars`) use independent mutexes; no
  operation spans both, so the read and write sides need not serialize against
  each other.

## 4. Trade-offs / Alternatives Rejected

- **Client-side only; no unified RPC-framework `Registrar`.** kitex
  `registry.Registry`, kratos `registry.Registrar`, dubbo-go's config-only
  registration and go-zero's `discov.EtcdConf` differ enough that a wrapper is
  just a translator. Framework-native registration is used everywhere it
  exists; `Registrar` covers only the traffic-agnostic case (bare gRPC / thrift
  / HTTP, VM / bare-metal / hybrid).
- **`Resolver.Pick` is minimal round-robin, not weighted / consistent-hash.**
  Strategy belongs one layer up; keeping discovery focused prevents overlap with
  `loadbalance` (which owns strategy + eviction).
- **Channel-based `Watch` over a pull-based `Watcher.Next`.** A `<-chan
  WatchResult` makes the context the single lifecycle control, lets the first
  send be the seed (no separate Resolve-then-Watch dance), and reads as the
  idiomatic Go stream shape (`for r := range ch`). Errors travel on the channel
  as a `WatchResult.Err`.
- **`Endpoint` and `Instance` are separate structs, not one unified type.** A
  single struct (kratos `ServiceInstance`) would force each side to carry fields
  only meaningful on the other (`ID` write-only, `Healthy` read-only). Splitting
  is honest about the asymmetry.
- **Mesh switch and trace propagation live outside this package.** Earlier
  drafts kept a mesh branch inside the resolver and a trace seam here; both were
  removed because neither is discovery's concern, and the mesh branch could not
  even produce a dialable address from a service name alone.
