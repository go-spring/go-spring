# loadbalance

[English](README.md) | [中文](README_CN.md)

`loadbalance` is the client-side load-balancing layer on top of
`go-spring.org/cloud/governance/capability/discovery`. Discovery answers "which instances exist
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

    "go-spring.org/cloud/governance/capability/discovery"
    "go-spring.org/cloud/governance/capability/loadbalance"
)

backend := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "10.0.0.1:8080", Scheme: "tcp"},
    discovery.Endpoint{Addr: "10.0.0.2:8080", Scheme: "tcp"},
)
resolver, err := discovery.NewResolver(ctx, backend, "orders", "")
if err != nil { return err }

bal, err := loadbalance.New(loadbalance.Selection{Balancer: loadbalance.RoundRobin})
if err != nil { return err }
pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold:  3,              // consecutive failures before suspension
    SuspendFor: 5 * time.Second, // half-open trial after a 5s cool-down
}), loadbalance.Selection{})

for {
    ep, err := pool.Pick(ctx, loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(ep.Addr)     // your RPC/HTTP call
    pool.Complete(ctx, ep, err)  // must be paired: settles accounting + feeds the tracker
}
```

## Pool: assembly and filtering

`Pool` glues three things into a runtime: an endpoint source, a strategy, and a
`Tracker` — all three built by the caller, so the pool creates nothing. A
tracker from a zero `TrackerConfig` (the stated "no suspension") is fully
transparent; an explicit threshold makes it governable without any extra
wiring.

```go
pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{}), loadbalance.Selection{})
```

- **The endpoint source** is a `discovery.Resolver` —
  `func(ctx context.Context) ([]discovery.Endpoint, error)`. Freshness lives entirely inside the
  discovery backend, so each `Pick` re-reads the latest snapshot.
- Each `Pick` filters in order: **discovery eligibility** (disabled/unhealthy
  instances) → **suspension** (instances cooling down in the `Tracker`) →
  **zero-weight drain** (instances whose weight was set to 0). The survivors
  go to the strategy. No filter may empty a non-empty set — traffic is never
  black-holed.
- **Mesh mode**: no pool is built at all — the client's entry asks
  `Runtime.UsesDiscovery`, gets "no", and dials its configured stable address
  directly, letting the sidecar balance. `loadbalance` itself holds zero mesh
  references.

## Balancer: strategies

The strategy decides "which survivor wins". Seven are built in; a governance
rule names one by name, and any strategy a linked package registered can be
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
`Complete` receives the context of the request it settles) and provides a
`Factory` — the registered name is the strategy name a rule cites, and the
strategy's own parameters arrive in the loose `Selection.Params` map:

```go
func myFactory(sel loadbalance.Selection) (loadbalance.Balancer, error) {
    window := time.Second // the strategy's own parameter, defaulted here
    if v, ok := sel.Params["window"]; ok {
        w, err := time.ParseDuration(v)
        if err != nil {
            return nil, err
        }
        window = w
    }
    for k := range sel.Params { // reject keys this strategy does not own
        if k != "window" {
            return nil, fmt.Errorf("my_strategy: unknown parameter %q", k)
        }
    }
    return &myBalancer{window: window}, nil
}

