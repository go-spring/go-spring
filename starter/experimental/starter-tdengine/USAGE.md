# starter-tdengine Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `driver.go`,
`health/health.go`, `tdengine_test.go`) and the runnable [example/](example/) — file:line
spot-checks in brackets below. **TDengine SQL semantics, the DSN format, and the driver-go
WebSocket connector are [TDengine's own documentation](https://docs.taosdata.com/reference/connector/go/)
(the WebSocket endpoint is served by
[taosAdapter](https://docs.taosdata.com/reference/taosadapter/))** — everything below is
go-spring's increment.

**Activation**: any `spring.tdengine.*` key (the module is `OnProperty("spring.tdengine")`, a
prefix check). Each `spring.tdengine.<name>` entry creates one `*StarterTdengine.Client` bean
named `<name>`, plus a health indicator named `tdengine:<name>`.

---

## 1. Complete worked project

One service with two TDengine clients against the same instance, plus probes, metrics and
tracing. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/taosdata/driver-go/v3 latest      // pulled in by the starter
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-tdengine latest
    go-spring.org/starter-actuator latest        // optional: readiness + /metrics
    go-spring.org/starter-otel     latest        // optional: real trace/metric export
    go-spring.org/starter-governance latest      // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-tdengine"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper, run real SQL through the super-table idiom:

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterTdengine "go-spring.org/starter-tdengine"
)

type Service struct {
    // Always the wrapper type *StarterTdengine.Client. It embeds *sql.DB,
    // so ExecContext/QueryContext/QueryRowContext/PingContext promote
    // unchanged [client.go:38-55].
    Admin *StarterTdengine.Client `autowire:"a"` // no default database
    Power *StarterTdengine.Client `autowire:"b"` // DSN pins /power
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            for _, q := range []string{
                "CREATE DATABASE IF NOT EXISTS power",
                "CREATE STABLE IF NOT EXISTS power.meters (ts TIMESTAMP, current FLOAT) TAGS (location BINARY(24))",
                "INSERT INTO power.d001 USING power.meters TAGS('beijing') VALUES (NOW, 10.5)",
            } {
                if _, err := s.Admin.ExecContext(ctx, q); err != nil {
                    panic(q + ": " + err.Error())
                }
            }
            var n int
            // Uses the "b" client whose DSN already selects the power database.
            if err := s.Power.QueryRowContext(ctx, "SELECT COUNT(*) FROM meters").Scan(&n); err != nil {
                panic(err)
            }
        }
    })
}
```

**conf/app.properties** — the complete surface actually used above (DSN values copied from
[example/conf/app.properties](example/conf/app.properties)):

```properties
# --- two clients, one instance ------------------------------------------------
# DSN format is the driver's unified form; ws() = WebSocket via taosAdapter:6041.
spring.tdengine.a.dsn=root:taosdata@ws(127.0.0.1:6041)/
spring.tdengine.a.max-open-conns=4
spring.tdengine.a.max-idle-conns=2

spring.tdengine.b.dsn=root:taosdata@ws(127.0.0.1:6041)/power

# --- observability is always on: spans + db.client.* metrics ride ------------
# starter-otel's globals, and the access log rides the log package's levels.

# --- actuator + otel ---------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**Verify** (start TDengine first — the [example's docker-compose.yml](example/docker-compose.yml)
ships 3.3.6.0 with taosAdapter on 6041):

```bash
docker compose -p gs-tdengine-demo up -d          # wait ~30s; taosAdapter boots late
go run .                        # boot fails fast if the DSN is unreachable (§2.1)
curl -s :9370/readyz | jq .     # components include tdengine:a and tdengine:b
curl -s :9370/metrics | grep -E 'db.client.*tdengine'   # per-statement histograms
grep _app_tdengine_access app.log | tail -3        # one record per statement
docker exec -it <container> taos -s "SELECT COUNT(*) FROM power.meters"   # 1
```

The [example/](example/) itself (single client `a`, self-asserting round trip via
`check.sh`) is the smoke-verified anchor; the otel/actuator block above is the standard
combo from the client family.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-tdengine
  └─ gs.Module(OnProperty("spring.tdengine")) fires when any spring.tdengine.* key exists
        └─ conf.BindEach("${spring.tdengine}") → one Config per <name> entry
              ├─ Provide(newClient).Name(<name>)
              │       .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator named "tdengine:<name>", exported as
                  health.Indicator (always registered; no opt-out key)

gs.Run()
  ├─ ctor newClient [starter.go:61]: optional Driver bean → when none present the
  │     starter falls back to the bundled DefaultDriver (d == nil [starter.go:65-67])
  │     → d.CreateClient [starter.go:68]: ParseDSN → taosws.NewConnector →
  │       guardedConnector → sql.OpenDB → pool settings applied
  │     → fail-fast PingContext bounded by 10s [starter.go:74-77]; on error the
  │       half-built client is Closed and the boot fails
  ├─ Init [client.go:58]: resourceLabel ("tdengine:<dsn addr>") →
  │     fault.WrapExecutor(resilience.ExecutorFor(resource)) →
  │     resilience.WrapExecutor(exec, "tdengine") → newDBObserver("tdengine")
  │     armed on the slot
  ├─ readiness: indicator runs db.PingContext per instance
  └─ SIGTERM → Destroy [client.go:81]: exec.Close → db.Close
