# discovery

[English](README.md) | [中文](README_CN.md)

`discovery` answers one question for infrastructure clients (Redis, MySQL,
MongoDB, Kafka, ...): *"given a logical service name, which live host:port
addresses can I connect to right now?"* A naming service is adapted once;
every client consumes the same contract. Both sides live here: this package
drives the process's own publication (`Server`), and each `starter-discovery-*`
backend contributes the `Registry` that talks to its center.

## Installation

```
go get go-spring.org/cloud
```

## Quick start

```go
import (
    "context"
    "net"

    "go-spring.org/cloud/discovery"
)

// d is the discovery backend bean a starter injected (named after its config
// label, e.g. the "etcd.main" backend derived from ${spring.discovery.etcd.main}); nil means "no discovery".
load, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }                    // fail-fast: no endpoints at construction
if load == nil { return err }                    // "not in effect" (no backend/name/mesh): dial addr directly

eps, err := load()                               // the live snapshot, error surfaced
if err != nil { return err }
conn, err := net.Dial("tcp", eps[0].Addr)        // the socket + pool are the client's
```

## Discovery: the backend contract

```go
type Discovery interface {
    Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
}
```

- `Resolve` returns the current snapshot. It may block on the first call for a
  service (seed fetch, bounded by ctx); later calls are cheap reads — freshness
  lives INSIDE the backend, which keeps its cache current however the
  underlying backend notifies it (watch, subscription, poll).
- Backends are NAMED BEANS in the IoC container (each discovery starter derives
  one from its `${spring.discovery.<backend>.<name>}` block named
  "<backend>.<name>", e.g. "etcd.main"); a client
  cites the label in its config and the starter injects the bean by that name.
  The container is the discovery directory — duplicate labels and typos fail
  loudly at wiring time.

No live backend? Use the built-in static backend:

```go
d := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "127.0.0.1:6379", Scheme: "tcp"},
)
```

## Endpoint: eligibility

```go
type Endpoint struct {
    Addr     string            // host:port
    Scheme   string            // "tcp"/"" plain, or "tls", "grpc", ...
    Weight   int               // consumed by loadbalance
    Disabled bool              // operator/provider decree: drain, maintenance
    Healthy  bool              // probe result
    Metadata map[string]string
}
```

`Disabled` and `Healthy` are independent dimensions, and the eligibility order
matters:

```
pick from: !Disabled && Healthy
   none healthy? degrade to: !Disabled
   never: Disabled — not even as a fallback
```

This prevents the classic bug where an operator-disabled instance is
resurrected by the "no healthy → use all" fallback.

## Options: narrowing a lookup

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"), discovery.WithTag("v2"))
```

| Option | Semantics | Honored by |
|---|---|---|
| `WithScheme(s)` | restrict to one transport scheme; empty scheme and `"tcp"` are equivalent | every backend, via `FilterByScheme` |
| `WithTag(t)` | a backend-native marker (Consul service tag, discovery label) | backends that support tags, in the backend call; others ignore it |

Both are no-ops when empty — pass config values through unconditionally.

## Resolver: the bound by-name consumer

```go
backend := discovery.NewStaticDiscovery(/* ... live endpoints from the backend ... */)
load, err := discovery.NewResolver(ctx, backend, "orders-redis")
bal := loadbalance.NewRoundRobin()
pool := loadbalance.NewPool(load, bal)
ep, err := pool.Pick(loadbalance.PickInfo{})
```

- `NewResolver` binds a backend label + service name (plus options) once and
  seeds with one synchronous `Resolve` (fail-fast); it returns `(nil, nil)` —
  "discovery not in effect" — when name is empty or mesh mode is on, so the
  caller dials its configured address directly.
- Each call to the resolver re-reads the backend snapshot and surfaces the
  error — a cheap in-memory read for a cache-backed backend, and honest (a
  backend hiccup is not hidden).
- Endpoint selection — round-robin, weights, consistent hash, failure
  ejection — all of that lives one layer up in
  [`loadbalance`](../loadbalance/README.md), which takes the resolver (via
  the resolver func) as its endpoint source. Discovery itself carries
  no selection policy, and the resolver owns no resources — freshness lives
  inside the backend, so there is nothing to stop.

Prefer to own the endpoint set yourself? Call `Resolve` whenever you need a
fresh snapshot:

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"))
```

## Writing a backend

