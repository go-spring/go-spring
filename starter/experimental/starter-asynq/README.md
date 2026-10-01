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

- **Two roles, one instance** — a `Client` (always wired, guarded enqueue) and
  a `Server` (only when `server.enabled=true`; a long-running worker is an
  opt-in). Both share the same Redis connection settings.
- **Guarded enqueue** — `Client.Enqueue` routes through the governance
  executor (rate limit / circuit breaking); with no governance center linked (no
  rules armed) it degrades to `Client.EnqueueContext`.
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

The starter **declares** what each guarded enqueue is (`observe.go`): the span name `enqueue`, the
`messaging.system`/`messaging.operation` labels, and the task type as `messaging.destination.name`
span/log detail. The signals themselves are emitted by the resilience layer — the one point on the
executor chain that sees a whole call, retries included — so beside the call-level
`messaging.client.operation.duration` histogram every call also reports an attempt-level
`messaging.client.attempt.duration` one, plus the `messaging.client.active_requests` gauge, the
`resilience.client.calls` counter and an access log tagged `_app_asynq_access`. All of it is a no-op
unless `starter-otel` installs providers.