```

A wrong DSN, wrong credentials, or a server older than the driver's minimum fails the boot —
the process never reaches "serving" with a dead TDengine (version floor: driver-go v3.8.2
needs server ≥ 3.3.6.0 on the WebSocket path [DESIGN.md §3]).

**Assembly extension point**: client assembly is owned by a `Driver` (interface,
`driver.go:45-46`). A company/umbrella starter may provide its own `Driver` as an **optional
container bean** (a `gs.Provide(func() StarterTdengine.Driver{...})`, so it can inject config
bound from the properties file at wiring time); every instance under `spring.tdengine` is then
built through it. When no such bean exists the starter falls back to the bundled `DefaultDriver`
(`driver.go:50`) inside assembly (`starter.go:65-67`). There is no per-config `driver` key.

### 2.2 The statement seam — exact order and why

`database/sql` offers no interceptor chain, so the guard lives at the `driver.Conn` level:
DefaultDriver wraps the taosWS connector in `guardedConnector`, and every pooled connection
is a `guardedConn` [driver.go:96-124]. This is the database/sql analog of the gorm callback
chain and the HTTP RoundTripper adapters — and it means protection is **per-statement and
transparent**: no opt-in at call sites, ORMs layered on the pool are covered too
[driver.go:115-124, DESIGN.md §4].

One statement, e.g. `QueryContext("SELECT COUNT(*) ...")`:

```
*sql.DB pool
  └─ guardedConn.QueryContext [driver.go:143]
        ├─ guard: exec.Execute(ctx, resource, call) [driver.go:160-165]
        │    (OUTER: rate limit / breaker / fault decide BEFORE the statement runs; a
        │     rejection never reaches the connection — unit-tested [tdengine_test.go:62-76].
        │     When governance is off the executor is a transparent no-op.)
        │    └─ queryObserved: observer.Start(ctx, "query", sql) — span + in-flight
        │         metric + access log wrap EACH attempt [driver.go:179-187]: a retry
        │         loop emits one record per attempt, and a rejection emits none.
        └─ taosWS conn → websocket → taosAdapter
```

Contrast with starter-go-redis, where the access log sits outside the breaker (one record
per command): here the executor is outermost, so the log answers "what did each attempt do"
and the resilience metrics answer "what did the executor decide". driver-go ships no
instrumentation of its own, so the module-local observer owns all three signals here
(spans + metrics + log — see observe.go).

What is NOT covered by the guard [driver.go:189-198]:

- `Prepare` delegates to the raw connection — statements executed through a prepared
  `*sql.Stmt` bypass resilience/observation. Use `ExecContext`/`QueryContext` (which
  `database/sql` prefers anyway, per the source comment).
- `Begin` delegates too: TDengine has no transactions; the underlying driver reports that.
- The health probe (`PingContext`) does not pass through observer/executor.

Before Init arms the slot, statements pass through untouched (nil exec, nil obs — the
zero-config pass-through is unit-tested [tdengine_test.go:51-58]).

### 2.3 Resource label

`resourceLabel` extracts a display-safe address from the DSN
("root:taosdata@ws(127.0.0.1:6041)/power" → "127.0.0.1:6041") and builds
`tdengine:<addr>` [client.go:91-94, starter.go:89-96]. Limiter/breaker state is scoped per
TDengine instance, not per statement or per database: two clients to the same host:port
(instances `a` and `b` above) share one bucket even though their DSNs differ.

---

## 3. Per-key behavior reference

All keys live under `spring.tdengine.<name>.` — bound per instance via `conf.BindEach`
(not the absolute-property starter-Pool rule). Complete list, verified against
`grep -rhoE 'value:"[^"]+"'`:

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `dsn` | string | — | **required** (expr `$ != ''` [config.go:33]). Driver's unified DSN `[user[:password]@]ws(host:port)/[dbname][?params]`. Also the source of the resilience resource label (§2.3) and of boot error messages. TLS is expressed here (`wss(...)`, cert params) — there is no `tls.*` block. | Empty → boot error at BindEach. Wrong addr/credentials → fail-fast ping error "failed to reach tdengine at <addr>". |
| `max-open-conns` | int | 8 | `db.SetMaxOpenConns` on the embedded pool [driver.go:81]. | Too low → statements queue waiting for a free conn. |
| `max-idle-conns` | int | 2 | `db.SetMaxIdleConns`. ⚠ Should be ≤ max-open-conns (database/sql silently caps it, but a value above is a config smell). | Larger than open conns → clamped, idle churn. |
| `conn-max-lifetime` | duration | 0s | `db.SetConnMaxLifetime`; 0 = never retire. ⚠ Unlike redis (2m default), there is no discovery to follow here, so 0 is safe. | — |

No `driver` key: client assembly is owned by an optional `Driver` bean (see §2.1) or the bundled
`DefaultDriver`. No observability keys exist — instrumentation is always on (§4.2).

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components: tdengine:a, tdengine:b (probe = PingContext)
docker stop <tdengine>           # indicator's ping fails → component flips DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <tdengine>
```

