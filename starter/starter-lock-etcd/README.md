# starter-lock-etcd

[English](README.md) | [中文](README_CN.md)

`starter-lock-etcd` is the etcd-backed implementation of the
[`go-spring.org/stdlib/lock`](../../stdlib/lock) distributed-lock abstraction.
Blank-importing this starter registers one `lock.Locker` bean per configured
instance; switching the backend to Redis or Consul is a blank-import swap and
no business code changes.

## Installation

```bash
go get go-spring.org/starter-lock-etcd
```

## Quick Start

### 1. Import the `starter-lock-etcd` package

```go
import _ "go-spring.org/starter-lock-etcd"
```

### 2. Configure a lock instance

Add an entry under `spring.lock.<name>` in your
[configuration file](example/conf/app.properties), for example:

```properties
spring.lock.main.endpoints=127.0.0.1:2379
spring.lock.main.ttl=30s
spring.lock.main.key-prefix=/lock/
```

Only `endpoints` is required; every other field has a sensible default and an
empty `endpoints` fails fast at startup.

### 3. Inject the `lock.Locker`

```go
import "go-spring.org/stdlib/lock"

type Service struct {
    Locker lock.Locker `autowire:"main"`
}
```

### 4. Acquire and release

```go
l, ok, err := s.Locker.TryAcquire(ctx, "invoice/42")
if err != nil {
    return err
}
if !ok {
    return nil // held elsewhere
}
defer l.Unlock(ctx)

select {
case <-l.Lost():
    // lease expired, abort the critical section
case <-workDone:
}
```

## Configuration Keys

All keys live under `spring.lock.<name>`:

| Key             | Default   | Description                                      |
|-----------------|-----------|--------------------------------------------------|
| `endpoints`     | (required)| etcd cluster addresses                           |
| `username`      | `""`      | etcd auth username                               |
| `password`      | `""`      | etcd auth password                               |
| `dial-timeout`  | `5s`      | initial connect timeout / startup probe budget   |
| `ttl`           | `30s`     | lease TTL per acquired lock (min 1s, seconds)    |
| `key-prefix`    | `/lock/`  | prefix prepended to every lock key               |
| `tls.enabled`   | `false`   | enable TLS                                       |
| `tls.cert-file` | `""`      | client certificate (mutual TLS)                  |
| `tls.key-file`  | `""`      | client private key (mutual TLS)                  |
| `tls.ca-cert-file` | `""`   | PEM bundle of trusted CAs                        |

## Core Behavior

* **Independent leases.** Each `Acquire`/`TryAcquire` opens its own
  `concurrency.Session`, so one hold's `Lost()` channel and lease renewals are
  isolated from every other outstanding hold.
* **Automatic keepalive.** The etcd concurrency package keeps the session's
  lease alive; no application renew goroutine is needed.
* **Idempotent `Unlock`.** A second `Unlock` returns `nil`. Locks handed out
  before shutdown remain valid until they are released or their leases expire.
* **Fail-fast boot.** An unreachable cluster, bad credentials, or empty
  `endpoints` cause the application to fail at startup rather than at first
  acquisition.

## Leader Election

Because `lock.NewElection` is built on `lock.Locker`, the same election code
runs over any backend. Wire an `Election` in your application:

```go
elec := lock.NewElection(lock.ElectionConfig{
    Locker: locker, // injected lock.Locker
    Key:    "workers/leader",
    OnElected: func(ctx context.Context) { runLeaderWork(ctx) },
})
go elec.Run(ctx)
```
