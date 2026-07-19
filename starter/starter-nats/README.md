# starter-nats

[English](README.md) | [中文](README_CN.md)

`starter-nats` provides a NATS client wrapper based on github.com/nats-io/nats.go,
making it easy to integrate core messaging and JetStream in Go-Spring applications.
The library is pure Go (no cgo), so cross-compilation stays clean.

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

## Messaging Binder

Beyond the raw connection, this starter can expose a broker-neutral
`messaging.Binder` (from `go-spring.org/stdlib/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
`nats.go` API — swapping the broker underneath does not touch business code.

Register the binder as a bean from a `*Conn` (select the named instance with
`gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterNats "go-spring.org/starter-nats"
)

gs.Provide(StarterNats.NewBinder, gs.TagArg("main"))
```

Then publish and subscribe through the envelope:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "workers")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // handle m.Payload / m.Headers
    return nil
})
```

The subscriber `group` maps onto a NATS queue group (competing consumers). Trace
context rides the message `Header`, so with starter-otel a trace links producer
to consumer. The raw `*Conn` bean stays available for JetStream, request-reply
and other NATS features the binder does not model.

## Advanced Features

* **JetStream**: Set `spring.nats.instances.<name>.jetstream.enabled=true` to expose
  a JetStream context on `Conn.JetStream` for that instance, derived from the same
  connection.
* **Multiple connections**: Every entry under `spring.nats.instances` becomes an
  independently configured `*Conn` bean; inject them by name to talk to different
  clusters or JetStream domains.
* **Health check**: `Conn.Healthy()` reflects the live state of the auto-reconnecting
  client, so health/readiness probes can query it at any time without relying only on
  the connection-event logs.
* **Authentication**: Beyond username/password and token, the starter supports NATS 2.x
  decentralized auth via a credentials file (`creds-file`) or an nkey seed file
  (`nkey-file`).
* **TLS**: Set `spring.nats.instances.<name>.tls.enabled=true` to negotiate TLS.
  Optionally pin a CA bundle (`tls.ca-file`) and supply a client certificate
  (`tls.cert-file`/`tls.key-file`) for mutual TLS.

## Observability

Distributed tracing is available through native OTel helpers that ride the
global `TracerProvider` and propagator installed by
[starter-otel](../starter-otel). Without starter-otel they are no-ops and change
no message bytes, so instrumenting your code is a safe, zero-config opt-in.

```go
import starter "go-spring.org/starter-nats"

// Producer: publish with PublishMsg so trace context can ride the message header.
msg := &nats.Msg{Subject: "demo.pubsub", Data: []byte("hello")}
_, span := starter.StartPublishSpan(ctx, msg)
err := conn.PublishMsg(msg)
starter.EndSpan(span, err)

// Consumer: continue the trace carried in the message header.
ctx, span := starter.StartConsumeSpan(ctx, msg)
err := handle(ctx, msg)
starter.EndSpan(span, err)
```

Why call-site helpers instead of a wrapped connection:

* `nats.go` has no official OTel instrumentation, and its API (`Publish`,
  `Subscribe`, `Request`, `QueueSubscribe`, plus JetStream) is broad and
  callback-driven, so a wrapper would have to shadow all of it and still leave the
  JetStream context uninstrumented.
* `nats.Msg` carries a `Header` (since NATS 2.2) that survives the broker
  round-trip. Propagating context requires publishing a `*nats.Msg` via
  `PublishMsg`/`RespondMsg` rather than the header-less `Publish(subject, data)`;
  instrumenting at the call site is what links producer to consumer across
  services.

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
