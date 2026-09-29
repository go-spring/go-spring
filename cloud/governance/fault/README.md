# fault

[![Go-Spring](https://img.shields.io/badge/Go--pring-cloud-blue)](https://github.com/go-spring/go-spring)

`fault` is the in-process **fault-injection** companion to
[cloud/governance/resilience](../resilience). It wraps a `resilience.ClientExecutor` so a
configurable fraction of operations are made to fail or slow down on demand —
"setting fire" to a running client — to verify that retry, circuit-breaker,
per-attempt timeout and Fallback actually engage, and that the observe kit
records the resulting outcomes.

## Features

- Two seams: `WrapClientExecutor` (client side — wraps an executor so injected faults
  land *inside* the retry/broker loop) and `ApplyServer` (server side — gates an
  inbound handler call). Both validate resilience, not bypass it.
- Centralized, hot-reloadable config. fault rides the same `governance.Config`
  (and so the same source document) as resilience (see
  [README 设计说明 §8](../README.md)); starter-governance owns the one
  `*Injector` — exported as a bean — and swaps its config in place via
  `SetConfig`, so toggling fires at runtime with no restart.
- One config per DIRECTION, so a fire on one side cannot touch the other:
  govern.client.fault.* drives outbound calls, govern.server.fault.* drives
  inbound requests, and each side counts its own MaxDuration/MaxAffected
  guardrails. An outbound fire that self-heals leaves the inbound one armed.
- Three injection kinds: `generic` (a retryable injected error), `timeout`
  (`context.DeadlineExceeded`), `reset` (`syscall.ECONNRESET`); plus a pure
  latency mode. Per-service rules via `Config.Rules`.
- Injected errors implement `resilience.Retryable`, so they deterministically
  drive retries regardless of the host's retry predicate.
- stdlib + resilience only — no third-party deps, no gs/spring dependency. The
  gs wiring lives in starter-govern, not here.

## Install

```sh
go get go-spring.org/cloud
```

## Usage

In a starter, the executor and injector arrive as injected beans (a
*resilience.Manager and a *fault.Injector, both nullable):

```go
import (
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/cloud/governance/resilience"
)

// The injector is a bean: starter-governance registers one *fault.Injector, and
// a starter takes it as a nullable constructor parameter
// (gs.IndexArg(n, gs.TagArg("?"))). nil means no governance is in the container,
// and both entry points below are nil-safe, so the wrap is a transparent
// pass-through — a client is never forced to depend on the starter.
//
// client side: mgr.ClientExecutorFor yields the governed executor with the observe
// layer already applied; fault wraps it from the outside. Holding the injector
// pointer is enough to observe config changes: the center swaps its config in
// place with Injector.SetConfig rather than replacing it.
exec := fault.WrapClientExecutor(mgr.ClientExecutorFor("redis", service), service, inj)

// server side: the injector is captured ONCE, when the middleware is built —
// not re-resolved per request.
err := fault.ApplyServer(ctx, inj, "gin", func() error { return next(ctx) })
```

For a self-contained injector (tests, cloud/experimental/loadtest), build one
with `fault.NewInjector(fault.Configs{Client: ..., Server: ...})` and pass it the
same way. One [Config] type serves both directions — the model is symmetric, so
only the values differ — and the direction is fixed by which entry point reads
it, never by a parameter a caller could aim wrong.

Config (centralized in the governance rules document, one block per direction —
see [README 配置指南 §6](../README.md)):

```properties
# outbound: WrapExecutor's per-attempt gate (mgr.ClientExecutorFor → fault.WrapClientExecutor)
govern.client.fault.enabled=true
govern.client.fault.rate=0.5
govern.client.fault.error=generic        # "" | "generic" | "timeout" | "reset" | "refused"
govern.client.fault.latency=50ms         # optional, applied to every call
govern.client.fault.latency-jitter=20ms   # optional, sleep latency ± U[0,20ms]

# inbound: Apply's inbound-handler gate (gin / echo / grpc / hertz / trpc / dubbo)
govern.server.fault.enabled=true
govern.server.fault.rate=0.2
govern.server.fault.error=timeout
```

Read the live values with `Injector.ClientConfig()` / `Injector.ServerConfig()`.

See the [Design](#design) section below for the injection-point rationale and boundaries, and
`starter-redigo/example-load` for a runnable load test that toggles fault.

## Status

`WrapClientExecutor` (client) + `ApplyServer` (server) seams, per-service `Rules`, and
centralization under the governance center (whose one process-wide
`*Injector` is a bean) are all landed.

---

# Design

`fault` is the in-process fault-injection companion to
[cloud/governance/resilience](../resilience). Where resilience *protects* a client against
downstream failures, fault *manufactures* them on demand so the protection
stack can be proven under load. It ships two seams — [WrapClientExecutor] for outbound
calls, [ApplyServer] for inbound requests — plus the load-test binary
`starter-redigo/example-load` that drives the outbound one end to end.

## 1. Responsibilities & Boundaries

- **Does:** wrap a [resilience.ClientExecutor] so a configurable fraction of
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

**One seam: [WrapClientExecutor].** A [faultExecutor] wraps the operation `fn`
*inside* its `Execute`, then delegates to the inner executor:

```
faultExecutor.Execute(ctx, fn) =>
    inner.Execute(ctx, func(attempt) {
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
`Manager.ClientExecutorFor` applies the observe layer itself, on the still-private
governed executor (that is what lets observe attach its breaker listener at
construction); fault then wraps that from the outside. The injected fault still
reaches the real executor's retry loop, so it is both *handled* by resilience
and *recorded* by observe.

**Retryability.** [InjectedError] implements `Retryable() bool` returning true,
which [resilience.ClientPolicy.ShouldRetry] consults first — so injected faults
deterministically drive retries regardless of the host's configured predicate.
The typed kinds wrap a real error so `errors.Is(err, context.DeadlineExceeded)`
works and downstream classifiers (observe's outcome map) label the call like a
genuine timeout/reset.

**Two directions, one Injector.** [Injector] holds one [side] (config +
guardrail counters) per direction, so an outbound fire and an inbound fire never
see or trip each other: [WrapClientExecutor]'s per-attempt gate reads the client side,
[ApplyServer] reads the server side. That is why the two configs must be separate keys
(`govern.client.fault.*` / `govern.server.fault.*`) rather than one block with a
per-rule direction flag — the knobs that need the direction most
(`rate`/`latency`/`error`/`scope`/guardrails) are the GLOBAL ones, which carry no
service label for a flag to hang on.

**Hot-reload.** Each side holds its [Config] behind `atomic.Pointer`; the
governance center calls `SetConfig` on every source push, so faults toggle at
runtime without a restart. There is no "must be enabled at boot" caveat: the
wrap and the middleware are structural and always installed (`nil` injector ⇒
transparent pass-through), so a `enabled=false → true` push takes effect on the
next call.

## 3. Constraints

- **stdlib + resilience only.** No third-party deps; the package must stay at
  the same zero-dependency layer as resilience so a starter that imports fault
  pulls in nothing new.
- **nil transparency.** `WrapClientExecutor(nil, service, nil)` returns nil, and a nil *injector*
  is transparent: `WrapClientExecutor(exec, service, nil)` runs `fn` untouched while no fault is configured
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
- **Dialer / RoundTripper seams in the first cut.** Deferred until an HTTP or
  gRPC starter pilots fault; the package is shaped so `WrapDialer` /
  `WrapRoundTripper` drop in alongside `WrapClientExecutor`.
