# starter-cassandra Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `driver.go`, `observe.go`,
`health/health.go`) and the runnable [example/](example/) (self-asserting smoke via
`example/check.sh`) — file:line spot-checks in brackets below. **CQL semantics and the gocql API
are [gocql's own documentation](https://pkg.go.dev/github.com/gocql/gocql)** (ScyllaDB speaks the
same native protocol, so one starter covers both [config.go:26-28]) — everything below is
go-spring's increment.

**Activation**: any `spring.cassandra.*` key (the module is `OnProperty("spring.cassandra")`, a
prefix check [starter.go:38]). Each `spring.cassandra.<name>` entry creates one
`*StarterCassandra.Client` bean named `<name>` (embedding `*gocql.Session`), plus a health
indicator named `cassandra:<name>` [starter.go:47-49].

---

## 1. Complete worked project

Two instances against one cluster — every statement through the guarded `Query`/`Exec` path —
plus probes/metrics/tracing. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter — module names follow the example's go.mod layout):

```
require (
    github.com/gocql/gocql          latest   // pulled in by the starter
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-cassandra latest
    go-spring.org/starter-actuator  latest   // optional: readiness + /metrics
    go-spring.org/starter-otel      latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest  // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cassandra"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper; both writes and reads ride the guarded path:

```go
package service

import (
    "context"

    StarterCassandra "go-spring.org/starter-cassandra"
)

type Service struct {
    // Always the wrapper type *StarterCassandra.Client. It embeds
    // *gocql.Session, so Query/Iter/Scan/Close promote unchanged.
    Main *StarterCassandra.Client `autowire:"a"`
    Raw  *StarterCassandra.Client `autowire:"b"` // second instance, same cluster
}

func (s *Service) Run(ctx context.Context) error {
    // Guarded + observed: breaker/limiter/fault + span + metric + access log.
    if err := s.Main.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?) IF NOT EXISTS",
        1, "hello"); err != nil {
        return err
    }
    // Equally guarded + observed: Client.Query returns the guarded wrapper
    // (§2.3) — Scan/Iter/ScanCAS/MapScan... all route through the executor.
    var msg string
    return s.Main.Query("SELECT message FROM demo.greetings WHERE id = 1").
        WithContext(ctx).Scan(&msg)
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- instance "a": default consistency, auth off (matches the local container)
spring.cassandra.a.hosts=127.0.0.1
spring.cassandra.a.consistency=local-quorum

# --- instance "b": same cluster, keyspace preselected
spring.cassandra.b.hosts=127.0.0.1
spring.cassandra.b.keyspace=demo

