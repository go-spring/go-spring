# fault Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`fault` is the in-process fault-injection companion to
[cloud/governance/resilience](../resilience). Where resilience *protects* a client against
downstream failures, fault *manufactures* them on demand so the protection
stack can be proven under load. It ships a single seam in this first cut —
[WrapExecutor] — plus the load-test binary `starter-redigo/example-load` that
drives it end to end.

## 1. Responsibilities & Boundaries

- **Does:** wrap a [resilience.Executor] so a configurable fraction of
  operations are made to fail (or slow down) before the real executor sees
  them; hot-swap the live config behind an atomic pointer; expose neutral
  [InjectedError] values that are [resilience.Retryable] and surface as familiar
  Go errors (`context.DeadlineExceeded`, `syscall.ECONNRESET`).
- **Refuses:**
  - No gs / spring dependency. fault is stdlib + resilience only; the hot-reload
    wiring lives in the governance center (it shares the single Config — and so
    the same source — with resilience), not in each client starter.
  - No metrics, tracing or logging of its own. Injected failures flow through
    the host's observe layer (the executor sits inside it), so they are recorded
    exactly like real failures — that is the whole point.
  - No external chaos. Container/network-level faults (kill the Redis container,
    inject packet loss) belong to infra chaos tools on the docker-compose, not
    to this package.

## 2. Key Abstraction / Seam

**One seam: [WrapExecutor].** A [faultExecutor] wraps the operation `fn`
*inside* its `Execute`, then delegates to the inner executor:

```
faultExecutor.Execute(ctx, res, fn) =>
    inner.Execute(ctx, res, func(attempt) {
        sleep? -> cancel? return ctx.Err()
        inject? -> return InjectedError
        return fn(attempt)
    })
```

The injection point is deliberate: because `fn` is faulted **inside** the inner
executor's retry loop, an injected failure is retried, counted by the breaker,
bounded by the per-attempt timeout and surfaced to Fallback — exactly the path a
real downstream failure takes. Short-circuiting at the `Execute` boundary
instead would bypass all of that and defeat the purpose.

**Stack order in a host starter** (e.g. redigo): `fault( observe( rawExec ) )`.
`resilience.ExecutorFor` applies the observe layer itself, on the still-private
governed executor (that is what lets observe attach its breaker listener at
construction); fault then wraps that from the outside. The injected fault still
reaches the real executor's retry loop, so it is both *handled* by resilience
and *recorded* by observe.

**Retryability.** [InjectedError] implements `Retryable() bool` returning true,
which [resilience.Policy.ShouldRetry] consults first — so injected faults
deterministically drive retries regardless of the host's configured predicate.
The typed kinds wrap a real error so `errors.Is(err, context.DeadlineExceeded)`
works and downstream classifiers (observe's outcome map) label the call like a
genuine timeout/reset.

**Hot-reload.** [Injector] holds the [Config] behind `atomic.Pointer`; a
starter's `gs.Dync[fault.Config].OnChanged` calls `SetConfig`, so faults toggle
at runtime without a restart. Caveat: the wrap layer must exist at startup (it
is structural), so fault must be `enabled=true` at boot to be wrappable; once
present, `rate`/`error`/`latency`/`enabled` all hot-toggle freely.

## 3. Constraints

- **stdlib + resilience only.** No third-party deps; the package must stay at
  the same zero-dependency layer as resilience so a starter that imports fault
  pulls in nothing new.
- **nil transparency.** `WrapExecutor(nil)` returns nil, and a nil *injector*
  is transparent: `WrapExecutor(exec)` (lazy `InjectorFor()`) and
  `WrapExecutorWith(exec, nil)` run `fn` untouched while no fault is configured
  — the same zero-config invariant resilience's `NewDialer`/`NewRoundTripper`
  uphold.
- **Forwarded lifecycle.** `faultExecutor.Close` and `Refresh` delegate to the
  inner executor; fault has no resources or policy of its own to manage.

## 4. Trade-offs / Alternatives Rejected

- **Short-circuit at the Execute boundary** (inject, return, never call inner):
  rejected — it bypasses retry/breaker/timeout, so it validates the *caller's*
  reaction, not the resilience stack. Kept as a future opt-in only if a use case
  appears.
- **A `FaultDriver` registry mirroring resilience's `Driver`.** Rejected for
  now: there is exactly one in-process injection strategy, so a registry is
  speculative. Add it when a second backend (e.g. chaos-mesh-driven) lands.
- **Per-resource `[]Rule` config now.** Deferred: the MVP global rule is
  sufficient for single-resource starters (redigo) and avoids unconfirmed gs
  value-binding for lists. The `Injector.maybe(resource)` signature already
  takes a resource so the extension is additive.
- **Dialer / RoundTripper seams in the first cut.** Deferred until an HTTP or
  gRPC starter pilots fault; the package is shaped so `WrapDialer` /
  `WrapRoundTripper` drop in alongside `WrapExecutor`.