```go
type myBackend struct{ /* naming client */ }

func (b *myBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
    q := discovery.NewQuery(name, opts...)
    // read the cached snapshot (seed it with a backend query on first call,
    // keep it fresh with the backend's own watch/subscribe/poll mechanism);
    // apply discovery.FilterByScheme(raw, q.Scheme);
    // honor q.Tag in the backend call if it supports tags
}

// in a starter's module wiring:
r.Provide(func() (discovery.Discovery, error) { return &myBackend{}, nil }).Name("default")
```

Concurrency-safe, and SDK-backed adapters (Nacos / Consul / etcd / DNS /
Kubernetes) live in their own starters so this package stays dependency-free.

## Publishing: the registration core

`Server` owns this process's publication lifecycle across every configured
center — the ONE bean that registers everywhere, deregisters everywhere and
broadcasts weight changes. Each backend starter derives one `Registry` per
configured `${spring.discovery.<backend>.<name>}` block; `Server` collects them
ALL, across backends, and drives them in lockstep:

- registered everywhere once the application is ready;
- deregistered everywhere as shutdown begins (PreStop, before any server stops
  — the lossless-drain sequence);
- weight changes broadcast to all centers (via `UpdateWeight`).

Any registration failure fails startup: a center missing this instance would
split consumers' views. Nothing registers until the app is ready, and the
whole thing is conditional on the registration intent signal below.

```go
type Registry interface {
    Register(ctx context.Context, inst Instance) error      // called once, after ready
    Deregister(ctx context.Context, inst Instance) error    // idempotent
    UpdateWeight(ctx context.Context, inst Instance, weight int) error
}
```

`UpdateWeight(ctx, weight)` on the `discoveryServer` bean re-advertises the
instance with a new weight in every center — the entry never leaves discovery;
watchers (loadbalance pools included) observe the new value on their next
snapshot.

### Configuration

The discovery configuration surface splits into **three parts**, each
independent and each optionally present on its own:

| part | answers | keys | who sets it |
|------|---------|------|-------------|
| ① instance identity | who am I | `${spring.discovery}.*` | providers only |
| ② discovery centers | where to register / discover | `${spring.discovery.<backend>.<name>}.*` | providers and/or consumers |
| ③ citation | which center a consumer uses | each client starter's own config | consumers only |

**① Instance identity** — `${spring.discovery}.*`: ONE set of fields shared by
every center. `service-name` is the registration intent signal: **set = publish
this process; unset = pure consumer** (registers nothing anywhere).

```properties
spring.discovery.service-name=orders
spring.discovery.addr=10.0.0.5:8080      # required when registering
# spring.discovery.id=                   # optional, derived when empty
# spring.discovery.weight=100            # 0 = drained; negative normalized to 1
# spring.discovery.version=              # app version, consumers may canary on it
# spring.discovery.zone=                 # availability zone, consumers may prefer same-zone
# spring.discovery.scheme=               # transport hint (tcp/tls/http/https)
# spring.discovery.metadata.zone=cn-north
```

**② Discovery centers** — one NAMED block per center: any number of blocks, any
mix of backends (etcd + zookeeper in one process is fine). Each block is a bean
named `<backend>.<name>` serving **both halves**: the write side collected by
`Server` as a `Registry`, and the read side cited by consumers as a `Discovery`
backend. Each backend's full key set lives in its starter's README/USAGE.

```properties
spring.discovery.etcd.main.endpoints=10.0.0.1:2379
spring.discovery.etcd.dr.endpoints=10.9.0.1:2379      # dual registration
spring.discovery.zookeeper.bz.servers=10.1.0.1:2181   # mixed backends
```

**③ Citation** — consumers name a center by its bean name, written in each
client starter's own config; discovery itself needs no configuration, and the
the backend never knows who cites it.

```properties
spring.http-client.instances.users.service-name=users
spring.http-client.instances.users.discovery=etcd.main    # bean name "<backend>.<name>"
spring.http-client.default.discovery=etcd.main           # family default
```

How the three relate:

- **① and ② are decoupled**: a block plus `service-name` registers, even if no
  consumer ever cites it; conversely a pure consumer configures blocks with no
  `service-name` and registers nothing.
- **`service-name` set with no block = startup fails** (fail-fast).
- **k8s appears only in ② and ③**: it is a discovery-only backend — citable,
  never registered through (the platform already registers Pods), so it never
  touches ①.
- **One process can be both provider and consumer**: configure ① + ② + ③.

