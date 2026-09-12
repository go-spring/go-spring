# starter-lock-k8s Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `lock.go`, `observe.go`), the shared
abstraction [cloud/lock](../../../cloud/lock), and the
[example/](example) (including `deploy/` for in-cluster runs). Lease semantics mirror client-go
leader election — see the [Kubernetes Lease API](https://kubernetes.io/docs/concepts/architecture/leases/);
everything below is go-spring's increment.

**Activation**: any `spring.lock.instances.<name>.*` property registers one Lease-backed `lock.Locker`
instance per `<name>`. This is the K8s-native backend: locking/election rides the control plane's
own `coordination.k8s.io/Lease` API (the mechanism behind `--leader-elect`), so an in-cluster app
needs **no extra middleware**. Blank-import one lock backend per binary — the `spring.lock`
prefix is shared by all four backends.

---

## 1. Complete worked project

A leader-elected singleton worker deployed in-cluster. File tree:

```
demo/
├── go.mod
├── main.go
├── leader.go
├── conf/
│   └── app.properties
└── deploy/
    ├── deployment.yaml
    └── rbac.yaml
```

**go.mod** (module deps that matter):

```
require (
    k8s.io/client-go              latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-lock-k8s latest
    go-spring.org/starter-actuator latest   // optional: probes
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-lock-k8s"
)

func main() { gs.Run() }
```

**leader.go** — the application's entire lock surface:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Worker struct {
    // The bean under the instance name is already observe-wrapped by default
    // (trace span + metric + access log); observe.enabled=false opts out (§6).
    Locker lock.Locker `autowire:"default"`
}

func init() {
    gs.Provide(&Worker{}).Export(gs.As[gs.Rooter]())
}

func (w *Worker) Init(ctx context.Context) {
    e := lock.NewElection(lock.ElectionConfig{
        Locker: w.Locker,
        Key:    "demo-leader",
        OnElected: func(context.Context) {
            log.Infof(ctx, log.TagAppDef, "elected leader over Lease demo-leader")
        },
        RetryInterval: 500 * time.Millisecond,
    })
    go func() { _ = e.Run(ctx) }()
}
```

**conf/app.properties** — the complete surface (in-cluster, minimal):

```properties
# Zero-key activation works in-cluster (ServiceAccount + "default" namespace).
# Shown for clarity:
spring.lock.default.namespace=default
# Out-of-cluster only (local dev/tests):
# spring.lock.default.kubeconfig=/home/me/.kube/config
# Lease names are key-prefix + key and must be DNS-1123 subdomains
# (lowercase alphanumerics, '-' and '.'):
# spring.lock.default.key-prefix=demo-

# Access-log verbosity of the observe-lock adapter (defaults shown).
```

**deploy/rbac.yaml** — the ServiceAccount needs get/create/update on
`coordination.k8s.io/leases` in the target namespace (see `example/deploy/rbac.yaml` for a
working Role).

**Verify**:

```bash
kubectl apply -f deploy/
kubectl logs deploy/demo -f          # "elected leader over Lease demo-leader"
kubectl get lease demo-leader -o yaml   # holderIdentity = the fencing token
```

Out of cluster, the example boots in a wiring-only mode (no `spring.lock` entry is declared, so
no Locker bean is built) — see `example/example.go`.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-lock-k8s
  └─ gs.Module(gs.OnProperty("spring.lock"))
        └─ conf.BindEach("${spring.lock}") per entry <name>:
             ├─ Provide newLocker  → bean "<name>"            (Export lock.Locker,
             │                                                          Destroy → Close)
             └─ newLocker wraps it with observe-lock unless observe.enabled=false
  ├─ newK8sLocker: buildClient — in-cluster ServiceAccount config, or kubeconfig
  │  file when set. Eager: a missing ServiceAccount / bad kubeconfig fails boot.
  ├─ bean wiring: consumers' autowire:"<name>" resolved (bean already observe-wrapped)
  └─ on SIGTERM: Destroy → Close is a no-op (the clientset holds no connection
                 that must be torn down). Outstanding holds keep their renewal
                 goroutines; their Leases expire once the process dies.
```

