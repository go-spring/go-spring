# cloud

[English](README.md) | [中文](README_CN.md)

`cloud` is go-spring's library of distributed-application building blocks —
service discovery, load balancing, distributed locking, governance, caching,
messaging, scheduling, security, ... Every package is a small, explicit
contract around one question ("which instances exist right now?", "may this
replica run this job?"), usable with nothing but a `context.Context` — no IoC
container required. Container wiring for each family lives in the
`starter-*` modules, not here.

## The packages

| Package | What it answers |
| --- | --- |
| [actuator](actuator/) | Management endpoints: probe paths ([endpoint](actuator/endpoint/)), component health checks ([health](actuator/health/)), Kubernetes Pod metadata ([podinfo](actuator/podinfo/)). |
| [breaker](runtime/dataplane/breaker/) | The direction-neutral circuit-breaker state machine shared by [resilience](runtime/dataplane/resilience/) (outbound) and [guard](runtime/dataplane/guard/) (inbound). |
| [cache](cache/) | A cache API declared once (`spring.cache`) and backed by go-redis / redigo / bigcache / memcached via starters. |
| [chain](runtime/dataplane/chain/) | The governed-call vocabulary every layer shares: `Executor`, `Operation`, `Recorder` and the rejection sentinels. |
| [discovery](runtime/dataplane/discovery/) | "Which live `host:port` addresses serve this logical name right now?" — the **read side** of a naming service (resolver + local endpoint directory, data plane). The write side is [registration](runtime/controlplane/registration/). |
| [registration](runtime/controlplane/registration/) | The **write side** of service discovery (control plane): publishes this process into naming centers and maintains that publication across the `gs.Server` lifecycle. |
| [fault](runtime/dataplane/fault/) | In-process fault injection: a client-side executor wrapper and a server-side stage (`guard.Fault`) that make a call/request fail, slow down, or return an error on demand. |
| [governance](runtime/controlplane/governance/) | Runtime service governance: the control plane — it fetches the hot-reloadable policy document from a remote center and fans it out to [resilience](runtime/dataplane/resilience/) (outbound), [guard](runtime/dataplane/guard/) (inbound), and [fault](runtime/dataplane/fault/) (fault injection). It exposes no data-plane authority; the process's single injection point is `runtime.Runtime`. |
| [guard](runtime/dataplane/guard/) | Server-side inbound governance behind one `Guard` object: per-instance rate limiting, bulkhead, circuit breaking and fault injection, with one request judgment plus one completion. |
| [label](runtime/dataplane/label/) | Service-label composition (`ServiceLabel`) — the key governance rules and managers match on. |
| [loadbalance](runtime/dataplane/loadbalance/) | "Given the live instance set, which one gets this request?" — client-side balancing with failure-based suspension. |
| [lock](lock/) | "May this replica run this exclusive work right now?" — distributed locking and leader election behind one contract. |
| [mesh](mesh/) | "Am I behind a service-mesh sidecar?" — when yes, the app's own discovery/load-balancing steps aside. |
| [messaging](messaging/) | Publish/consume `Message` envelopes through one `Publisher`/`Subscriber` pair; switching brokers is a wiring change. |
| [observability](observability/) | **Plane-neutral telemetry vocabulary**: [attrs](observability/attrs/) (per-request attribute carrier, shared duration buckets) and [conf](observability/conf/) (the property-refresh funnel). Neither imports governance code, so cache/lock/scheduling/config starters use them without pulling the governance stack. |
| [observability layer](runtime/dataplane/observability/) | The **outermost data-plane layer**: it wraps `chain.Executor` outbound and drives the guard's Admit/Done inbound. It may depend on chain/breaker/guard; the neutral telemetry above lives outside the planes. |
| [propagate](propagate/) | Moves a domain marker — the load-test flag, say — across protocol boundaries: one small `Carrier` seam over HTTP headers, gRPC metadata, Kafka record headers, dubbo attachments. |
| [ratelimit](runtime/dataplane/ratelimit/) | The direction-neutral rate-limit stage: counting algorithms, `Spec`, the process-local budget and the `ClusterLimiter` store seam. Shared by both call directions. |
| [resilience](runtime/dataplane/resilience/) | Client-side fault tolerance behind one `chain.Executor` seam: circuit breaking, rate limiting, retry, backoff, with a pluggable `Driver`. Outbound-only; inbound is [guard](runtime/dataplane/guard/). |
| [runtime](runtime/) | The governance runtime: the root package's `Runtime` is the **composition root and single injection point** (belonging to neither plane); [controlplane](runtime/controlplane/) acquires and publishes desired state (`governance` / `registration`), while [dataplane](runtime/dataplane/) executes it on the request path and owns process-local state (`chain`, `traffic`, `discovery` read side, `loadbalance`, `observability`, `resilience`, `guard`, `fault`, `breaker`, `ratelimit`, `label`). |
| [scheduling](scheduling/) | Periodic and cron-scheduled background jobs: triggers (`FixedRate`, `FixedDelay`, `After`, `ParseCron`, `DailyWindow`), concurrency policies, distributed-lock de-duplication. |
| [security](security/) | Framework-agnostic authentication and authorization — identity model plus per-family middleware shells. |
| [traffic](runtime/dataplane/traffic/) | "Is the request in flight load-test traffic?" — one process-wide marker carried across protocol hops via [propagate](propagate/). |
| [experimental](experimental/) | Evolving additions, not yet committed to the stable surface: [batch](experimental/batch/), [contract](experimental/contract/) testing, [loadtest](experimental/loadtest/), [outbox](experimental/outbox/), [session](experimental/session/), [transaction](experimental/transaction/). |

Each package has its own `README.md` (and Chinese `README_CN.md`) covering usage
and design rationale. Families with a larger design surface — `resilience`,
`fault`, `security` — carry a full `## Design` section in their README rather
than a separate file.

## How it fits with the rest

The packages here are plain Go libraries: construct them directly, or let the
matching `starter-*` module register them from configuration. Backends
(Redis, etcd, Nacos, ...) are never imported here — each package defines the
contract, the corresponding starter supplies the SDK-backed implementation.
