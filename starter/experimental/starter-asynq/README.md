# starter-asynq

[English](README.md) | [中文](README_CN.md)

`starter-asynq` provides [Asynq](https://github.com/hibiken/asynq) support for
Go-Spring: Redis-backed task queues with a producer `Client` (enqueue) and an
opt-in worker `Server` (dequeue + run), both per `spring.asynq.instances.<name>` instance.

## Installation

```bash
go get go-spring.org/starter-asynq
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-asynq"
```

### 2. Configure

```properties
spring.asynq.instances.a.addr=127.0.0.1:6379
spring.asynq.instances.a.concurrency=4
spring.asynq.instances.a.server.enabled=true
```

### 3. Inject

```go
type Service struct {
    Client *StarterAsynq.Client `autowire:"a"`
    Server *StarterAsynq.Server `autowire:"a:server"`
}
```

### 4. Use

```go
// Register the handler before the worker starts.
s.Server.RegisterHandler("example:greet", func(ctx context.Context, t *asynq.Task) error {
    return nil
})

// Enqueue from the producer.
info, err := s.Client.Enqueue(ctx, asynq.NewTask("example:greet", payload))
```

## Core Features

- **Two roles, one instance** — a `Client` (always wired, observed enqueue) and
  a `Server` (only when `server.enabled=true`; a long-running worker is an
  opt-in). Both share the same Redis connection settings.
- **One injected center** — the constructor takes the governance family's sole
  injection point, the required `*governance.Center` (the `center` parameter),
  the one object a client reads its resilience/fault authorities from
  (`center.Resilience()`, `center.Fault()`). An asynq producer client exposes no
  seam for a resilience executor, so nothing from the center governs the enqueue
  path: `Client.Enqueue` runs under the observer, which emits the span, the
  metrics and the access log, and nothing throttles or retries.
- **Graceful shutdown** — `Server` drains in-flight tasks up to
  `shutdown-timeout` on destroy.
- **Health indicator** — an `asynq:<name>` probe pings Redis through a fresh
  inspector.

## Advanced Features

**Multiple queues** — declare priority weights per queue:

```properties
spring.asynq.instances.a.queues.critical=6
spring.asynq.instances.a.queues.default=3
```

**Multiple instances** — additional `spring.asynq.instances.<name>` entries, each an
independent producer/worker pair.

## Observability

The starter **declares** what each observed enqueue is (`observe.go`): the span name `enqueue`, the
`messaging.system`/`messaging.operation` labels, and the task type as `messaging.destination.name`
span/log detail. The signals themselves are emitted by the observer — the one point on the call
path that sees a whole call — so beside the call-level
`messaging.client.operation.duration` histogram every call also reports an attempt-level
`messaging.client.attempt.duration` one, plus the `messaging.client.active_requests` gauge, the
`resilience.client.calls` counter and an access log tagged `_app_asynq_access`. All of it is a no-op
unless `starter-otel` installs providers.

## Design Notes

* **The worker is a `gs.Server`, not a `gs.Runner`.** Go-Spring's `gs.Runner` is a
  startup-time, must-not-block interface; a long-running consumer belongs on
  `gs.Server`, so the worker never blocks startup and stops gracefully. It drives
  asynq's `Start` + wait-on-ctx rather than `asynq.Server.Run`, whose own signal
  handler would race Go-Spring's shutdown.
* **Register handlers any time before the worker runs.** `RegisterHandler` may be
  called after the constructor (for example from your own `Init`): the mux is built
  lazily on first use and shared with the running server.
* **The payload is your own `[]byte`.** Serialization, retry/queue policy (asynq
  `Options` pass through unchanged) and cron scheduling are asynq's own concerns —
  configure them through asynq's API, not starter config.
* **Panics are recovered by asynq, not the starter.** asynq's processor guard
  recovers handler panics; the starter deliberately does not wrap handlers a second
  time.
