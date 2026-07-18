# starter-mqtt

[English](README.md) | [中文](README_CN.md)

`starter-mqtt` provides an MQTT client wrapper based on github.com/eclipse/paho.mqtt.golang,
making it easy to integrate and use MQTT in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-mqtt
```

## Quick Start

### 1. Import the `starter-mqtt` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-mqtt"
```

### 2. Configure the MQTT Clients

Define one or more named clients under `spring.mqtt.instances.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.mqtt.instances.a.broker=tcp://127.0.0.1:1883
spring.mqtt.instances.b.broker=tcp://127.0.0.1:1883
```

### 3. Inject the MQTT Client

Refer to the [example.go](example/example.go) file. Each named instance is registered
as an `mqtt.Client` bean under that name; inject the one you need by name.

```go
import mqtt "github.com/eclipse/paho.mqtt.golang"

type Service struct {
    Client mqtt.Client `autowire:"a"`
}
```

### 4. Use the MQTT Client

Refer to the [example.go](example/example.go) file. The client is connected on
startup and disconnected on shutdown, so you can publish and subscribe directly.

```go
token := s.Client.Publish("go-spring/hello", 1, false, "value")
token.Wait()
_ = token.Error()
```

## Core Features

The [example](example/example.go) demonstrates a pub/sub round-trip: subscribe to a
topic at QoS 1, publish a message to it, and assert the payload is delivered back to
the subscription handler. It also checks `Client.IsConnected()` before publishing.

Connection-layer events (connect, connection lost, reconnecting) are bridged into
go-spring's log.

## Observability

Distributed tracing is **not applicable** to this starter, and this is a
deliberate decision rather than a gap:

* `paho.mqtt.golang` has no official OTel instrumentation.
* More fundamentally, MQTT 3.1.1 — the protocol version this client speaks — has
  no per-message metadata channel. `Client.Publish(topic, qos, retained, payload)`
  exposes nowhere to attach a W3C `traceparent`, and the broker delivers only the
  raw payload. User Properties, which would carry trace context, exist only in
  MQTT 5, unsupported by this paho v3 client. Smuggling trace context into the
  application payload or the topic would corrupt the message contract, so it is
  not done.

The practical consequence: producer and consumer spans cannot be linked across
the broker. Connection-layer events (connect, connection lost, reconnecting) are
still bridged into go-spring's log for operational visibility.

## Advanced Features

* **Multiple MQTT clients**: Every entry under `spring.mqtt.instances` becomes an
  independently configured `mqtt.Client` bean; inject them by name to talk to
  different brokers.
* **TLS (MQTTS)**: Set `spring.mqtt.instances.<name>.tls.enabled=true` with a
  `ssl://`/`tls://` broker URL to negotiate TLS. Optionally pin a CA bundle
  (`tls.ca-file`) and supply a client certificate (`tls.cert-file`/`tls.key-file`)
  for mutual TLS.
* **Last Will and Testament (LWT)**: Set `spring.mqtt.instances.<name>.will.topic`
  to have the broker publish a will message on your behalf when the client
  disconnects ungracefully.

## Configuration

Each client under `spring.mqtt.instances.<name>` reads the following properties:

| Property | Default | Description |
| --- | --- | --- |
| `broker` | (required) | MQTT broker address, e.g. `tcp://127.0.0.1:1883` (`ssl://` for MQTTS). |
| `client-id` | `` | Client identifier; the library generates one when empty. |
| `username` / `password` | `` | Authentication credentials. |
| `clean-session` | `true` | Whether the broker discards session state on disconnect. |
| `keep-alive` | `30s` | Interval between PING packets. |
| `connect-timeout` | `10s` | Bound on `Connect`; `0` disables the timeout. |
| `tls.enabled` | `false` | Attach a `*tls.Config` for MQTTS. |
| `tls.ca-file` | `` | PEM CA bundle to verify the broker certificate; system roots when empty. |
| `tls.cert-file` / `tls.key-file` | `` | Client certificate and key for mutual TLS (set together). |
| `tls.insecure-skip-verify` | `false` | Disable broker certificate verification (testing only). |
| `will.topic` | `` | Will topic; empty disables the will. |
| `will.payload` | `` | Will message body. |
| `will.qos` | `0` | Will delivery QoS (0, 1, or 2). |
| `will.retained` | `false` | Whether the broker retains the will message. |
