# starter-cassandra Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `driver.go`, `observe.go`,
`health.go`) and the runnable [example/](example/) (self-asserting smoke via
`example/check.sh`) — file:line spot-checks in brackets below. **CQL semantics and the gocql API
are [gocql's own documentation](https://pkg.go.dev/github.com/gocql/gocql)** (ScyllaDB speaks the
same native protocol, so one starter covers both [config.go:26-28]) — everything below is
go-spring's increment.

**Activation**: any `spring.cassandra.instances.*` key (the module is `OnProperty("spring.cassandra")`, a
prefix check [starter.go:36]). Each `spring.cassandra.instances.<name>` entry creates one
`*StarterCassandra.Client` bean named `<name>` (wrapping an unexported `*gocql.Session`), plus a
health indicator named `cassandra:<name>` [starter.go:46-60].

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
    go-spring.org/starter-governance-file latest  // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cassandra"
    _ "go-spring.org/starter-governance-file"
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
    // Always the wrapper type *StarterCassandra.Client. The raw
    // *gocql.Session is an unexported field; Query/Bind return the guarded
    // wrapper, and the remaining session methods (Close, NewBatch,
    // ExecuteBatch, ...) are delegated explicitly.
    Main *StarterCassandra.Client `autowire:"a"`
    Raw  *StarterCassandra.Client `autowire:"b"` // second instance, same cluster
}

func (s *Service) Run(ctx context.Context) error {
    // Guarded: breaker/limiter/fault. The statement's identity is declared
    // here; the span/metric/access-log are emitted by the resilience layer.
    if err := s.Main.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?) IF NOT EXISTS",
        1, "hello"); err != nil {
        return err
    }
    // Equally guarded + declared: Client.Query returns the guarded wrapper
    // (§2.3) — Scan/Iter/ScanCAS/MapScan... all route through the executor.
    var msg string
    return s.Main.Query("SELECT message FROM demo.greetings WHERE id = 1").
        WithContext(ctx).Scan(&msg)
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- instance "a": default consistency, auth off (matches the local container)
spring.cassandra.instances.a.hosts=127.0.0.1
spring.cassandra.instances.a.consistency=local-quorum

# --- instance "b": same cluster, keyspace preselected
spring.cassandra.instances.b.hosts=127.0.0.1
spring.cassandra.instances.b.keyspace=demo

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
curl -s :9370/metrics | grep -E 'db.client.(operation.duration|attempt.duration|active_requests)'
grep _app_cassandra_access app.log | tail -3   # one record per Exec call
docker exec -it cassandra-example cqlsh -e "SELECT * FROM demo.greetings"
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-cassandra
  └─ gs.Module(OnProperty("spring.cassandra.instances")) fires when any spring.cassandra.instances.* key exists
        └─ conf.BindEach(p, "${spring.cassandra.instances}") → one Config per <name> entry
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver),
              │            IndexArg(3, *resilience.Manager),
              │            IndexArg(4, *fault.Injector)).Name(<name>)
              │       .Destroy((*Client).Destroy)
              └─ Provide health.Indicator named "cassandra:<name>",
                      injecting the client by name (TagArg) [starter.go:58-61]

gs.Run()
  ├─ ctor newClient [starter.go:74]:
  │     username/password pairing check → optional Driver bean
  │     (none → bundled DefaultDriver; several coexist → the entry selects
  │     one by name: spring.cassandra.instances.<name>.driver = <bean-name>, empty =
  │     `spring.cassandra.default.driver`, then the single Driver bean by type, naming a missing bean fails startup)
  │     → driver.CreateClient(c, cloud.ClientParams{Resilience: mgr, Fault: inj}) returns the *Client
  │       (session + identity + governance): NewClient sets exec = params.ExecutorFor("cassandra", service),
  │       which builds fault.WrapClientExecutor(mgr.ClientExecutorFor("cassandra", service), service, inj)
  │       over the injected *resilience.Manager / *fault.Injector beans — exec chain complete at construction
  ├─ fail-fast probe: newClient calls HealthCheck [starter.go:100] — one system.local scan straight to the raw session
  ├─ readiness: indicator delegates to HealthCheck, which queries system.local [health.go:34]
  └─ SIGTERM → Destroy [client.go]: exec.Close → session.Close
