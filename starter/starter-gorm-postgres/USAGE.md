# starter-gorm-postgres Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Anchored to [example/](example/),
[example-health/](example-health/), [example-load/](example-load/), [example-otel/](example-otel/) and
[example-cloudnative/](example-cloudnative/). **GORM semantics are [gorm's](https://gorm.io/docs/);
PostgreSQL connection-string semantics are [pgx's](https://github.com/jackc/pgx/blob/master/connstring.go)
(libpq-compatible, incl. `sslmode`)** — this doc covers the binding surface and the go-spring increment
(wiring, discovery, TLS, observe, health).

**Activation**: one client bean per `spring.gorm.postgres.<name>` entry (`OnProperty` prefix check on
`spring.gorm.postgres`). No `enabled` key. Shared lifecycle (wrapper, pool, observe plugin, health,
`UseDBCustomizer`, the 10 `Common` keys) is documented in
[starter-gorm's USAGE](../starter-gorm/USAGE.md) and not repeated here.

---

## 1. Complete worked project

A service with a fixed-host `primary` instance, actuator health and OTel observability. File tree:

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
    go-spring.org/spring                 v1.3.4
    go-spring.org/starter-gorm-postgres  latest
    go-spring.org/starter-actuator       latest   // /health + /readyz
    go-spring.org/starter-otel           latest   // trace/metric export
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
    _ "go-spring.org/starter-gorm-postgres"
)

func main() {
    gs.Provide(dao.NewService).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**dao/dao.go** — inject by instance name:

```go
package dao

import (
    StarterPostgres "go-spring.org/starter-gorm-postgres"
)

type Service struct {
    DB *StarterPostgres.DB `autowire:"primary"`
}

func NewService() *Service { return &Service{} }

func (s *Service) Init() error { // gs InitMethod: table + smoke query
    if err := s.DB.AutoMigrate(&KV{}); err != nil {
        return err
    }
    return s.DB.Exec("SELECT 1").Error
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- primary: fixed host ------------------------------------------------------
spring.gorm.postgres.primary.host=127.0.0.1
spring.gorm.postgres.primary.port=5432
spring.gorm.postgres.primary.user=postgres
spring.gorm.postgres.primary.password=123456
spring.gorm.postgres.primary.db=test
spring.gorm.postgres.primary.sslmode=disable      # smoke DB is plaintext
spring.gorm.postgres.primary.max-open-conns=10
spring.gorm.postgres.primary.max-idle-conns=5
spring.gorm.postgres.primary.conn-max-lifetime=30m
spring.gorm.postgres.primary.slow-threshold=200ms
# TLS via sslmode + cert material (commented; see §3):
# spring.gorm.postgres.primary.sslmode=verify-full
# spring.gorm.postgres.primary.sslrootcert=/path/ca.pem
# spring.gorm.postgres.primary.sslcert=/path/client-cert.pem
# spring.gorm.postgres.primary.sslkey=/path/client-key.pem

# --- actuator (aggregates gorm:postgres:<name> indicators) ---------------------
spring.actuator.addr=:9370

# --- observability --------------------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
```

**Bring up and verify**:

```bash
docker run -d --name postgres -p 127.0.0.1:5432:5432 \
  -e POSTGRES_PASSWORD=123456 -e POSTGRES_DB=test postgres:16
# Jaeger all-in-one for traces (optional):
docker run -d --name jaeger -p 127.0.0.1:16686:16686 -p 127.0.0.1:4317:4317 jaegertracing/all-in-one
go run .

curl -i 127.0.0.1:9370/readyz                 # UP: gorm:postgres:primary
curl -i 127.0.0.1:9370/health
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
curl -s '127.0.0.1:16686/api/traces?service=demo&limit=1' | grep '"data":\['
```

---

## 2. Assembly & timing

```
import starter-gorm-postgres
  └─ init: gormcore.Register(Dialect{Prefix: "spring.gorm.postgres", Engine: "postgresql", ...})
gs.Run()
  ├─ OnProperty("spring.gorm.postgres") fires; conf.BindEach binds one Config per <name> entry
  ├─ per instance: build(ctx, c)
  │    ├─ host/service-name presence check (one must be set)
  │    ├─ DSN(): "host=.. port=.. user=.. password=.. dbname=.. sslmode=.." (+ optional parts)
  │    └─ discovery (service-name set, mesh off): pgx.ParseConfig(DSN) once, then the pgx
  │         DialFunc is replaced with a pool-backed round-robin dialer; plain mode uses the
  │         DSN directly
  ├─ gormcore.Open: gorm.Open -> ApplyPool (incl. startup ping, ping-timeout bound)
  │    -> ApplyDBCustomizers -> *DB bean (Name=<name>, Init, Destroy)
  ├─ health indicator "gorm:postgres:<name>" exported as health.Indicator
  ├─ DB.Init (after Observability field-inject): observe plugin (db.system=postgresql)
  │    + resilience/fault executor callbacks on resource "gorm:postgresql:<host|service-name>"
  └─ DB.Destroy (SIGTERM): executor close -> discovery watch stop -> pool close
```

**One query, layer by layer** (starter-otel imported, `observe.enabled=true`):
`db.WithContext(ctx).Exec("SELECT 1")` → resilience callbacks wrap the processor → the gorm observe
plugin's before/after callbacks open a span (`db.system=postgresql`), record
`db.client.operation.duration`, emit an access-log record (level from the wrapper `observability`
field, default `brief`) → database/sql takes a pooled conn (a new physical conn dials through the
replaced pgx `DialFunc` in discovery mode) → pgx executes.

**Why pgx `DialFunc` and not a DSN rewrite**: unlike the mysql driver (custom dial network names),
pgx exposes the dial hook directly on its parsed config. `build` calls `pgx.ParseConfig` once and
swaps `DialFunc` for a closure that ignores the network/addr arguments and dials
the pick pool's endpoint over TCP. Consequence: in discovery mode `host`/`port` never reach the
network, but they **must still parse as a valid pgx DSN** — `ParseConfig` runs before the swap, so a
non-numeric or out-of-range `port` fails the build even though the value is never dialed. (The
example uses `host=0.0.0.0 port=5432` as plausible dummies.)

---

## 3. Per-key behavior reference

PostgreSQL-specific keys under `spring.gorm.postgres.<name>.*` (the 10 shared `Common` keys are in
[starter-gorm's USAGE](../starter-gorm/USAGE.md)):

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `host` | string | — | DSN `host=`. Required **unless** `service-name` is set. ⚠ In discovery mode it is dead (never dialed) but still parsed. | Neither host nor service-name → build error `one of host or service-name must be set`. |
| `port` | string | `5432` | DSN `port=`. ⚠ Discovery mode: never dialed but must parse as a valid TCP port (`pgx.ParseConfig` runs first). | Non-numeric port fails the build even in discovery mode. |
| `user` | string | — | DSN `user=`. **Required** (no expr validation). | Startup ping auth failure. |
| `password` | string | — | DSN `password=`. **Required.** Values containing spaces break the space-separated DSN — no quoting/escaping is applied. | Auth failure, or DSN parse error for spaced passwords. |
| `db` | string | — | DSN `dbname=`. **Required.** | Ping passes; first query fails (`does not exist` / wrong db). |
| `sslmode` | string | `disable` | DSN `sslmode=` — the primary TLS switch (libpq values: `disable`/`allow`/`prefer`/`require`/`verify-ca`/`verify-full`; semantics per [pgx/libpq](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNECT-SSLMODE)). A `tls.*` block (added 2026-08, symmetric with mysql) injects a `*tls.Config` into the pgx connection; `sslmode` still decides whether TLS negotiates — `tls.enabled=true` with `sslmode=disable` is rejected at startup. | Left at `disable` against a TLS-only server → connection refused; `tls.enabled` + `sslmode=disable` → loud startup error. |
| `sslrootcert` | string | "" | DSN `sslrootcert=` (PEM CA path). Meaningful for `verify-ca`/`verify-full`. | Unreadable path → pgx connection error at first use. |
| `sslcert` / `sslkey` | string | "" | DSN `sslcert=` / `sslkey=` (client-cert pair) for mTLS auth. ⚠ Supplying them while `sslmode=disable` does nothing — dead keys until the mode demands TLS. | Pair set without sslmode bump → silently plaintext. |
| `timezone` | string | "" | DSN `TimeZone=` (literal capital-T parameter, as the gorm postgres driver expects). Sets the session time zone. | Unset = server default. |
| `connectTimeout` | duration | 0 | DSN `connect_timeout=` in whole **seconds** (rounded **up**: 500ms → 1, 2500ms → 3 — added 2026-08; truncation would turn sub-second into 0 = unbounded). | 0 = no explicit bound (OS default). |

Dialect-specific ⚠ couplings: `sslmode` gates `sslrootcert`/`sslcert`/`sslkey`; `tls.enabled` + `sslmode=disable` → startup error; discovery mode kills
`host`/`port` dialing but not their parsing; DSN values with spaces are unescapable through these keys.

Config-audit (value tags in this starter): `host`, `port`, `user`, `password`, `db`, `sslmode`,
`timezone`, `connectTimeout`, `sslrootcert`, `sslcert`, `sslkey`, `tls.*` — all and only the table above.

---

## 4. Verification & fault drills

### 4.1 Baseline

```bash
curl -i 127.0.0.1:9370/readyz                       # gorm:postgres:primary indicator UP
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
```

### 4.2 TLS drill

1. Point `sslmode=verify-full` + `sslrootcert` at your CA and restart; the startup ping
   (`ping-timeout` bound) fails fast if negotiation fails.
2. Verify server-side: `SELECT ssl, version FROM pg_stat_ssl WHERE pid = pg_backend_pid();`
   returns `ssl | TLSv1.3`.
3. Negative check: flip `sslmode=disable` against a TLS-only server → startup fails with a
   connection error from the fail-fast ping (not a runtime surprise later).

### 4.3 Wrong-db drill

Remove `db` (or typo it): the app still starts — the startup ping doesn't touch the database —
then `AutoMigrate` in `Init` fails with `database "x" does not exist`. Keep `db` in the required set.

### 4.4 Discovery failover (outline)

Mirror the mysql starter's drill (§4.3 of its USAGE): add an instance with
`service-name=postgres-cluster` + an etcd discovery backend; register
`/services/postgres-cluster/<id>` keys with `{"service_name":"postgres-cluster","addr":"host:5432"}`;
`etcdctl del` one key → the watch pushes a fresh snapshot → new physical connections (bounded by
`conn-max-lifetime`) dial the survivor only. Distinguish servers with
`SELECT inet_server_addr()` in the drill's queries.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Build error `one of host or service-name must be set` | Neither key configured | Set `host` or `service-name`. |
| Build fails in discovery mode with a port/DSN parse error | Dummy `port` not a valid port (`pgx.ParseConfig` runs before the dial swap) | Keep a plausible `port=5432`. |
| Startup fails `gorm ping:` | Wrong host/credentials, server down, or TLS mismatch | Fix config; check `sslmode` vs server posture; `ping-timeout` only bounds the wait. |
| Server requires TLS, connection refused | `sslmode=disable` default | Set `sslmode=require` (or a verify mode + `sslrootcert`). |
| `sslcert`/`sslkey` set but connection still plaintext | `sslmode` still `disable` — the keys are dead until the mode demands TLS | Raise `sslmode`. |
| Starts fine, first query fails `database ... does not exist` | `db` missing/typo — ping doesn't touch the db | Set `db` correctly. |
| Password with a space breaks startup | Space-separated DSN, no quoting applied | Change the password or raise as a design issue. |
| Discovery client dials the config `host` | `service-name` unset — plain-DSN path | Set `service-name` (+ `discovery` if not "default"). |
| No spans/metrics per query | `observe.enabled=false` or starter-otel missing | Re-enable / import starter-otel. |
| `connect_timeout` seems ignored for sub-second values | Truncated to whole seconds (1500ms → 1) | Use whole-second durations. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 11 dialect-specific (+10 shared Common, +1 wrapper `observability`) |
| Required | 3 (`user`/`password`/`db`) + one of `host`/`service-name` |
| Quickstart external deps | 1 (PostgreSQL; +Jaeger for traces) |
| "Watch out" entries | 6 (tls/sslmode conflict guard, sslmode gating, dummy-port parsing, ping doesn't check db, spaced passwords, connect_timeout rounding) |

Design suspects: `user`/`password`/`db` lack expr `Require` validation (empty values surface as
driver errors at ping/first-query); no DSN value escaping for spaced passwords.
(The mysql/postgres TLS vocabulary mismatch and the sub-second `connectTimeout` truncation
were fixed 2026-08: a `tls.*` block now exists and sub-second values round up.)
