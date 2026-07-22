# starter-nacos

[English](README.md) | [中文](README_CN.md)

`starter-nacos` provides a Nacos client wrapper based on github.com/nacos-group/nacos-sdk-go/v2,
exposing both a naming (service discovery) client and a config (configuration management) client
for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-nacos
```

## Quick Start

### 1. Import the `starter-nacos` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-nacos"
```

### 2. Configure the Nacos Instance

Add Nacos configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.nacos.a.ip-addr=127.0.0.1
spring.nacos.a.port=8848
```

Each entry under `spring.nacos` is a named client: the key (`a`, `b`, ...) becomes the
bean name you inject. Both a naming client and a config client are created per entry.

### 3. Inject the Nacos Clients

Refer to the [example.go](example/example.go) file. Both a naming client and a
config client are created per entry; inject whichever you need by its interface type
and the entry's key as the bean name.

```go
import (
    "github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
    "github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
)

type Service struct {
    Config config_client.IConfigClient `autowire:"a"`
    Naming naming_client.INamingClient `autowire:"a"`
}
```

### 4. Use the Nacos Clients

Refer to the [example.go](example/example.go) file.

```go
_, err := s.Config.PublishConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP", Content: "value"})
content, err := s.Config.GetConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP"})
```

## Core Features

The [example](example/example.go) demonstrates three core Nacos capabilities end-to-end:

1. **Config publish + get**: `PublishConfig` writes a value, then `GetConfig` reads it back
   (polled to tolerate Nacos's asynchronous propagation).
2. **Config listen**: `ListenConfig` registers an `OnChange` callback; a subsequent
   `PublishConfig` triggers the callback and delivers the new value.
3. **Service register + discovery**: `RegisterInstance` publishes a service instance and
   `GetService` discovers it via the naming client.

## Remote Configuration Provider

Beyond the injectable config client, `starter-nacos` registers a `nacos` remote
configuration provider with the Go-Spring config system. This lets you pull
application configuration directly from a Nacos config server at startup and
hot-reload it at runtime — no restart required.

### 1. Import config from Nacos

Declare the import in your configuration file using the provider syntax
`[optional:]nacos:<host>:<port>/<dataId>?<query>`:

```properties
spring.app.imports=optional:nacos:127.0.0.1:8848/gs-config-demo?group=DEFAULT_GROUP&format=properties
```

Query parameters:

| Key          | Default         | Description                              |
|--------------|-----------------|------------------------------------------|
| `group`      | `DEFAULT_GROUP` | Nacos config group                       |
| `namespace`  | (public)        | Namespace id                             |
| `format`     | data id ext, else `properties` | Content format: `properties`/`yaml`/`toml`/`json` |
| `username`   | (empty)         | Auth username                            |
| `password`   | (empty)         | Auth password                            |
| `timeout-ms` | `5000`          | Request timeout in milliseconds          |

Prefix with `optional:` so the application still starts when the data id does
not exist yet; the value is filled in once it is published.

### 2. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When the remote config changes, the provider's change listener triggers an
application property refresh, and all bound `gs.Dync` fields are updated
atomically. See [example-config](example-config/example.go) for the full
publish → hot-reload flow.

## Advanced Features

* **Naming + config clients**: Each entry under `spring.nacos` creates both an
  `INamingClient` and an `IConfigClient` (distinct interface types) under the same name.
* **Multiple Nacos instances**: Define one named entry per Nacos cluster under
  `spring.nacos` and inject each by its key.
