# lock
[English](README.md) | [中文](README_CN.md)

`lock` is a framework-agnostic, zero-dependency abstraction for distributed
locking and leader election. It answers the multi-replica question "may this
replica run this exclusive piece of work right now?" — for scheduled jobs,
singleton workers, one-off migrations, and anything that must run at most
once at a time across a deployment.

## Features

- Zero third-party dependencies in the abstraction.
- `Locker` interface backed by Redis, etcd, Consul (each an independent
  starter) or the bundled in-process `MemoryLocker` for tests / single-node
  deployments — switching backend is a blank-import swap.
- `Lock` handle exposes `Key`, `Token` (fencing token), idempotent `Unlock`,
  and a `Lost()` channel that closes when the lease expires so the critical
  section can abort.
- Configurable `TTL`, `RenewInterval` (negative disables auto-renew),
  `RetryInterval`, and explicit `Token` via functional options.
- `Election` builds leader election on top of any `Locker`, so the same code
  elects a leader whether backed by Redis, etcd, Consul or in-memory.

## Quick Start

Import path: `go-spring.org/spring/lock`.

```go
package main

import (
    "context"
    "log"
    "time"

    "go-spring.org/spring/lock"
)

func main() {
    locker := lock.NewMemoryLocker()
    defer locker.Close()

    l, err := locker.Acquire(context.Background(), "jobs/rollup",
        lock.WithTTL(30*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    defer l.Unlock(context.Background())

    select {
    case <-l.Lost():
        log.Println("lock lost, aborting")
    default:
        // do exclusive work
    }
}
```

Leader election on the same abstraction:

```go
elect := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "leaders/reporter",
    OnElected:  func(ctx context.Context) { /* run leader work until ctx done */ },
    OnResigned: func()                    { /* cleanup */ },
})
_ = elect.Run(context.Background())
```

For a real cluster use a starter that contributes a `Locker` bean over Redis /
etcd / Consul; business code keeps injecting `lock.Locker` and never changes.
