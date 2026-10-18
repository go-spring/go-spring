# starter-influxdb

[English](README.md) | [中文](README_CN.md)

`starter-influxdb` provides InfluxDB 2.x support for Go-Spring: multi-instance
`influxdb2.Client` beans with opt-in fail-fast startup probes, per-request
observability declared by the starter and emitted by the resilience layer
(span + metric + access log), resilience on the blocking write
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
spring.influxdb.instances.a.server-url=http://127.0.0.1:8086
spring.influxdb.instances.a.auth-token=my-token
spring.influxdb.instances.a.org=my-org
spring.influxdb.instances.a.bucket=my-bucket
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
DeleteAPI, Setup, ...) is promoted unchanged.

## Core Features

- **Multi-instance clients** — every `spring.influxdb.instances.<name>` entry is its
  own bean with independent settings.
- **Two write paths** — `WritePoints` (blocking, resilience-guarded,
  fails per call) and `ManagedWriteAPI` (buffered batches on a background
  goroutine, flushed on shutdown; failed batches are drained into go-spring's
  log so the writer never blocks). See the Design Notes below for the split.
- **Fail-fast startup probe + health indicator** — an opt-in `/health` round trip
  at boot (`ping=true`; off by default) and an `influxdb:<name>` indicator for
  `starter-actuator` (`health=false` to skip it).
- **Observability** — the starter *declares* each request's identity
  (`db.system=influxdb`, a bounded `db.operation=<method>`, and the URL path as
  `db.statement`); the resilience layer *emits* it. It opens one client span per
  call, records the call-level `db.client.operation.duration` histogram and the
  attempt-level `db.client.attempt.duration` histogram + the
  `db.client.active_requests` gauge, and writes one access-log line via the
  `_app_influxdb_access` tag at the log package's native levels.
- **Resilience** — blocking writes route through the executor built from the
  injected `*resilience.Manager`; with no governance center linked that
  executor is a pass-through and the write path emits nothing (the starter owns
  no emitter).

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.influxdb.instances.metrics.server-url=http://influx-a:8086
spring.influxdb.instances.metrics.auth-token=...
spring.influxdb.instances.events.server-url=http://influx-b:8086
spring.influxdb.instances.events.auth-token=...
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

## Design Notes

* **Two write methods, two failure contracts.** `WritePoints` is blocking and
  resilience-guarded (rate limit / circuit breaking / fault injection) and fails
  per call; `ManagedWriteAPI` batches on a background goroutine whose retries are
  the SDK's own, so it is deliberately unguarded — guarding per point would
  double-count, and its failures surface as log lines, not caller errors.
* **Async errors are drained for you.** The starter drains `ManagedWriteAPI`'s
  `Errors()` channel into go-spring's log, because an undrained channel blocks the
  writer on its first failure; if you need custom handling, use the embedded
  `WriteAPI` directly.
* **Only the blocking write is guarded.** `QueryRaw` and the other query paths
  carry no per-call guard — the blocking write is the overload-sensitive path. A
  guarded query would be additive later, not breaking.
* **`org`/`bucket` gate the write helpers, not the connection.** A client
  configured without them still serves Query/Delete APIs; the write helpers fail
  with a pointed message instead of at wiring time.
