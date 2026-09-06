# starter-influxdb

[English](README.md) | [中文](README_CN.md)

`starter-influxdb` provides InfluxDB 2.x support for Go-Spring: multi-instance
`influxdb2.Client` beans with fail-fast startup probes, per-request
observability (span + metric + access log), resilience on the blocking write
path (rate limit / circuit breaking / fault injection), a managed async
writer whose errors are drained into the log, and per-instance health
indicators. Built on the official
[influxdb-client-go](https://github.com/influxdata/influxdb-client-go) v2.

## Installation

```bash
go get go-spring.org/starter-influxdb
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-influxdb"
```

### 2. Configure

```properties
spring.influxdb.a.server-url=http://127.0.0.1:8086
spring.influxdb.a.auth-token=my-token
spring.influxdb.a.org=my-org
spring.influxdb.a.bucket=my-bucket
```

### 3. Inject

```go
type Service struct {
    Client *StarterInfluxdb.Client `autowire:"a"`
}
```

### 4. Use

```go
p := influxdb2.NewPointWithMeasurement("cpu").
    AddTag("host", "server-01").
    AddField("usage_idle", 42.5)
err := s.Client.WritePoints(ctx, p)

// Flux query through the embedded client
raw, err := s.Client.QueryAPI(s.Client.Org()).
    QueryRaw(ctx, `from(bucket:"my-bucket") |> range(start: -1m)`, influxdb2.DefaultDialect())
```

The wrapper embeds `influxdb2.Client`, so every SDK method (QueryAPI,
DeleteAPI, Setup, ...) promotes unchanged.

## Core Features

- **Multi-instance clients** — every `spring.influxdb.<name>` entry is its
  own bean with independent settings.
- **Two write paths** — `WritePoints` (blocking, resilience-guarded,
  fails per call) and `ManagedWriteAPI` (buffered batches on a background
  goroutine, flushed on shutdown; failed batches are drained into go-spring's
  log so the writer never blocks). See DESIGN for the split.
- **Fail-fast startup probe + health indicator** — a `/health` round trip at
  boot and an `influxdb:<name>` indicator for `starter-actuator`.
- **Observability** — every HTTP request emits a client span
  (db.system/db.operation/db.statement), the `db.client.operation.duration`
  histogram + `db.client.active_requests` gauge, and an access-log line via
  the `_app_influxdb_access` tag at the log package's native levels.
- **Resilience** — blocking writes route through the governance seams; with
  `starter-governance` absent the write path is observe-only.

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.influxdb.metrics.server-url=http://influx-a:8086
spring.influxdb.metrics.auth-token=...
spring.influxdb.events.server-url=http://influx-b:8086
spring.influxdb.events.auth-token=...
```

**Custom driver** — replace client assembly (e.g. to plug a session-token
credential flow) by providing your own `Driver` bean. The `Driver` is an
optional container bean: every client under `${spring.influxdb}` is built
through it, and the starter falls back to its bundled `DefaultDriver` when none
is provided. Register it in a package init (its constructor returns
`StarterInfluxdb.Driver`):

```go
func init() {
    gs.Provide(func() StarterInfluxdb.Driver {
        return v1CompatDriver{}
    })
}
```
### Log tag

Runtime logs from this module carry the tag `_app_influxdb` (influxdb starter). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.influxdb.type=Logger
logger.influxdb.level=WARN
logger.influxdb.tag=_app_influxdb
```
