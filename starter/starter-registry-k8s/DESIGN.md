# starter-registry-k8s Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-registry-k8s` is a Config-provider-archetype starter (`starter/DESIGN.md`
§2.5) for `cloud/discovery`: it contributes named `discovery.Discovery`
backends that resolve a Kubernetes Service name to live Pod endpoints. It is the
read-only member of the `starter-registry-*` family.

## 1. Responsibilities & Boundaries

- Binds one `spring.registry.k8s.<name>` block to one `discovery.Discovery`
  bean, named `k8s.<name>` like every sibling backend (`consul.<name>`,
  `etcd.<name>`, ...). Blocks across backends never collide because the bean
  name carries the backend type.
- Discovery backends are cited **by bean name**. Client starters
  (redis / gorm / grpc) reference one via a `discovery: k8s.<name>` field and
  the container injects that named bean; there is no separate registry to look
  up in.
- **No registrar.** Unlike its siblings this backend never publishes this
  process anywhere, so the family-wide registration keys
  (`${spring.registry.service-name}` / `.addr`) are meaningless here and are not
  read. Configuring them with only k8s blocks present fails startup in the
  registry core ("no registry center is configured"), which is the intended
  loud signal.
- Deliberately client-side only: no controller, no CRD, no push into a
  registry. Kubernetes itself is the source of truth.

## 2. Key Abstractions & Seams

- **Declared before any client bean.** The block-to-bean mapping is set up
  inside a `gs.Module` callback (bean-registration phase), which the framework
  runs before any client bean constructor — so a Redis/GORM client can cite
  `k8s.<name>` in its config without racing the declaration. Construction
  itself is deferred to injection time, so a backend nobody cites is never
  built.
- **Two modes, one seam.** `Mode=dns` uses headless Service DNS
  (SRV/A) — zero dependency, no RBAC — with a periodic re-resolve loop
  because DNS has no push channel. `Mode=endpointslice` runs a client-go
  informer over EndpointSlices for real-time updates and per-endpoint
  metadata (zone, ready), at the cost of get/list/watch RBAC.
- **Duplicate name fails fast.** Two blocks resolving to the same bean name
  are rejected by the container, not silently overwritten.
- **Shutdown is the bean destructor.** The backend is provided with
  `Destroy`; `Close` is called only on backends that implement `io.Closer`.
  DNS mode holds nothing, only the informer-based backend needs shutdown.

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
  the named backend bean; client starters cite it by name, so
  DNS/EndpointSlice/Nacos are interchangeable without touching clients.
- **Camping in a `spring.discovery.*` namespace — rejected.** It would give
  the family two config roots and two bean-naming schemes for the same
  concept; the backend is a discovery-only member of the registry family, not
  a separate family.
