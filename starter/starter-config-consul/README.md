# starter-config-consul

[English](README.md) | [中文](README_CN.md)

`starter-config-consul` integrates
[Consul KV](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv) as
a **remote configuration center** for Go-Spring, built on
github.com/hashicorp/consul/api. Blank-importing it registers a `consul` config
provider that pulls application configuration from a Consul agent at startup and
hot-reloads it at runtime — no restart required.

This starter covers the config-center role only. Service discovery through the
Consul catalog is a separate concern and is not provided here.

## Installation

```bash
go get go-spring.org/starter-config-consul
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-consul"
```

### 2. Import config from Consul

Declare the import in your configuration file using the provider syntax
`[optional:]consul:<host>:<port>/<kv-path>?<query>`:

```properties
spring.app.imports=optional:consul:127.0.0.1:8500/gs-config-demo?format=properties
```

Query parameters:

| Key          | Default                        | Description                                           |
|--------------|--------------------------------|-------------------------------------------------------|
| `format`     | KV path ext, else `properties` | Content format: `properties`/`yaml`/`toml`/`json`     |
| `scheme`     | `http`                         | `http` or `https`                                     |
| `token`      | (empty)                        | ACL token; empty means anonymous                      |
| `datacenter` | (agent default)                | Datacenter override                                   |

Prefix with `optional:` so the application still starts when the KV path does
not exist yet; the value is filled in once it is published.

### 3. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When the KV value changes, the provider's blocking-query watcher triggers an
application property refresh, and all bound `gs.Dync` fields are updated
atomically. See [example-config](example-config/example.go) for the full
publish -> hot-reload flow.

## How It Works

- On startup, `spring.app.imports` invokes the `consul` provider, which builds a
  Consul API client from the source string, reads the KV path, and starts a
  blocking-query watcher against it.
- A KV change bumps the query's `LastIndex`, which fires the framework's
  `PropertiesRefresher`. That reloads all configuration sources (re-running this
  provider) and re-binds every `gs.Dync` field via a two-phase, atomic commit.