```

There is no `Init` hook: `NewClient` fixes the client's identity and applies the
governance bundle as part of driver assembly, so the bean is complete the moment `newClient`
returns. Because governance is applied inside the constructor — before the probe — a probe
failure tears down an already-governed client (`Destroy`) rather than leaking the executor.

An unknown consistency value or an unreachable cluster fails the boot — the process never reaches
"serving" with a dead Cassandra.

### 2.2 Fail-fast probe semantics

`newClient` ends with a one-shot probe — `HealthCheck` (`health.go:34`), the module's single health
implementation: on the raw session (`client.session`) it runs
`SELECT release_version FROM system.local` scanned into a throwaway string, invoked from
`newClient` at [starter.go:100]. The Actuator indicator's probe delegates to the same `HealthCheck`. The probe goes to the raw session on purpose — it is a connectivity check, not
business traffic, so it opens no span and spends no limiter/breaker budget. On failure the client
is destroyed and the boot aborts with "failed to reach cassandra cluster …". The probe verifies protocol version,
auth, and cluster state in one round trip — the same query the health indicator uses at runtime.
There is no retry and no skip switch: a Cassandra entry in your config means "must be up at
boot". The probe is bounded by gocql's own `ConnectTimeout`/`Timeout` (defaults 11s each here),
not by a starter-specific deadline.

### 2.3 What IS and IS NOT instrumented — per-operation walkthrough

gocql exposes no reject-capable middleware (no hook chain like go-redis), so the guard rides
the query object itself: `Client.Query`/`Client.Bind` mirror the raw session methods and
return a guarded `*Query` wrapper [query.go] whose execution methods route through the
guard + executor. `Client.Exec` is a thin alias over that wrapper (kept for callers written
against the earlier opt-in helper). Every statement execution method — `Exec`, `Iter`, `Scan`,
`ScanCAS`, `MapScan`, `MapScanCAS` — is guarded+declared with no opt-in at the call site:

`Client.Exec(ctx, stmt, values...)` / `Client.Query(...).Exec()` [client.go, query.go]:

1. `guard` declares the statement's identity on the ctx (`observability.WithOperation`,
   [observe.go]): span name `exec`, the `db.system`/`db.operation` labels, and the bounded
   `db.statement` detail (truncated to 512 bytes).
2. `exec.Execute(ctx, call)` asks the governance executor for a permit — limiter/
   breaker scoped to the service label `cassandra:<hosts[0]>` (per instance, keyed on the
   FIRST host only [client.go]). On rejection the statement is **never attempted**.
   With governance off the client runs on the observed-only, loudly-unmanaged executor: the
   statement still runs and is traced/measured, but no limiter/breaker/retry applies to it.
3. The raw session's `Query(stmt, values...).WithContext(ctx).Exec()` runs (via the wrapper's embedded
   `*gocql.Query`) — full gocql semantics
   (prepared-statement caching, retries per gocql's config, default consistency from the
   instance config).
4. Once the call returns, the resilience layer — the single emitter on the chain — records the
   call-level duration histogram, emits the attempt-level `db.client.attempt.duration` per try,
   balances the in-flight gauge, ends the span, and writes the access-log line (ok/error status;
   errors keep gocql's error verbatim — there is no nil-as-success carve-out here, unlike
   go-redis). No-op span/metric without starter-otel's globals; the access log always emits.

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

Layer order on `Exec` (outside-in): fault injector (`fault.WrapClientExecutor` over the executor the
injected manager resolves) → resilience emitter (`resilience.WrapClientExecutor`: the statement's
span, the call/attempt histograms, the in-flight gauge, the outcome counters and the access log —
it reads the operation the guard declared) → resilience executor (limiter/breaker/retry core) →
gocql. The fault injector wraps the operation fn that core runs, so injected faults and breaker
rejections both flow through retry/breaker/timeout and are counted and logged.

### 2.4 Driver seam

`Driver.CreateClient(ctx, Config, cloud.ClientParams) (*Client, error)` owns full session
assembly — hosts, PasswordAuthenticator, consistency, timeouts, CQL version, TLS [driver.go:63-95] —
and applies the governance bundle while building, returning the exported `*Client` wrapper complete
(identity and governance both fixed via `NewClient` [driver.go:95]); only the startup probe stays in
the starter's lifecycle (`newClient`). Session
assembly is an **optional container bean**: a company or umbrella starter may provide its own
`Driver` bean (a `gs.Provide(func() StarterCassandra.Driver{...})`, so it can inject config bound
from the properties file at wiring time); every instance under `spring.cassandra` is then built
through it. When no such bean exists the starter falls back to the bundled `DefaultDriver`
(`driver.go:56`) inside assembly (`starter.go:82-84`). There is no per-config `driver` key.

---

## 3. Per-key behavior reference

All keys live under `spring.cassandra.instances.<name>.` — bound per instance via `conf.BindEach` (the
ctor's `Config` arg), NOT the absolute-property starter-Pool rule.

### 3.1 Connection & session

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `hosts` | list | — | Initial contact points; entries may carry `:port` (default port 9042); the driver discovers the rest of the cluster from these. Required (expr `len($) > 0`, config.go:33). | Missing/empty → BindEach validation error at boot. |
| `keyspace` | string | — | Default keyspace for the session. Leave empty to connect without one (e.g. to run `CREATE KEYSPACE` first). | Wrong name → every keyed statement errors "Keyspace … does not exist". |
| `username` / `password` | string | — | Both set → `gocql.PasswordAuthenticator` (driver.go:74-76); both empty → no auth. ⚠ Pairing enforced: exactly one set → boot error "username and password must be set together" (starter.go:77-79). | Only one set → boot error. |
| `consistency` | string | `local-quorum` | Default consistency for the session; exact-match enum `any\|one\|two\|three\|quorum\|all\|local-quorum\|each-quorum\|local-one` (driver.go:99-120). No kebab/camel variants. | Typo → boot error naming the wanted set. |
| `timeout` | duration | `11s` | gocql per-query timeout — also bounds the fail-fast probe's query round trip. | Too low → slow queries aborted client-side. |
| `connect-timeout` | duration | `11s` | gocql connection-setup timeout. | Too low → boot probe fails on slow networks. |
| `cql-version` | string | `3.0.0` | CQL dialect version passed to gocql. | Mismatch vs server → handshake errors. |
| `tls.enabled` | bool | false | Gates the whole `tls.*` group; maps onto `gocql.SslOptions` [driver.go:77-88]. | — |
| `tls.ca-file` | string | — | CA path (`CaPath`). ⚠ Required for verification unless `insecure-skip-verify=true`. | Missing CA + verification on → handshake failure at boot. |
| `tls.cert-file` / `tls.key-file` | string | — | Client cert/key paths (mutual TLS). ⚠ Both together. | One-sided → handshake failure. |
| `tls.server-name` | string | — | SNI/verification name when it differs from the host. | Verification fails on IP+differing cert CN. |
| `tls.insecure-skip-verify` | bool | false | Skips host verification (`EnableHostVerification = !value`, driver.go:86). | true in prod = MITM-open TLS. |

### 3.2 Instrumentation

There are no observability config keys in this starter — instrumentation is always on.
No `otel.*` keys exist either (unlike starter-go-redis): the emitted signals ride the OTel
globals directly; import starter-otel or spans/metrics are no-ops. No service-discovery keys
either — no `service-name`, no `scheme`, no `discovery`.

### 3.3 Metrics / log field reference (declared by Exec, emitted by the resilience layer)

- Metrics: `db.client.operation.duration` (histogram, seconds) and `db.client.attempt.duration`
  (histogram, seconds, one per downstream try), plus `db.client.active_requests` (UpDownCounter)
  — attributes `db.system=cassandra`, `db.operation`, `status` (declared in observe.go; the
  emitter appends `status` and the `resilience.client.calls` counter).
- Access log: tag `_app_cassandra_access` (`log.RegisterAppTag("cassandra","access")`),
  one record per Exec at the resilience layer's levels: error → Warn; success with the
  statement argument (truncated to 512 bytes) → Debug; plain success → Info.
- Span: name `exec`, with `db.system` / `db.operation` / `db.statement`
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
# db.system=cassandra db.operation=exec db.statement="INSERT ..." status=ok duration_ms=...
curl -s :9370/metrics | grep db.client   # call/attempt histograms + active_requests
```

