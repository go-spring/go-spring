# health Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`health` lives in stdlib (zero-dependency foundation) so any starter can
implement it and any collector can consume it without either importing the
other.

## 1. Responsibilities & Boundaries

- **Does:** define the `Indicator` contract, name the three
  Kubernetes-aligned probe groups, provide `Grouped` as an optional
  refinement, and offer small helpers (`GroupsOf`, `InGroup`) so collectors
  filter per probe.
- **Refuses:**
  - No HTTP / gRPC / RPC surface, no aggregation logic, no scheduling of
    checks. That is the collector's job.
  - No dependency on a container or DI framework. `Indicator` is a plain
    Go interface; wiring is the collector's concern.

## 2. Key Abstractions / Seams

- **`Indicator` is intentionally two methods.** Any narrower (a plain
  `func(context.Context) error`) loses the stable per-component name that
  aggregated output needs. Any wider drags scheduling or reporting into the
  contract.
- **Groups mirror the K8s container lifecycle.** Liveness / readiness /
  startup exist to be mapped directly onto container probes; keeping the
  same three names avoids a translation layer at the collector.
- **`Grouped` is optional with a safe default.** `GroupsOf` returns
  `{readiness, startup}` for indicators that do not implement it — never
  `liveness`. A dependency check that could trip liveness would cause a
  pod restart on a transient downstream outage, which is worse than a
  degraded readiness probe.
- **Collector autowires by `Export`ing `Indicator`.** Go-Spring's container
  matches beans to injection points by their exported interfaces (see
  `spring/gs` docs). Placing `Indicator` in stdlib is what makes each
  contributor (e.g. `starter-go-redis`) and the collector
  (`starter-actuator`) depend only on stdlib — the whole point of the split
  captured in `starter/DESIGN.md` §3.
- **Lazy bean instantiation.** A contributed indicator bean is only built
  when the collector wires it in. If nothing collects, the indicator costs
  nothing.

## 3. Constraints

- `HealthName()` should be short, stable, and unique within an application;
  it is used as a map key in aggregated output.
- `CheckHealth` must honour `ctx` (deadline, cancellation); a slow
  dependency must not stall a probe.
- Indicators must be safe for concurrent use.
- Do not default any indicator into `GroupLiveness`; opt-in only.

## 4. Trade-offs / Alternatives Rejected

- **Interface in stdlib, not in the collector starter.** Keeping the
  interface in `starter-actuator` would force every contributor to import
  actuator — a cross-starter dependency `starter/DESIGN.md` forbids.
  stdlib is the only shared foundation both sides can depend on.
- **Two methods, no status enum on the return.** Returning `error` matches
  Go idioms and lets aggregators build any richer verdict they need;
  `StatusUp` / `StatusDown` are kept as strings for aggregated reports only.
- **No async / cached health.** Real cache logic (rate-limited probes,
  fallback windows) belongs to the collector, which knows how often it
  polls and what to expose. Overloading the contract would freeze policy
  here.
