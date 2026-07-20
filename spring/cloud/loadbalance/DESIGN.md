# loadbalance Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`loadbalance` layers strategy and outlier ejection on top of the discovery
snapshot. It is a stdlib (zero-dependency) package that imports only
`go-spring.org/spring/discovery`; RPC-framework adapters (gRPC balancer,
kitex loadbalancer, ...) translate this core into their picker interface in
their starters.

## 1. Responsibilities & Boundaries

- **Does:** define `Balancer`, ship five in-tree strategies with a shared
  registry, provide an outlier-ejection `Tracker`, and glue everything
  together in a `Pool` that also honours discovery health and mesh mode.
- **Refuses:**
  - No discovery. The candidate set is fed in every `Pick` (through
    `EndpointSource`); this package does not resolve or watch.
  - No RPC-framework adapter code. gRPC / kitex adapters live in their
    starters and reuse the strategies and `Tracker` here.
  - No dependency on `resilience`. The circuit-breaker semantics are
    duplicated in `Tracker` on purpose — see §4.

## 2. Key Abstractions / Seams

- **`Balancer.Pick(eps, info)` takes candidates every call.** The balancer
  only owns selection state (rr cursor / hash ring / SWRR current-weight /
  least-conn in-flight); the caller owns discovery and eviction. This keeps
  strategies composable (`zone_aware` wraps another `Balancer`) and lets
  the same balancer be reused as topology churns.
- **`Factory` (not `Balancer`) is registered.** Balancers hold mutable
  per-target state, so each target must get its own instance; storing a
  factory in the registry enforces that.
- **`Result.Done` closes the request loop.** `least_conn` decrements its
  in-flight count in `Done`; `Pool` wraps every strategy's `Done` so
  `Tracker` sees the outcome even for stateless strategies (rr / hash /
  weighted). Callers must always invoke a non-nil `Done` exactly once.
- **`Tracker` is queryable by design.** It answers `Eligible(eps)` and
  `Ejected(addr)`, so the pool drops bad instances from the candidate set
  *before* a call is routed. A `resilience.Executor` rejects only at
  invocation and is not queryable — a proper source of truth for LB-layer
  eviction, but the wrong shape here.
- **`Pool` merges the two health signals.** `discovery.Endpoint.Healthy` and
  `Tracker.Eligible` are applied in sequence; both fall back to their input
  when a filter would empty the set. The final `Pool.Pick` result carries
  a wrapped `Done` that feeds the tracker.

## 3. Constraints

- Balancers and `Tracker` must be safe for concurrent use.
- Neither `healthy(eps)` nor `Tracker.Eligible` may return an empty slice
  when `eps` is non-empty solely due to filtering — probing a degraded
  instance is strictly better than black-holing all traffic.
- A `Tracker` with `Threshold <= 0` is a transparent pass-through
  (`Eligible` returns the input, `Record` is a no-op). Wiring one in is
  free until eviction is configured.
- Mesh mode is honoured at `Pool.Pick`, not inside individual balancers, so
  every strategy degrades uniformly.
- The strategy names (`RoundRobin`, `LeastConn`, `ConsistentHash`,
  `Weighted`, `ZoneAware`) are the wire-visible names in service configs
  and gRPC LB config, so they are stable.

## 4. Trade-offs / Alternatives Rejected

- **Duplicate breaker semantics rather than reuse `resilience`.** A
  resilience Executor is keyed by resource, only rejects at call time, and
  is not queryable. `Tracker` is keyed by endpoint address and must be
  queryable so `Pool` filters candidates in advance. Reusing the executor
  would force a shape mismatch; sharing the same `DoneInfo.Err` signal
  keeps the two consistent without a code dependency (central concept,
  edge bridging).
- **Push the candidate set every call, do not cache it in the balancer.**
  Discovery already owns the snapshot; caching it in every balancer would
  duplicate state and complicate topology change.
- **Ring rebuild in `consistent_hash` uses an order-independent
  fingerprint** (XOR of per-address hashes + length) so a reordered set
  keeps the same ring — necessary because discovery snapshots do not
  guarantee stable ordering.
- **`weighted` drops stale addresses from its state map on each Pick** so
  churning topologies do not leak weights.
