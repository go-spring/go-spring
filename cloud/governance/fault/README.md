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
  [../DESIGN_CN.md §8](../DESIGN_CN.md)); starter-governance owns the one
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

For a self-contained injector (tests, cloud/loadtest), build one explicitly and
pass it to `WrapExecutorWith(exec, inj)` or `Apply(ctx, inj, ...)`.

Config (centralized in the governance rules document, keys `govern.fault.*` — see
[../CONFIG_CN.md §6](../CONFIG_CN.md)):

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic        # "" | "generic" | "timeout" | "reset"
govern.fault.latency=50ms         # optional, applied to every call
```

See [DESIGN.md](DESIGN.md) for the injection-point rationale and boundaries, and
`starter-redigo/example-load` for a runnable load test that toggles fault.

## Status

`WrapExecutor` (client) + `Apply` (server) seams, per-resource `Rules`, and
centralization under the governance center (the process-wide injector behind
`InjectorFor()`) are all landed.