The probe draws a real websocket connection and exercises taosAdapter's action chain
[health/health.go:30-33].

### 4.2 What observability actually emits

| Signal | Name / shape | Attributes |
|--------|--------------|------------|
| Span | `exec` / `query`, kind = client, tracer `go-spring.org/starter-tdengine` | `db.system=tdengine`, `db.operation=exec\|query`, `db.statement=<sql, bounded>` |
| Metric | `db.client.operation.duration` (histogram, s) | `db.system`, `db.operation`, `status=ok\|error` |
| Metric | `db.client.active_requests` (up-down counter) | `db.system`, `db.operation` |
| Metric | `resilience.calls` (counter) | `resilience.system=tdengine`, `resilience.resource`, `resilience.outcome=success\|rate_limited\|circuit_open\|bulkhead_full\|timeout\|error` |
| Metric | `resilience.breaker.state_change` (counter) | from/to attrs |
| Log | tag `_app_tdengine_access` | system=tdengine op=… status duration; error → Warn, success with the SQL (truncated to 512 bytes) → Debug, plain success → Info |
| Log | tag `_app_tdengine_resilience` | resilience rejections |

Without starter-otel, spans/metrics are no-ops (global providers empty) — only the access
log emits, and it carries no trace_id.

```bash
curl -s :9370/metrics | grep -E 'db.client_operation_duration|db.client_active' 
grep _app_tdengine_access app.log | tail -1
# system=tdengine op=exec status ok duration=... "INSERT INTO power.d001 ..."
```

### 4.3 Resilience drill

With starter-governance imported, define a policy for resource `tdengine:127.0.0.1:6041`
(§2.3) — e.g. a rate limit. Hammer `ExecContext`; over-limit statements are rejected with
`resilience.ErrRateLimited` **without reaching the connection** (unit-tested
[tdengine_test.go:62-76]), surface in `resilience.calls{outcome="rate_limited"}` and the
`_app_tdengine_resilience` log. Flip the policy at runtime — the executor hot-reloads via
the governance center without restart.

### 4.4 Fail-fast drill

```bash
docker stop <tdengine> && go run .    # exits with "failed to reach tdengine at 127.0.0.1:6041"
```

The startup ping is unconditional and bounded by 10s [starter.go:72-77] — a boot that
succeeds proves credentials, DSN and server version are all good.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "failed to reach tdengine at ..." | Unreachable addr / wrong credentials / taosAdapter not up yet | Fix DSN; wait for port 6041 (the image boots several services — check.sh waits 90s + 5s). |
| Boot fails, driver version error | Server < 3.3.6.0 on the WebSocket path (driver-go v3.8.2 floor) | Upgrade the server image. |
| Boot fails at BindEach on `dsn` | Empty or missing `spring.tdengine.<name>.dsn` | The expr tag enforces non-empty — set it. |
| Health DOWN though SQL works | Probe draws a fresh conn while the pool is exhausted (max-open-conns too low) | Raise max-open-conns; inspect the component error body in /readiness. |
| No spans/metrics | starter-otel not imported | The observer rides the OTel globals; import starter-otel. |
| No access log lines | Logger level drops Debug/Info, or the log tag is filtered | Check the logger level and logger config for `_app_tdengine_access`. |
| Statements through db.Prepare are unguarded/unobserved | `Prepare` bypasses the slot by design [driver.go:189-191] | Use ExecContext/QueryContext. |
| Breaker state shared across "databases" | Resource label is per host:port, DSN params ignored | Intentional (per-instance scoping); split backends by host to get separate buckets. |
| `Begin` errors | TDengine has no transactions | By design — the driver reports it. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 4 instance keys |
| Required | 1 (`dsn`) |
| Quickstart external deps | 1 (TDengine + its bundled taosAdapter) |
| "Watch out" entries | 4 |

Design suspects (kept from the previous audit, plus new):
- DSN is an opaque string — the resilience resource label is derived by parsing the address
  out of it, so two DSNs differing only in params or database share one bucket; no
  `tls.*`/`service-name` unlike sibling starters (family asymmetry).
- `Prepare` escapes the guard seam entirely — an ORM that prepares statements silently loses
  resilience + observation coverage.
- Health indicator has no opt-out key (same family asymmetry as starter-go-redis; redigo
  has `health.enabled`).
