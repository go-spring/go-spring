# runtime

`cloud/runtime` is go-spring's governance runtime. It consists of **two planes** plus a **composition root**:

| Role | Package | Responsibility |
| --- | --- | --- |
| Composition root | [root package](runtime.go) (`runtime.Runtime`) | Owns the control plane and every data-plane authority; it is the process's single injection point. It belongs to neither plane. |
| Control plane | [controlplane/governance](controlplane/governance/), [controlplane/registration](controlplane/registration/) | Both ends of desired state: **acquiring** the governance document (`Source` + `Center`) and **publishing** this process's endpoints (`Registry` + `Server`). It implements no request-path policy and exposes no data-plane authority. |
| Data plane | [dataplane/](dataplane/) | Executes policy on the request path and owns process-local state: `chain`, `traffic`, `discovery` (read side), `loadbalance`, `observability`, `resilience`, `guard`, `fault`, `breaker`, `ratelimit`, `label`. |

## How the planes are decided

- **Control plane = acquiring and publishing desired state.** Rule-file watching, HTTP polling, config-center streams, document parsing and distribution live here. Service registration (`registration`) lives here too: writing this process's address/weight/version into a naming center produces desired endpoint state and is driven by the `gs.Server` lifecycle. The transport adapters (etcd, nacos, a governance console) live in the starter modules.
- **Data plane = process-local state + request-path actions.** The **read side** of service discovery (`Discovery` / `Resolver` / the local endpoint directory) belongs here: `loadbalance.Pick` reads it on every request. This is the SDK client-side model — the SDK's resolver and cache are the data plane's local view, while the authoritative registry and the publication action are control plane.
- **Membership follows the execution system a package serves, not the role of the objects it defines.** The data plane is the dependency closure of request execution, so the direction-neutral leaves it runs (`breaker`, `ratelimit`) and the vocabulary its adapters share (`label`, `traffic`) belong to it — "neutral" does not mean "outside the plane". A dependency moves out only when a consumer outside the request path also needs it (a cache, a lock, a scheduler, a config store), which is why `observability/attrs` and `observability/conf` live outside the planes.
- **The observability layer is data plane, and the outermost layer of it.** It implements the governed call: it wraps `chain.Executor` outbound and drives the guard's Admit/Done inbound. It may therefore depend on `guard` / `breaker`, and nothing on the request path depends on it. The neutral telemetry helpers it used to export live outside the planes (see below), so the layer itself carries no plane-neutral vocabulary.
- **The composition root is neither plane.** It is `runtime.Runtime`: it assembles both planes' objects into one graph and hands that graph to starters as the single injection point. `Center` no longer plays that role.

## Dependency direction

One-way: the control plane may import the data plane (it aggregates their policy types in `Config`), and the composition root may import both, but a data-plane package must never import the control-plane root. The control plane delivers one whole document; the data plane reads its current generation per call and rebuilds its per-service state when the generation moves.

Two couplings are deliberate, not defects: the control-plane `Config` embeds data-plane policy types (`resilience.Policy`, `guard.Policy`, `loadbalance.Selection`, `fault.Config`) because an in-process control plane has no independent deployment or schema, so type/config unity is worth more than schema independence; and the write side (`registration.Instance`) and the read side (`discovery.Endpoint`) keep separate vocabularies, synced at the backend starters. Unifying either is not planned.

Plane-neutral vocabulary stays at the `cloud/` top level: [propagate](../propagate/) (cross-protocol carriers), [mesh](../mesh/) (sidecar switch), and [observability/attrs](../observability/attrs/) / [observability/conf](../observability/conf/) (telemetry vocabulary and config-refresh instrumentation). Both planes and every capability may depend on them, but none of them owns them. The capability SDKs — [cache](../cache/), [lock](../lock/), [messaging](../messaging/), [scheduling](../scheduling/), [security](../security/) — are top-level siblings; they may depend on those neutral packages, but the runtime never depends on the SDKs.

## Reference models

The split follows the mainstream two-plane model rather than a project-specific one:

- [Istio Architecture](https://istio.io/latest/docs/ops/deployment/architecture/) — the data plane (Envoy) performs dynamic discovery, load balancing, circuit breaking, health checks, fault injection and telemetry; the control plane (istiod) translates policy and distributes configuration.
- [Envoy xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol) — the control plane pushes versioned resources (including the EDS endpoint list); a data plane keeps serving from its last-known local configuration, which is why the request path reads a local generation instead of querying the center.
- [Dubbo Proxyless Service Mesh](https://apache.googlesource.com/dubbo-website/+/f8a3fb2078c04b501167ee7eb462cc816c73fe39/content/en/docs3-v2/golang-sdk/tutorial/governance/service-mesh/proxyless_service_mesh.md) — the in-process SDK is the data plane and the control plane distributes configuration without a sidecar, which is this runtime's deployment model.
