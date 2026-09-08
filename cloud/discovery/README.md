# discovery

[English](README.md) | [中文](README_CN.md)

`discovery` answers one question for infrastructure clients (Redis, MySQL,
MongoDB, Kafka, ...): *"given a logical service name, which live host:port
addresses can I connect to right now?"* A naming service is adapted once;
every client consumes the same contract. The read side only — publishing this
process to a registry is the `starter-registry-*` starters' job.

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
// label, e.g. ${spring.discovery.etcd.<name>}); nil means "no discovery".
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
  underlying registry notifies it (watch, subscription, poll).
- Backends are NAMED BEANS in the IoC container (bean name = config label,
  e.g. `spring.discovery.etcd.prod` registers a bean named "prod"); a client
  cites the label in its config and the starter injects the bean by that name.
  The container is the discovery directory — duplicate labels and typos fail
  loudly at wiring time.

No live registry? Use the built-in static backend:

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
| `WithTag(t)` | a registry-native marker (Consul service tag, registry label) | backends whose registry supports tags, in the registry call; others ignore it |

Both are no-ops when empty — pass config values through unconditionally.

## Resolver: the bound by-name consumer

```go
load, err := discovery.NewResolver(ctx, "default", "orders-redis")
bal, _ := loadbalance.New(loadbalance.RoundRobin)
pool := loadbalance.NewPool(loadbalance.SourceFunc(load), bal)
ep, err := pool.Pick(loadbalance.PickInfo{})
```

- `NewResolver` binds a backend label + service name (plus options) once and
  seeds with one synchronous `Resolve` (fail-fast); it returns `(nil, nil)` —
  "discovery not in effect" — when name is empty or mesh mode is on, so the
  caller dials its configured address directly.
- Each call to the resolver re-reads the backend snapshot and surfaces the
  error — a cheap in-memory read for a cache-backed backend, and honest (a
  registry hiccup is not hidden).
- Endpoint selection — round-robin, weights, consistent hash, failure
  ejection — all of that lives one layer up in
  [`loadbalance`](../loadbalance/README.md), which takes the resolver (via
  `loadbalance.SourceFunc`) as its endpoint source. Discovery itself carries
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
    // read the cached snapshot (seed it with a registry query on first call,
    // keep it fresh with the registry's own watch/subscribe/poll mechanism);
    // apply discovery.FilterByScheme(raw, q.Scheme);
    // honor q.Tag in the registry call if it supports tags
}

// in a starter's module wiring:
r.Provide(func() (discovery.Discovery, error) { return &myBackend{}, nil }).Name("default")
```

Concurrency-safe, and SDK-backed adapters (Nacos / Consul / etcd / DNS /
Kubernetes) live in their own starters so this package stays dependency-free.