Each acquisition maps to **one Lease object** (name = `keyPrefix + key`) and runs **its own
renewal goroutine** (`lock.go tryOnce`), so holds are independent of each other.

### 2.2 Three-layer timing resolution (all lock backends)

TTL / renew / retry resolve through `lock.Resolve` (cloud/lock/resolve.go), higher
layer wins:

| Layer | Source | This backend |
|-------|--------|--------------|
| 1. per-call option | `lock.WithTTL` / `WithRenewInterval` / `WithRetryInterval` | the **only** layer this starter feeds — there are no timing keys at all |
| 2. starter default | none | `lock.Resolve(lock.DefaultOptions{}, opts...)` — a zero defaults layer |
| 3. package default | TTL `30s`, renew `TTL/3`, retry `100ms` | fill whatever layer 1 leaves unset |

The resolved TTL becomes the Lease's `leaseDurationSeconds` — whole seconds, rounded up with a
1s floor (`lock.go leaseSeconds`).

### 2.3 One lock, layer by layer (acquire → hold → release)

`TryAcquire(ctx, "demo-leader")`:

1. `lock.Resolve(lock.DefaultOptions{}, opts...)` — TTL/renew/retry from per-call options or package defaults; fencing
   token generated unless `WithToken`.
2. A `resourcelock.LeaseLock` is built over the Lease `<namespace>/<keyPrefix+key>` with
   `Identity = token`, then one **acquire-or-renew** round (`tryAcquireOrRenew`, mirroring
   client-go leaderelection):
   - Lease absent → create it (create race lost ⇒ treated as contended);
   - Lease validly held by another identity (`RenewTime + ttl` still ahead) ⇒ `ok=false, err=nil`;
   - expired / unheld / already ours ⇒ take/renew it — `AcquireTime` preserved and
     `LeaderTransitions` bumped only on a real handover (update conflict ⇒ contended).
3. Held: a renewal goroutine refreshes the Lease every `RenewInterval` (default TTL/3).
   - Renewal proves takeover (`ok=false`) ⇒ `Lost()` fires immediately.
   - Transient API errors are tolerated until `time.Since(lastRenew) >= TTL`, then `Lost()` —
     so a blip shorter than the TTL does not lose the lock, and a longer outage does.
4. `Acquire` retries `tryOnce` every `RetryInterval` (default 100ms) until held or ctx ends; a
   transient API error aborts (only ordinary contention is retried silently).
5. `Unlock`: stops renewal, fires `Lost()`, then best-effort releases by **clearing the Lease's
   `holderIdentity`** (and dropping `leaseDurationSeconds` to 1) — a waiter can take over
   immediately instead of waiting out the TTL. If the Lease is already owned by someone else,
   nothing is done. Idempotent.

---

## 3. Per-key behavior reference

