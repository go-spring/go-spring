# starter-gorm-sqlserver Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `starter_test.go`) and the runnable
examples: [example/](example/), [example-load/](example-load/), [example-otel/](example-otel/)
(docker-gated `check.sh`). **SQL Server connection semantics are the driver's** —
[microsoft/go-mssqldb](https://github.com/microsoft/go-mssqldb) (via
[gorm.io/driver/sqlserver](https://gorm.io/docs/connecting_to_the_database#SQL-Server));
GORM semantics are [gorm's](https://gorm.io/docs/). Shared wrapper lifecycle, pool/observe/
health wiring and `UseDBCustomizer` live in [gormcore](../starter-gorm/USAGE.md) and are
not repeated here.

**Activation**: one `*starter.DB` bean (plus a paired `health.Indicator`) per entry under
`spring.gorm.sqlserver`; zero entries → the starter registers nothing
(`starter_test.go:TestSqlserverNotTriggered`).

---

## 1. Complete worked project

A service with a direct-dial instance, plus a discovery-backed instance whose address comes
from a registered discovery backend. File tree (mirrors [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── discovery.go
├── conf/
│   └── app.properties
├── docker-compose.yml   # from example/docker-compose.yml
└── check.sh             # from example/check.sh
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-gorm-sqlserver latest
    gorm.io/gorm                     v1.31.x
)
```

**main.go** (trimmed from `example/example.go`):

```go
package main

import (
    "flag"
    "net/http"
    "os"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    starter "go-spring.org/starter-gorm-sqlserver"
    "gorm.io/gorm"
)

// `key`/`value` are reserved words in SQL Server — remap the columns.
type KV struct {
    ID    uint   `gorm:"primaryKey"`
    Key   string `gorm:"column:kkey;size:64;uniqueIndex"`
    Value string `gorm:"column:vvalue;size:255"`
}

type Service struct {
    DB          *starter.DB `autowire:"primary"`
    DiscoveryDB *starter.DB `autowire:"discovery"`
}

var manual = flag.Bool("manual", false, "keep the server up")

func main() {
    flag.Parse()
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    http.HandleFunc("/sqlserver_version", func(w http.ResponseWriter, r *http.Request) {
        s := svrBean.Interface().(*Service)
        var version string
        if err := s.DB.Raw("SELECT @@VERSION").Scan(&version).Error; err != nil {
            _, _ = w.Write([]byte(err.Error()))
            return
        }
        _, _ = w.Write([]byte(version))
    })
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(svrBean.Interface().(*Service))  // migrate/CRUD/tx + discovery round trip
        }()
    }
    gs.Run()
}
```

**discovery.go** — registers the discovery backend the `discovery` instance resolves
against (in a real deployment this is your company's adapter):

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(    discovery.Endpoint{Addr: "127.0.0.1:1433", Healthy: true}), nil
    }).Name("default")
}
```

**conf/app.properties** (copied from `example/conf/app.properties`):

```properties
spring.gorm.sqlserver.primary.user=sa
spring.gorm.sqlserver.primary.password=Str0ng!Passw0rd
spring.gorm.sqlserver.primary.host=127.0.0.1
spring.gorm.sqlserver.primary.port=1433
spring.gorm.sqlserver.primary.db=master
# pool / ping / slow log
spring.gorm.sqlserver.primary.dialTimeout=5s
spring.gorm.sqlserver.primary.connectTimeout=10s
spring.gorm.sqlserver.primary.max-open-conns=10
spring.gorm.sqlserver.primary.max-idle-conns=5
spring.gorm.sqlserver.primary.conn-max-lifetime=30m
spring.gorm.sqlserver.primary.conn-max-idle-time=5m
spring.gorm.sqlserver.primary.ping-timeout=5s
spring.gorm.sqlserver.primary.slow-threshold=200ms
# TLS (off here). To enable encryption:
# spring.gorm.sqlserver.primary.tls.enabled=true
# spring.gorm.sqlserver.primary.tls.insecure-skip-verify=true
# spring.gorm.sqlserver.primary.tls.ca-file=/path/server-cert.pem

# Discovery instance: host/port are dummies on purpose — ignored because
# service-name is set; the address comes from the discovery backend.
spring.gorm.sqlserver.discovery.user=sa
spring.gorm.sqlserver.discovery.password=Str0ng!Passw0rd
spring.gorm.sqlserver.discovery.host=0.0.0.0
spring.gorm.sqlserver.discovery.port=0
spring.gorm.sqlserver.discovery.db=master
spring.gorm.sqlserver.discovery.service-name=sqlserver-cluster
```

**docker-compose.yml** — SQL Server 2022 with a healthcheck (startup is slow; the port
opens well before the server can serve queries):

```yaml
services:
  sqlserver:
    image: mcr.microsoft.com/mssql/server:2022-latest
    ports: ["127.0.0.1:1433:1433"]
    environment: { ACCEPT_EULA: "Y", MSSQL_SA_PASSWORD: "Str0ng!Passw0rd" }
    healthcheck:
      test: ["CMD-SHELL", >-
        /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "Str0ng!Passw0rd" -No -Q "SELECT 1"
        || /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P "Str0ng!Passw0rd" -Q "SELECT 1"]
      interval: 5s
      retries: 40
      start_period: 30s
```

**Verify**:

```bash
docker compose up -d && wait-healthy     # or just run ./check.sh (does all of it)
go run .                                  # prints "Response from server:" + "Response from discovered server:"
go run . -manual &                        # then:
curl http://127.0.0.1:9090/sqlserver_version
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gorm-sqlserver
  └─ init(): gormcore.Register(Dialect{Prefix: "spring.gorm.sqlserver",
        Engine: "microsoft.sql_server", HealthPrefix: "gorm:sqlserver:"})
gs.Run()
  ├─ conf.BindEach → Config per entry
  ├─ build()                                        — starter.go:63-107
  │    ├─ guard: host or service-name must be set (fail fast)
  │    ├─ c.NewResolver(ctx)                        — nil when no service-name or mesh on
  │    ├─ discovery path: msdsn.Parse(DSN) → mssql.NewConnectorConfig
  │    │    → connector.Dialer = resolverDialer     — every dial re-picks a live endpoint
  │    │    → sqlserver.New(Config{Conn: sql.OpenDB(connector)})
  │    └─ plain path: sqlserver.Open(DSN)
  ├─ gormcore.Open: gorm.Open → ApplyPool (startup ping) → DBCustomizers
  ├─ DB.Init(): observe plugin (db.system=microsoft.sql_server) + resilience callbacks
  │            (resource "gorm:sqlserver" + service-name/host)
  ├─ run; the resolver's background watch keeps the endpoint set fresh
  └─ SIGTERM → DB.Destroy(): executor → closers (stop the discovery watch) → pool close
```

### 2.2 Discovery dial path — and the dummy-addr requirement

The adapter is **not** `database/sql`'s `RegisterDialContext`; go-mssqldb exposes a
dedicated hook: `mssql.NewConnectorConfig(msdsn.Parse(dsn))` gives a Connector whose
`Dialer` field accepts any `mssql.Dialer`. The starter sets
`resolverDialer{r: resolver, nd: &net.Dialer{}}`; its `DialContext` **ignores the network
and addr arguments** and dials `ep.Addr` from the round-robin pick pool (starter.go:124-130) — so
each new pool connection lands on a currently-live instance and address changes take effect
without rebuilding the client.

Consequence: with `service-name` set, `host`/`port` are never dialed — but they are **not
optional either**: the DSN still embeds them (`sqlserver://...@host:port?...`) and must
parse through `msdsn.Parse`. The example sets the deliberate dummies `0.0.0.0`/`0` to prove
the address really comes from discovery (example/conf/app.properties, `discovery` block).
In mesh mode (`NewResolver` returns nil) the configured Host is used as-is — the sidecar
owns discovery+LB.

### 2.3 One query, layer by layer

`db.WithContext(ctx).Raw("SELECT @@VERSION").Scan(&v)`:

1. the `gorm:raw` processor — replaced by gormcore's executor wrapper (`${govern}`
   timeout/retry/breaker, fault injector when armed; `gorm.ErrRecordNotFound` = success).
2. observe plugin `before_raw`-anchored span (db.system=microsoft.sql_server) + metric.
3. the original processor: pool checkout → resolverDialer.DialContext (discovery
   instances) or the driver's plain dial → TDS login (encrypt per DSN, §3.2) → query.
4. `after_*` sets the SQL, ends span/metric, writes the access log.

---

## 3. Per-key behavior reference

Keys under `spring.gorm.sqlserver.<name>.*`. Common keys (10) and wrapper `observability`:
[gormcore](../starter-gorm/USAGE.md#2-configuration-reference).

### 3.1 Connection keys (config.go:32-45)

| Key | Type | Default | Behavior / coupling | Misconfiguration consequence |
|-----|------|---------|---------------------|------------------------------|
| `user` / `password` | string | — **required** (no default in tag) | Query-escaped into the DSN userinfo. | Missing → bind error at startup. |
| `host` | string | "" | Dialed when no `service-name`. ⚠ One of `host`/`service-name` is mandatory (build guard, starter.go:64-66). | Both empty → instance fails fast with "one of host or service-name must be set". |
| `port` | string | 1433 | String, appended after `:`. | Wrong port → startup ping fails within `ping-timeout`. |
| `db` | string | — **required** | DSN `database=` parameter. | Missing → bind error. |
| `dialTimeout` | duration | 0 | → DSN `dial+timeout=<seconds>` (integer seconds; sub-second values **round up** to 1s — truncation to 0 would mean "no timeout"). 0 = driver default. | Too low → intermittent dial timeouts on loaded networks. |
| `connectTimeout` | duration | 0 | → DSN `connection+timeout=<seconds>` (same round-up rule). 0 = driver default. SQL Server has no DSN read/write timeout — per-op deadlines go through `WithContext` + governance. | Too low → login timeouts under load. |

### 3.2 TLS block (`tls.*`, a SQL Server-local subset — config.go)

The mapping is onto **DSN parameters**, not a `*tls.Config`. Only the three keys the DSN
can express are bound; the wider shared tlsconf block's `cert-file`/`key-file`/`server-name`
were **removed** (2026-08) — they had no DSN slot and were silently ignored:

| Key | Default | Maps to | Note |
|-----|---------|---------|------|
| `tls.enabled` | false | `encrypt=true` | Off → no `encrypt` param emitted; the go-mssqldb default applies (see [its docs](https://github.com/microsoft/go-mssqldb#connection-parameters)). |
| `tls.insecure-skip-verify` | false | `TrustServerCertificate=true` | Only emitted when `encrypt=true` (nested under Enabled, config.go:81-83). |
| `tls.ca-file` | "" | `certificate=<url-escaped path>` | A PEM server certificate path; verified by `starter_test.go` (`certificate=%2Fca.pem`). |
Removed keys: `tls.cert-file`, `tls.key-file`, `tls.server-name` — the sqlserver Config now binds its own three-field TLS block (enabled / insecure-skip-verify / ca-file). Setting them now fails with an unknown-key error. mTLS remains inexpressible through the DSN — use a custom connector if needed.

### 3.3 Discovery keys (from Common)

`service-name` (activates the resolver dialer), `scheme`, `discovery` (backend name,
default "default") — see gormcore. ⚠ When `service-name` is set, keep `host`/`port` at
parseable dummies; they are ignored but still embedded in the DSN that `msdsn.Parse`
validates.

---

## 4. Verification & fault drills

### 4.1 Drill: TLS handshake failure (encrypt against a plaintext/untrusted server)

1. Start the compose SQL Server (its self-signed cert is untrusted by your host).
2. Enable encryption without trusting it:

```properties
spring.gorm.sqlserver.primary.tls.enabled=true
```

3. `go run .` → startup ping fails within `ping-timeout` with a TLS trust error
   (certificate signed by unknown authority).
4. Add trust (either fixes it):

```properties
spring.gorm.sqlserver.primary.tls.insecure-skip-verify=true   # dev only
# or: tls.ca-file=/path/to/server-cert.pem
```

5. Flip `tls.enabled=false` again — plain TDS reconnects. This drills both directions of
   the encrypt mapping.

### 4.2 Verify discovery addressing

The example's `discovery` instance uses dummy `0.0.0.0:0`; a successful
`Response from discovered server:` line proves the dialed address came from the backend.
With a live backend, scale the service and watch new pool connections land on the new
instance (per-dial `Pick()`).

### 4.3 Resilience / fault drill (example-load)

```bash
cd example-load && docker compose up -d
go run . -duration=10s                       # baseline SELECT 1 throughput
# set fire — edit conf/app.properties (hot-reload via starter-governance):
#   govern.fault.enabled=true  govern.fault.rate=0.5  govern.fault.error=generic
go run . -duration=10s                       # error breakdown shows ~50% injected
```

The breaker (`govern.default.error-threshold=20`) trips visible in the same breakdown;
`WithContext` threads the harness deadline so the 500 ms timeout can interrupt in-flight
queries.

### 4.4 Observability (example-otel)

Run `example-otel` (Jaeger via its compose): per-query spans `db.system=microsoft.sql_server`
with the SQL statement, duration/in-flight metrics on :9090/metrics, access log by
`observability` level. `slow-threshold=200ms` additionally swaps in gorm's slow-query
logger (routed through go-spring.org/log, TagAppDef, plain-text body).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails: "one of host or service-name must be set" | both empty | set one (build guard, starter.go:64-66). |
| Startup ping timeout against a healthy host | server still initializing (mssql image is slow) | rely on the compose healthcheck, not the open port. |
| TLS trust error at startup | `tls.enabled=true` without `insecure-skip-verify`/`ca-file` | add trust or disable encrypt (§4.1). |
| Expecting mTLS, client never presents a cert | removed `tls.cert-file`/`key-file` | not expressible via config; needs a custom connector. |
| Cert hostname mismatch with discovery dummies | no `tls.server-name` key; dummy `0.0.0.0` in DSN | one of: real hostnames in the cert, `insecure-skip-verify` (dev), or a custom dialer. |
| Discovery instance never connects | backend label mismatch (`discovery` key) or service not registered | check the bean name vs config; resolver errors log at startup. |
| Login timeouts under load | `connectTimeout` too low | raise, or leave 0 for the driver default. |
| `key`/`value` column SQL errors | reserved words in SQL Server | remap via gorm tags (`column:kkey`), as the example does. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Dialect-specific keys | 7 conn + 6 tls = 13 (+10 Common, +1 wrapper) |
| Required | 3 (`user`, `password`, `db`) + host-or-service-name |
| Quickstart external deps | 1 (SQL Server, docker) |
| Dead bound keys | 0 (unused tlsconf keys removed 2026-08) |
| "Watch out" entries | 5 |

Design suspects: dummy host/port still required for DSN parsing when
discovery owns addressing; no path to a `*tls.Config`-based connector for mTLS (the
shared-block-vs-dialect mismatch and the sub-second truncation were fixed 2026-08).
