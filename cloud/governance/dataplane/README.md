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
| Packages | `outbound/observe`, `inbound/observe` | `outbound/resilience`, `inbound/admission`, the sentinel starter, `../capability/loadbalance`, `../capability/discovery` |

## The rule

> **A signal belongs to the plane that owns its lifetime.** If it lives and dies
> with one call, it goes to the request plane. If it describes an organ that
> lives for the process (breaker state, budget usage, slot occupancy, endpoint
> suspension), the organ publishes it from its own package.

Two consequences, both normative:

1. **The request plane never reads governance state.** It records what the call
   returned. A refused call is visible as `resilience.error_type=circuit_open`
   (and its siblings); that is a request-level fact, not a breaker read.
2. **The capability packages never import an observability SDK.** They expose read-only
   accessors (`Breaker.Stats`, `Budget.Usage`, `Bulkhead.Stats`); the holding
   organ registers its own observable callback and unregisters it on `Close`.
   There is no cross-layer state listener, and no state query method on
   `chain.Executor`.

## Who publishes what

Request plane (existing):

| Metric | Meaning |
| --- | --- |
| `resilience.client.duration`, `.calls` | Outbound latency and outcome, with `resilience.error_type` when protection refused the call |
| `*.operation.duration`, `*.attempt.duration` | Per-operation and per-attempt latency |
| `resilience.server.duration`, `.calls` | Inbound latency and outcome |
| `*.request.duration`, `*.active_requests` | Per-route inbound latency and in-flight requests |

Organ state plane:

| Metric | Owner | Attributes |
| --- | --- | --- |
| `resilience.client.breaker.state` (1-hot), `.transitions_total{to}` | outbound resilience | `service`, `unit` |
| `resilience.client.ratelimit.usage_ratio` | outbound resilience | `service`, `unit` |
| `resilience.server.ratelimit.usage_ratio` | inbound admission | `system`, `service` |
| `resilience.server.bulkhead.usage_ratio`, `.queued` | inbound admission | `system`, `service` |
| `loadbalance.endpoint.suspended`, `.suspension.total` | load balancing | — |
| `discovery.cache.age_seconds`, `discovery.sync_total` | discovery | depends on the observer |
| `discovery.instance.registered` | registration | depends on the observer |
| sentinel: `resilience.client.breaker.state`, `.transitions_total{to}` | sentinel starter | `service` |

Notes:

- The outbound state plane carries no `system` attribute: one executor is shared
  by every client backend of a label, so it does not know the system. `system`
  stays a request-plane dimension.
- Start and end times of an open circuit are the edges of the `state` gauge in
  the time series; "open for more than N seconds" is an alert `for:` on
  `state{state="open"} == 1`. No separate age metric exists.
- `transitions_total` is a cumulative count per breaker instance and restarts
  when a policy generation rebuilds the breaker; use `increase()`/`rate()`.

## Deliberately not covered

- **Cluster rate-limit usage.** A `ClusterLimiter` store is charged instead of
  the local budget and exposes no usage query, so no local usage series is
  published. Refusals still appear per call as `rate_limited`.
- **Injected-fault attribution.** The request plane records an injected fault as
  the real error it mimics, so injected and genuine failures are
  indistinguishable in metrics. This is an existing, known gap.
- **Load-test traffic tagging.** The `traffic` package only propagates the
  marker; whether metrics consume it is an application decision.

## Enforcement

There is no automated test for this boundary. It is a design contract, reviewed
against this page: a new metric must name the plane it belongs to and the
decision or alert it serves.