All keys live under `spring.lock.instances.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `namespace` | string | `default` | Namespace the Lease objects live in; the ServiceAccount needs lease RBAC there. | Missing RBAC → API errors on first acquire (boot still succeeds — client build does not talk to leases). |
| `kubeconfig` | string | — | Out-of-cluster kubeconfig path; empty uses the in-cluster ServiceAccount config. | Empty outside a cluster → boot fails `in-cluster config (set kubeconfig when running outside a cluster)`. |
| `key-prefix` | string | — | Prepended to each lock key to form the Lease name. Result must be a valid DNS-1123 subdomain. | Invalid name (underscores, uppercase) → Lease create/update rejected at acquire time. |
| `observe.enabled` | bool | `true` | Wrap the primary `<name>` Locker bean with the observe-lock adapter (trace span + metric + access log). `false` = bare locker. | Migration: the `<name>-observed` bean no longer exists — inject `<name>`. |

⚠ There are **no timing keys at all**: TTL/renew/retry ride per-acquisition `lock.Option` values
only (§2.2). Instance weight (`Weight=0` drain) is a registry/loadbalance concept and does not
apply to lock backends.

---

## 4. Verification & fault drills

### 4.1 Contended lock (two replicas)

```bash
kubectl scale deploy/demo --replicas=2
kubectl get lease demo-leader -o jsonpath='{.spec.holderIdentity}'   # one token only
kubectl logs deploy/demo -c demo --prefix | grep -c 'elected leader' # exactly one replica
```

The losing replica's `TryAcquire` returns `ok=false, err=nil`; `Acquire` retries every
`RetryInterval`.

### 4.2 TTL expiry mid-hold (failover drill)

1. Acquire with `WithTTL(10*time.Second)`, then `kill -9` the holder pod.
2. Renewal stops; the Lease's `renewTime` goes stale. After ~TTL the lease reads expired and a
   waiting `Acquire` wins (client-go-style `RenewTime + ttl` check).
3. Faster path: a **graceful** shutdown runs `Unlock`, which clears `holderIdentity` and sets
   `leaseDurationSeconds=1` — takeover in ~1s instead of a full TTL. Verify:

```bash
kubectl delete pod <leader-pod>    # graceful → fast handover
kubectl get lease demo-leader -o jsonpath='{.spec.holderIdentity}'   # new token
```

### 4.3 API-outage tolerance drill

The renewal loop tolerates transient API errors until `lastRenew + TTL`. With `WithTTL(30s)` a
<30s API-server blip does not lose leadership; `Lost()` fires only past the TTL — watch for the
log line / `Lost()` consumer firing after a deliberately long outage.

### 4.4 Observing the locker (on by default)

Inject `autowire:"default"` and generate acquire traffic with starter-otel configured:
spans `acquire`/`try_acquire` with `lock.system="k8s"` and metric `lock.operation.duration`.
Without starter-otel the wrapper is a silent near-no-op.

### 4.5 Out-of-cluster wiring check

`KUBECONFIG=~/... go run ./example` against a dev cluster; with no cluster at all the example
boots in wiring-only mode and exits cleanly (no `spring.lock` entry declared).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `in-cluster config` | running outside a cluster without `kubeconfig` | Set `spring.lock.instances.<n>.kubeconfig`. |
| Boot fails loading kubeconfig | bad path/format | Error names the file; fix path. |
| Acquire errors `forbidden` | ServiceAccount lacks lease get/create/update RBAC | Apply a Role like `example/deploy/rbac.yaml`; note boot itself succeeds. |
| Lease create/update rejected on name | `key-prefix`/key not a DNS-1123 subdomain | Lowercase alphanumerics, `-` and `.` only. |
| Leadership flaps every few seconds | renewal losing to API latency/quotas; TTL too tight | Raise `WithTTL`; renewal tolerance is bounded by TTL. |
| Failover takes the full TTL | holder crashed without Unlock | Expected for crashes; graceful shutdown clears the holder for ~1s handover. |
| No `<name>-observed` bean anymore | removed 2026-08 | Inject `<name>` — it is observed by default; `observe.enabled=false` gives the bare locker. |
| Two replicas both act as leader | election logic ignoring `Lost()`, or manual TryAcquire used as ownership | Select on `Lost()` and abort; the Lease is the single source of truth. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (incl. observer) | 7 |
| Required | 0 |
| Quickstart external deps | 1 (K8s API server) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger):

- RESOLVED (2026-08): the primary `<name>` bean is now observed by default (transparent wrap
  in `newLocker`); the separate `<name>-observed` bean was removed — migration: inject `<name>`.
- RESOLVED: the wrap decision (`wrapIfObserved`, default on + opt-out) is covered by `observe_test.go`.
- K8s is the only lock backend with zero starter-level timing knobs while redis has three — the
  asymmetry is deliberate (control-plane Lease timing is coarse) but forces per-call options for
  any tuning.
- RBAC gaps surface at first acquire, not boot (client build does not touch leases) — a startup
  canary acquire would restore fail-fast.
