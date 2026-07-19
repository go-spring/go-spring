# starter-discovery-k8s Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-discovery-k8s` is a Config-provider-archetype starter (`starter/DESIGN.md`
§2.5) for `stdlib/discovery`: it contributes named `discovery.Discovery`
backends that resolve a Kubernetes Service name to live Pod endpoints.

## 1. Responsibilities & Boundaries

- Binds `spring.discovery.k8s.<name>` entries to `discovery.Discovery`
  backends, one per entry, registered in the process-global
  `stdlib/discovery` registry under `<name>`.
- Discovery backends are **not injectable beans**. Client starters
  (redis / gorm / grpc) reference them by name via a `discovery: <name>`
  field and look them up through `discovery.MustGet`.
- A single lifecycle bean (`manager`) is registered so background
  informer goroutines are torn down on container shutdown.
- Deliberately client-side only: no controller, no CRD, no push into a
  registry. Kubernetes itself is the source of truth.

## 2. Key Abstractions & Seams

- **Register before any client bean.** The registration runs inside a
  `gs.Module` callback (bean-registration phase), which the framework
  executes before any client bean constructor — so a Redis/GORM client
  calling `discovery.MustGet` never races the registry.
- **Two modes, one seam.** `Mode=dns` uses headless Service DNS
  (SRV/A) — zero dependency, no RBAC — with a periodic re-resolve loop
  because DNS has no push channel. `Mode=endpointslice` runs a client-go
  informer over EndpointSlices for real-time updates and per-endpoint
  metadata (zone, ready), at the cost of get/list/watch RBAC.
- **Duplicate name fails fast.** If a name is already claimed (e.g. by a
  company's own `discovery.Register`), startup is rejected rather than
  silently overwritten.
- **`Close` is optional.** `manager.Stop` calls `Close` only on backends
  that implement `io.Closer`; DNS-mode holds nothing, only the informer-
  based backend needs shutdown.

## 3. Constraints

- **DNS mode requires port information.** SRV mode needs `port-name`;
  A-record mode needs `port` (records carry no port). Missing both is
  rejected in `validate`.
- **Cluster domain is DNS-only.** `cluster-domain` (default
  `cluster.local`) shapes the Service FQDN in DNS mode and is ignored in
  endpointslice mode.
- **In-cluster vs kubeconfig.** With `Kubeconfig` empty the starter uses
  the in-cluster ServiceAccount config (deployed-in-K8s path); with a
  kubeconfig path it dials via that file (local dev / tests).
- **No local unit test against a real cluster.** Endpointslice-mode
  smoke tests use a fake clientset; DNS-mode uses an injected resolver.

## 4. Trade-offs / Alternatives Rejected

- **Server-side registration into K8s — rejected.** Every Pod is already
  in EndpointSlices via the platform; a second registrar would duplicate
  and desynchronise.
- **Baking discovery into every client starter — rejected.** The seam is
  the `stdlib/discovery` registry; client starters look up by name so
  DNS/EndpointSlice/Nacos are interchangeable without touching clients.
