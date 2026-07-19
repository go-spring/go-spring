# starter-lock-k8s Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-k8s` is a Contributor-archetype starter (`starter/DESIGN.md`
§2.3) in the integration layer. It contributes named `lock.Locker` beans
backed by Kubernetes `coordination.k8s.io/Lease` objects.

## 1. Responsibilities & Boundaries

- Binds `spring.lock.<name>` entries to Lease-backed `lock.Locker` beans,
  one per entry, registered under the config name and exported as
  `lock.Locker`.
- Uses the client-go `resourcelock.LeaseLock` primitive under the hood,
  which is the same building block `kube-controller-manager` and
  spring-cloud-kubernetes use for leader election.
- Explicitly does **not** require any external middleware (Redis, etcd,
  Consul); the Lease API is part of every Kubernetes control plane.

## 2. Key Abstractions & Seams

- **Seam is the bean type.** No package-level driver string; switching
  backend is a blank-import change. Existence of a K8s backend proves the
  seam scales beyond storage systems.
- **`buildClient` seam.** Tests inject a client-go fake clientset via
  `newK8sLockerWithClient`, so the RBAC / API-server behaviors can be
  unit-tested without a live cluster (`k8slock_test.go`).
- **Per-hold renewal goroutine.** Each held lock owns its own renewal
  ticker and `Lost()` channel; the shared clientset is reused across
  holds. `renewLoop` mirrors client-go's leaderelection: create the Lease
  when absent, take it when expired/ours, tolerate transient API errors
  until the lease would have expired, then fire `Lost`.
- **Best-effort release on Unlock.** Clears `holderIdentity` so a waiter
  picks it up immediately rather than waiting the full lease duration.
  Failures are swallowed (the lease will expire regardless) to keep
  `Unlock` idempotent.

## 3. Constraints

- **Lease name must be a DNS-1123 subdomain.** The Lease object's name is
  `KeyPrefix + key`, so keys must consist of lowercase alphanumerics, `-`,
  and `.` — pick `KeyPrefix` and lock keys accordingly.
- **RBAC.** The application's ServiceAccount must have `get/create/update`
  on `coordination.k8s.io/leases` in `Config.Namespace`.
- **Kubeconfig vs in-cluster.** With `Kubeconfig` empty the starter uses
  the in-cluster ServiceAccount config (the deployed-in-K8s path); with a
  kubeconfig path it dials via that file (local dev / tests). A misconfigured
  path fails fast at boot.
- **Lease TTLs are integer seconds.** `leaseSeconds` rounds up to at least
  one, preserving the abstraction's TTL contract on the K8s side.
- **Lock timing lives on `lock.Option`, not on config.** TTL, RenewInterval
  and RetryInterval are carried per-acquire so the same knobs work across
  every backend.

## 4. Trade-offs / Alternatives Rejected

- **Custom CRD — rejected.** Lease is a standard, GA API; requiring a
  custom CRD would break every stock cluster and every developer's local
  minikube.
- **Reusing client-go's `leaderelection.LeaderElector` — rejected.** That
  helper owns its own loop with callbacks; it does not model an on-demand
  `Acquire(key)` API, so the lock semantics would leak through the
  abstraction.
