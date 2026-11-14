# Fault injection: purpose, scope and platform reference

## What this package is

`fault` is the in-process, call-level fault injector of go-spring's governance
stack. It sits on the client/server call path the framework already governs, so
a drill targets one service call, one interface or one traffic slice — and the
injected failure flows through the same retry, breaker, timeout and
load-balancing logic a real failure would.

It is complementary to infrastructure chaos platforms (packet-, node-, cloud-
or proxy-level): those act at layers the application call path never sees,
while this package acts exactly where the application and its governance live.

## Purpose

Inject downstream misbehaviour **on demand, under control and attributably**, so
that governance policies and the caller's tolerance paths can be observed under
failure without a real incident.

- **On demand** — the failure appears when the drill asks.
- **Under control** — dose, duration and blast radius have guardrails.
- **Attributable** — `RunID` and rule identity ride spans, metrics and logs, so
  drill traffic is separable from real traffic.

## Scope

### What is injected: downstream behaviour

Fault injection produces what the other side does.

| Layer | Effects |
|---|---|
| transport | `refused`, `reset`, `io-timeout` (a read/write deadline, not a context deadline) |
| protocol | an error answer carrying a status; an inbound drill can also declare a response body |
| latency | fixed / jittered / slow response |
| payload | a declared inbound response body; outbound payloads are not replaced |

### How it is dosed

- `rate` (probability) or `every` (deterministic every-Nth call);
- `start-after` — delay the whole rule from its first matching call;
- `on`/`off` — intermittent windows, for a bad-then-recovery shape;
- `max-affected` / `max-duration` — blast-radius and self-healing guardrails.

Together these express both a steady outage and a bounded one: "bad for 30s,
then recover" is a first-class drill.

All dose knobs count ATTEMPTS: an outbound call that retries is N downstream
requests, each judged independently — the same per-request model Envoy and
Istio use. The call-level failure rate is the composition of those attempts.

### How calls are selected

Direction, service/interface, metadata (including traffic class), endpoint or
instance. Rules are an ordered list; the first match governs the call, and
every rule must name a service — there is no catch-all.

### Both directions

- **Outbound** — the client seam injects inside the resilience executor's retry
  loop, so retry, breaker and load balancing react to the fault.
- **Inbound** — the server seam decides before the handler runs; the adapter
  answers with the declared response.

The two directions own separate configuration and separate guardrail counters.

### Attribution

Every drill carries a `RunID`; injections are tagged in spans, metrics and logs,
with the rule that fired and the declared effect.

## Boundary

The package deliberately stays within the application call path:

- no load or concurrency generation, and no resource stress;
- no infrastructure state chaos (pod/process/instance, DNS, clock, failover);
- no packet-level effects (loss, corruption, partition, bandwidth shaping);
- no governance-verdict manipulation (forcing a breaker open, suspending an
  endpoint);
- no orchestration — scheduling, stop conditions and post-condition assertions
  belong to the calling platform.

## Why it fits inside the governance stack

- **The governance layers react for real.** A fire lands inside the retry loop,
  so retry counts, breaker state and endpoint health move exactly as they would
  for a genuine downstream failure.
- **It reaches every client, not only HTTP.** Cache, SQL, messaging and RPC
  calls share the same seam.
- **It speaks the governance vocabulary.** Transport failures, protocol error
  answers and I/O timeouts are distinguished; an inbound drill declares the
  response status it wants rather than a failure class.
- **Call-level precision.** Target one interface, one traffic class or one
  tenant — finer than a route or proxy rule.
- **Hot configuration.** Rules are pushed and swapped in place, with no restart.
- **Attributable by construction.** `RunID` and rule identity make drill
  signals distinguishable from real incidents in traces, metrics and logs.

## Platform reference

Comparison premise: only effects an in-process injector can implement, and only
platforms whose fault capabilities are expressible from the application call
path (Envoy, Istio and Toxiproxy are the L4/L7 references here). Packet-,
resource- and state-level chaos lives below the call path and is outside this
comparison.

| Capability | Mainstream reference (L4/L7) | This package |
|---|---|---|
| Percentage of calls affected | Envoy/Istio `percentage` | yes (`rate`) |
| Fault persistence / window | faults persist until the config is removed | yes (`start-after`, `every`, `on`/`off`, `max-duration`) |
| Match by call/route/header | Envoy cluster/header; Istio header/path | yes (service/action/metadata) |
| Concurrency cap | Envoy `max_active_faults` | `max-affected` |
| Delay + jitter | Envoy/Istio `delay`; Toxiproxy latency+jitter | yes |
| Error answer with status | Envoy/Istio `abort` | inbound: `status` + `body`; outbound: status on the injected error |
| Connection reset / I/O timeout | Toxiproxy `reset_peer`/`timeout` | yes |

The L4/L7 platforms express packet and status code; this package adds the
governance vocabulary around them and reaches application clients a proxy never
sees.

## References

- Envoy HTTP fault filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/fault_filter
- Istio HTTPFaultInjection: https://istio.io/latest/docs/reference/config/networking/virtual-service/#HTTPFaultInjection
- Toxiproxy: https://github.com/Shopify/toxiproxy
