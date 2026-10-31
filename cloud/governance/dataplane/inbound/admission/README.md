# admission

`admission` is the server-side counterpart of `chain`: the shape of one inbound
protected request. It is not a server-side executor.

## Why not `chain`

| | client (`chain`) | server (`admission`) |
|---|---|---|
| execution | one around-function, N attempts | one attempt, two hook points |
| retry | first-class | inexpressible by type |
| identity | per outbound service/client | per server instance |
| failure | one `error` | protocol-specific (status, code, result) |
| observability | uniform, carried on `Operation` | optional and protocol-specific |

A server request cannot be expressed as an `Executor`: admission happens before
the handler, and the bulkhead state can only be completed after it.

## Model

```
one server instance  =  one Guard  =  one state manager
```

- Created at wiring time, bound to an `Identity{System, Service}`, captured by
  the protocol middleware.
- Owns the instance's rate budget, bulkhead count and the config generation it
  materialized. Two instances never share this state, even under the same label.
- The governance authority pushes a new configuration generation; the Guard
  replaces its materialized state. The call path only reads the current
  generation.

## Circuit breaking: inbound has none

Circuit breaking is **outbound-only**; its contract is specified in
[`resilience`](../../outbound/resilience/README.md#circuit-breaking-the-rule).
An inbound breaker scoped to one server instance can only reject every request,
which has no safe blast radius (one bad route or shared dependency would take
out every healthy route on the instance). The former per-instance breaker stage
was therefore removed on 2026-10-24, together with its policy fields, verdicts,
`Outcome` classification and metrics.

If inbound self-protection returns, it will be an inbound-side composition of
its own — per resource (action/route), with degradation as the action — not the
client's service-scoped breaker moved across the direction. The capability state
machine may be reused as a building block; the scope, the decision point and
the effect are not.

## Surface

```go
type Guard interface {
    Admit(ctx context.Context, req Request) (Result, Done)
    Close() error
}

type Done func()

// Standalone, fixed policy. The error reports a policy that cannot be built
// (a negative max-concurrent).
func New(id Identity, p Policy, opts ...Option) (Guard, error)

// Managed: follows a pushed Settings document.
m := admission.NewManager()
g, err := m.GuardFor(id, admission.WithFault(f))   // one Guard per server instance
m.Apply(admission.Settings{Default: ..., Rules: ...})
```

`Result` is read-only: `Admitted`, `Verdict`, `Err`, `RetryAfter`, `Latency`,
`Rule`, `Timeout`, `Fault`. `Timeout` is the handling budget the adapter applies
to the handler's context (the Guard cannot cancel a context it does not hand
out).

`WithFault` injects the inbound fault stage; the `Fault` implementation lives in
the `fault` package. The guard publishes its OWN stage state (rate usage,
bulkhead occupancy) through observable instruments registered from `observe.go`;
the request-plane observer never reads this state.

`WithClusterLimiter` makes the rate-limit stage charge a shared store (Redis,
typically) instead of the instance-local budget, so a limit can span every
replica. The store failure policy is "allow": an unreachable counter backend must
not become an outage. `governance` adapts the process's existing
`ratelimit.ClusterLimiter` bean, so one limiter covers both directions.

The stages are also available as standalone objects, for a caller assembling
its own middleware over the same state model. Only the rate limiter is exported
by this package; the bulkhead comes straight from its capability package, with no
re-export here:

```go
lim := admission.NewRateLimiter(p, id.Service, nil)   // Allow(ctx) bool
bulk := bulkhead.New(p.MaxConcurrent, bulkhead.WithQueueTimeout(p.BulkheadQueueDuration)) // Acquire -> release
```

`Admit` is the composition of exactly these; there is no second implementation.

`Admit` runs the pre-handler stages — bulkhead → rate limit → fault — and
returns this request's judgment. It never runs the handler.

`Done` is the completion half. It is always non-nil (a no-op when no bulkhead is
active), idempotent — the completion folds a deadline fallback and an explicit
call into one release, which is what keeps the bulkhead's exactly-once return
exact — and normally deferred by the adapter:

