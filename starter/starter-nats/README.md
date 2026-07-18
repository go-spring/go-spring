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
member), and JetStream (publish to a stream then pull the message back).

Connection-layer events (async errors, disconnect, reconnect, close) are bridged into
go-spring's log.

## Advanced Features

* **JetStream**: Set `spring.nats.instances.<name>.jetstream.enabled=true` to expose
  a JetStream context on `Conn.JetStream` for that instance, derived from the same
  connection.
* **Multiple connections**: Every entry under `spring.nats.instances` becomes an
  independently configured `*Conn` bean; inject them by name to talk to different
  clusters or JetStream domains.
