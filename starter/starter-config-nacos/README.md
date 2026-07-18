# starter-config-nacos

[English](README.md) | [中文](README_CN.md)

`starter-config-nacos` integrates [Nacos](https://nacos.io/) as a **remote
configuration center** for Go-Spring, built on
github.com/nacos-group/nacos-sdk-go/v2. Blank-importing it registers a `nacos`
config provider that pulls application configuration from a Nacos config server
at startup and hot-reloads it at runtime — no restart required.

This starter covers the config-center role only. Service discovery (Nacos
naming) is a separate concern and is not provided here.

## Installation

```bash
go get go-spring.org/starter-config-nacos
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-nacos"
```

### 2. Import config from Nacos

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

### 3. Bind a dynamic field

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

## How It Works

- On startup, `spring.app.imports` invokes the `nacos` provider, which builds a
  config client from the source string, fetches the data id, and registers a
  change listener.
- A remote change fires the listener, which calls the framework's
  `PropertiesRefresher`. That reloads all configuration sources (re-running this
  provider) and re-binds every `gs.Dync` field via a two-phase, atomic commit.
