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

resolver, err := discovery.NewResolver(ctx, "default", "orders")
if err != nil { return err }

bal, _ := loadbalance.New(loadbalance.RoundRobin)
tracker := loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold:  3,              // consecutive failures before suspension
    SuspendFor: 5 * time.Second, // half-open trial after a 5s cool-down
})
pool := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal, loadbalance.WithTracker(tracker))

for {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(ep.Addr)     // your RPC/HTTP call
    pool.Complete(ep, err)  // must be paired: settles accounting + feeds the tracker
}
```

## Pool: assembly and filtering

`Pool` glues three things into a runtime: an endpoint source, a strategy, and
an optional `Tracker`.

```go
pool := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal, loadbalance.WithTracker(tracker))
```

- **The endpoint source** is anything implementing
  `Endpoints() ([]discovery.Endpoint, error)`; a discovery `Resolver` plugs in via
  `loadbalance.SourceFunc(resolver)` — freshness lives entirely inside the
  discovery backend, so each `Pick` re-reads the latest snapshot.
- Each `Pick` filters in order: **discovery eligibility** (disabled/unhealthy
  instances) → **suspension** (instances cooling down in the `Tracker`) →
  **zero-weight drain** (instances whose weight was set to 0). The survivors
  go to the strategy. No filter may empty a non-empty set — traffic is never
  black-holed.
- **Mesh mode** (`discovery.MeshMode()` on) degrades to a single stable
  endpoint automatically — the sidecar owns LB, no code change needed.

## Balancer: strategies

The strategy decides "which survivor wins". Seven are built in, registered
under stable names; `New` fetches by name, `Register` adds your own.

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

A custom strategy implements the `Balancer` interface and registers like any
built-in:

```go
loadbalance.Register("my_strategy", func() loadbalance.Balancer {
    return &myBalancer{} // implement Pick and Complete; be concurrency-safe
})
```

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
the full set. `Threshold <= 0` (or no `WithTracker`) is fully transparent.

## Managed selection (governance)

A pool's strategy and suspension thresholds can be driven from outside the
process, by resource label rather than by construction-time argument:

```go
stop := pool.BindSelection("http:user-svc") // no-op when governance is absent
defer stop()
```

- `BindSelection(label)` applies the label's current `Selection` **immediately**
  and again on every change, in place — no rebuild, no re-dial, the very next
  `Pick` sees it. It returns the detach func; a pool that is not process-lifetime
  MUST call it, or the authority keeps a callback pointing at a dead pool.
- With no provider registered (the governance starter is not imported, or
  governance is off) the call is a no-op and the pool keeps the strategy it was
  built with — the same transparent pass-through as `resilience.ExecutorFor`.
- An empty strategy name leaves the current strategy alone; an **unknown** name
  is ignored and the last good strategy stays in force. The suspension thresholds
  are always applied.
- The suspension half only has an effect on a pool with a `Tracker` attached
  (`WithTracker`) whose `Pick` is paired with `Complete` — without both, the
  thresholds are set on nothing.

`RegisterSelectionProvider` is called once, by whoever owns the policy — the
governance starter calls it when it goes live; clients never call it. Passing
`nil` disarms the seam again, which is what makes it testable in-process.

`Pool.ApplySelection` is the same sink reached directly, and `Pool.Selection()`
reads back the policy most recently accepted — useful when you drive selection
from your own config instead of a provider.

## The Pick/Complete contract

The two calls must be paired **exactly once**. Skipping `Complete`:
`least_conn`'s in-flight count leaks (that instance starves), `p2c`'s latency
model drifts, and the `Tracker` goes blind (suspension stops working).
