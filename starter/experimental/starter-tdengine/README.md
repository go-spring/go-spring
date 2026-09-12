# starter-tdengine

[English](README.md) | [中文](README_CN.md)

`starter-tdengine` provides TDengine support for Go-Spring over the official
[driver-go](https://github.com/taosdata/driver-go) v3 **websocket driver**
(taosWS) — pure Go, no client library install, no CGO. The bean is a
`*sql.DB` pool: multi-instance clients with fail-fast startup pings,
per-statement observability and resilience at the connection seam, and
per-instance health indicators.

## Installation

```bash
go get go-spring.org/starter-tdengine
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-tdengine"
```

### 2. Configure

```properties
spring.tdengine.instances.a.dsn=root:taosdata@ws(127.0.0.1:6041)/power
spring.tdengine.instances.a.max-open-conns=8
```

### 3. Inject

```go
type Service struct {
    Client *StarterTdengine.Client `autowire:"a"`
}
```

### 4. Use

```go
_, err := s.Client.ExecContext(ctx,
    "INSERT INTO power.d001 USING power.meters TAGS('beijing') VALUES (NOW, 10.5)")
rows, err := s.Client.QueryContext(ctx, "SELECT COUNT(*) FROM power.meters")
```

The wrapper embeds `*sql.DB`, so `Query/Exec/BeginTx/PingContext` and the
whole `database/sql` ecosystem promote unchanged.

## Core Features

- **Multi-instance clients** — every `spring.tdengine.instances.<name>` entry is its
  own bean with independent settings.
- **Fail-fast startup ping + health indicator** — a `PingContext` at boot and
  a `tdengine:<name>` indicator for `starter-actuator`.
- **Per-statement resilience + observability** — statements flow through a
  guarded driver.Conn: rate limiting, circuit breaking, fault injection, span
  + metric + access log per statement, all armed in Init.
- **Websocket wire, zero CGO** — works against any TDengine ≥ 3.3.6 running
  taosAdapter; `wss://` DSNs for TLS.

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.tdengine.instances.hot.dsn=root:taosdata@ws(10.0.0.1:6041)/power
spring.tdengine.instances.cold.dsn=root:taosdata@ws(10.0.0.2:6041)/archive
```

**Custom driver** — replace client assembly (e.g. to pin a different wire
protocol or connection tuning) by providing your own `Driver` bean. The
`Driver` is an optional container bean: every client under `${spring.tdengine}`
is built through it, and the starter falls back to its bundled `DefaultDriver`
when none is provided. Register it in a package init (its constructor returns
`StarterTdengine.Driver`):

```go
func init() {
    gs.Provide(func() StarterTdengine.Driver {
        return restDriver{}
    })
}
```
