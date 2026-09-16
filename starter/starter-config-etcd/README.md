# starter-config-etcd

[English](README.md) | [中文](README_CN.md)

`starter-config-etcd` integrates [etcd](https://etcd.io/) as a **remote
configuration center** for Go-Spring, built on go.etcd.io/etcd/client/v3.
Blank-importing it registers an `etcd` config provider that pulls application
configuration from an etcd cluster at startup and hot-reloads it at runtime
without restarting.

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
spring.config.import=optional:etcd:127.0.0.1:2379/gs-config-demo?format=properties
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
[example](example/example.go) for the full publish to hot-reload
flow.

## How It Works

- On startup, `spring.config.import` invokes the `etcd` provider, which builds a
  clientv3 from the source string, reads the key, and installs an
  `etcd Watch` on it.
- A key change delivers a watch event, whose callback calls the framework's
  process-level `gs.RefreshProperties()` facade. That reloads all configuration
  sources (re-running this provider) and re-binds every `gs.Dync` field via a
  two-phase, atomic commit.

## Design Notes

**The provider builds its own client.** The provider runs during property
refresh, before any bean exists, so it cannot receive an injected connection —
it constructs a clientv3 from the import string itself. Clients are cached per
`(endpoint, username, password)` tuple; since the provider re-runs on every
refresh, an uncached client would leak its background goroutines each time.

**The watcher is armed before the read.** The watch is registered before the
key is fetched, so an `optional:` import whose key does not exist yet still
hot-loads: the first PUT triggers a refresh that picks the value up. Reversing
the order would silently break this case.

**One key per import, no prefix watch.** An import targets a single key holding
one config document; multi-key fan-out is left to the application. This keeps
the mental model identical to the Consul and Nacos config providers.

**Discovery in the same starter — rejected.** etcd can serve both roles, but
config and discovery live at different layers. Splitting by role (mirroring
Spring Cloud Alibaba) keeps the module graph clean; etcd naming belongs to a
separate starter.

### Log tag

Runtime logs from this module carry the tag `_app_config_etcd` (etcd config source). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.config_etcd.type=Logger
logger.config_etcd.level=WARN
logger.config_etcd.tag=_app_config_etcd
```
