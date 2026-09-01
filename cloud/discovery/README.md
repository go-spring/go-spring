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

d, err := discovery.GetDiscovery("default")     // a backend registered by a starter
if err != nil { return err }

r, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }                    // fail-fast: no endpoints at construction
defer r.Stop()

ep, err := r.Pick()                             // one eligible endpoint, round-robin
if err != nil { return err }
conn, err := net.Dial("tcp", ep.Addr)           // the socket + pool are the client's
```

## Discovery: the backend contract

```go
type Discovery interface {
    Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
    Watch(ctx context.Context, name string, opts ...Option) (<-chan WatchResult, error)
}
```

- `Resolve` returns the current snapshot — called once at cold start.
- `Watch` returns a channel of snapshots; the first one arrives immediately and
  is the current state, later ones are full replacements (never deltas). Cancel
  ctx to close the channel; a terminal backend error arrives as
  `WatchResult.Err` and the channel then closes — keep serving from the last
  snapshot (stale addresses beat none).
- Backends register themselves by label (`RegisterDiscovery("default", b)`);
  `GetDiscovery` resolves the label and its error lists every registered name,
  so a typo or a missing starter is obvious at construction. Empty name, nil
  backend, or a duplicate registration panics — it is a wiring bug.

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

## Resolver: the ready-made consumer

```go
r, err := discovery.NewResolver(ctx, d, "orders-redis")
defer r.Stop()
ep, err := r.Pick()
```

- Seeds from one synchronous `Resolve` (fail-fast), then refreshes via a
  background `Watch` — the snapshot is always fresh, no polling on your side.
- `Pick` is plain round-robin over the eligible set. Weights, consistent
  hash, failure ejection — all of that belongs one layer up in
  [`loadbalance`](../loadbalance/README.md), which wraps a `Resolver` as its
  endpoint source.
- Concurrency-safe; `Stop` may run concurrently with `Pick` and from a bean
  destructor.

Prefer to own the endpoint set yourself? Watch directly:

```go
ch, _ := d.Watch(ctx, "orders", discovery.WithScheme("grpc"))
for res := range ch {
    if res.Err != nil { break }        // keep the last snapshot and keep serving
    replaceAllEndpoints(res.Endpoints)
}
```

## Catalog: optional enumeration

Backends that can list every service name (gateway route building,
dashboards) implement `Catalog`; those that can't (DNS, static, a k8s headless
Service reached by name) simply don't:

```go
if c, ok := d.(discovery.Catalog); ok {
    names, _ := c.Services(ctx)
}
```

## Writing a backend

```go
type myBackend struct{ /* naming client */ }

func (b *myBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
    q := discovery.NewQuery(name, opts...)
    // query the registry; apply discovery.FilterByScheme(raw, q.Scheme);
    // honor q.Tag in the registry call if it supports tags
}

func (b *myBackend) Watch(ctx context.Context, name string, opts ...discovery.Option) (<-chan discovery.WatchResult, error) {
    // full snapshot on every topology change (first one immediately);
    // close on ctx cancellation, or deliver WatchResult.Err then close
}

func init() { discovery.RegisterDiscovery("default", &myBackend{}) }
```

Concurrency-safe, and SDK-backed adapters (Nacos / Consul / etcd / DNS /
Kubernetes) live in their own starters so this package stays dependency-free.
