# starter-gorm-mysql Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Anchored to [example/](example/),
[example-health/](example-health/), [example-load/](example-load/), [example-otel/](example-otel/) and
[example-cloudnative/](example-cloudnative/). **GORM semantics are [gorm's](https://gorm.io/docs/);
MySQL DSN parameter semantics are [go-sql-driver/mysql's](https://github.com/go-sql-driver/mysql#dsn-data-source-name)**
— this doc covers the binding surface and the go-spring increment (wiring, discovery, TLS, observe, health).

**Activation**: one client bean per `spring.gorm.mysql.<name>` entry (`OnProperty` prefix check on
`spring.gorm.mysql`). No `enabled` key — presence of any entry activates the starter; no entries,
nothing registers. Shared lifecycle (wrapper, pool, observe plugin, health, `UseDBCustomizer`, the 10
`Common` keys) is documented in [starter-gorm's USAGE](../starter-gorm/USAGE.md) and not repeated here.

---

## 1. Complete worked project

A realistic service: a `primary` instance dialed from a fixed addr, a `cluster` instance resolved
through etcd service discovery, actuator health, and OTel observability. File tree:

```
demo/
├── go.mod
├── main.go
├── dao/
│   └── dao.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
module demo

go 1.26

require (
    go-spring.org/spring               v1.3.4
    go-spring.org/starter-gorm-mysql   latest
    go-spring.org/starter-registry-etcd latest   // etcd discovery backend
    go-spring.org/starter-actuator     latest   // /health + /readyz
    go-spring.org/starter-otel         latest   // trace/metric export
)
```

**main.go**:

```go
package main

import (
    "demo/dao"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-registry-etcd" // registers the etcd discovery backends
    _ "go-spring.org/starter-gorm-mysql"
)

func main() {
    gs.Provide(dao.NewService).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**dao/dao.go** — both instances autowired by instance name:

```go
package dao

import (
    StarterMySql "go-spring.org/starter-gorm-mysql"
)

type Service struct {
    Primary *StarterMySql.DB `autowire:"primary"` // fixed addr, pool-tuned
    Cluster *StarterMySql.DB `autowire:"cluster"` // discovery-resolved (etcd)
}

func NewService() *Service { return &Service{} }

func (s *Service) Init() error { // gs InitMethod: table + smoke query on both paths
    if err := s.Primary.AutoMigrate(&KV{}); err != nil {
        return err
    }
    return s.Cluster.Exec("SELECT 1").Error
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- primary: fixed addr ------------------------------------------------------
spring.gorm.mysql.primary.user=root
spring.gorm.mysql.primary.password=123456
spring.gorm.mysql.primary.addr=127.0.0.1:3306
spring.gorm.mysql.primary.db=test
spring.gorm.mysql.primary.parseTime=true          # scan DATETIME into time.Time
spring.gorm.mysql.primary.max-open-conns=10
spring.gorm.mysql.primary.max-idle-conns=5
spring.gorm.mysql.primary.conn-max-lifetime=30m
spring.gorm.mysql.primary.slow-threshold=200ms

# --- cluster: discovery-resolved (etcd) ----------------------------------------
spring.gorm.mysql.cluster.user=root
spring.gorm.mysql.cluster.password=123456
spring.gorm.mysql.cluster.db=test
spring.gorm.mysql.cluster.service-name=mysql-cluster
spring.gorm.mysql.cluster.discovery=local        # -> spring.discovery.etcd.local.*
spring.gorm.mysql.cluster.conn-max-lifetime=30m  # bounds failover lag (see §4.3)

# --- etcd discovery backend named "local" --------------------------------------
spring.discovery.etcd.local.endpoints=127.0.0.1:2379
spring.discovery.etcd.local.key-prefix=/services/

# --- actuator (aggregates gorm:mysql:<name> indicators) ------------------------
spring.actuator.addr=:9370

# --- observability --------------------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
```

**Bring up the dependencies**:

```bash
docker run -d --name mysql -p 127.0.0.1:3306:3306 \
  -e MYSQL_ROOT_PASSWORD=123456 -e MYSQL_DATABASE=test mysql:8
docker run -d --name etcd -p 127.0.0.1:2379:2379 \
  quay.io/coreos/etcd:v3.5.16 etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:2379
# Jaeger all-in-one for traces (optional):
docker run -d --name jaeger -p 127.0.0.1:16686:16686 -p 127.0.0.1:4317:4317 jaegertracing/all-in-one
# Register two MySQL instances into etcd (the registrar's key/value layout):
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/a \
  '{"service_name":"mysql-cluster","addr":"127.0.0.1:3306"}'
go run .
```

**Verify**:

```bash
curl -i 127.0.0.1:9370/readyz                 # UP: gorm:mysql:primary + gorm:mysql:cluster
curl -i 127.0.0.1:9370/health                 # aggregated indicators
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
curl -s '127.0.0.1:16686/api/traces?service=demo&limit=1' | grep '"data":\['
```

---

## 2. Assembly & timing

```
import starter-gorm-mysql
  └─ init: gormcore.Register(Dialect{Prefix: "spring.gorm.mysql", Engine: "mysql", ...})
gs.Run()
  ├─ OnProperty("spring.gorm.mysql") fires; conf.BindEach binds one Config per <name> entry
  ├─ per instance: build(ctx, c)
  │    ├─ addr/service-name presence check (one must be set)
  │    ├─ TLS: c.TLS.Build() -> *tls.Config; mysql.RegisterTLSConfig("gstls_<n>", cfg)
  │    ├─ DSN(): user:pass@tcp(addr)/db?params
  │    └─ discovery (service-name set, mesh off): mysql.RegisterDialContext("gsdisco_<svc>_<n>",
  │         dial via Resolver.Pick) and the DSN is rewritten to net(gsdisco_...)(<service-name>)
  ├─ gormcore.Open: gorm.Open -> ApplyPool (incl. startup ping, ping-timeout bound)
  │    -> ApplyDBCustomizers -> *DB bean (Name=<name>, Init, Destroy)
  ├─ health indicator "gorm:mysql:<name>" exported as health.Indicator
  ├─ DB.Init (after Observability field-inject): observe plugin (db.system=mysql)
  │    + resilience/fault executor callbacks on resource "gorm:mysql:<addr|service-name>"
  └─ DB.Destroy (SIGTERM): executor close -> discovery watch stop + TLS deregister -> pool close
```

**One query, layer by layer** (with starter-otel imported and `observe.enabled=true`):
`db.WithContext(ctx).Exec("SELECT 1")` → resilience callbacks wrap the processor (governance
executor when configured, no-op otherwise) → the gorm observe plugin's before/after callbacks open a
span (`db.system=mysql`), record the `db.client.operation.duration` metric, and emit an access log
record (level from the wrapper `observability` field, default `brief`) → database/sql picks a pooled
conn (dialing through the registered discovery dialer for a new conn) → the driver executes.

**Why the DSN rewrite**: go-sql-driver only routes through a `RegisterDialContext` dialer when the
DSN's *network* part is the registered name — the host part is then ignored by the dialer (it picks a
live endpoint itself). The starter rewrites both (`Network=gsdisco_...`, `Addr=<service-name>`), so
the `addr` you configured is not dialed at all in discovery mode; only `user`/`password`/`db` and the
option params survive from the config. If you hand-build the same DSN yourself and forget the rewrite,
the driver falls back to a normal TCP dial of whatever host you wrote.

---

## 3. Per-key behavior reference

MySQL-specific keys under `spring.gorm.mysql.<name>.*` (the 10 shared `Common` keys — pool, ping,
slow-threshold, service-name/scheme/discovery, observe.enabled — are in
[starter-gorm's USAGE](../starter-gorm/USAGE.md)):

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `user` | string | — | DSN user part. **Required** (no expr validation; empty user yields `":pass@..."` and a driver auth error). | Auth failure at startup ping. |
| `password` | string | — | DSN password part. **Required**, no URL-escaping applied (go-sql-driver requires percent-encoding of `:` `/` etc. — escape it yourself). | Auth failure; a password with special chars breaks parsing unless escaped. |
| `db` | string | — | Database name. **Required.** | `No database selected` on first query (ping passes — ping doesn't touch the db). |
| `addr` | string | — | `host:port` inside `tcp(...)`. Required **unless** `service-name` is set. ⚠ In discovery mode `addr` is dead — it is replaced by the service name before dialing. | Empty with no `service-name` → build error `one of addr or service-name must be set`. |
| `net` | string | "" | DSN network word; empty renders as `tcp`. ⚠ In discovery mode replaced by the dialer name — dead. Only meaningful value other than tcp is `unix` (addr = socket path). | Setting `net=udp` etc. → driver dial error. |
| `timeout` | duration | 0 | DSN `timeout=` (dial + auth bound), `time.Duration.String()` format. | 0 = driver default (OS-dependent); too low → intermittent dial failures. |
| `readTimeout` / `writeTimeout` | duration | 0 | DSN `readTimeout=` / `writeTimeout=`. | readTimeout < slowest legitimate query → `invalid connection` mid-read. |
| `charset` | string | "" | DSN `charset=`. | Empty = server default (often latin1 on old servers) → mojibake. |
| `parseTime` | bool | **false** | DSN `parseTime=true` only when set. ⚠ Classic gotcha: with the default false, scanning `DATETIME`/`TIMESTAMP` columns into `time.Time` fails (`unsupported Scan`); gorm models with time fields need `parseTime=true`. | Runtime scan errors on every time column. |
| `loc` | string | "" | DSN `loc=`, QueryEscaped (`Asia/Shanghai` → `Asia%2FShanghai`). Only meaningful with `parseTime=true`. | Unset = UTC; TIME_ZONE columns shift surprisingly. |
| `tls.*` | block | off | 6 shared `tlsconf` keys (`tls.enabled`, `tls.ca-file`, `tls.cert-file`, `tls.key-file`, `tls.server-name`, `tls.insecure-skip-verify`). When enabled, a `*tls.Config` is registered with the driver as `gstls_<n>` and the DSN gets `tls=gstls_<n>`. ⚠ The DSN's built-in `tls=true`/`skip-verify`/`preferred` short names are **not reachable** through config — the starter always uses its own registered config. Empty `ca-file` falls back to system roots. | Unreadable `ca-file` fails the build (fail-fast). `server-name` unset + addr-as-IP → cert hostname mismatch in verify modes. |

Dialect-specific ⚠ couplings: `parseTime`+`loc` travel together; `addr`+`net` are both dead under
`service-name`; `tls.enabled=true` requires the cert files it references to exist at bind time.

Config-audit (value tags in this starter): `user`, `password`, `net`, `addr`, `db`, `timeout`,
`readTimeout`, `writeTimeout`, `charset`, `parseTime`, `loc`, `tls` — all and only the table above.

---

## 4. Verification & fault drills

### 4.1 Baseline

```bash
curl -i 127.0.0.1:9370/readyz                       # both gorm:mysql:* indicators UP
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration   # per-op histogram
```

### 4.2 TLS

Point `tls.*` at real material and restart; the startup log line
`creating gorm mysql client, addr=... ` comes from `build` — a bad CA file fails *before* any dial.
Verify negotiation server-side: `SHOW STATUS LIKE 'Ssl_cipher'` returns non-empty for TLS sessions.

### 4.3 Discovery failover drill (deregister an endpoint)

1. Register two instances (point the second at another MySQL or the same one with a distinct key):

```bash
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/a '{"service_name":"mysql-cluster","addr":"127.0.0.1:3306"}'
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/b '{"service_name":"mysql-cluster","addr":"127.0.0.1:3307"}'
```

2. Start the app; both keys are in the resolver's endpoint set; every **new** connection dials one of
   them via `Resolver.Pick()`.
3. Deregister one endpoint:

```bash
ETCDCTL_API=3 etcdctl del /services/mysql-cluster/b
```

4. The etcd watch pushes a fresh snapshot; new dials only reach `a`. Already-open connections keep
   serving until closed — bound them with `conn-max-lifetime` (30m above) so failover lags at most
   that long. To observe immediately, set `conn-max-lifetime=30s` in the drill.
5. Watch the fire: `docker stop` the MySQL behind the deleted key — new dials succeed against the
   survivor (queries keep returning 200), the `db.client.operation.duration` metric keeps recording,
   and the access log shows no new dial errors after the watch settles.

### 4.4 parseTime drill

Remove `parseTime=true`, run a model with a `time.Time` field → every scan of that column errors with
`unsupported Scan, storing driver.Value type []uint8 into type *time.Time`. Restore the key.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Build error `one of addr or service-name must be set` | Neither key configured | Set `addr` or `service-name`. |
| Startup fails `gorm ping:` | Wrong addr/credentials, or DB down within `ping-timeout` | Fix config or raise `ping-timeout`; this is the fail-fast ping, not a runtime failure. |
| `unsupported Scan ... []uint8 ... time.Time` | `parseTime` default false | Set `parseTime=true` (+ `loc` if needed). |
| Discovery client dials the config `addr`, not the registry's endpoints | `service-name` unset — plain-addr path | Set `service-name` (+ `discovery` if not "default"). |
| Discovery dial error `no endpoints` / empty set | Backend name mismatch or no live keys | Check `spring.discovery.etcd.<name>` matches the `discovery` key; check etcd keys under the prefix. |
| Time values shifted by hours | `loc` unset (UTC default) with `parseTime=true` | Set `loc=Asia/Shanghai` (or your zone). |
| Connection dies mid-slow-query | `readTimeout` smaller than the query | Raise `readTimeout` or tune the query. |
| No spans/metrics per query | `observe.enabled=false`, or starter-otel not imported | Re-enable / import starter-otel (observe is a silent no-op without OTel globals). |
| TLS works locally, fails with hostname mismatch in prod | `tls.server-name` unset, addr is an IP | Set `tls.server-name` to the cert's CN. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 12 dialect-specific (+6 tls, +10 shared Common, +1 wrapper `observability`) |
| Required | 3 (`user`/`password`/`db`) + one of `addr`/`service-name` |
| Quickstart external deps | 1 (MySQL; +etcd for discovery, +Jaeger for traces) |
| "Watch out" entries | 6 (parseTime, password escaping, tls short names unreachable, dead addr/net in discovery, ping passes with bad db, loc coupling) |

Design suspects: `user`/`password`/`db` lack expr `Require` validation (empty values surface as
driver auth/`No database selected` errors instead of a config error at bind time); the driver's
built-in `tls=true`/`skip-verify` modes are unreachable without the full tlsconf block;
`parseTime=false` default contradicts the common gorm-with-time-fields setup.