# --- actuator + otel ------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**Verify** (start Cassandra first — `docker compose up -d` with the
[example's docker-compose.yml](example/docker-compose.yml), Cassandra 5, port 9042; boot is slow,
the example's check.sh waits up to 240s on `cqlsh -e "DESCRIBE CLUSTER"`):

```bash
go run .                          # boot fails fast if the cluster is unreachable
curl -s :9370/readyz | jq .       # components include "cassandra:a", "cassandra:b"
curl -s :9370/metrics | grep -E 'db.client.(operation.duration|active_requests)'
grep _app_cassandra_access app.log | tail -3   # one record per Exec call
docker exec -it cassandra-example cqlsh -e "SELECT * FROM demo.greetings"
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-cassandra
  └─ gs.Module(OnProperty("spring.cassandra")) fires when any spring.cassandra.* key exists
        └─ conf.BindEach(p, "${spring.cassandra}") → one Config per <name> entry
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver)).Name(<name>)
              │       .Init((*Client).Init).Destroy((*Client).Destroy)
              └─ Provide health.Indicator named "cassandra:<name>",
                      injecting the client by name (TagArg) [starter.go:47-49]

gs.Run()
  ├─ ctor newClient [starter.go:60]:
  │     username/password pairing check → optional Driver bean
  │     (none → bundled DefaultDriver) → driver.CreateClient
  │     → HealthCheck probe (fail fast, see below)
  ├─ Init [client.go:59]: newDBObserver("cassandra") → resource label
  │     → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │     → resilience.WrapExecutor(exec, "cassandra") — exec chain complete
  ├─ readiness: indicator queries system.local per instance
  └─ SIGTERM → Destroy [client.go:75]: exec.Close (if armed) → Session.Close
```

An unknown consistency value or an unreachable cluster fails the boot — the process never reaches
"serving" with a dead Cassandra.

### 2.2 Fail-fast probe semantics

`newClient` ends with a one-shot `HealthCheck`: `SELECT release_version FROM system.local`
scanned into a throwaway string [starter.go:75-78, 84-87]. On failure the session is closed and
the boot aborts with "failed to reach cassandra cluster …". The probe verifies protocol version,
auth, and cluster state in one round trip — the same query the health indicator uses at runtime.
There is no retry and no skip switch: a Cassandra entry in your config means "must be up at
boot". The probe is bounded by gocql's own `ConnectTimeout`/`Timeout` (defaults 11s each here),
not by a starter-specific deadline.

### 2.3 What IS and IS NOT instrumented — per-operation walkthrough

gocql exposes no reject-capable middleware (no hook chain like go-redis), so the guard rides
the query object itself: `Client.Query`/`Client.Bind` shadow the embedded session methods and
return a guarded `*Query` wrapper [query.go] whose execution methods route through the
executor + observer. `Client.Exec` is a thin alias over that wrapper (kept for callers written
against the earlier opt-in helper). Every statement execution method — `Exec`, `Iter`, `Scan`,
`ScanCAS`, `MapScan`, `MapScanCAS` — is guarded+observed with no opt-in at the call site:

`Client.Exec(ctx, stmt, values...)` / `Client.Query(...).Exec()` [client.go, query.go]:

1. `obs.Start(ctx, "exec", stmt)` opens a client-kind span named `exec` with the statement as
   the bounded `db.statement` attribute, bumps the in-flight gauge, and starts an access-log
   record. No-op span/metric without starter-otel's globals; the access log always emits.
2. `exec.Execute(ctx, resource, call)` asks the governance executor for a permit — limiter/
   breaker scoped to the resource label `cassandra:<hosts[0]>` (per instance, keyed on the
   FIRST host only [client.go:66]). On rejection the statement is **never attempted**.
   With governance off the executor is a transparent no-op.
3. `Session.Query(stmt, values...).WithContext(ctx).Exec()` runs (via the wrapper's embedded
   `*gocql.Query`) — full gocql semantics
   (prepared-statement caching, retries per gocql's config, default consistency from the
   instance config).
4. `sp.End(err)` records the duration histogram, balances the gauge, ends the span, and emits
   the access-log line (ok/error status; errors keep gocql's error verbatim — there is no
   nil-as-success carve-out here, unlike go-redis).

What is still NOT covered: (a) `Iter` paging beyond the first page — the guard bounds the
statement execution; subsequent page fetches happen inside the returned `*gocql.Iter` (the same
statement-level fidelity as the database/sql starters), and `Iter`'s own error surfaces on
Scan/Close because gocql exposes no way to read it earlier; (b) `Batch` (NewBatch/
ExecuteBatch) — batches are intentionally raw, route protect-worthy batch writes through
`Query` statements or wrap them yourself; (c) chaining off the embedded configurators
(`Consistency(...)` etc. return `*gocql.Query`, dropping the wrapper) — configure via the
wrapper's own `WithContext`/`Bind` to stay guarded. Consequence: reads and writes both get
breaker/limiter/metrics/access-log as long as they start from `Client.Query`/`Client.Bind`/
`Client.Exec`.

Layer order on `Exec` (outside-in): observer start → fault injector (fault.WrapExecutor,
process-wide) → resilience executor (governance center) → gocql. The resilience observer
(`resilience.WrapExecutor`) adds outcome counters around the executor itself, so injected
faults and breaker rejections are both counted and logged.

### 2.4 Driver seam

`Driver.CreateClient(ctx, Config) (*gocql.Session, error)` owns full session assembly — hosts,
PasswordAuthenticator, consistency, timeouts, CQL version, TLS [driver.go:51-86] — while the
startup probe, resource label, and resilience wiring stay in the starter's lifecycle. Session
assembly is an **optional container bean**: a company or umbrella starter may provide its own
`Driver` bean (a `gs.Provide(func() StarterCassandra.Driver{...})`, so it can inject config bound
from the properties file at wiring time); every instance under `spring.cassandra` is then built
through it. When no such bean exists the starter falls back to the bundled `DefaultDriver`
(`driver.go:45-50`) inside assembly (`starter.go:66-68`). There is no per-config `driver` key.

---

## 3. Per-key behavior reference

All keys live under `spring.cassandra.<name>.` — bound per instance via `conf.BindEach` (the
ctor's `Config` arg), NOT the absolute-property starter-Pool rule.

### 3.1 Connection & session

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `hosts` | list | — | Initial contact points; entries may carry `:port` (default port 9042); the driver discovers the rest of the cluster from these. Required (expr `len($) > 0`, config.go:33). | Missing/empty → BindEach validation error at boot. |
| `keyspace` | string | — | Default keyspace for the session. Leave empty to connect without one (e.g. to run `CREATE KEYSPACE` first). | Wrong name → every keyed statement errors "Keyspace … does not exist". |
| `username` / `password` | string | — | Both set → `gocql.PasswordAuthenticator` (driver.go:73-74); both empty → no auth. ⚠ Pairing enforced: exactly one set → boot error "username and password must be set together" (starter.go:62-64). | Only one set → boot error. |
| `consistency` | string | `local-quorum` | Default consistency for the session; exact-match enum `any\|one\|two\|three\|quorum\|all\|local-quorum\|each-quorum\|local-one` (driver.go:96-119). No kebab/camel variants. | Typo → boot error naming the wanted set. |
| `timeout` | duration | `11s` | gocql per-query timeout — also bounds the fail-fast probe's query round trip. | Too low → slow queries aborted client-side. |
| `connect-timeout` | duration | `11s` | gocql connection-setup timeout. | Too low → boot probe fails on slow networks. |
| `cql-version` | string | `3.0.0` | CQL dialect version passed to gocql. | Mismatch vs server → handshake errors. |
| `tls.enabled` | bool | false | Gates the whole `tls.*` group; maps onto `gocql.SslOptions` [driver.go:76-87]. | — |
| `tls.ca-file` | string | — | CA path (`CaPath`). ⚠ Required for verification unless `insecure-skip-verify=true`. | Missing CA + verification on → handshake failure at boot. |
| `tls.cert-file` / `tls.key-file` | string | — | Client cert/key paths (mutual TLS). ⚠ Both together. | One-sided → handshake failure. |
| `tls.server-name` | string | — | SNI/verification name when it differs from the host. | Verification fails on IP+differing cert CN. |
| `tls.insecure-skip-verify` | bool | false | Skips host verification (`EnableHostVerification = !value`, driver.go:85). | true in prod = MITM-open TLS. |

### 3.2 Instrumentation

There are no observability config keys in this starter — instrumentation is always on.
No `otel.*` keys exist either (unlike starter-go-redis): the observer rides the OTel
globals directly; import starter-otel or spans/metrics are no-ops. No service-discovery keys
either — no `service-name`, no `scheme`, no `discovery`.

### 3.3 Metrics / log field reference (as emitted by Exec)

- Metrics: `db.client.operation.duration` (histogram, seconds), `db.client.active_requests`
  (UpDownCounter) — attributes `db.system=cassandra`, `db.operation` (observe.go).
- Access log: tag `_app_cassandra_access` (`log.RegisterAppTag("cassandra","access")`),
  one record per Exec at the log package's native levels: error → Warn; success with the
  statement argument (truncated to 512 bytes) → Debug; plain success → Info.
- Span: name `exec`, kind client, with `db.system` / `db.operation` / `db.statement`
  attributes (statement truncated to 512 bytes).

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .          # components include "cassandra:a"
docker stop cassandra-example        # indicator re-queries system.local → DOWN
curl -s :9370/readyz                 # 503
docker start cassandra-example
```

### 4.2 Observe the Exec path

```bash
grep _app_cassandra_access app.log | tail -1
# system=cassandra op=exec status=ok duration=...
curl -s :9370/metrics | grep db.client   # duration histogram + active_requests
```

Then confirm the asymmetry: run a `Query(...).Scan` read — no new access-log line, no metric
delta, no span (that path is unobserved by design, §2.3).

### 4.3 Fault / resilience drill (needs starter-governance)

Configure a breaker or limiter for resource `cassandra:127.0.0.1` under `govern.*`, hammer
`Exec` inserts, and watch rejections surface as fast errors WITHOUT the statement executing
(no Cassandra-side rows), plus outcome counters from the resilience observer. Flip the policy
at runtime — the executor hot-reloads without restart. Note the resource label uses hosts[0]
only: instance `b` with the same first host shares instance `a`'s breaker bucket.

### 4.4 Fail-fast probe drill

```bash
docker stop cassandra-example && go run .
# boot aborts: "failed to reach cassandra cluster [127.0.0.1]" — the process
# never reaches serving. Restart the container and the same config boots clean.
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "failed to reach cassandra cluster" | Unreachable hosts / wrong credentials / TLS mismatch | The startup probe is unconditional (§2.2); fix connectivity or auth; wait for full CQL readiness (Cassandra 5 boots slowly — check.sh allows 240s). |
| Boot fails "username and password must be set together" | Only one of the pair configured | Set both or neither [starter.go:62-64]. |
| Boot fails "unknown consistency" | Typo in `consistency`; enum is exact-match | Use one of the nine listed values [driver.go:117]. |
| No spans/metrics from Exec | starter-otel not imported | The observer rides the OTel globals; import starter-otel (access log still emits). |
| No access-log lines at all | The logger's level filter drops Debug/Info, or the `_app_cassandra_access` tag is filtered | Check the logger's level and its tag filter for `_app_cassandra_access`. |
| Breaker/limiter never triggers | Calls use the raw `*gocql.Session` (e.g. a session obtained elsewhere), or a batch, or chained configurators that dropped the wrapper | Start statements from `Client.Query`/`Client.Bind`/`Client.Exec` (§2.3). |
| Two instances share one breaker unexpectedly | Resource label is `cassandra:<hosts[0]>` [client.go:66] | By design (multi-seed configs collapse to the first host); split contact lists if isolation is needed. |
| Health DOWN though Exec works | Probe scans system.local with the indicator ctx; check permissions/timeout | Inspect the component error body in /readiness. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 9 instance keys + tls group (6) |
| Required | 1 (`hosts`) |
| Quickstart external deps | 1 (Cassandra) |
| "Watch out" entries | 4 |

Design suspects (audit ledger — kept from the previous audit, still true):

- ~~Only `Exec` is guarded/observed~~ Fixed: the guarded `*Query` wrapper covers the normal
  statement path (§2.3); batches and deep `Iter` paging remain outside the guard.
- Resource label uses `hosts[0]` only, so multi-seed configs share one resilience bucket keyed
  on the first host.
- Health indicator has no opt-out key (family asymmetry with redigo's `health.enabled`).