// in the contributing package's init:
func init() { loadbalance.Register("my_strategy", myFactory) }
```

A strategy registers from the init of the package that defines it, into one
process-wide registry, so linking that package is what puts the strategy in
place.

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
ep, _ := pool.Pick(ctx, loadbalance.PickInfo{HashKey: userID})

// Zone locality (consumed by zone_aware); accepts an ordered fallback
// list, tried level by level, spilling over only when every level is empty.
ep, _ := pool.Pick(ctx, loadbalance.PickInfo{Zone: "us-east-1a,us-east-1"})
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
the full set. `Threshold <= 0` (a tracker built from a zero `TrackerConfig`) is
fully transparent.

## Managed selection (governance)

A chain's strategy and suspension thresholds can be driven from outside the
process, by service label rather than by construction-time argument. The caller
injects the `*loadbalance.Manager` bean (see [Manager](manager.go)) and builds its
chain under it:

```go
// mgr is the injected *loadbalance.Manager; nil means no governance is present.
var ep discovery.Endpoint
if mgr != nil {
	chain, err := mgr.Chain(ctx, resolver, "http:user-svc") // follows the label
	if err != nil {
		return err
	}
	ep, err = chain.Pick(ctx, loadbalance.PickInfo{})
} else {
	bal, err := loadbalance.New(loadbalance.Selection{Balancer: loadbalance.RoundRobin})
	if err != nil {
		return err
	}
	pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{}), loadbalance.Selection{}) // standalone
	ep, err = pool.Pick(ctx, loadbalance.PickInfo{})
}
```

- `mgr.Chain(resolver, label)` builds the chain under the label, so the
  label's current `Selection` applies to the generation it builds **immediately**,
  and a later change **rebuilds the chain from scratch** — a new balancer, a new
  tracker, a new pool, sharing nothing with the generation it replaces. The
  rebuild is PULLED, not pushed: the chain holds a `SelectionSource` (the
  manager) and reads its label's generation on every request, rebuilding when it
  moves. The `Chain` is the ONLY outward surface: a request path calls
  `chain.Pick` and `chain.Complete` directly; the inner `Pool` is not exposed. The
  registration is **one-way, and permanent**:
  there is no detach, so a chain must outlive the manager that governs it (which
  is what a bean-managed chain is). Everything is released by `Manager.Close`, at
  the end of the manager's life.
- Building under an **unarmed** manager is safe and is the normal case for a
  chain built during container wiring: the base generation is built, and the chain
  moves to the document's generation on its first request after it arrives. With
  no manager at all (a container with no policy-root bean, or a standalone
  caller) the chain is built once and never rebuilt — a transparent pass-through.
- A resolved `Selection` is complete, so it must name a strategy: an empty name
  is a construction error, not an implied base. Round-robin is the base, but it
  is stated by whoever produces the settings (the manager's base document, the
  governance translation). An **unknown** name is ignored and the last good
  generation stays in force — there is no "current strategy" a rebuild could
  leave alone. The suspension thresholds always apply.
- The suspension half only has an effect on a pool whose `Pick` is paired with
  `Complete` — the tracker is always there, but unpaired calls leave it nothing
  to count, so the thresholds are set on nothing.

`Manager.Apply(Settings{...})` is the single entry point the governance center
calls — once with the source's snapshot, then on every push. The center PUSHES the
rules and this package does the matching — a `Settings.Default` plus a `Rules` list,
the first Rule whose `Service` equals the label wins — the same shape `fault` already
had. Apply then STOPS: it publishes the document as a new generation and does not
touch a single chain. Each chain reads its label's generation on the request path
and rebuilds it right there on the push when its label's resolved selection
moved, so the request path is one atomic load of the generation in force. The
change test compares the resolved selection BY VALUE: a push that changes
nothing for a label costs its chains nothing, and re-applying an identical
document is not a change.

A rule names the strategy; the built-ins carry their parameters in their own
grouped sub-maps, and the flat `balancer-params` map is the extension seam
reserved for contributed strategies:

```yaml
balancer: consistent_hash
consistent-hash:
  replicas: 200
outlier-threshold: 5
selector:                 # endpoint narrowing, applied before the strategy picks
  - key: version
    kind: prefix
    value: v2.
```

The selector is the dynamic half of endpoint narrowing: `Pool.Pick` applies it
with `discovery.FilterByMetadata` as the first filter stage, after reading the
source and before eligibility — it does not wrap the source, which stays the
chain's stable target across generations. It composes with the static narrowing
of the resolver's construction-time scheme (both must hold), and a pushed change
moves the label's generation like any other selection change.
There is no implicit fallback — a selector that matches no endpoint surfaces as
`ErrNoEndpoints`, because falling back silently would widen a canary rule to
every instance.

`buildGeneration` is where a strategy name becomes a strategy: it resolves the name
against the package registry — the built-in strategies plus anything a linked
package registered — and builds the pool with it, so a pool never holds the
factory table. `Chain.Selection()` reads back the selection the chain's
generation in force was built from — an inspection helper, not a request-path
concern.

## Coverage: what is governed, and the re-pick test

Selection is driven **only** through the governance channel — the `balancer`
name plus its grouped parameters (`consistent-hash.replicas`,
`zone-aware.zone-key` / `zone-aware.delegate`) or the contributed seam's
`balancer-params`, via `Manager.Apply` → a rebuilt `Chain` generation (hot, but
from scratch). `zone_aware` has no strategy parameters of its own: the grouped
parameters beside it describe the DELEGATE, so
`zone-aware.delegate=consistent_hash` is combined with `consistent-hash.replicas`
on the same rule. A starter invents no local static `balancer` key of its own; each
chain's base generation is round-robin, stated by the manager's base document
and by the governance translation rather than implied by an empty name. The
`Factory` parameterization is kept, to carry parameters on the governance
channel.

Covered clients: every discovery-mode `loadbalance.Pool` consumer —
`httpx`/`gateway`, all four gorm dialects, `redigo`, `go-redis`, `mongodb`,
`grpc`.

> Self-check: **does this client pick an endpoint again on every request /
> every connect?** Yes → its label can carry a `balancer`. No → don't expect
> `spring.governance.client.loadbalance.services[N].balancer` to affect it.

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
`RegisterBalancer` (a custom name keeps its own strategy, exempt from the
process-level override by design).

**Building a pool** takes three parts: a `Tracker` + a paired
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
