# starter-config-etcd

[English](README.md) | [中文](README_CN.md)

`starter-config-etcd` integrates [etcd](https://etcd.io/) as a **remote
configuration center** for Go-Spring, built on go.etcd.io/etcd/client/v3.
Blank-importing it registers an `etcd` config provider that pulls application
configuration from an etcd cluster at startup and hot-reloads it at runtime —
no restart required.

This starter covers the config-center role only. Service discovery (etcd
naming) is a separate concern and is not provided here.

## Installation

```bash
go get go-spring.org/starter-config-etcd
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-etcd"
```

### 2. Import config from etcd

Declare the import in your configuration file using the provider syntax
`[optional:]etcd:<host>:<port>/<key>?<query>`:

```properties
spring.app.imports=optional:etcd:127.0.0.1:2379/gs-config-demo?format=properties
```

Query parameters:

| Key            | Default                     | Description                                       |
|----------------|-----------------------------|---------------------------------------------------|
| `format`       | key ext, else `properties`  | Content format: `properties`/`yaml`/`toml`/`json` |
| `username`     | (empty)                     | Auth username                                     |
| `password`     | (empty)                     | Auth password                                     |
| `dial-timeout` | `5s`                        | Client dial timeout                               |

Prefix with `optional:` so the application still starts when the key does not
exist yet; the value is filled in once it is written.

### 3. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When the etcd key changes, the provider's watcher triggers an application
property refresh, and all bound `gs.Dync` fields are updated atomically. See
[example-config](example-config/example.go) for the full publish -> hot-reload
flow.

## How It Works

- On startup, `spring.app.imports` invokes the `etcd` provider, which builds a
  clientv3 from the source string, reads the key, and installs an
  `etcd Watch` on it.
- A key change delivers a watch event, which calls the framework's
  `PropertiesRefresher`. That reloads all configuration sources (re-running this
  provider) and re-binds every `gs.Dync` field via a two-phase, atomic commit.