```go
r, done := g.Admit(ctx, req)
defer done()
if !r.Admitted() {
    return mapVerdict(r.Verdict()), r.Err()
}
return handler(ctx)
```

The returned-function convention is deliberate: it is the Go idiom for "this
must be paired", and the closure already holds the request's ticket, so it
cannot settle the wrong request.

## Stage classes

**One-phase** — finished at `Admit`, `Done` is a no-op:

- rate limit
- fault injection (delay / error)

**Two-phase** — acquired at `Admit`, completed by `Done`:

- bulkhead (slot acquisition / release)

The reason for the split is lifetime, not API convenience: a slot cannot be
returned until the handler has finished.

## Ownership

The package owns:

- the shape (`Identity`, `Request`, `Verdict`, `Result`, `Done`)
- the policy shapes (`Policy`, `Settings`, `Rule`)
- the per-instance state manager and the default engine

It does not own:

- handler execution or callbacks
- retry
- observability (span, metric, access log)
- protocol status mapping
- degradation response bodies
- the client `Driver`

Fault is a `Fault` interface implemented by the `fault` package and injected at
wiring time; `admission` does not import `fault`.

## Relationship to `resilience`

`admission` is the server side, `resilience` is the client side. Only the
rate-limit vocabulary is shared today: the `Spec`, local budget,
`ClusterLimiter` seam and counting algorithms live in
`cloud/governance/capability/ratelimit`, and each direction maps its own
policy into them. The circuit breaker is an outbound organ
(`cloud/governance/capability/circuitbreaker`) with no inbound consumer.

## Migration plan

1. **Shape** (done): public types, policy shapes, package contract.
2. **Engine** (done): per-instance state manager, default stages, tests.
    - `engine.go` — fixed-policy `New`, managed `Manager.GuardFor`, Admit/Done
    - `ratelimit.go` — policy → shared `cloud/governance/capability/ratelimit` budget
    - `manager.go` — push registry (no shared state)
3. **Wiring** (done): `governance.Runtime.InboundFor` composes `observability`
   (outer) around `admission` (inner) and injects the inbound fault stage.
    - gin / echo / hertz / http-server / grpc (unary + stream) / trpc / thrift
      drive admission and get fault injection from the same `Inbound`.
    - dubbo is the documented exception: it keeps its own URL-param governance
      model and fires its own inbound fault shell, with no admission and therefore no
      second injection path.
    - Adapters map `runtime.Refusal.Verdict()` instead of
      `resilience.Rejection`.
4. **Deletion** (done): the server half of `resilience` is gone
   (`Manager` / `ServerEngine` / `ServerPolicy` / `ServerSettings` /
   `Rejection` / `Driver.NewServerExecutor`). `resilience` is outbound-only.
   sentinel and luohua contribute outbound behavior only; their inbound
   implementations were retired.
5. **Inbound circuit breaking removed** (2026-10-24): the per-instance breaker
   stage, its policy fields (`ErrorThreshold`, `BreakerStrategy`,
   `ErrorRateThreshold`, `MinRequests`, `BreakerWindow`,
   `SlowCallDurationThreshold`, `SlowCallRateThreshold`, `HalfOpenRequests`,
   `HalfOpenTimeout`, `OpenDuration`, `DegradeOnOpen`), verdicts
   (`VerdictCircuitOpen`, `VerdictDegraded`), `Outcome` classification,
   breaker metrics and `errHTTP5xx`-to-breaker feedback were all deleted.

## Open items

- Deferred: gRPC streaming and custom server middlewares all use the same
  `Guard`; a future backend seam would be needed only if a provider wants to
  replace admission's bundled inbound engine wholesale (sentinel's system/hotspot
  rules are not expressible through the current `Policy`).
- Inbound self-protection, if ever built, is per-resource (action/route) with
  degradation as the action; it is not a revival of the removed instance-level
  breaker.
