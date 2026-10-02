# starter-nats

[English](README.md) | [中文](README_CN.md)

`starter-nats` provides a NATS client wrapper based on github.com/nats-io/nats.go.
It covers core messaging and JetStream for Go-Spring applications.
The library is pure Go (no cgo).

## Installation

```bash
go get go-spring.org/starter-nats
```

## Quick Start

### 1. Import the `starter-nats` Package

Refer to the [main.go](example/main.go) file.

```go
import _ "go-spring.org/starter-nats"
```

### 2. Configure the NATS Connections

Define one or more named connections under `spring.nats.instances.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.nats.instances.main.url=nats://127.0.0.1:4222
spring.nats.instances.main.jetstream.enabled=true
spring.nats.instances.work.url=nats://127.0.0.1:4222
```

### 3. Inject the NATS Connection

Refer to the [main.go](example/main.go) file. Each named instance is registered
as a `*Conn` bean under that name; the injected bean re-exposes the raw connection's
methods (`Publish`/`Subscribe`/`Request`/...) as delegations, so you can call them
directly on it; `Conn.JetStream` is non-nil when JetStream is enabled on that instance.

```go
import StarterNats "go-spring.org/starter-nats"

type Service struct {
    Conn *StarterNats.Conn `autowire:"main"`
}
```

### 4. Use the Connection

Refer to the [main.go](example/main.go) file. The connection is established on
startup and drained on shutdown, so you can publish and subscribe directly.

```go
_ = s.Conn.Publish("demo.subject", []byte("value"))
reply, _ := s.Conn.Request("demo.rpc", []byte("ping"), time.Second)
```

## Core Features

The [example](example/main.go) self-asserts four features against a live server:
core pub/sub, request-reply, queue groups (each message delivered to exactly one
member), and JetStream (publish to a stream then pull the message back). It also
checks `HealthCheck(ctx, conn)` reports the connection as up before exercising them.

Connection-layer events (async errors, disconnect, reconnect, close) are bridged into
go-spring's log.

## Messaging Driver

Beyond the raw connection, this starter can expose a broker-neutral
`messaging.Driver` (from `go-spring.org/cloud/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
`nats.go` API — swapping the broker underneath does not touch business code.

The driver is registered as a bean per configured instance, under the same name
as the connection, so inject it by name like any client bean (`Driver
messaging.Driver \`autowire:"main"\``). Beans are keyed by name *and* type, so
it stays distinct from the `*Conn` bean. To build one by hand instead, call
`StarterNats.NewDriver(conn, prop)`, where `prop` is the process's
`traffic.Propagator` (a nil propagator falls back to
`traffic.NewDefaultPropagator(traffic.DefaultBinding())`).

Then publish and subscribe through the envelope:

```go
pub, _ := driver.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := driver.NewSubscriber(ctx, "orders", "workers")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // handle m.Payload / m.Headers
    return nil
})
```

The subscriber `group` maps onto a NATS queue group (competing consumers). Trace
context rides the message `Header`, so with starter-otel a trace links producer
to consumer. The `*Conn` bean stays available for JetStream, request-reply
and other NATS features the driver does not model.

## Advanced Features

* **JetStream**: Set `spring.nats.instances.<name>.jetstream.enabled=true` to expose
  a JetStream context on `Conn.JetStream` for that instance, derived from the same
  connection.
* **Multiple connections**: Every entry under `spring.nats` becomes an
  independently configured `*Conn` bean; inject them by name to talk to different
  clusters or JetStream domains.
* **Health check**: `HealthCheck(ctx, conn)` reflects the live state of the
  auto-reconnecting client, so health/readiness probes can query it at any time
  without relying only on the connection-event logs. Each instance also registers a
  `health.Indicator` (`nats:<name>`) whose probe only calls it, which
  starter-actuator folds into `/readiness`.
* **Authentication**: Beyond username/password and token, the starter supports NATS 2.x
  decentralized auth via a credentials file (`creds-file`) or an nkey seed file
  (`nkey-file`).
* **TLS**: Set `spring.nats.instances.<name>.tls.enabled=true` to negotiate TLS.
  Optionally pin a CA bundle (`tls.ca-file`) and supply a client certificate
  (`tls.cert-file`/`tls.key-file`) for mutual TLS.

## Observability

Both directions DECLARE their operation on the call — the publish/consume
identity (`messaging.system`, `messaging.operation`) plus the subject — and the
resilience layer, the single emitter on the executor chain, emits the signals
from that declaration: the call span, the call-level
`messaging.client.operation.duration` histogram, the attempt-level
`messaging.client.attempt.duration` histogram, the in-flight gauge, the
`resilience.client.calls` counter, and one access log per call. They ride the
global `TracerProvider` installed by [starter-otel](../starter-otel); without it
the span and metrics are no-ops, while the access log always writes through
go-spring's log.

