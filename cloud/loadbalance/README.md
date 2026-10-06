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
    pool.Complete(ctx, ep, err)  // must be paired: settles accounting + feeds the tracker
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

A custom strategy implements `Balancer` (Pick/Complete, concurrency-safe;
`Complete` receives the context of the request it settles) and is
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
(a zombie). It learns only from the `err` passed to `Complete` — no extra calls:

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

## Coverage: what is governed, and the re-pick test

Selection is driven **only** through the governance channel — the `balancer`
name plus the `balancer-params` sub-map's `replicas` / `zone-key` / `delegate`,
via `Selection.Params` → `Manager.Apply` → `Pool.ApplyBalancer` (hot-swapped in
place). A starter invents no local static `balancer` key of its own; each
starter's pool defaults to `loadbalance.NewRoundRobin()` directly (no lookup by
name, no error). The `Factory` parameterization is kept, to carry parameters on
the governance channel.

Covered clients: every discovery-mode `loadbalance.Pool` consumer —
`httpx`/`gateway`, all four gorm dialects, `redigo`, `go-redis`, `mongodb`,
`grpc`.

> Self-check: **does this client pick an endpoint again on every request /
> every connect?** Yes → its label can carry a `balancer`. No → don't expect
> `spring.governance.client.rules[N].balancer` to affect it.

The "no" cases, each deliberate:

- **Direct connect** (a fixed addr/host): no candidate set to choose from.
- **Pick-once** (`neo4j`): the host is frozen into the startup URI; there is no
  per-request pick left to govern (its governance stops at the protection policy).
- **Mature library owns the choice** (`elasticsearch` / `memcached` / the MQ
  family / `s3`): the selection lives inside the library (ES's node selector,
  memcached's per-key consistent hash). The rule: don't build a second selector
  when the library already has a good one — feed it the live addresses instead.

**Eviction granularity for a DB/cache client is the connection, not the query.**
For gorm / redigo / go-redis / mongodb the pick happens at connect time, so the
only success/failure signal the `Tracker` can see is the dial result; a failed
single query is handled by that client's own resilience executor and does not
feed the tracker.

**`grpc` picks under a process-level label.** Its selection label is the
process-wide `grpc:client`, so setting `balancer=` on it also retunes every
builtin `gs_*` balancer; to isolate per service, register a custom name via
`RegisterBalancer` (custom `Factory` beans are exempt from the process-level
override by design).

**Building a pool** takes three parts: `WithTrackerConfig` + a paired
`Pick` / `Complete` at the connect site + subscribing to the governance-pushed
`Selection`. New clients copy this (it is written into `starter/DESIGN.md` §4 as
a checklist). One label may map to several pools (the sink is a pool, not an
object), so the manager does **not** memoize: each `Bind` is an independent
subscription owned by the pool, and a pool must cancel it on destruction.

## Weight semantics

- `Endpoint.Weight == 0` means **drain**. (2026-08-27)
- Every consumer-side LB strategy uniformly filters out `Weight == 0` endpoints
  (after health/tracker filtering); when all are zero it falls back to the
  pre-filter set (never black-holes), and **negative** weights stay in rotation.
- On the write side, registrar `Register` normalizes `weight <= 0` to 1 ("unset"
  never lands at 0); `UpdateWeight(0)` is an explicit drain and is allowed to
  land at 0. Do not normalize 0 back to 1 — that is `Register`'s job.
- **Drain and deregister (`Disabled`) are two different things**: the former
  rotates with zero loss, the latter removes immediately.

## Address freshness: a separate axis

Who picks a node is independent of whether the address set follows the naming
service. The covered clients re-read on every connect/request; the
library-owned ones attach their own live set: ES installs a
`ConnectionPoolFunc`, memcached installs a `ServerSelector` (keeping its
CRC32-of-key hash plus address ordering), and neo4j installs the driver's
`AddressResolver` (only in `neo4j://` routing mode). On a read failure each
keeps the last usable set and **never returns an empty set**.

## The Pick/Complete contract

The two calls must be paired **exactly once**. Skipping `Complete`:
`least_conn`'s in-flight count leaks (that instance starves), `p2c`'s latency
model drifts, and the `Tracker` goes blind (suspension stops working).

`Complete` takes the context of the request it settles, which is what lets a
strategy's own telemetry and logging attach to the request that caused the
state change — the tracker's suspension and recovery lines are traced to it.
What it does is in-memory bookkeeping: it must not perform I/O, and it must
not depend on the context still being live.