`Server` opens no port; it plugs into the Go-Spring server lifecycle. It ships
no backend of its own — with `service-name` set and no center
configured, startup fails fast ("no discovery center is configured") rather than
registering nowhere.

## The Manager: a directory over backends

`cloud/discovery` also ships a small `Manager`:

```go
func NewManager(backends map[string]Discovery) *Manager
func (m *Manager) Get(label string) (Discovery, bool)
func (m *Manager) Labels() []string
```

- **nil-receiver safe** (a nil `Manager` misses on everything, never panics),
  immutable after construction (lock-free), and `Labels()` returns nil rather
  than an empty slice when there are no backends. (2026-10-03)
- It is wired as
  `gs.Provide(func(backends map[string]Discovery) *Manager, gs.IndexArg(0, gs.TagArg("?")))`
  — exactly isomorphic with the resilience / loadbalance / fault authorities
  (whoever owns the type registers it); with no backend beans the map is nil =
  an empty directory.
- The `Manager` is carried by the governance center: `Center` holds a
  `disc *discovery.Manager` field with a `Discovery()` accessor, and
  `NewCenter`'s fifth parameter is that `*discovery.Manager`. **Discovery does
  not enter governance semantics** (the rules document / `adopt` / `dispatch` /
  `Config` schema never touch it) — the center only holds it, never rules on it.
  This single-entry-point decision is deliberate even though it is semantically
  impure; it is not to be reopened. (2026-10-03)
- Clients all resolve through `center.Discovery().Get(label)`: map-injected
  clients (http-client `newRoute`, gateway `newRouteTable`, grpc) hold a
  `*discovery.Manager`; name-injected clients (mongodb / elasticsearch / neo4j /
  memcached / go-redis / redigo / gorm) take a `discoveryLabel string` and do
  `disc, _ := center.Discovery().Get(discoveryLabel)` inside the ctor. The
  parameter name is uniformly `discoveryLabel` (to avoid colliding with the
  governance service `label`).
- Collection injection is still a valid shape for multi-entry clients
  (`map[string]discovery.Discovery` + `TagArg("?")`, the http-client idiom);
  downstream clients have converged on `center.Discovery().Get(label)`.

## Freshness, and why the resolver holds no policy

- `Discovery` keeps only `Resolve` (a snapshot read; the first call seeds it).
  `Watch` / `WatchResult` / the channel contract are deleted, and all freshness
  lives inside each backend — do not propose restoring the `Watch` interface.
  (2026-09-04)
- Freshness mechanism per backend: event push (etcd `clientv3.Watch`, nacos
  `Subscribe` callback refreshing a cache), read-through TTL (the DNS type, no
  goroutine), informer (k8s endpointslice, `Close` stops them all), or a
  blocking-query loop (consul).
- `discovery.Resolver` is the function type `func() ([]Endpoint, error)`; the
  constructor `NewResolver(ctx, backend, name, opts)` binds backend+name once,
  seeds with a synchronous `Resolve` (fail-fast), returns `(nil, nil)` for
  mesh/empty-name, and holds **no resources and no `Stop`**.
- **The resolver does no endpoint selection.** `Pick` and the internal mini-RR
  are deleted; selection belongs entirely to `loadbalance.Pool`. The resolver
  (connect time, slow) and the pool (per request, fast) are two selectors for two
  timescales — connection-pool clients use the former, RPC/httpx use the latter.
  A future infra client that needs weighted reconnects should layer
  `loadbalance` internally; **do not add a policy to the resolver**. (2026-09-04)
- `Pool` takes a `discovery.Resolver` directly:
  `NewPool(src discovery.Resolver, bal Balancer, opts ...PoolOption)`, and
  `Pool.Pick` propagates the source error. `EndpointSource` / `SourceFunc` are
  deleted; `discovery.Allows` long ago moved to loadbalance's private
  `admission`.
- Connection-pool consumption shape: `NewPool(resolver, bal)`, then
  `pool.Pick(PickInfo{})` inside the dial closure; gormcore offers the shared
  `Common.NewResolver` / `NewPickPool`.
- "Cloud has no spring dependency" was downgraded from an iron rule to a
  default preference (the user may explicitly break it).

## Backend starter conventions

- A discovery backend is a **named bean** in the container — the bean name is
  the config label verbatim, no prefix — not a global map inside
  `cloud/discovery`. `RegisterDiscovery` / `GetDiscovery` are deleted.
  (2026-09-08)
