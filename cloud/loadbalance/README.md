# loadbalance

[English](README.md) | [中文](README_CN.md)

`loadbalance` is the client-side load-balancing layer on top of
`go-spring.org/cloud/discovery`. Discovery answers "which instances exist
right now?"; this package answers "given that live set, which one do I send
this request to?" — and suspends instances that keep failing.

## Installation

```
go get go-spring.org/cloud
```

## Quick start

```go
import (
    "context"
    "time"

    "go-spring.org/cloud/discovery"
    "go-spring.org/cloud/loadbalance"
)

backend := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "10.0.0.1:8080", Scheme: "tcp"},
    discovery.Endpoint{Addr: "10.0.0.2:8080", Scheme: "tcp"},
)
resolver, err := discovery.NewResolver(ctx, backend, "orders")
if err != nil { return err }

bal := loadbalance.NewRoundRobin()
pool := loadbalance.NewPool(resolver, bal, loadbalance.WithTrackerConfig(loadbalance.TrackerConfig{
    Threshold:  3,              // consecutive failures before suspension
    SuspendFor: 5 * time.Second, // half-open trial after a 5s cool-down
}))

for {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(ep.Addr)     // your RPC/HTTP call
    pool.Complete(ep, err)  // must be paired: settles accounting + feeds the tracker
}
```

## Pool: assembly and filtering

`Pool` glues three things into a runtime: an endpoint source, a strategy, and
its own `Tracker` (built disabled, so suspension becomes governable without
any wiring; set initial thresholds with `WithTrackerConfig`).

```go
pool := loadbalance.NewPool(resolver, bal)
```

- **The endpoint source** is a `discovery.Resolver` —
  `func() ([]discovery.Endpoint, error)`. Freshness lives entirely inside the
  discovery backend, so each `Pick` re-reads the latest snapshot.
- Each `Pick` filters in order: **discovery eligibility** (disabled/unhealthy
  instances) → **suspension** (instances cooling down in the `Tracker`) →
  **zero-weight drain** (instances whose weight was set to 0). The survivors
  go to the strategy. No filter may empty a non-empty set — traffic is never
  black-holed.
- **Mesh mode** (`mesh.Enabled()`): no pool is built at all —
  `discovery.NewResolver` returns a nil Resolver and the caller dials its
  configured address directly, letting the sidecar balance.

## Balancer: strategies

The strategy decides "which survivor wins". Seven are built in; a governance
rule names one by name, and any `Factory` bean the container contributed can be
named the same way.

| Scenario | Strategy | Why |
|---|---|---|
| Homogeneous instances, no special need | `round_robin` | simplest, zero state |
| Uneven instance performance (slow disk, GC pressure) | `least_conn` or `p2c` | adapts by in-flight count / measured latency |
| Widely varying request durations (e.g. export endpoints) | `p2c` | in-flight is a lagging signal; p2c's EWMA tells busy from slow |
| Session / cache affinity needed | `consistent_hash` | same key lands on the same instance; scaling moves few keys |
| Mixed instance classes (4C8G vs 8C16G) | `weighted` | splits by capacity, interleaved smoothly |
| Multi-AZ / multi-region deployment | `zone_aware` | local first, level-by-level fallback, saves cross-zone latency and egress |
| Very high concurrency, stateless is fine | `random` | no shared cursor, no atomic hotspot |

Strategies come in two kinds: **stateless** (round_robin, weighted,
consistent_hash, random — `Complete` is a no-op) and **stateful** (least_conn
keeps an in-flight table; p2c keeps a latency model). All state is keyed by
endpoint address, so instances coming and going — or a reordered snapshot —
never disturbs the state of the survivors.

A custom strategy implements `Balancer` (Pick/Complete, concurrency-safe) and is
contributed as a named `Factory` bean — the bean name is the strategy name a
rule cites, and the strategy's own parameters arrive in `Params`:

```go
type myFactory struct{}

func (myFactory) Build(_ loadbalance.Directory, p *loadbalance.Params) (loadbalance.Balancer, error) {
    window, err := p.Duration("window", time.Second) // the strategy's own parameter
    if err != nil {
        return nil, err
    }
    if err := p.Done(); err != nil { // reject keys this strategy does not know
        return nil, err
    }
    return &myBalancer{window: window}, nil
}

// in a starter's init:
gs.Provide(func() loadbalance.Factory { return myFactory{} }).
    Name("my_strategy").
    Export(gs.As[loadbalance.Factory]()).Caller(1)
```

