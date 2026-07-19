# discovery Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`discovery` sits in stdlib (zero-dependency foundation). It defines the
narrow contract every naming-service adapter satisfies and hosts the shared
dialer plumbing every infrastructure client reuses, so a company's naming
service is adapted once and works across Redis, MySQL, MongoDB, Kafka, gRPC.

## 1. Responsibilities & Boundaries

- **Does:** define `Discovery` (Resolve / Watch), `Endpoint`, `LiveDialer`
  (cold-start snapshot + background Watch + round-robin dial), a mesh
  degradation switch, and a companion `Registrar` for publishing the current
  process.
- **Refuses:**
  - No provider-side registration of RPC frameworks (kitex, kratos,
    dubbo-go, ...). Each framework already has its own registry model; a
    unifying wrapper would only be a translation layer per framework — a
    net negative. See `starter/DESIGN.md` §3.
  - No load-balancing strategy. `LiveDialer.Pick` is minimal round-robin;
    real strategies (weighted, least-conn, consistent-hash, zone-aware) live
    in `go-spring.org/stdlib/loadbalance`, which sits *above* discovery.
  - No concrete backend. Nacos / Consul / etcd / DNS / Kubernetes adapters
    live in their starters and register themselves under a name.

## 2. Key Abstractions / Seams

- **Two-method `Discovery` interface** (Resolve + Watch) keeps the adapter
  surface as small as possible. Backends that cannot stream expose a Watcher
  that polls internally, so the contract stays uniform.
- **Package-level backend registry with panic-on-init errors** mirrors the
  driver-registry idiom used by `resilience` and `cache`. Duplicate/empty/
  nil registration is a wiring bug, not a runtime condition.
- **`LiveDialer` is the shared dialer surface.** Its `DialContext(ctx, network,
  addr)` matches Redis and pgx; its `Dial(ctx, addr)` matches go-sql-driver
  and ClickHouse; it also satisfies mssql's `Dialer` interface directly.
  Clients pass a service label as `Addr`, the dialer ignores it and dials
  the currently picked endpoint.
- **`Registrar` is the write-side counterpart to `Discovery`.** It exists
  for VM / bare-metal / hybrid deployments where the platform does not
  register instances for you; in Kubernetes the platform already does that
  for every Pod. The `Registrar` and `Discovery` registries share the same
  mutex — both are populated at init before any lookup.
- **Mesh switch is read centrally** at `NewLiveDialer` / `NewClientDialer`
  / `loadbalance.Pool.Pick`, not at every client starter, so degradation
  happens uniformly with no per-component branching. In mesh mode the dialer
  exposes a single stable endpoint (the service name / ClusterIP) so the
  sidecar owns discovery and LB.

## 3. Constraints

- Backends and dialers must be safe for concurrent use; `LiveDialer` uses
  an `atomic.Pointer[[]Endpoint]` for the snapshot and `sync.Once` for
  `Stop`, so callers can Stop concurrently with dialing.
- Endpoints marked `Healthy` are preferred by `Pick`; when *no* endpoint is
  marked healthy (backends that do not track health), all endpoints are
  eligible — discovery must not black-hole traffic just because a backend
  omits health reporting.
- `Watcher.Next` blocks until a new snapshot or a stop/error; when it errors
  the watch loop exits — callers must handle a stalled watch by relying on
  the last-known snapshot rather than retrying inside the loop.
- Mesh mode is set once at startup, before any dialer is built. Changing it
  at runtime is not supported.
- No provider registration seam is added here. `Registrar` is for
  traffic-agnostic instance registration; RPC-framework provider
  registration stays inside each framework's own configuration.

## 4. Trade-offs / Alternatives Rejected

- **Client-side only.** A single unifying `Registrar` for RPC frameworks was
  rejected because kitex `registry.Registry`, kratos `registry.Registrar`,
  dubbo-go's config-only registration and go-zero's `discov.EtcdConf` all
  differ enough that a wrapper is just a translator; opt-in framework-native
  registration is used everywhere it exists. Only when a framework has no
  native mechanism (bare gRPC/thrift/HTTP) is `Registrar` used.
- **`LiveDialer.Pick` is minimal round-robin, not weighted/consistent-hash.**
  Selection strategy belongs one layer up; keeping discovery focused
  prevents overlap with `loadbalance` (which owns strategy + eviction).
- **Mesh switch is a process-global atomic, not a per-client flag.** Mesh
  mode is an infrastructure decision — either the sidecar is injected or
  it is not — so it degrades every client at once instead of demanding
  per-config plumbing.
