# loadbalance
[English](README.md) | [中文](README_CN.md)

`loadbalance` is the client-side load-balancing layer on top of
`go-spring.org/stdlib/discovery`. Discovery answers "which instances exist
right now?"; this package answers "given that live set, which one do I send
this request to?" — and evicts instances that keep failing.

## Features

- Five built-in strategies (each registered under a stable name):
  - `round_robin` — stateless, cycles evenly.
  - `least_conn` — picks the endpoint with fewest in-flight requests.
  - `consistent_hash` — FNV-32 ring with virtual nodes; hash-key affinity.
  - `weighted` — nginx smooth weighted round-robin (SWRR).
  - `zone_aware` — locality preference over a delegate balancer.
- `Factory` registry (`Register` / `New`) — strategies register themselves
  in `init` and can be swapped by name.
- `Tracker` — outlier-ejection with consecutive-failure threshold + half-open
  probe, keyed by endpoint address; queryable so `Pool` can evict before
  routing.
- `Pool` — binds an `EndpointSource` (a `discovery.LiveDialer` satisfies it
  directly), a `Balancer` and an optional `Tracker`; two-stage filtering
  (`Healthy` first, `Tracker.Eligible` next) with a never-black-hole
  guarantee — an empty filter always falls back to its input.
- Mesh mode: when `discovery.MeshMode()` is on, `Pool.Pick` degrades to a
  single stable endpoint and skips eviction so the sidecar owns LB.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

```go
import (
    "context"

    "go-spring.org/stdlib/discovery"
    "go-spring.org/stdlib/loadbalance"
)

ld, err := discovery.NewClientDialer(ctx, "default", "orders")
if err != nil { return err }
defer ld.Stop()

bal, _ := loadbalance.New(loadbalance.RoundRobin)
tracker := loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold: 3,
    EjectFor:  5 * time.Second,
})
pool := loadbalance.NewPool(ld, bal, loadbalance.WithTracker(tracker))

for {
    res, err := pool.Pick(loadbalance.PickInfo{Ctx: ctx})
    if err != nil { return err }
    err = call(res.Endpoint.Addr) // your RPC/HTTP call
    if res.Done != nil {
        res.Done(loadbalance.DoneInfo{Err: err})
    }
}
```

Route on a hash key or zone by populating `PickInfo`:

```go
res, _ := pool.Pick(loadbalance.PickInfo{Ctx: ctx, HashKey: userID, Zone: "us-east-1a"})
```
