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
the subscription handler.

## Advanced Features

* **Multiple MQTT clients**: Every entry under `spring.mqtt.instances` becomes an
  independently configured `mqtt.Client` bean; inject them by name to talk to
  different brokers.
