# dataplane

[English](README.md) | [中文](README_CN.md)

The data plane is split into two independent trees — [outbound](outbound/) and
[inbound](inbound/) — over the plane-neutral [capability](../capability/) tier. This page
records one boundary that cuts across both trees: **the observability split**.

## Two observation planes

Observability is not one layer. It is two planes with different owners,
lifetimes and inputs:

```
                  request plane                     organ state plane
              (per call / per request)           (per organ instance)

  caller ──▶ observe ──▶ governance organ ──▶ transport
                │                 │
                │                 └── publishes its OWN state (pull)
                └── records the outcome of this call
```

| | Request plane | Organ state plane |
| --- | --- | --- |
| Question | What happened to **this call**: status, latency, error type, is it worth recording? | What state does **this organ** hold now? |
| Input | The call's result and `Recorder` | The organ's own fields |
| Lifetime | One call | The organ instance (`Close` unregisters) |
| Publish mode | Per call (span, metric, log) | Per collection, by pull (observable instruments) |
| Packages | `outbound/observe`, `inbound/observe` | `inbound/admission`, `../capability/loadbalance`, `../../discovery` |

## The rule

> **A signal belongs to the plane that owns its lifetime.** If it lives and dies
> with one call, it goes to the request plane. If it describes an organ that
> lives for the process (breaker state, budget usage, slot occupancy, endpoint
> suspension), the organ publishes it from its own package.

Three consequences, all normative:

1. **The request plane never reads governance state.** It records what the call
   returned. A refused call is visible as `resilience.error_type=circuit_open`
   (and its siblings); that is a request-level fact, not a breaker read.
2. **A decision already expressed by the call outcome is not published a second
   time as organ state.** Outbound protection is this case: every effect a
   breaker has on a call (refusal, admission, unavailability) is already on the
   call's own error, so the outbound executor publishes no breaker series. The
   request plane is the source of truth; there is no gauge to keep in step and
   no per-rule key to turn into a label.
3. **The capability packages never import an observability SDK.** They expose read-only
   accessors (`Breaker.Stats`, `Bulkhead.Stats`); an organ that still needs its
   own series — because its state is not derivable from call outcomes — registers
   its own observable callback and unregisters it on `Close`. There is no
   cross-layer state listener, and no state query method on `chain.Executor`.

## Who publishes what

Request plane (existing):

| Metric | Meaning |
| --- | --- |
| `resilience.client.duration`, `.calls` | Outbound latency and outcome, with a total `resilience.error_type`: the gate that refused the call or the timeout budget (`call_timeout`), `connect_failed`, `resource_exhausted`, `context_canceled`, or `unknown` (the residual) |
| `*.operation.duration`, `*.attempt.duration` | Per-operation and per-attempt latency |
| `resilience.server.duration`, `.calls` | Inbound latency and outcome |
| `*.request.duration`, `*.active_requests` | Per-route inbound latency and in-flight requests |

Organ state plane (only where the state is not derivable from call outcomes):

| Metric | Owner | Attributes |
| --- | --- | --- |
| `resilience.server.bulkhead.usage_ratio`, `.queued` | inbound admission | `system`, `service` |
| `loadbalance.endpoint.suspended`, `.suspension.total` | load balancing | — |
| `discovery.cache.age_seconds`, `discovery.sync_total` | discovery | depends on the observer |
| `discovery.instance.registered` | registration | depends on the observer |

Notes:

- The outbound breaker is deliberately absent: "is this dependency cut off"
  is `resilience.error_type=circuit_open` on the call counter, and "how often"
  is its `increase()`. The executor's per-rule state key is an internal
  implementation detail and never becomes a metric label.
- An open-circuit duration is the width of the `circuit_open` error burst in the
  time series; no separate age or transition counter is published.

## Deliberately not covered

- **Injected-fault attribution.** The request plane records an injected fault as
  the real error it mimics, so injected and genuine failures are
  indistinguishable in metrics. This is an existing, known gap.
- **Load-test traffic tagging.** The `traffic` package only propagates the
  marker; whether metrics consume it is an application decision.

## Enforcement

There is no automated test for this boundary. It is a design contract, reviewed
against this page: a new metric must name the plane it belongs to and the
decision or alert it serves.
