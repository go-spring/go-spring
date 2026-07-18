# starter-lock-redis

[English](README.md) | [中文](README_CN.md)

`starter-lock-redis` contributes a Redis-backed
[`lock.Locker`](../../stdlib/lock) bean to a Go-Spring application, giving you
distributed locks and leader election over an existing Redis (single, sentinel,
or cluster) with no extra connection.

It follows the *Contributor* archetype (see
[starter/DESIGN.md](../DESIGN.md)): the starter exports no port and holds no
client of its own. It reuses the `*redis.Client` bean registered by
`starter-go-redis` and contributes a bean behind the framework-neutral
`lock.Locker` seam. Switching the lock backend to etcd or consul is therefore a
blank-import swap — no business code changes.

## Installation

```bash
go get go-spring.org/starter-lock-redis
```

## Quick Start

### 1. Import both starters

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"
)
```

### 2. Configure a Redis client, then a Locker that references it

```properties
# A Redis client managed by starter-go-redis.
spring.go-redis.cache.addr=127.0.0.1:6379

# A Locker bound to that client. `client` is the redis instance name.
spring.lock.jobs.client=cache
spring.lock.jobs.ttl=30s
spring.lock.jobs.key-prefix=myapp:
```

The `client` property is **required**. Booting without it fails fast — the
starter refuses to silently default to some arbitrary Redis instance.

### 3. Inject `lock.Locker`

```go
import "go-spring.org/stdlib/lock"

type Service struct {
    Lock lock.Locker `autowire:"jobs"`
}

func (s *Service) RunOnce(ctx context.Context) error {
    held, ok, err := s.Lock.TryAcquire(ctx, "nightly-report")
    if err != nil || !ok {
        return err
    }
    defer held.Unlock(ctx)
    // ...critical section...
    return nil
}
```

## Configuration

All keys sit under `spring.lock.<name>`:

| Key              | Default | Description                                                                                     |
|------------------|---------|-------------------------------------------------------------------------------------------------|
| `client`         | —       | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.<client>`.                |
| `ttl`            | `30s`   | Default lease TTL. Callers can override per acquisition with `lock.WithTTL`.                    |
| `renew-interval` | `0`     | Lease refresh interval. `0` → `ttl/3`; a negative value disables auto-renew.                    |
| `retry-interval` | `100ms` | Poll interval used by `Acquire` while the lock is contended.                                    |
| `key-prefix`     | *empty* | Prepended to every key so multiple apps can share a Redis instance without colliding.           |

## Leader election

Leader election is available for free on top of any `lock.Locker` via
[`lock.NewElection`](../../stdlib/lock/election.go):

```go
el := lock.NewElection(lock.ElectionConfig{
    Locker: s.Lock,
    Key:    "scheduler-leader",
    OnElected: func(ctx context.Context) {
        // Run leader-only work; return promptly when ctx is cancelled.
    },
})
go el.Run(ctx)
```

Because election is defined against the `Locker` interface, it works identically
over Redis, etcd, consul, or the bundled in-memory locker.

## Guarantees

* **Correctness under contention** — Redis `SET NX PX` for acquisition;
  compare-and-DEL (Lua) for release, so a caller whose lease expired cannot
  accidentally free another owner's lock.
* **Idempotent `Unlock`** — the second (and later) `Unlock` call is a no-op;
  it only returns `lock.ErrNotHeld` when Redis proves another token now owns
  the key.
* **Loss signal** — the handle's `Lost()` channel closes the moment the renew
  loop detects the key is gone or has been taken over, so critical sections
  can bail out promptly.
* **Fail-fast configuration** — missing `client` refuses to boot instead of
  surfacing on the first `Acquire`.

## Single-Redis Redlock

This starter implements the single-node Redlock recipe. Multi-node Redlock is
intentionally not built in: sentinel failover / cluster HA cover the common
case, and stronger consistency needs are better served by the etcd or consul
lock backends (blank-import swap, same code).
