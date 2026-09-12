# starter-gorm (gormcore) Usage — Reference

Detailed usage reference for the shared scaffolding behind every go-spring gorm
dialect starter (mysql, postgres, sqlite, sqlserver, clickhouse). It is not
imported by applications directly — import a `starter-gorm-<dialect>` and
everything documented here comes along. All behavior claims are verified against
this module's source (`gorm.go`, `open.go`, `module.go`, `extension.go`,
`health.go`, `observe/plugin.go`, `resilience/callbacks.go`) and the runnable
examples under `starter-gorm-mysql/example*`. **GORM semantics (models,
associations, transactions, migrator) are [gorm's documentation](https://gorm.io/docs/)**
— everything below is go-spring's increment: assembly, config binding, observe,
resilience, health, teardown.

This doc is **the single shared reference**; each dialect USAGE documents only
its own DSN/TLS/discovery keys and points here for everything else.

---

## 1. Complete worked project

A realistic service using gorm over MySQL with health probes, metrics, tracing
and runtime fault injection. File tree:

```
demo/
├── go.mod
├── main.go
├── dao.go
├── conf/
│   ├── app.properties
│   └── govern.yaml
```

**go.mod** (module deps that matter):

```
require (
    gorm.io/gorm                latest
    gorm.io/driver/mysql        latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-gorm-mysql latest  // brings in starter-gorm (gormcore)
    go-spring.org/starter-actuator latest   // optional: probes + /metrics
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest // optional: runtime fault injection
)
```

**main.go**:

```go
package main

import (
    "demo/dao"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-gorm-mysql" // registers instances under spring.gorm.mysql.instances.*
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run(dao.NewRepository) }
```

**dao.go** — the application's entire data access surface:

```go
package dao

import (
    "context"

    gormcore "go-spring.org/starter-gorm"
    _ "go-spring.org/starter-gorm-mysql"
    "gorm.io/gorm"
)

// User is a demo model (gorm semantics: https://gorm.io/docs/models.html).
type User struct {
    ID    uint   `gorm:"primaryKey"`
    Email string `gorm:"size:128;uniqueIndex"`
    Name  string `gorm:"size:64"`
}

// Repository injects the "primary" instance by name. The bean type is the
// shared gormcore.DB (embeds *gorm.DB, so all gorm methods promote unchanged),
// the same type for every gorm dialect starter. A second instance ("replica")
// would be autowire:"mysql.replica"; each instance also contributes its own health
// indicator.
type Repository struct {
    DB *gormcore.DB `autowire:"mysql.primary"`
}

func NewRepository(r *Repository) { /* register as Rooter or provide HTTP handlers */ }

var _ = func() any {
    // Extension point (dialect-agnostic, shared by all gorm starters):
    // tweak every freshly-opened *gorm.DB before the bean is returned.
    // Must run in an init function, before the container wires.
    gormcore.UseDBCustomizer(func(db *gorm.DB) error {
        // e.g. prepared-statement caching, an extra Plugin, pool knobs the
        // config does not expose. First error fails the instance.
        return nil
    })
    return nil
}()

// PoolStats exposes runtime connection-pool numbers without OTel.
func (r *Repository) PoolStats() (open, inUse, idle int) {
    st, err := gormcore.Stats(r.DB)
    if err != nil {
        return 0, 0, 0
    }
    return st.OpenConnections, st.InUse, st.Idle
}

var _ = context.Background
```

**conf/app.properties** — the complete shared surface (dialect-specific
`user/password/addr/db/tls/...` keys are documented in the dialect USAGE):

```properties
# --- gorm mysql instance "primary" (dialect keys abbreviated) ---------------
spring.gorm.mysql.instances.primary.user=root
spring.gorm.mysql.instances.primary.password=123456
spring.gorm.mysql.instances.primary.addr=127.0.0.1:3306
spring.gorm.mysql.instances.primary.db=test

# Shared keys documented in THIS reference (Common block):
spring.gorm.mysql.instances.primary.max-open-conns=10
spring.gorm.mysql.instances.primary.max-idle-conns=5
spring.gorm.mysql.instances.primary.conn-max-lifetime=30m
spring.gorm.mysql.instances.primary.conn-max-idle-time=5m
spring.gorm.mysql.instances.primary.ping-timeout=5s
spring.gorm.mysql.instances.primary.slow-threshold=200ms

# --- actuator (aggregates the per-instance gorm health indicators) ----------
spring.http.server.enabled=false
spring.actuator.addr=:9370

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics via actuator only

# --- governance (runtime fault injection / breaker / retry) ------------------
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.error-threshold=20
govern.default.open-duration=5s
govern.default.max-retries=1
govern.default.timeout=500ms
```

External dependencies (docker):

```bash
docker run -d --name mysql -e MYSQL_ROOT_PASSWORD=123456 -e MYSQL_DATABASE=test -p 3306:3306 mysql:9
docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest
```

**Verify**:

```bash
go run .                                        # container starts; client pings at creation
curl -s :9370/readyz                            # UP, includes gorm:mysql:primary
curl -s :9370/metrics | grep db_client_operation_duration
# Jaeger UI: http://127.0.0.1:16686 — service "demo", span "query" per operation
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-gorm-mysql
  └─ init: gormcore.Module(Dialect[Config]{Prefix:"spring.gorm.mysql", ...})
        └─ gs.Module(gs.OnProperty("spring.gorm.mysql"))     [prefix check: any entry fires it]
gs.Run()
  ├─ config bind: conf.BindEach over ${spring.gorm.mysql} → one Config per <name>
  ├─ per instance <name>:
  │    ├─ Dialect.Build(ctx, c)      dialect builds DSN/dialector; resolves TLS,
  │    │                             service discovery (mysql only), resource label
  │    ├─ gormcore.Open:  gorm.Open → ApplyPool (pool knobs + fail-fast ping)
  │    │                  → ApplyDBCustomizers (user seam, registration order)
  │    ├─ Provide *DB .Name(<dialect>.<entry>).Init((*DB).Init).Destroy((*DB).Destroy)
  │    └─ Provide health.Indicator "gorm:mysql:<name>" (injects the *DB by name,
  │       exports as health.Indicator)  ← .Name is required: multi-instance beans
  │          share type (Indicator, *DB); without distinct names the container
  │          reports duplicate (Name,Type) keys
  ├─ bean wiring: gs assembles the *DB wrapper beans by name
  ├─ DB.Init: observe plugin (unless observe.enabled=false) → resilience
  │    executor chain → ApplyCallbacks replaces the six gorm processors
  ├─ Run / readiness: actuator aggregates the indicators → /readyz UP
  └─ SIGTERM: DB.Destroy → executor.Close → dialect closers (discovery watch,
     TLS deregistration) → underlying *sql.DB pool closed
```

Failure at any open step (dialect build, gorm.Open, ping, customizer) fails that
instance's creation and runs the dialect's closers — misconfigured address or
credentials surface at boot, not on first query.

### 2.2 The callback chain for ONE query — exact order and why

`db.Raw("SELECT ...")` (or First/Create/Update/Delete/Row) executes:

```
gorm:query processor chain
  1. go-spring:observe:before_query   span start + metric start + in-flight +1
  2. gorm:query  ← REPLACED by the resilience wrapper:
        resilience.Run(ctx, exec, "gorm:mysql:<addr>", op)
          ├─ admission (rate limit / bulkhead, if configured)
          ├─ fault injector (govern.fault.* — may short-circuit the attempt)
          ├─ timeout / breaker / retry envelope
          └─ the ORIGINAL gorm:query body: builds SQL, executes, applies
             gorm's own logger (slow-query warn, see slow-threshold)
  3. go-spring:observe:after_query    SetArg(SQL) → span end + duration metric
                                      + in-flight -1 + access log record
```

Rationale (from source comments, verified):

- **Observe hooks wrap the gorm processor, not the driver** (observe/plugin.go:44-50):
  the SQL statement is only known after gorm builds it, so the Before callback
  opens the observation for timing and the After callback attaches the SQL via
  SetArg before End — landing the statement in the detailed log and the span.
- **Correlation key is the *gorm.DB pointer** (observe/plugin.go:39-43): gorm hands
  the same fresh `*gorm.DB` to both callbacks, making it a unique per-operation key
  (`sync.Map`, no locks on the hot path beyond the map's).
- **Resilience replaces the processor itself** (resilience/callbacks.go:53-90): the
  same backend-neutral Executor other starters drive through a redis Hook or
  grpc interceptor is here driven through gorm's callback chain — one shared
  implementation instead of five per-dialect copies.
- **`gorm.ErrRecordNotFound` is success** (resilience/callbacks.go:29-30): "no rows"
  is a normal outcome, not a fault — it must not trip the breaker (the DB analog
  of redis.Nil).
- **Rejection propagation** (resilience/callbacks.go:76-83): a resilience rejection
  (rate-limited / circuit-open / bulkhead-full) or a fault-injected error is put
  on `tx.Error`; a real op error is left as gorm set it.
- **No-op by default**: with governance unconfigured, `resilience.ExecutorFor`
  returns a transparent executor — callbacks still wrap but add no behavior, so
  the chain costs nothing to leave installed.

### 2.3 Transactions

`db.Transaction(...)` runs on a session `*gorm.DB`; the replaced processors are
inherited by sessions, so every statement inside the transaction is individually
observed and guarded (the transaction as a whole is not a separate span).

---

## 3. Per-key behavior reference

All keys live at `spring.gorm.<dialect>.<name>.*` (embedded `Common`, bound at
the same level as the dialect's own fields). Reconciled against
`grep -rhoE 'value:"[^"]+"' starter/starter-gorm` — exactly these 10 shared keys.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `max-open-conns` | int | 0 | `>0` → `sql.DB.SetMaxOpenConns`; 0 = database/sql unlimited. | Too low under concurrency → `WaitCount` grows, queries queue (visible via `gormcore.Stats`). |
| `max-idle-conns` | int | 0 | `>0` → `SetMaxIdleConns`; 0 = database/sql default (2). | 0 with high QPS → constant reconnect churn; > max-open is clamped by database/sql. |
| `conn-max-lifetime` | duration | 0 | `>0` → `SetConnMaxLifetime`; 0 = unlimited. | 0 against a LB that silently drops idle conns → stale-connection errors mid-run. |
| `conn-max-idle-time` | duration | 0 | `>0` → `SetConnMaxIdleTime`; 0 = unlimited. | Mostly interacts with lifetime above. |
| `ping-timeout` | duration | 5s | Bounds the fail-fast startup ping (`PingContext`). `<=0` falls back to 5s. | Too small → boot failure on a cold/slow DB; huge → slow boot when the DB is down. |
| `slow-threshold` | duration | 0 | `>0` installs a warn-level gorm slow-query logger whose output is routed through **go-spring.org/log** (`log.Warnf`, TagAppDef) — it lands in the configured appenders, not raw stdout. 0 keeps gorm's default logger. ⚠ The message body is GORM's one-line text, not structured fields. | 0 → no slow log at all; expecting structured fields → the payload is plain text (use the access log for that). |
| `service-name` | string | — | Switches addressing to service discovery: the dialect binds a discovery-backed dialer so each new connection reaches a live instance. When set, `addr` is ignored (the example uses a dummy `0.0.0.0:0` to prove it). In mesh mode (`GS_MESH=on`) a sidecar owns discovery and `addr` is used as-is. | Unset + no `addr` → dialect build error ("one of addr or service-name must be set"). |
| `scheme` | string | — | Narrows discovery to endpoints of one transport scheme (e.g. `tls`). ⚠ Dead unless `service-name` is set (only consulted then). | Set without service-name → silently ignored. |
| `discovery` | string | — | Which registered discovery backend resolves `service-name`. ⚠ Dead unless `service-name` is set. | Unset or an unregistered name while service-name is set → boot error; set without service-name → silently ignored. |
| `observe.enabled` | bool | true | Hard kill switch for the gorm observe plugin: when false the plugin is not installed at all — no span, no metric, no access log, no per-query callbacks. | false → per-query observability silently absent (deliberate for hot instances). |

Dead-key notes: for **sqlite** the whole discovery trio (`service-name`/`scheme`/
`discovery`) is structurally unusable (no server to discover) but still binds —
recorded in the suspects list.

---

## 4. Verification & fault drills

### 4.1 Observe signals (per query)

With starter-otel imported (see §1 config):

- **Span**: one per operation, named by kind (`query`/`create`/`update`/`delete`),
  attribute `db.system=mysql`, SQL statement attached; check Jaeger
  (`http://127.0.0.1:16686`, service = your `spring.observability.service-name`).
- **Metrics**: histogram `db.client.operation.duration`, attributes `db.system`,
  `db.operation`, status:

```bash
curl -s :9370/metrics | grep -E 'db_client_operation_duration'
```

- **Access log**: tag `_app_gorm_access`, one structured record per operation —
  system, operation, status, duration, error, and the SQL statement (truncated
  at 512 bytes) once gorm has built it. Severity: error → Warn, plain success →
  Info, success with SQL → Debug.

```bash
go run . 2>&1 | grep _app_gorm_access
```

### 4.2 Slow-log drill

1. Configure `slow-threshold=200ms` (as in §1).
2. Trigger a slow query (must run as its own statement):

```bash
# via your app's SQL surface, or add a debug handler executing:
#   db.Raw("SELECT SLEEP(1)").Scan(&x)
```

3. Expect a gorm slow-query warn line via **go-spring.org/log** (TagAppDef, plain-text
   message body) after ~1s. The same query also shows in the access log /
   duration histogram regardless of slow-threshold.

### 4.3 Health flip drill (stop the DB → readiness goes down)

```bash
go run . -manual &                      # or your long-running mode
curl -s :9370/readyz                    # 200 UP — includes gorm:mysql:primary
docker stop mysql                       # kill the database
curl -s :9370/readyz                    # 503 DOWN — the per-instance indicator
                                        # (pool PingContext) fails
docker start mysql && sleep 3
curl -s :9370/readyz                    # recovers to 200 once the DB is back
```

The indicator name is `gorm:<dialect>:<name>` (e.g. `gorm:mysql:primary`);
`/health` on the actuator lists the component detail. Rejections here do NOT
trip the resilience breaker — the health ping does not go through the callback
chain (it calls `sqlDB.PingContext` directly, health.go:31-38).

### 4.4 Fault / resilience drill (no restart)

Using the governance config from §1 plus a file source (see starter-governance)
or the example-load layout (`starter-gorm-mysql/example-load`):

1. Start the app with `govern.fault.enabled=false`; baseline queries succeed.
2. Flip `govern.fault.enabled=true` (+ `rate`, `error`) — the governance source
   hot-reloads.
3. Faulted attempts short-circuit before the SQL runs: the injected error lands
   on `tx.Error`, the breaker counts it, and the observe layer still records the
   failed operation — you can watch the fire in the access log and the
   `db.client.operation.duration` error-status buckets.
4. With `error-threshold=20`, sustained fire opens the circuit: further queries
   fail fast with the circuit-open rejection instead of hitting the DB.
   `open-duration=5s` later it half-opens and probes.
5. Flip back to false to extinguish (reset the breaker via the governance
   surface if needed).

### 4.5 Connection-pool verification

`gormcore.Stats(db)` returns `sql.DBStats` without OTel — expose it on a debug
endpoint and watch `OpenConnections`/`InUse`/`WaitCount` under load
(the `example-load` harness drives exactly this loop).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "gorm ping: ..." | DB unreachable/credentials wrong within `ping-timeout` | Fix addr/credentials; raise `ping-timeout` for cold starts. Deliberate fail-fast, not a bug. |
| No beans register at all | No `spring.gorm.<dialect>.*` entries — `OnProperty(prefix)` never fires | Add at least one instance block; nothing activates by default. |
| Container fails: duplicate beans | A second Provide of `*DB`/`health.Indicator` without `.Name` | Don't re-provide DB beans yourself; the DB beans are named `<dialect>.<entry>` (e.g. `mysql.primary`) and the health indicators `gorm:<dialect>:<entry>` (module.go:85-97). |
| Injection error "not a simple value"/type mismatch | Injecting `*gorm.DB` instead of the wrapper | Autowire the shared `*gormcore.DB` bean; it embeds `*gorm.DB`. |
| No spans/metrics/access log | starter-otel not imported, or `observe.enabled=false` | Import starter-otel; check the per-instance kill switch — it removes the plugin entirely. |
| Slow-query lines are plain text | `slow-threshold` routes GORM's warn output through go-spring.org/log, but the message body is GORM's one-line text | Filter by message; for structured slow logs use the access log instead. |
| Queries rejected with rate-limited/circuit-open errors | Governance resilience engaged (or fault fired) | Intended protection; check `govern.*` config and the fault drill steps (§4.4). |
| Stale connection errors after hours | LB/firewall dropping idle TCP; `conn-max-lifetime=0` | Set `conn-max-lifetime` below the infrastructure's idle cut. |
| `breakers trip on legitimate "not found"` — they don't | `gorm.ErrRecordNotFound` treated as success | By design (callbacks.go:29-30); only real errors feed the breaker. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Shared config keys | 10 (Common) |
| Required | 0 here (dialects own their required keys, e.g. mysql `user`/`db`) |
| Quickstart external deps | 1 (a database; +1 collector for full observability) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger): slow-threshold logger carries GORM's
plain-text message body (routed via go-spring.org/log since 2026-08, no longer
stdlib stdout); `Common` no longer carries the discovery trio for sqlite —
the sqlite starter embeds only `PoolSettings`; per-operation span but no
transaction-level span (correlation of a transaction's statements is by context
only).