- `NewResolver(ctx, d Discovery, name string, opts...)` takes the injected
  instance; `d` nil / empty name / mesh → `(nil, nil)`.
- Copy `starter-discovery-etcd` when writing a new backend (files
  `discovery.go` + `registry.go`): `r.Provide().Name()` + a bean `Destroy`; a
  label that is configured but injects nil must **fail loud**, listing the
  available names.
- Non-exported backend implementation type names (`etcdRegistry` /
  `zkRegistry` / `consulRegistry` / `nacosRegistry`) need not mirror the
  interface name (precedent: `staticBackend implements Discovery`).
- `Registry.UpdateWeight(ctx, inst, weight)` signature trap: in the etcd
  implementation `inst` is used only for locating; its other fields — including
  `inst.Weight` — are ignored. Do not take the current behavior as the contract.

## Registry family configuration (v2 — the old single-block scheme is dead)

- The namespace is **`spring.discovery.*`** (NOT `spring.registry.*` — the
  latter has zero hits in code). Blocks are
  `spring.discovery.<backend>.<name>.*`, and there is **no default block**
  (consistent with the client starter's multi-instance convention; a default
  block at the same level as named blocks is undecidable because scalar/subtree
  cannot be told apart). (2026-09-09)
- One block, one backend bean named `<backend>.<name>`, the **same bean
  implementing both `discovery.Registry` and `discovery.Discovery`** (one bean,
  two Exports); connection / liveness / key-prefix or namespace have a single
  source.
- `discoveryServer` is the one `gs.Server`, injecting
  `Registries []discovery.Registry  autowire:"?"` to collect **every** registrar
  across backends: register all when ready / deregister all on PreStop /
  broadcast `UpdateWeight`; it binds the global `${spring.discovery}` identity
  (`service-name` being the registration-intent signal). Backend starters
  `import _` transitively; Go package-init dedup guarantees a single `Server` —
  do **not** use `OnMissingBean` to negotiate.
- A client cites a center with `discovery=etcd.main`: the key is `discovery`,
  the value is the backend bean name (prefixed with the backend type, so it never
  collides across backends). Client side: `spring.go-redis.default.discovery`.
- Server side defaults to writing everywhere: several blocks + a `service-name`
  = Dubbo-style full dual registration; a pure consumer = blocks with no
  `service-name`.
- Value dispatch (`type=` as a string) is rejected — it needs a global registry
  plus activation negotiation and late binding, contradicting "the type is in
  the key" and "the container only assembles".
- Registration shape: each backend builds one shared client bean (with a
  startup probe); both the registrar `Server` and the discovery-side backend
  derive from it; the discovery-side bean is derived from the center
  automatically (default name = backend name), so a dual-role app configures the
  cluster once and reads and writes under the same prefix/scope without drift.
- New backends copy the four existing shapes (`BindEach(p, "${spring.discovery.<backend>}", …)`
  + `OnProperty` prefix + double Export + import the core); registrars need no
  name (interface collection); multiple clusters = multiple blocks.
- `k8s` is in the backend family — config `spring.discovery.k8s.<name>`, bean
  `k8s.<name>`, client `discovery: k8s.<name>` — but the **directory name stays
  `starter-discovery-k8s`** (it was not renamed to `starter-registry-*`). Naming
  uniformity beats the "registry implies a write side" argument: backends with no
  registrar are still in the family, documented as discovery-only.
- **Discovery-only member marker:** it does not import the registry core, offers
  no `discovery.Registry`, and reads neither `${spring.discovery.service-name}`
  nor `.addr`. That exception is registered in three places (don't flag it as a
  misconfiguration): `scripts/check-observability.sh`'s `NO_REGISTRAR="k8s"` and
  `starter/DESIGN{,_CN}.md` §3/§4. A discovery-only backend still reports the
  read side (`obsSystem` + both success and failure `discovery.Synced`).
- etcd and zookeeper each hold a verbatim-duplicate `instanceValue` JSON struct
  (and `withDimensions`) and are **not merged**: the wire format is each
  backend's frozen compatibility surface; sharing the type would couple the two
  formats' evolution (a field added for one would silently become another's
  stored data). The three "similar-looking" layers — `config` (binding),
  `discovery.Instance` (domain), `instanceValue` (storage) — serve different
  roles; don't propose sinking them down.
