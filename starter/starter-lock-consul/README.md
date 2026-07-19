# starter-lock-consul

[English](README.md) | [中文](README_CN.md)

`starter-lock-consul` integrates [Consul](https://www.consul.io/) as a
**distributed-lock backend** for the framework-agnostic `lock.Locker`
abstraction in
[`go-spring.org/stdlib/lock`](../../stdlib/lock). Blank-importing this starter
registers one `lock.Locker` bean per entry under `spring.lock.<name>`, each
backed by a Consul session with automatic renewal.

Because the injected type is the neutral `lock.Locker` interface, switching
between the Redis, etcd and Consul backends is a one-line blank-import swap —
no application code changes.

## Installation

```bash
go get go-spring.org/starter-lock-consul
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-lock-consul"
```

### 2. Configure a lock instance

Add one entry per named instance in your
[configuration file](example/conf/app.properties):

```properties
spring.lock.jobs.address=127.0.0.1:8500
spring.lock.jobs.ttl=30s
spring.lock.jobs.key-prefix=demo/lock/
```

### 3. Inject and use `lock.Locker`

```go
import "go-spring.org/stdlib/lock"

type Service struct {
    Locker lock.Locker `autowire:"jobs"`
}

func (s *Service) Run(ctx context.Context) error {
    l, ok, err := s.Locker.TryAcquire(ctx, "nightly-sync")
    if err != nil {
        return err
    }
    if !ok {
        return nil // another replica is running it
    }
    defer l.Unlock(ctx)

    select {
    case <-l.Lost():
        return errors.New("lost lock, aborting")
    default:
    }
    return doWork(ctx)
}
```

## Configuration

All keys live under `spring.lock.<name>`:

| Key                | Default  | Description                                                                            |
|--------------------|----------|----------------------------------------------------------------------------------------|
| `address`          | required | Consul agent endpoint, e.g. `127.0.0.1:8500`. Fail-fast at startup when empty.         |
| `scheme`           | `http`   | `http` or `https`. Auto-promoted to `https` when `tls.enabled=true` and left as `http`.|
| `token`            | (empty)  | Consul ACL token used by the API client (distinct from the per-lock fencing token).    |
| `ttl`              | `30s`    | Session TTL. Consul clamps to `[10s, 86400s]`; auto-renewed behind `api.Lock`.         |
| `key-prefix`       | `lock/`  | Prepended to every lock key so many apps can share a Consul cluster.                   |
| `tls.enabled`      | `false`  | Turn on TLS to the Consul agent.                                                       |
| `tls.server-name`  | (empty)  | Overrides the server name checked against the presented certificate.                   |
| `tls.ca-file`      | (empty)  | PEM bundle of CAs used to verify the agent's cert.                                     |
| `tls.cert-file`    | (empty)  | Client certificate for mutual TLS.                                                     |
| `tls.key-file`     | (empty)  | Client key for mutual TLS.                                                             |

## Leader Election

Any `lock.Locker` composes with `lock.NewElection`, so the same election code
runs over any registered backend:

```go
e := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "singleton-worker",
    OnElected: func(ctx context.Context) {
        // I'm the leader until ctx is cancelled.
    },
})
go e.Run(ctx)
```

## How It Works

- Each instance owns its own `*consul/api.Client`, built from the bound `Config`.
- Every `Acquire` / `TryAcquire` builds a fresh `*api.Lock` with the configured
  `SessionTTL`; Consul auto-renews the session while the handle is alive.
- The `<-chan struct{}` returned by `api.Lock.Lock` is used verbatim as the
  handle's `Lost()` channel, so a session invalidation or agent partition is
  observable to the critical section.
- `Unlock` releases the lock and destroys its session; it is idempotent and
  treats `api.ErrLockNotHeld` as a benign "already released" signal.