The core knows no strategy parameter — `Params` is the flat `balancer-params`
map of the rule, and the strategy reads the keys it owns and rejects the rest.
Adding a strategy, with parameters of its own, changes nothing in this package.

Under retry, simply `Pick` again per attempt — the candidate set is re-filtered
each time, so there is no (and needs no) failed-endpoint blacklist API;
persistent failures are removed by the `Tracker` automatically.

## PickInfo: routing hints

`Pick`'s second argument carries what this request may route on. All fields
are optional; stateless strategies ignore them:

```go
// Hash-key affinity (consumed by consistent_hash).
ep, _ := pool.Pick(loadbalance.PickInfo{HashKey: userID})

// Zone locality (consumed by zone_aware); accepts an ordered fallback
// list, tried level by level, spilling over only when every level is empty.
ep, _ := pool.Pick(loadbalance.PickInfo{Zone: "us-east-1a,us-east-1"})
```

## Tracker: outlier suspension

The `Tracker` covers the failure mode discovery cannot see: an instance that
is still registered and passes health checks but keeps failing real requests
(a zombie). It learns only from `Complete(err)` — no extra calls:

```
healthy --consecutive failures reach Threshold--> suspended (cooling down for
SuspendFor, no longer picked)
                        |
                  cool-down elapses → half-open trial (one request admitted)
          success → state cleared, back in service
          failure → re-suspended, cycle repeats
```

A single success resets the failure count, so sporadic failures never
trigger suspension; when every instance is suspended the filter falls back to
the full set. `Threshold <= 0` (the default when `WithTrackerConfig` is not used)
is fully transparent.

## Managed selection (governance)

A pool's strategy and suspension thresholds can be driven from outside the
process, by service label rather than by construction-time argument. The caller
injects the `*loadbalance.Manager` bean (see [Manager](manager.go)) and hands it
the pool:

```go
// mgr is the injected *loadbalance.Manager; nil means no governance is present.
stop := mgr.Bind(pool, "http:user-svc")
defer stop()
```

- `Bind(pool, label)` applies the label's current `Selection` **immediately** and
  again on every change, in place — no rebuild, no re-dial, the very next `Pick`
  sees it. It returns the detach func; a pool that is not process-lifetime MUST
  call it, or the manager keeps a callback pointing at a dead pool.
- Binding an **unarmed** manager is safe and is the normal case for a pool built
  during container wiring: the subscription is remembered and armed by the first
  `Apply`. With no manager injected at all (a container with no `cloud/governance`
  bean, or a standalone caller) the pool keeps the strategy it was
  built with — a transparent pass-through.
- An empty strategy name leaves the current strategy alone; an **unknown** name
  is ignored and the last good strategy stays in force. The suspension thresholds
  are always applied.
- The suspension half only has an effect on a pool whose `Pick` is paired with
  `Complete` — the tracker is always there, but unpaired calls leave it nothing
  to count, so the thresholds are set on nothing.

`Manager.Apply(Settings{...})` is the single entry point the governance center
calls — once with the source's snapshot, then on every push — and
`Manager.SelectionFor(label)` reads back what a label currently resolves to.

A rule names the strategy and carries its parameters as a flat sub-map, opaque to
this package:

```yaml
balancer: consistent_hash
balancer-params:
  replicas: 200
outlier-threshold: 5
```

`Bind` is where a strategy name becomes a strategy: it resolves the name against
the manager's `Directory` — the built-in strategies plus any `Factory` beans the
container contributed — and hands the pool a built `Balancer`, so a pool never
holds the factory table. `Pool.ApplyBalancer` (strategy) and
`Pool.ApplySuspension` (thresholds) are the same two halves reached directly, and
`Pool.Selection()` reads back the policy most recently accepted — useful when you
drive selection from your own config instead of the manager.

## The Pick/Complete contract

The two calls must be paired **exactly once**. Skipping `Complete`:
`least_conn`'s in-flight count leaks (that instance starves), `p2c`'s latency
model drifts, and the `Tracker` goes blind (suspension stops working).
