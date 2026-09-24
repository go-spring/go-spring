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

Refer to the [example.go](example/example.go) file.

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

Refer to the [example.go](example/example.go) file. Each named instance is registered
as a `*Conn` bean under that name; the injected bean embeds `*nats.Conn`, so you can
call `Publish`/`Subscribe`/`Request` directly on it; `Conn.JetStream` is non-nil when
JetStream is enabled on that instance.

```go
import StarterNats "go-spring.org/starter-nats"

type Service struct {
    Conn *StarterNats.Conn `autowire:"main"`
}
```

### 4. Use the Connection

Refer to the [example.go](example/example.go) file. The connection is established on
startup and drained on shutdown, so you can publish and subscribe directly.

```go
_ = s.Conn.Publish("demo.subject", []byte("value"))
reply, _ := s.Conn.Request("demo.rpc", []byte("ping"), time.Second)
```

## Core Features

The [example](example/example.go) self-asserts four features against a live server:
core pub/sub, request-reply, queue groups (each message delivered to exactly one
member), and JetStream (publish to a stream then pull the message back). It also
checks `Conn.Healthy()` reports the connection as up before exercising them.

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
it stays distinct from the raw `*Conn` bean. To build one by hand instead, call
`StarterNats.NewDriver(conn)`.

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
to consumer. The raw `*Conn` bean stays available for JetStream, request-reply
and other NATS features the driver does not model.

## Advanced Features

* **JetStream**: Set `spring.nats.instances.<name>.jetstream.enabled=true` to expose
  a JetStream context on `Conn.JetStream` for that instance, derived from the same
  connection.
* **Multiple connections**: Every entry under `spring.nats` becomes an
  independently configured `*Conn` bean; inject them by name to talk to different
  clusters or JetStream domains.
* **Health check**: `Conn.Healthy()` reflects the live state of the auto-reconnecting
  client, so health/readiness probes can query it at any time without relying only on
  the connection-event logs. Each instance also registers a `health.Indicator`
  (`nats:<name>`) on top of it, which starter-actuator folds into `/readiness`.
* **Authentication**: Beyond username/password and token, the starter supports NATS 2.x
  decentralized auth via a credentials file (`creds-file`) or an nkey seed file
  (`nkey-file`).
* **TLS**: Set `spring.nats.instances.<name>.tls.enabled=true` to negotiate TLS.
  Optionally pin a CA bundle (`tls.ca-file`) and supply a client certificate
  (`tls.cert-file`/`tls.key-file`) for mutual TLS.

## Observability

Both directions of an operation are instrumented on the connection itself — a
span, a duration/in-flight metric and an access log — and ride the global
`TracerProvider` and propagator installed by [starter-otel](../starter-otel).
Without starter-otel they are no-ops, so the instrumentation is a safe,
zero-config opt-in.

```go
import "github.com/nats-io/nats.go"

// Producer: the ctx-aware entry hangs the producer span off your trace and
// injects the trace context into the message header.
msg := &nats.Msg{Subject: "demo.pubsub", Data: []byte("hello")}
err := conn.PublishMsgContext(ctx, msg)

// Consumer: the handler receives a ctx carrying the producer's trace, so the
// consumer span continues it across the broker.
sub, err := conn.Consume(ctx, "demo.pubsub", "", func(ctx context.Context, msg *nats.Msg) error {
    return handle(ctx, msg)
})
```

* `PublishMsg(msg)` is the ctx-less twin, kept so it still shadows the embedded
  `*nats.Conn.PublishMsg`. `nats.go` gives it no ctx parameter, so its producer
  span is a new root; use `PublishMsgContext` when the caller has a trace.
* `Consume(ctx, subject, queue, handler)` takes a context-bearing handler and an
  optional queue group — an empty queue is a plain broadcast subscription. Its
  setup ctx bounds the subscribe only; the consumer span's parent comes from the
  message header.
* The [messaging.Driver](#messaging-driver) calls the raw *nats.Conn and adds only
  envelope conversion; its instrumentation comes from the broker-neutral
  `messaging.Observe` decorator, so the two paths never double-count a message.
* `Conn.Healthy()` reflects the live state of the auto-reconnecting client, and
  the starter registers a `health.Indicator` per instance (`nats:<name>`) so an
  app that also imports starter-actuator gets nats connectivity folded into
  `/readiness`. Set `health.enabled=false` on an instance whose connectivity
  should not roll into readiness.

Not instrumented: JetStream operations and the raw `Subscribe`/`Publish`
methods on the embedded connection. Use `PublishMsgContext`/`Consume` for traced
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
| `health.enabled` | `true` | Contribute a `health.Indicator` (`nats:<name>`) for this instance. |
