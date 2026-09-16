# fault

[![Go-Spring](https://img.shields.io/badge/Go--pring-cloud-blue)](https://github.com/go-spring/go-spring)

`fault` is the in-process **fault-injection** companion to
[cloud/governance/resilience](../resilience). It wraps a `resilience.Executor` so a
configurable fraction of operations are made to fail or slow down on demand —
"setting fire" to a running client — to verify that retry, circuit-breaker,
per-attempt timeout and Fallback actually engage, and that the observe kit
records the resulting outcomes.

## Features

- Two seams: `WrapExecutor` (client side — wraps an executor so injected faults
  land *inside* the retry/broker loop) and `Apply` (server side — gates an
  inbound handler call). Both validate resilience, not bypass it.
- Centralized, hot-reloadable config. fault rides the same `governance.Config`
  (and so the same source document) as resilience (see
  [README 设计说明 §8](../README.md)); starter-governance owns the one
  process-wide `*Injector` behind the neutral `InjectorFor()` seam and swaps its
  config in place via `SetConfig` — toggle fires at runtime, no restart.
- Three injection kinds: `generic` (a retryable injected error), `timeout`
  (`context.DeadlineExceeded`), `reset` (`syscall.ECONNRESET`); plus a pure
  latency mode. Per-resource rules via `Config.Rules`.
- Injected errors implement `resilience.Retryable`, so they deterministically
  drive retries regardless of the host's retry predicate.
- stdlib + resilience only — no third-party deps, no gs/spring dependency. The
  gs wiring lives in starter-govern, not here.

## Install

```sh
go get go-spring.org/cloud
```

## Usage

In a starter, resolve the executor and injector through the neutral seams (no
cloud/governance import, no per-starter fault config):

```go
import (
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/cloud/governance/resilience"
)

// client side: ExecutorFor yields the governed executor with the observe layer
// already applied; fault wraps it from the outside. fault.WrapExecutor is
// nil-safe, and with no injector registered behind InjectorFor() it is a
// transparent pass-through resolved lazily on each Execute — so a starter can
// wire this in Init, before starter-govern registers the injector.
exec := fault.WrapExecutor(resilience.ExecutorFor("redis", resource))

// server side: resolve per call (nil injector => transparent pass-through)
err := fault.Apply(ctx, fault.InjectorFor(), "gin", func() error { return next(ctx) })
```

For a self-contained injector (tests, cloud/experimental/loadtest), build one explicitly and
pass it to `WrapExecutorWith(exec, inj)` or `Apply(ctx, inj, ...)`.

Config (centralized in the governance rules document, keys `govern.fault.*` — see
[README 配置指南 §6](../README.md)):

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic        # "" | "generic" | "timeout" | "reset"
govern.fault.latency=50ms         # optional, applied to every call
```

See the [Design](#design) section below for the injection-point rationale and boundaries, and
`starter-redigo/example-load` for a runnable load test that toggles fault.

## Status

`WrapExecutor` (client) + `Apply` (server) seams, per-resource `Rules`, and
centralization under the governance center (the process-wide injector behind
`InjectorFor()`) are all landed.

---

# Design

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
