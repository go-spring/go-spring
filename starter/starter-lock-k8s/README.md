# starter-lock-k8s

[English](README.md) | [中文](README_CN.md)

`starter-lock-k8s` provides a **Kubernetes-Lease-backed distributed lock and
leader election** for Go-Spring, with **no external middleware**. It backs
`stdlib/lock` with `coordination.k8s.io/Lease` objects — the same mechanism
`kube-controller-manager --leader-elect` and spring-cloud-kubernetes use — so an
in-cluster application elects a leader or guards an exclusive section using only
the control plane it already runs on.

Blank-importing this starter and declaring a `spring.lock.<name>` entry
registers one `lock.Locker` bean (from `stdlib/lock`) under `<name>`. Business
code injects `lock.Locker` / builds a `lock.Election` and never sees this
package, so switching to the etcd/consul/redis backend is a blank-import swap
under the shared `spring.lock` prefix.

## Installation

```bash
go get go-spring.org/starter-lock-k8s
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-lock-k8s"
```

### 2. Declare a Locker

```properties
spring.lock.default.namespace=default
# kubeconfig is empty for in-cluster auth; set a path to run out-of-cluster.
# spring.lock.default.kubeconfig=/home/me/.kube/config
```

### 3. Elect a leader

```go
type Worker struct {
    Locker lock.Locker `autowire:""`
}

func (w *Worker) Elect(ctx context.Context) {
    e := lock.NewElection(lock.ElectionConfig{
        Locker:    w.Locker,
        Key:       "example-leader",
        OnElected: func(ctx context.Context) { /* leader-only work */ },
    })
    _ = e.Run(ctx) // blocks; typically a background runner
}
```

Or guard a one-off exclusive section directly:

```go
l, err := w.Locker.Acquire(ctx, "nightly-migration")
if err == nil {
    defer l.Unlock(ctx)
    // exactly one replica runs this
}
```

## Configuration

Bound under `spring.lock.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `namespace` | `default` | Namespace the Lease objects live in. |
| `kubeconfig` | (empty) | Path to a kubeconfig; empty uses in-cluster ServiceAccount auth. |
| `key-prefix` | (empty) | Prepended to each lock key to form the Lease name. The result must be a valid DNS-1123 name. |

Lock timing (`TTL`, renew, retry) is not configured here: it is carried by the
per-acquire `lock.Option` values and their defaults, so the same knobs work
identically across every backend.

## How It Works

- Each `spring.lock.<name>` entry builds one Locker owning a shared clientset,
  created eagerly so a missing ServiceAccount or bad kubeconfig fails at boot.
- `Acquire`/`TryAcquire` map the lock key to a single Lease
  (`<key-prefix><key>`). The acquire-or-renew logic mirrors client-go's leader
  election: create the Lease if absent, take it if expired or already ours,
  otherwise report contention. `Acquire` retries contention every
  `RetryInterval`; `TryAcquire` returns `ok=false` at once.
- A held lock renews its lease in the background; if renewal proves the lease
  was taken over, or the API is unreachable past the lease duration, `Lost()`
  closes so a long critical section can abort.
- `Unlock` clears the Lease's `holderIdentity` so a waiter takes over
  immediately instead of waiting out the lease; it is idempotent.

## RBAC

The ServiceAccount needs `get/create/update` on `coordination.k8s.io/leases` in
its namespace (no `delete` — release clears `holderIdentity`). See
[example/deploy/rbac.yaml](example/deploy/rbac.yaml).

## Verifying in a cluster

The unit tests cover the full acquire/contend/renew/release/election logic with
the client-go fake clientset. End-to-end election needs a real cluster:

```bash
kubectl apply -f example/deploy/rbac.yaml
# build/push an image for example/ and apply example/deploy/deployment.yaml (replicas: 3), then:
kubectl logs deploy/lock-k8s-example --all-containers   # exactly one logs "became leader"
kubectl get lease example-leader -o yaml                # holderIdentity is the leader
```
