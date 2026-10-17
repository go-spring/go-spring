# guard

`guard` is the server-side counterpart of `chain`: the shape of one inbound
protected request. It is not a server-side executor.

## Why not `chain`

| | client (`chain`) | server (`guard`) |
|---|---|---|
| execution | one around-function, N attempts | one attempt, two hook points |
| retry | first-class | inexpressible by type |
| identity | per outbound service/client | per server instance |
| failure | one `error` | protocol-specific (status, code, result) |
| observability | uniform, carried on `Operation` | optional and protocol-specific |

A server request cannot be expressed as an `Executor`: admission happens before
the handler, and the breaker/bulkhead state can only be completed after it.

## Model

```
one server instance  =  one Guard  =  one state manager
```

- Created at wiring time, bound to an `Identity{System, Service}`, captured by
  the protocol middleware.
- Owns the instance's rate budget, bulkhead count, circuit breaker and the
  config generation it materialized. Two instances never share this state, even
  under the same label.
- The governance authority pushes a new configuration generation; the Guard
  replaces its materialized state. The call path only reads the current
  generation.

## Surface

```go
type Guard interface {
    Admit(ctx context.Context, req Request) (Result, Done)
    Close() error
}

type Done func(Outcome, error)

// Standalone, fixed policy.
func New(id Identity, p Policy, opts ...Option) Guard

// Managed: follows a pushed Settings document.
m := guard.NewManager()
g := m.GuardFor(id, guard.WithFault(f))   // one Guard per server instance
m.Apply(guard.Settings{Default: ..., Rules: ...})
```

`Result` is read-only: `Admitted`, `Verdict`, `Err`, `RetryAfter`, `Latency`,
`Rule`, `Timeout`, `Fault`. `Timeout` is the handling budget the adapter applies
to the handler's context (the Guard cannot cancel a context it does not hand
out).

`WithFault` injects the inbound fault stage; the `Fault` implementation lives in
the `fault` package. Observability attaches its breaker listener through the
optional `BreakerEventListenerSetter` interface.

`WithClusterLimiter` makes the rate-limit stage charge a shared store (Redis,
typically) instead of the instance-local budget, so a limit can span every
replica. The store failure policy is "allow": an unreachable counter backend must
not become an outage. `governance` adapts the process's existing
`resilience.ClusterLimiter` bean, so one limiter covers both directions.

The stages are also exported as standalone objects, for a caller assembling its
own middleware over the same state model:

```go
lim := guard.NewRateLimiter(p, id.Service, nil)   // Allow(ctx) bool
bulk := guard.NewBulkhead(p.MaxConcurrent, p.BulkheadQueueDuration) // Acquire -> release
br := guard.NewBreaker(p, id.Service, listener)   // Allow / Record(Outcome, dur)
```

`Admit` is the composition of exactly these; there is no second implementation.

Degradation is a policy switch: `Policy.DegradeOnOpen` reports an open circuit
as `VerdictDegraded` while keeping `ErrCircuitOpen` as the cause, so the adapter
can serve its own fallback. The Guard never produces a response body.

`Admit` runs the pre-handler stages — rate limit → bulkhead → breaker gate →
fault — and returns this request's judgment. It never runs the handler.

`Done` is the completion half. It is always non-nil (a no-op when neither the
bulkhead nor the breaker is active), idempotent, and normally deferred by the
adapter:

```go
r, done := g.Admit(ctx, req)
defer func() { done(classify(), err) }()
if !r.Admitted() {
    return mapVerdict(r.Verdict()), r.Err()
}
err = handler(ctx)
return err
```

The returned-function convention is deliberate: it is the Go idiom for "this
must be paired", and the closure already holds the request's ticket, so it
cannot settle the wrong request.

## Stage classes

**One-phase** — finished at `Admit`, `Done` is a no-op:

- rate limit
- fault injection (delay / error)
- static degradation

**Two-phase** — acquired at `Admit`, completed by `Done`:

- bulkhead (slot acquisition / release)
- circuit breaker (gate / outcome record, half-open trial)
- dynamic degradation (depends on breaker state)

The reason for the split is outcome dependency, not API convenience: a breaker
cannot know a request failed until the handler has run, and a bulkhead slot
cannot be released until then either.

## Ownership

The package owns:

- the shape (`Identity`, `Request`, `Verdict`, `Result`, `Outcome`, `Done`)
- the policy shapes (`Policy`, `Settings`, `Rule`)
- the per-instance state manager and the default engine (next step)

It does not own:

- handler execution or callbacks
- retry
- observability (span, metric, access log)
- protocol status mapping
- degradation response bodies
- the client `Driver`

Fault is a `Fault` interface implemented by the `fault` package and injected at
wiring time; `guard` does not import `fault`.

## Relationship to `resilience`

The target is: `guard` is the server side, `resilience` is the client side.
The neutral organs are shared: the breaker state machine lives in
`cloud/runtime/dataplane/breaker`, and the rate-limit vocabulary, local budget, `ClusterLimiter`
seam and counting algorithms live in `cloud/runtime/dataplane/ratelimit`. Neither direction owns
the other's implementation; each maps its own policy into the shared specs.

## Migration plan

1. **Shape** (done): public types, policy shapes, package contract.
2. **Engine** (done): per-instance state manager, default stages, tests.
    - `engine.go` — fixed-policy `New`, managed `Manager.GuardFor`, Admit/Done
    - `ratelimit.go` — policy → shared `cloud/runtime/dataplane/ratelimit` budget
    - `breaker.go` — policy → shared `cloud/runtime/dataplane/breaker` state machine
    - `manager.go` — push registry (no shared state)
3. **Wiring** (done): `runtime.Runtime.InboundFor` composes `observability`
   (outer) around `guard` (inner) and injects the inbound fault stage.
    - gin / echo / hertz / http-server / grpc (unary + stream) / trpc / thrift
      drive guard and get fault injection from the same `Inbound`.
    - dubbo is the documented exception: it keeps its own URL-param governance
      model and fires its own inbound fault shell, with no guard and therefore no
      second injection path.
    - Adapters map `runtime.Refusal.Verdict()` instead of
      `resilience.Rejection`.
4. **Deletion** (done): the server half of `resilience` is gone
   (`ServerManager` / `ServerEngine` / `ServerPolicy` / `ServerSettings` /
   `Rejection` / `Driver.NewServerExecutor`). `resilience` is outbound-only.
   sentinel and luohua contribute outbound behavior only; their inbound
   implementations were retired.

## Open items

- Deferred: gRPC streaming and custom server middlewares all use the same
  `Guard`; a future backend seam would be needed only if a provider wants to
  replace guard's bundled inbound engine wholesale (sentinel's system/hotspot
  rules are not expressible through the current `Policy`).
- Degradation: the shape currently only reports `VerdictDegraded`; response
  generation stays in the adapter. No fallback callback is planned.
