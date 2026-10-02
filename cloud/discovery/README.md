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
