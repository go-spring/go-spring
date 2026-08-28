# starter-gorm-clickhouse Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `starter_test.go`) and the runnable
examples: [example/](example/), [example-load/](example-load/), [example-otel/](example-otel/)
(docker-gated `check.sh`). **ClickHouse connection semantics are the driver's** —
[ClickHouse/clickhouse-go/v2](https://github.com/ClickHouse/clickhouse-go/v2) via
[gorm.io/driver/clickhouse](https://github.com/go-gorm/clickhouse); GORM semantics are
[gorm's](https://gorm.io/docs/). Shared wrapper lifecycle, pool/observe/health wiring and
`UseDBCustomizer` live in [gormcore](../starter-gorm/USAGE.md) and are not repeated here.

**Activation**: one `*starter.DB` bean (plus a paired `health.Indicator`) per entry under
`spring.gorm.clickhouse`; zero entries → the starter registers nothing
(`starter_test.go:TestClickhouseNotTriggered`).

---

## 1. Complete worked project

A service with a direct-dial instance and a discovery-backed instance, both against a
single-node ClickHouse. File tree (mirrors [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── discovery.go
├── conf/
│   └── app.properties
├── docker-compose.yml   # from example/docker-compose.yml (clickhouse-server:24)
└── check.sh             # from example/check.sh
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-gorm-clickhouse latest
    gorm.io/gorm                       v1.31.x
)
```

**main.go** (trimmed from `example/example.go`) — note the ClickHouse-specific modeling:

```go
package main

import (
    "flag"
    "net/http"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    starter "go-spring.org/starter-gorm-clickhouse"
)

// ClickHouse does NOT enforce unique indexes like an OLTP engine — no
// `uniqueIndex` here. An explicit engine is required for AutoMigrate.
type KV struct {
    ID    uint64 `gorm:"primaryKey"`
    Key   string `gorm:"column:kkey"`
    Value string `gorm:"column:vvalue"`
}
func (KV) TableName() string { return "kv" }

type Service struct {
    DB          *starter.DB `autowire:"primary"`
    DiscoveryDB *starter.DB `autowire:"discovery"`
}

var manual = flag.Bool("manual", false, "keep the server up")

func main() {
    flag.Parse()
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    http.HandleFunc("/clickhouse_version", func(w http.ResponseWriter, r *http.Request) {
        s := svrBean.Interface().(*Service)
        var version string
        if err := s.DB.Raw("SELECT version()").Scan(&version).Error; err != nil {
            _, _ = w.Write([]byte(err.Error()))
            return
        }
        _, _ = w.Write([]byte(version))
    })
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(svrBean.Interface().(*Service))
        }()
    }
    gs.Run()
}

func runTest(s *Service) {
    // AutoMigrate needs an engine: MergeTree + ORDER BY via table options.
    _ = s.DB.Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)").AutoMigrate(&KV{})
    _ = s.DB.Create(&KV{ID: 1, Key: "key", Value: "value"}).Error
    // No multi-statement transactions in ClickHouse — batch insert + Count
    // instead of s.DB.Transaction(...), as the example demonstrates.
    batch := []KV{{ID: 2, Key: "k2", Value: "v2"}, {ID: 3, Key: "k3", Value: "v3"}}
    _ = s.DB.Create(&batch).Error
    var discVersion string
    _ = s.DiscoveryDB.Raw("SELECT version()").Scan(&discVersion).Error // proves discovery
    syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}
```

**discovery.go** — registers the backend the `discovery` instance resolves against:

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:9000", Healthy: true}))
}
```

**conf/app.properties** (copied from `example/conf/app.properties`):

```properties
spring.gorm.clickhouse.primary.user=default
spring.gorm.clickhouse.primary.password=
spring.gorm.clickhouse.primary.addr=127.0.0.1:9000        # native protocol port
spring.gorm.clickhouse.primary.db=default
spring.gorm.clickhouse.primary.max-open-conns=10
spring.gorm.clickhouse.primary.max-idle-conns=5
spring.gorm.clickhouse.primary.conn-max-lifetime=30m
spring.gorm.clickhouse.primary.conn-max-idle-time=5m
spring.gorm.clickhouse.primary.ping-timeout=5s
spring.gorm.clickhouse.primary.slow-threshold=200ms
# TLS (off here; smoke server is plaintext). To enable a secure native connection:
# spring.gorm.clickhouse.primary.tls.enabled=true
# spring.gorm.clickhouse.primary.tls.insecure-skip-verify=false
# spring.gorm.clickhouse.primary.tls.ca-file=/path/ca.pem
# spring.gorm.clickhouse.primary.tls.cert-file=/path/client-cert.pem   # mTLS supported
# spring.gorm.clickhouse.primary.tls.key-file=/path/client-key.pem

# Discovery instance: addr is a dummy on purpose — ignored because service-name
# is set; the address comes from the discovery backend.
spring.gorm.clickhouse.discovery.user=default
spring.gorm.clickhouse.discovery.password=
spring.gorm.clickhouse.discovery.addr=0.0.0.0:0
spring.gorm.clickhouse.discovery.db=default
spring.gorm.clickhouse.discovery.service-name=clickhouse-cluster
```

**docker-compose.yml** — ClickHouse 24 with both ports and a healthcheck:

```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:24
    ports: ["127.0.0.1:9000:9000", "127.0.0.1:8123:8123"]
    ulimits: { nofile: { soft: 262144, hard: 262144 } }
    healthcheck:
      test: ["CMD-SHELL", "wget --spider -q localhost:8123/ping || exit 1"]
      interval: 3s
      retries: 60
```

**Verify**:

```bash
docker compose up -d && wait-healthy     # or just ./check.sh
go run .                                  # "Response from server:" + "Response from discovered server:"
go run . -manual & curl http://127.0.0.1:9090/clickhouse_version
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gorm-clickhouse
  └─ init(): gormcore.Register(Dialect{Prefix: "spring.gorm.clickhouse",
        Engine: "clickhouse", HealthPrefix: "gorm:clickhouse:"})
gs.Run()
  ├─ conf.BindEach → Config per entry
  ├─ build()                                        — starter.go:61-131
  │    ├─ guard: addr or service-name must be set (fail fast)
  │    ├─ useDiscovery = service-name != "" && !mesh.Enabled()
  │    ├─ useNative = useDiscovery || TLS.Enabled
  │    │    native: ch.Options{Addr, Auth, Dial/ReadTimeout}
  │    │      + opts.TLS = tlsconf.Build()          (when TLS on)
  │    │      + opts.DialContext = resolver pick     (when discovery)
  │    │      → clickhouse.New(Config{Conn: ch.OpenDB(opts)})
  │    └─ plain path: clickhouse.Open(DSN)           (no TLS, no discovery)
  ├─ gormcore.Open: gorm.Open → ApplyPool (startup ping) → DBCustomizers
  ├─ DB.Init(): observe plugin (db.system=clickhouse) + resilience callbacks
  │            (resource "gorm:clickhouse" + service-name/addr)
  └─ SIGTERM → DB.Destroy(): executor → closers (stop the discovery watch) → pool close
```

Why two paths (source comment, starter.go:73-76): the URL-style DSN can express neither a
custom `*tls.Config` nor a discovery-backed dialer — so either forces the native
`ch.OpenDB` construction; otherwise the plain DSN path stays.

### 2.2 Discovery dial path — and the dummy-addr requirement

The native driver's `ch.Options.DialContext` is a 2-arg `func(ctx, addr string)` — the
starter's implementation **ignores the addr argument** and dials `ep.Addr` from
`resolver.Pick()` (starter.go:107-113), so every new pool connection lands on a
currently-live instance; address changes take effect without rebuilding the client. With
`service-name` set, `addr` is never dialed — but `build` still requires a non-empty
`addr` **or** `service-name`, and on the native path `opts.Addr = []string{c.Addr}` is
populated before `DialContext` overrides it: keep a parseable dummy (`0.0.0.0:0` in the
example) to prove discovery owns addressing. In mesh mode the configured Addr is used
as-is (sidecar owns discovery+LB), though TLS may still force the native path.

### 2.3 One query, layer by layer

`db.WithContext(ctx).Raw("SELECT version()").Scan(&v)`:

1. the `gorm:raw` processor — replaced by gormcore's executor wrapper (`${govern}`
   timeout/retry/breaker, fault injector when armed; `gorm.ErrRecordNotFound` = success).