The connection-state counter (`messaging.client.connection.state_changes`),
driven by the NATS client's own disconnect/reconnect/close callbacks, stays in
the starter: it is not a per-call signal, so it is not the emitter's to produce.

```go
import "github.com/nats-io/nats.go"

// Producer: the ctx-aware entry hangs the call's span off your trace and
// injects the trace context into the message header.
msg := &nats.Msg{Subject: "demo.pubsub", Data: []byte("hello")}
err := conn.PublishMsgContext(ctx, msg)

// Consumer: the handler receives a ctx carrying the producer's trace, so the
// consume span continues it across the broker.
sub, err := conn.Consume(ctx, "demo.pubsub", "", func(ctx context.Context, msg *nats.Msg) error {
    return handle(ctx, msg)
})
```

* `PublishMsg(msg)` is the ctx-less twin, kept so it overrides the raw
  `*nats.Conn.PublishMsg`. `nats.go` gives it no ctx parameter, so its span is a
  new root; use `PublishMsgContext` when the caller has a trace.
* `Consume(ctx, subject, queue, handler)` takes a context-bearing handler and an
  optional queue group — an empty queue is a plain broadcast subscription. Its
  setup ctx bounds the subscribe only; the consume span's parent comes from the
  message header.
* The [messaging.Driver](#messaging-driver) adds only envelope conversion and routes
  each publish through `PublishMsgContext` and each subscribe through `Consume`, so the
  driver path declares its operation on the same seams and the resilience layer emits —
  no second emitter, and the two paths never double-count a message.
* `HealthCheck(ctx, conn)` reflects the live state of the auto-reconnecting client, and
  the starter registers a `health.Indicator` per instance (`nats:<name>`) so an
  app that also imports starter-actuator gets nats connectivity folded into
  `/readiness`. Set `health=false` on an instance whose connectivity
  should not roll into readiness.

Not instrumented: JetStream operations and the raw `Subscribe`/`Publish`
delegations. Use `PublishMsgContext`/`Consume` for traced
pub/sub, and the driver for traced messaging envelopes.

## Configuration

Each connection under `spring.nats.instances.<name>` reads the following properties:

| Property | Default | Description |
| --- | --- | --- |
| `url` | (required) | NATS server URL(s), comma-separated for a cluster. |
| `name` | `` | Connection name reported to the server. |
| `username` / `password` | `` | Username/password authentication. |
| `token` | `` | Token authentication (alternative to username/password). |
| `creds-file` | `` | Path to a NATS credentials file (JWT + nkey seed) for decentralized auth. |
| `nkey-file` | `` | Path to an nkey seed file (alternative to `creds-file`). |
| `tls.enabled` | `false` | Negotiate TLS for the connection. |
| `tls.ca-file` | `` | PEM CA bundle to verify the server certificate; system roots when empty. |
| `tls.cert-file` / `tls.key-file` | `` | Client certificate and key for mutual TLS (set together). |
| `tls.insecure-skip-verify` | `false` | Disable server certificate verification (testing only). |
| `max-reconnects` | `60` | Maximum reconnect attempts; `-1` means unlimited. |
| `reconnect-wait` | `2s` | Delay between reconnect attempts. |
| `connect-timeout` | `5s` | Bound on the initial dial. |
| `jetstream.enabled` | `false` | Expose a JetStream context on `Conn.JetStream`. |
| `health` | `true` | Contribute a `health.Indicator` (`nats:<name>`) for this instance. |

## Design Notes

* **Guarding is opt-in at the call site.** Plain `Publish`/`Request` are
  untouched; resilience (rate-limit + circuit breaker) applies only through the
  `PublishGuarded`/`RequestGuarded` helpers, because NATS exposes no
  reject-capable middleware seam and silently wrapping `Publish` would change
  its semantics.
* **Limiter/breaker state is scoped per connection, not per subject.** The
  resilience executor's resource key is the connection bean name, so all
  subjects on one connection share the same limiter/breaker state.
* **JetStream reuses the connection.** With `jetstream.enabled=true` the context
  is built from the same `*nats.Conn`; if that fails, the raw connection is
  closed and boot fails — there is no second connection.
* **Reconnection is the reliability mechanism.** `max-reconnects=-1` means
  infinite client-side reconnect with no external supervisor; the `*Conn` bean
  stays usable across reconnects.
