# lock

[English](README.md) | [中文](README_CN.md)

`lock` answers the multi-replica question *"may this replica run this
exclusive piece of work right now?"* — for scheduled jobs, singleton workers,
one-off migrations, and anything that must run at most once at a time across a
deployment. It provides distributed locking and leader election behind one
`Locker` interface, backed by Redis / etcd / Consul / Kubernetes-Lease starters
or the bundled in-process `MemoryLocker`.

## Installation

```
go get go-spring.org/cloud
```

## Quick start: guard a critical section

```go
import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
)

locker := lock.NewMemoryLocker()          // swap for any starter-provided Locker
defer locker.Close()

l, err := locker.Acquire(context.Background(), "jobs/rollup",
    lock.WithTTL(30*time.Second))
if err != nil {
    return err
}
defer l.Unlock(context.Background())

select {
case <-l.Lost():
    // lease expired or renewal failed — abort the work
default:
    // do exclusive work
}
```

Long-running work should keep selecting on `Lost()` inside the loop, not check
it once: `Lost()` is the mandatory abort signal for every lease-based backend.

## TryAcquire: skip when contended

`Acquire` blocks and retries while the lock is held elsewhere. When the right
behavior is "skip this run", use `TryAcquire`:

```go
l, ok, err := locker.TryAcquire(ctx, "jobs/rollup")
if err != nil {
    return err // backend failure, distinct from contention
}
if !ok {
    return nil // another replica owns it — skip
}
defer l.Unlock(ctx)
```

## The Lock handle

| Member | Semantics |
|---|---|
| `Key()` | the resource key this lock guards |
| `Token()` | unique fencing token for this acquisition; a downstream store can reject writes from a stale holder |
| `Unlock(ctx)` | idempotent — releasing an already-released or expired lock returns nil; `ErrNotHeld` only when the backend can prove takeover by someone else |
| `Lost()` | closed when the lease expires or renewal fails |

Every `Acquire` returns a new fencing token. There are no re-entrant locks —
distributed re-entrance is a footgun; build it on top if you truly need it.

## Options and timing precedence

```go
locker.Acquire(ctx, key,
    lock.WithTTL(10*time.Second),        // lease duration; default 30s
    lock.WithRenewInterval(3*time.Second), // default TTL/3; negative disables renew
    lock.WithRetryInterval(200*time.Millisecond), // Acquire retry pace; default 100ms
    lock.WithToken("worker-42"),         // explicit fencing token; default random 16-byte hex
)
```

Timing comes from three layers, resolved once so every backend composes them
identically:

```
per-acquisition Option  >  starter DefaultOptions  >  package default
```

A starter's `spring.lock.<name>.ttl` is an *overridable default*: it fills in
what the caller left unset, and a per-call `WithTTL` always wins. A negative
`WithRenewInterval` (auto-renew disabled) survives the layering. Backends call
`Resolve` at the top of `Acquire`/`TryAcquire` to get this precedence; a
backend with no starter-level knobs passes a zero `DefaultOptions`.

Pick TTL deliberately: it is the maximum blast radius of a crashed holder.
Auto-renew keeps a live holder's lease alive; when renew fails, `Lost()` fires.

## Leader election

`Election` builds leader election on top of any `Locker` — the same code
elects a leader whether backed by Redis, etcd, Consul or in-memory. It is
deliberately not delegated to etcd/consul-native election, so behavior is
identical across backends.

```go
elect := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "leaders/reporter",
    TTL:    15 * time.Second,
    OnElected: func(ctx context.Context) {
        // leader work; ctx is cancelled on loss — honour it and return
        <-ctx.Done()
    },
    OnResigned: func() { /* cleanup */ },
})

// blocks until ctx is done; run it on a background goroutine / as a Runner
err := elect.Run(ctx)
```

- `IsLeader()` reports current leadership.
- On lease loss the term context is cancelled first, then `Run` waits for
  `OnElected` to return, then unlocks — always in that order — before
  re-campaigning. `RetryInterval` also paces how often a follower
  re-campaigns.
- `NewElection` panics on missing `Locker`/`Key`: a misconfigured election can
  never elect anyone; fail fast.

## Backends

`Locker` is the seam. Unlike `discovery` there is no global string-keyed
driver registry, because a lock backend needs a live client (Redis connection,
etcd client, ...) rather than a declarative policy. Each starter builds its
client and exports one `Locker` bean; business code injects `lock.Locker` and
never changes.

| Backend | Module |
|---|---|
| Redis | `starter-lock-redis` |
| etcd | `starter-lock-etcd` |
| Consul | `starter-lock-consul` |
| Kubernetes Lease (`coordination.k8s.io`) | `starter-lock-k8s` |
| In-process (tests / single node) | `lock.NewMemoryLocker()` |

Switching backend is a blank-import swap. The K8s backend needs no external
middleware beyond the in-cluster control plane and exposes no starter-level
timing config — its knobs are the per-call `Option`s, identical to the other
backends.

## Observability

`WrapLocker` wraps any `Locker` with the observe kit's three signals (trace
span + duration/in-flight metrics + access log, `lock.*` conventions). Starters
install it centrally so backends don't each carry a copy:

```go
locker = lock.WrapLocker("redis", cfg, inner)
```

Without starter-otel the global providers are no-ops, so the wrapper adds
negligible overhead and changes no behavior.

## Writing a backend

```go
type myLocker struct{ /* live client */ }

func (b *myLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
    o := lock.Resolve(b.defaults, opts...) // keeps starter/per-call precedence
    // take the lease, start auto-renew, return a handle whose Lost() closes
    // on renewal failure or expiry
}
```

Contracts to honor:

- `Unlock` is idempotent; `ErrNotHeld` only on provable takeover.
- Fencing token is a required non-empty string.
- `Lost()` must close when the lease is gone — the critical section's abort
  signal.
- Concurrency-safe `Locker` and `Lock`; `Close` releases backend resources,
  not handed-out locks.

## Example

A runnable, self-asserting demo lives in [`example/`](example/):

```
cd cloud/lock/example && go run .
```

It exercises `Acquire`/`TryAcquire` contention, the fencing token, `Lost()` on
lease takeover, and a leader-election handover — all on `MemoryLocker`, no
external services required.
