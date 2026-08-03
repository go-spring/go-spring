# health Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`health` is a zero-dependency foundation package: it imports only the standard
library, so any starter can implement it and any collector can consume it
without either importing the other.

## 1. Responsibilities & Boundaries

- **Does:** define the `Indicator` contract (name + check + probe groups +
  criticality), name the three Kubernetes-aligned probe groups, and provide a
  `NewIndicator` factory with `WithGroups` / `NonCritical` options so a starter
  can contribute an indicator without hand-rolling a struct.
- **Refuses:**
  - No HTTP / gRPC / RPC surface, no aggregation, no scheduling, no routing
    policy - not even the default group routing. `GroupsOf` and `InGroup` live
    in the collector (`starter-actuator`); this package only declares what an
    indicator *is*, not how it is *consumed*.
  - No dependency on a container or DI framework. `Indicator` is a plain Go
    interface; wiring is the collector's concern.

## 2. Key Abstractions / Seams

- **`Indicator` is four methods.** `HealthName` (identity) and `CheckHealth`
  (the check) are the core; `HealthGroups` and `IsCritical` carry the
  collector-facing metadata (routing and severity). The two were once optional
  refinement interfaces (`Grouped`, `Critical`); folding them in as required
  methods means a single `NewIndicator`-built type satisfies everything, with
  no type-assertion branching at the collector. A hand-rolled `Indicator`
  supplies trivial defaults (`HealthGroups` returning nil, `IsCritical`
  returning true).
- **Groups mirror the K8s container lifecycle.** Liveness / readiness /
  startup map directly onto container probes; the same three names avoid a
  translation layer at the collector.
- **Empty `HealthGroups` means "no opinion", not "nowhere".** The collector
  applies its default - conventionally readiness + startup, never liveness -
  so a dependency check cannot trigger a pod restart on a transient downstream
  outage. The default is the collector's contract, not this package's; keeping
  it out preserves the "says nothing about how results are consumed" boundary.
- **Collector autowires by `Export`ing `Indicator`.** Go-Spring's container
  matches beans to injection points by their exported interfaces. Placing
  `Indicator` in the foundation package lets each contributor (e.g.
  `starter-go-redis`) and the collector (`starter-actuator`) depend only on
  this package.
- **Lazy bean instantiation.** A contributed indicator bean is only built
  when the collector wires it in. If nothing collects, the indicator costs
  nothing.

## 3. Constraints

- `HealthName()` should be short, stable, and unique within an application;
  it is used as a map key in aggregated output.
- `CheckHealth` must honour `ctx` (deadline, cancellation); a slow dependency
  must not stall a probe.
- Indicators must be safe for concurrent use.
- Do not return `GroupLiveness` from `HealthGroups` for a dependency check;
  opt into liveness only for self-checks, never for downstream resources.

## 4. Trade-offs / Alternatives Rejected

- **Interface in the foundation package, not in the collector starter.**
  Keeping the interface in `starter-actuator` would force every contributor to
  import actuator - a cross-starter dependency. The foundation package is the
  only shared base both sides can depend on.
- **Required methods, not optional refinement interfaces.** `Grouped` and
  `Critical` as optional interfaces let a bare struct satisfy `Indicator` with
  fewer methods, but forced the collector to type-assert and split the
  implementation across two types (`indicator` / `groupedIndicator`). Required
  methods cost a trivial `return nil` / `return true` for hand-rolled
  implementations and collapse the construction path to one type.
- **No status enum on the return.** Returning `error` matches Go idioms and
  lets aggregators build any richer verdict they need; `StatusUp` /
  `StatusDown` are kept as strings for aggregated reports only.
- **No async / cached health.** Real cache logic (rate-limited probes,
  fallback windows) belongs to the collector, which knows how often it polls
  and what to expose. Overloading the contract would freeze policy here.