2. observe plugin span (db.system=clickhouse) + in-flight metric.
3. the original processor: pool checkout → native DialContext (discovery re-pick) →
   native protocol handshake (TLS if `opts.TLS` set) → query.
4. `after_*` sets the SQL, ends span/metric, writes the access log.

---

## 3. Per-key behavior reference

Keys under `spring.gorm.clickhouse.<name>.*`. Common keys (10) and wrapper `observability`:
[gormcore](../starter-gorm/USAGE.md#2-configuration-reference).

### 3.1 Connection keys (config.go:31-45)

| Key | Type | Default | Behavior / coupling | Misconfiguration consequence |
|-----|------|---------|---------------------|------------------------------|
| `addr` | string | "" | host:port of the **native** protocol (typically 9000 — not the HTTP 8123 port). ⚠ One of `addr`/`service-name` mandatory (build guard). | HTTP port → handshake garbage / ping failure at startup. Both empty → "one of addr or service-name must be set". |
| `user` | string | "default" | DSN userinfo / `ch.Auth.Username`. | Wrong user → auth error at startup ping. |
| `password` | string | "" | DSN userinfo / `ch.Auth.Password`. | — |
| `db` | string | "default" | DSN path / `ch.Auth.Database`. | Unknown db → startup failure. |
| `dialTimeout` | duration | 0 | DSN `dial_timeout=2s` (Go duration string, not truncated) / `ch.Options.DialTimeout`. | Too low → dial timeouts on cold clusters. |
| `readTimeout` | duration | 0 | DSN `read_timeout=30s` / `ch.Options.ReadTimeout`. ⚠ Long analytical queries need this generous or unset. | Too low → big SELECTs aborted mid-read. |

### 3.2 TLS block (`tls.*`, shared tlsconf.TLSConfig at config.go:45)

Unlike sqlserver (DSN params), ClickHouse TLS is a real `*tls.Config`: enabling it **switches
the starter to the native driver path** (`useNative`, starter.go:78) and sets
`opts.TLS = c.TLS.Build()` — **no `secure_connection` / `https` DSN parameter is ever
emitted**. All six keys are live here:

| Key | Default | Behavior |
|-----|---------|----------|
| `tls.enabled` | false | On → native path + `opts.TLS`. An unreadable `ca-file` fails the build *before* any dial (starter_test.go pins this). |
| `tls.ca-file` | "" | Root CA bundle for server verification (`RootCAs`). |
| `tls.cert-file` + `tls.key-file` | "" | Client key pair — **mTLS is expressible** for this dialect (tlsconf `Build()` loads both). |
| `tls.server-name` | "" | Overrides the name checked against the server cert — useful when dialing by IP. |
| `tls.insecure-skip-verify` | false | Dev-only escape hatch. |

### 3.3 Discovery keys (from Common)

`service-name` (activates the resolver dialer), `scheme`, `discovery` (backend name,
default "default"). ⚠ With `service-name` set, `addr` becomes a dummy — still required
parseable for `opts.Addr`.

### 3.4 Dialect gotchas (ClickHouse is OLAP)

- No multi-statement transactions — `db.Transaction(...)` is not the tool; batch writes +
  async engine semantics are ([docs](https://clickhouse.com/docs)).
- No unique-index enforcement; `uniqueIndex` tags are decorative (example omits them).
- `AutoMigrate` needs an engine: pass `gorm:table_options` (e.g. `ENGINE=MergeTree
  ORDER BY (id)`), as the example does.

---

## 4. Verification & fault drills

### 4.1 Drill: TLS on/off

1. Baseline: run §1 as-is (plaintext 9000) — round trip OK.
2. Enable TLS without a TLS-enabled server:

```properties
spring.gorm.clickhouse.primary.tls.enabled=true
```

3. `go run .` → the native-path handshake hangs/fails and the **startup ping fails within
   `ping-timeout`** — proving TLS took effect (native path) rather than silently no-op'ing.
4. Against a TLS-enabled server (`docker run ... clickhouse/clickhouse-server:24` with
   `clickhouse-server-config` TLS, port 9440): set `addr=<host>:9440`, `tls.enabled=true`,
   plus `tls.ca-file`/`tls.server-name`; round trip returns. Add `cert-file`+`key-file` to
   require mTLS (all six keys live on this dialect).
5. Flip `tls.enabled=false` + `addr` back — plain DSN path reconnects (this also exercises
   the dual-path switch, §2.1).

### 4.2 Verify discovery addressing

The example's `discovery` instance uses dummy `0.0.0.0:0`; `Response from discovered
server:` proves the address came from the backend. Scale the backend endpoint and watch new
pool connections land on it (per-dial `Pick()`).

### 4.3 Resilience / fault drill (example-load)

```bash
cd example-load && docker compose up -d
go run . -duration=10s                        # baseline SELECT 1
# set fire (hot-reload via starter-governance):
#   govern.fault.enabled=true  govern.fault.rate=0.5  govern.fault.error=generic
go run . -duration=10s                        # ~50% injected in the breakdown
```

The op stays a pure read round trip on purpose — ClickHouse's column-store schema model
makes per-iteration inserts/migrations brittle (example-load comment).

### 4.4 Observability (example-otel)

Run `example-otel` (Jaeger via compose): per-query spans `db.system=clickhouse` with the
SQL statement, prometheus metrics on :9090/metrics, access log by `observability` level.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|------|--------------|------|
| Startup fails: "one of addr or service-name must be set" | both empty | set one (build guard, starter.go:62-64). |
| Handshake garbage / immediate ping failure | `addr` points at HTTP port 8123, not native 9000 | use the native port. |
| Startup ping fails right after enabling TLS | server not TLS-enabled on that port | §4.1: use 9440 + tls keys, or disable. |
| Big analytical queries abort mid-read | `readTimeout` too low | raise or leave 0. |
| `AutoMigrate` fails (missing ENGINE) | ClickHouse requires a table engine | `Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)")`. |
| Transaction code errors / no effect | ClickHouse has no multi-statement tx | batch writes; see dialect gotchas §3.4. |
| Duplicate rows despite uniqueIndex tag | uniqueIndex not enforced by ClickHouse | dedupe at write/model level. |
| Discovery instance never connects | backend name mismatch / service unregistered | check `RegisterDiscovery` name vs `discovery` key. |
| TLS works but cert hostname mismatch | dialing by IP | set `tls.server-name` (live on this dialect). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Dialect-specific keys | 6 conn + 6 tls = 12 (+10 Common, +1 wrapper) |
| Required | 0 hard (addr-or-service-name guarded at build) |
| Quickstart external deps | 1 (ClickHouse, docker) |
| Dead bound keys | 0 |
| "Watch out" entries | 5 |

Design suspects: dual construction paths (DSN vs native) mean TLS/discovery behavior
diverges from the DSN-only dialects — invisible in config; dummy `addr` still populates
`opts.Addr` on the discovery path; `readTimeout` semantics differ per path (DSN string vs
`ch.Options`) though both come from the same key.
