# starter-config-apollo

[English](README.md) | [中文](README_CN.md)

`starter-config-apollo` integrates [Apollo](https://github.com/apolloconfig/apollo)
as a **remote configuration center** for Go-Spring, built on
github.com/apolloconfig/agollo/v4. Blank-importing it registers an `apollo`
config provider consumed via `spring.config.import`; remote config changes are
hot-reloaded at runtime through agollo's change notifications, without a
restart.

This starter covers the config-center role only.

## Installation

```bash
go get go-spring.org/starter-config-apollo
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-config-apollo"
```

### 2. Import config from Apollo

Declare the import in your configuration file using the provider syntax
`[optional:]apollo:<host>:<port>/<namespace>?<query>`:

```properties
spring.config.import=optional:apollo:127.0.0.1:8080/application?appId=demo
```

Query parameters:

| Key       | Default                       | Description                                    |
|-----------|-------------------------------|------------------------------------------------|
| `appId`   | (required)                    | Apollo application id                          |
| `cluster` | `default`                     | Apollo cluster name                            |
| `secret`  | (empty)                       | Access key for namespaces that require one     |
| `format`  | namespace ext, else `properties` | Content format: `properties`/`yaml`/`yml`/`json` |

Prefix with `optional:` so the application still starts when the namespace
does not exist yet (or is empty); the value is filled in once it is published.

### 3. Bind a dynamic field

Bind imported keys to a `gs.Dync[T]` field so they update live:

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

When the remote namespace changes, the provider's change listener triggers an
application property refresh and every bound `gs.Dync` field is re-bound
atomically.

## How It Works

- One agollo client per `(server, appId, cluster, secret, namespace)` tuple —
  agollo fixes the namespace at client creation time.
- The change listener is installed **before** the first fetch, so importing a
  namespace that does not exist yet still hot-reloads once it appears.
- Plain namespace names (`application`) are Apollo-native `properties`; a
  namespace with a `.yaml`/`.yml`/`.json` extension makes the Apollo server
  return that format, which the provider parses accordingly.

## Limitations

- No `governance.Source` integration yet; if Apollo should back governance
  rules, follow the `governance.go` pattern in `starter-config-nacos`.
