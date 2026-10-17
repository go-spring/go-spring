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
| [breaker](governance/breaker/) | The direction-neutral circuit-breaker state machine shared by [resilience](governance/resilience/) (outbound) and [guard](governance/guard/) (inbound). |
| [cache](cache/) | A cache API declared once (`spring.cache`) and backed by go-redis / redigo / bigcache / memcached via starters. |
| [chain](chain/) | The governed-call vocabulary every layer shares: `Executor`, `Operation`, `Recorder` and the rejection sentinels. |
| [discovery](discovery/) | "Which live `host:port` addresses serve this logical name right now?" — the read side of a naming service, adapted once per backend. |
| [fault](governance/fault/) | In-process fault injection: a client-side executor wrapper and a server-side stage (`guard.Fault`) that make a call/request fail, slow down, or return an error on demand. |
| [governance](governance/) | Runtime service governance: a centralized, hot-reloadable policy center, with the authorities it coordinates and the neutral organs they share living under this directory. It fans out to [resilience](governance/resilience/) (outbound), [guard](governance/guard/) (inbound), and [fault](governance/fault/) (fault injection). |
| [guard](governance/guard/) | Server-side inbound governance behind one `Guard` object: per-instance rate limiting, bulkhead, circuit breaking and fault injection, with one request judgment plus one completion. |
| [label](governance/label/) | Service-label composition (`ServiceLabel`) — the key governance rules and managers match on. |
| [loadbalance](loadbalance/) | "Given the live instance set, which one gets this request?" — client-side balancing with failure-based suspension. |
| [lock](lock/) | "May this replica run this exclusive work right now?" — distributed locking and leader election behind one contract. |
| [mesh](mesh/) | "Am I behind a service-mesh sidecar?" — when yes, the app's own discovery/load-balancing steps aside. |
| [messaging](messaging/) | Publish/consume `Message` envelopes through one `Publisher`/`Subscriber` pair; switching brokers is a wiring change. |
| [observability](observability/) | Per-request attributes carried on the context, so spans started on your behalf carry them too; the client-side `Operation` / `Recorder` declarations the `resilience` emitter reads; plus `RefreshConf`, the shared funnel for property-refresh triggers. |
| [propagate](propagate/) | Moves a domain marker — the load-test flag, say — across protocol boundaries: one small `Carrier` seam over HTTP headers, gRPC metadata, Kafka record headers, dubbo attachments. |
| [ratelimit](governance/ratelimit/) | The direction-neutral rate-limit stage: counting algorithms, `Spec`, the process-local budget and the `ClusterLimiter` store seam. Shared by both call directions. |
| [resilience](governance/resilience/) | Client-side fault tolerance behind one `chain.Executor` seam: circuit breaking, rate limiting, retry, backoff, with a pluggable `Driver`. Outbound-only; inbound is [guard](governance/guard/). |
| [scheduling](scheduling/) | Periodic and cron-scheduled background jobs: triggers (`FixedRate`, `FixedDelay`, `After`, `ParseCron`, `DailyWindow`), concurrency policies, distributed-lock de-duplication. |
| [security](security/) | Framework-agnostic authentication and authorization — identity model plus per-family middleware shells. |
| [traffic](traffic/) | "Is the request in flight load-test traffic?" — one process-wide marker carried across protocol hops via [propagate](propagate/). |
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