Then confirm what stays outside the guard: run a `NewBatch`/`ExecuteBatch` or chain a bare
configurator (`Consistency(...)`, which returns a `*gocql.Query`), and see no new access-log
line, no metric delta, no span (those paths are undeclared by design, §2.3).

### 4.3 Fault / resilience drill (needs starter-governance-file)

Configure a breaker or limiter for service `cassandra:127.0.0.1` under `spring.governance.*`, hammer
`Exec` inserts, and watch rejections surface as fast errors WITHOUT the statement executing
(no Cassandra-side rows), plus outcome counters from the resilience emitter. Flip the policy
at runtime — the executor hot-reloads without restart. Note the service label uses hosts[0]
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
| Boot fails "username and password must be set together" | Only one of the pair configured | Set both or neither [starter.go:77-79]. |
| Boot fails "unknown consistency" | Typo in `consistency`; enum is exact-match | Use one of the nine listed values [driver.go:120]. |
| No spans/metrics from Exec | starter-otel not imported | The emitted signals ride the OTel globals; import starter-otel (access log still emits). |
| No access-log lines at all | The logger's level filter drops Debug/Info, or the `_app_cassandra_access` tag is filtered | Check the logger's level and its tag filter for `_app_cassandra_access`. |
| Breaker/limiter never triggers | The statement ran through a batch (`NewBatch`/`ExecuteBatch`), or through chained configurators that returned a bare `*gocql.Query` and dropped the guard | Start statements from `Client.Query`/`Client.Bind`/`Client.Exec` (§2.3). |
| Two instances share one breaker unexpectedly | Service label is `cassandra:<hosts[0]>` [client.go:101] | By design (multi-seed configs collapse to the first host); split contact lists if isolation is needed. |
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
- Service label uses `hosts[0]` only, so multi-seed configs share one resilience bucket keyed
  on the first host.
- Health indicator has no opt-out key (family asymmetry with redigo's `health.enabled`).
