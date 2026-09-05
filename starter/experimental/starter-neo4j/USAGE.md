# starter-neo4j Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health/health.go`) and the runnable examples ([example/](example/), [example-otel/](example-otel/),
[example-cloudnative/](example-cloudnative/), [example-load/](example-load/)) — file:line
spot-checks in brackets below. **Cypher semantics and the neo4j-go-driver API are
[the driver's own documentation](https://neo4j.com/docs/go-manual/current/)** — everything below
is go-spring's increment.

**Activation**: any `spring.neo4j.*` key (the module is `OnProperty("spring.neo4j")`, a prefix
check). Each `spring.neo4j.<name>` entry creates one `*StarterNeo4j.Client` bean named `<name>`,
plus a health indicator named `neo4j:<name>`.

---

## 1. Complete worked project

Two instances (direct + pool-tuned), probes, metrics/tracing, and the governance guard. File tree:

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
    github.com/neo4j/neo4j-go-driver/v5 v5.28.4
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-neo4j        latest
    go-spring.org/starter-actuator     latest   // optional: readiness + /metrics
    go-spring.org/starter-otel         latest   // optional: real trace/metric export
    go-spring.org/starter-governance   latest   // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-neo4j"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper and run real Cypher through the instrumented seam:

```go
package service

import (
    "context"

    "github.com/neo4j/neo4j-go-driver/v5/neo4j"
    "go-spring.org/spring/gs"
    StarterNeo4j "go-spring.org/starter-neo4j"
)

type Service struct {
    // Always the wrapper type *StarterNeo4j.Client — it embeds
    // neo4j.DriverWithContext, so every driver method promotes unchanged.
    Graph     *StarterNeo4j.Client `autowire:"graph"`
    Analytics *StarterNeo4j.Client `autowire:"analytics"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // Write + read through the instrumented Query (span + metric +
            // access log + resilience guard). Same signature as neo4j.ExecuteQuery.
            _, err := StarterNeo4j.Query(ctx, s.Graph,
                "MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
                map[string]any{"name": "alice", "age": 30},
                neo4j.EagerResultTransformer)
            if err != nil {
                panic(err)
            }
            // Typed single-value result via a transformer.
            n, err := StarterNeo4j.Query[int](ctx, s.Graph,
                "MATCH (p:Person) RETURN count(p)",
                nil, neo4j.SingleResultTransformer[int]())
            _ = n // 1
            _ = err

            // Manual session code: wrap it in the resilience guard yourself —
            // it is NOT protected automatically (see §2.3).
            err = StarterNeo4j.RunWithResilience(ctx, s.Analytics, func(ctx context.Context) error {
                sess := s.Analytics.NewSession(ctx, neo4j.SessionConfig{})
                defer sess.Close(ctx)
                _, err := sess.Run(ctx, "MATCH (p:Person) RETURN p.name", nil)
                return err
            })
            _ = err
        }
    })
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- instance "graph": direct address, defaults elsewhere -------------------
spring.neo4j.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.graph.username=neo4j
spring.neo4j.graph.password=password

# --- instance "analytics": tuned pool, own health indicator -----------------
spring.neo4j.analytics.uri=bolt://127.0.0.1:7687
spring.neo4j.analytics.username=neo4j
spring.neo4j.analytics.password=password
spring.neo4j.analytics.max-connection-pool-size=50
spring.neo4j.analytics.connection-acquisition-timeout=30s

# --- observability (starter-otel: OTLP exporter + prometheus) ---------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus

# --- actuator: readiness folds in neo4j:graph and neo4j:analytics -----------
spring.actuator.addr=:9370
# --- governance: guard for Query / RunWithResilience ------------------------
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=100
govern.default.max-retries=1
govern.default.timeout=500ms
```

**Verify** (start Neo4j first — `docker run -d -e NEO4J_AUTH=neo4j/password -p 7687:7687 -p 7474:7474 neo4j:5`,
or use [example/docker-compose.yml](example/docker-compose.yml)):

```bash
go run .                          # boot fails fast if Neo4j is unreachable
curl -s :9370/readyz | jq .       # components include "neo4j:graph", "neo4j:analytics"
curl -s :9370/metrics | grep -E 'db.client.operation'   # per-query duration histogram
grep _app_neo4j_access app.log | tail -3   # one access record per Query call
cypher-shell -a bolt://127.0.0.1:7687 -u neo4j -p password \
  'MATCH (p:Person {name:"alice"}) RETURN p.age'        # 30
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-neo4j
  └─ gs.Module(OnProperty("spring.neo4j")) fires when any spring.neo4j.* key exists
        └─ conf.BindEach("${spring.neo4j}") → one Config per <name> entry
              ├─ Provide(newClient).Name(<name>)
              │    .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator named "neo4j:<name>", exported
                 (injects the wrapper by name; hands the embedded driver to
                 the indicator) [starter.go:44-53]

gs.Run()
  ├─ ctor newClient [starter.go:77]: log instance creation
  │   ├─ if service-name set and mesh off: resolveURI → one endpoint picked,
  │   │  its address spliced into the URI host [starter.go:81-89, driver.go:128-145]
  │   ├─ driverRegistry lookup ("neo4j driver not found" on miss) [starter.go:91-98]
  │   ├─ driver.CreateClient (auth + pool knobs + TLS) [driver.go:61-82]
  │   └─ fail-fast VerifyConnectivity, bounded by socket-connect-timeout or
  │      5s; on failure the client, resolver are closed and the boot aborts
  │      [starter.go:109-119, 132-137]
  ├─ Init [client.go:68-69]: resource = resilience.ResourceLabel("neo4j",
  │   ServiceName, URI) → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │   → resilience.WrapExecutor(exec, "neo4j") — no-op executor when
  │   governance is off
  ├─ readiness: indicator runs VerifyConnectivity per probe
  └─ SIGTERM → Destroy [client.go:89-95]: exec.Close → stopLiveResolver →
      driver.Close(context.Background())
```

A misconfigured `driver`, an unreachable server, or bad TLS material fails the boot — the
process never reaches "serving" with a dead Neo4j.

Teardown is deliberately named `Destroy`, not `Close`: the embedded
`neo4j.DriverWithContext` already exposes `Close(context.Context)`, and shadowing it with a
different signature would stop the wrapper satisfying the interface (client.go:81-88 comment).

### 2.2 What instrumentation actually exists — and what does not

Be honest about the family asymmetry: **there is no transparent per-request instrumentation**.
neo4j-go-driver speaks the binary Bolt protocol, ships no official OpenTelemetry
instrumentation, and its `ExecuteQuery` is a package-level generic function — not a method on
the driver — so there is no transport/dialer/hook to intercept (starter.go:63-68 and
command.go:31-44 comments call this a documented gap, not an oversight). What exists:

| Helper | What it adds | Level |
|--------|--------------|-------|
| `StarterNeo4j.Query[T]` | drop-in for `neo4j.ExecuteQuery` (same signature): span + duration/in-flight metric + access log, plus the call-site resilience guard | opt-in, per call site |
| `StarterNeo4j.RunWithResilience` | wraps arbitrary session/transaction code in the resilience guard only (no span/metric/log) | opt-in |
| `StarterNeo4j.StartSpan` / `EndSpan` | manual span + metric + access log for ops you drive via `driver.NewSession` | opt-in |
| health indicator `neo4j:<name>` | `VerifyConnectivity` per actuator probe | automatic, always |
| `resilience.WrapExecutor` in Init | outcome metrics (`resilience.*`) for guarded executions | automatic when governance on |

`Query`'s span/metric/log ride a **package-level** default observer (built lazily on first
use, command.go:50) that emits through this module's own instrumentation ([observe.go]) on the
OTel globals starter-otel installs — there is no config gate on it; the access log always
emits via the package observer at the log package's native levels.

Emissions when `Query` is used:

- span: kind client, name = `op` (`"query"` for `Query`), attributes `db.system=neo4j`,
  `db.operation=<op>`, `db.statement=<Cypher, truncated at 512 bytes>`
- metrics: `db.client.operation.duration` (histogram, seconds) and
  `db.client.active_requests` (in-flight gauge), both labeled with db.system/operation
- access log: one record per call under log tag `_app_neo4j_access` (`log.RegisterAppTag`)
  at native levels — error → Warn; success with the captured Cypher argument → Debug;
  plain success → Info

### 2.3 One query through the actual layers: `Query(... "MATCH ...")`

1. `defaultObs.Start(ctx, "query", cypher)` starts the span, bumps the in-flight gauge, and
   opens an access-log record [command.go:66].
2. `queryResilience(driver)` type-asserts the driver back to `*Client` [command.go:111-116].
   On the wrapper, the executor is always resolved (a no-op when governance is off), so the
   call routes through `exec.Execute(ctx, resource, fn)` — rate limit / breaker / retry /
   bulkhead / timeout scoped to the resource label `neo4j:<service-name|uri>` [client.go:75].
   A **raw** `neo4j.DriverWithContext` passed instead of the wrapper yields `(nil, "")` and
   runs unguarded, silently.
3. `neo4j.ExecuteQuery[T]` runs the Cypher (driver retries transient errors up to
   `max-transaction-retry-time` — driver semantics, see the
   [driver manual](https://neo4j.com/docs/go-manual/current/)).
4. `sp.End(err)` records the duration histogram, balances the gauge, ends the span, and emits
   the access-log record with the outcome.

Code that calls `neo4j.ExecuteQuery` directly, or drives `NewSession`/`session.Run` without
`RunWithResilience`/`StartSpan`, bypasses everything in steps 1-2 — unobserved and unguarded.
This is the documented cost of the missing seam (§6).

### 2.4 Discovery addressing — one-shot

When `service-name` is set and mesh mode is off, `resolveURI` builds a Resolver on the
`discovery` backend, picks one endpoint, and splices its address into the URI host
[driver.go:128-145]. The neo4j driver exposes no dialer injection point, so this is a **one-shot
resolution at startup** — address changes after startup are not picked up until the client is
rebuilt (config.go:79-83 comment). The Resolver is kept alive only for lifecycle uniformity and
stopped on shutdown. In mesh mode (`GS_MESH=on`) the sidecar owns discovery+LB and the URI is
used unchanged [starter.go:70-76].

---

## 3. Per-key behavior reference

All keys live under `spring.neo4j.<name>.` — bound per instance via `conf.BindEach` (ctor
IndexArg(1)), not the absolute-property Pool rule.

### 3.1 Addressing & discovery

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `uri` | string | — | **required** (`expr:"$ != ''"`). Scheme selects routing+encryption: `bolt`/`neo4j` plain, `neo4j+s`/`bolt+s` TLS, `+ssc` self-signed. ⚠ Host is replaced by discovery output when `service-name` is set (example uses dummy `bolt://0.0.0.0:0` on purpose). | Missing → BindEach error naming the instance; bad scheme → driver error at ctor. |
| `service-name` | string | — | Resolve the address through a discovery backend, once at startup (§2.4). ⚠ Requires a matching backend registered via `discovery.RegisterDiscovery`. | Unregistered backend → boot error "neo4j: resolve service …". |
| `scheme` | string | — | Narrows discovery to endpoints of one transport scheme; only consulted with `service-name`. | — |
| `discovery` | string | `default` | Which registered backend resolves `service-name`. | Wrong name → boot error from discovery. |
| `driver` | string | `DefaultDriver` | Selects a registered `Driver` (registry in driver.go:37). | Unknown name → boot error "neo4j driver not found". Duplicate `RegisterDriver` panics. |

### 3.2 Auth & connection pool

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `username` | string | — | BasicAuth together with `password`/`realm`. ⚠ Empty `username` ⇒ NoAuth — anonymous connection. | Empty on an auth-enabled server → fail-fast connectivity error at boot. |
| `password` | string | — | See above. | Wrong → fail-fast error. |
| `realm` | string | — | Auth realm passed to BasicAuth. | — |
| `max-connection-pool-size` | int | 100 | Max connections per host (driver semantics). | Too low → `connection-acquisition-timeout` errors under burst. |
| `max-connection-lifetime` | duration | 1h | Retire-and-reconnect window. | — |
| `connection-acquisition-timeout` | duration | 1m | Max wait for a pooled connection. | Too low → spuriously failed queries under burst. |
| `socket-connect-timeout` | duration | 5s | TCP connect timeout; ⚠ also bounds the startup fail-fast probe (starter.go:110,132-137). | 0/negative silently falls back to 5s for the probe. |
| `max-transaction-retry-time` | duration | 30s | Driver-level transient-error retry budget. ⚠ Stacks with `govern.*.max-retries` — two retry loops can multiply attempts. | Large value + governance retry → multiplied latency. |

### 3.3 TLS

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `tls.ca-file` | string | — | CA bundle into `RootCAs`. ⚠ Only effective for `+s`/`+ssc` URI schemes — encryption is chosen by the scheme; `tls.enabled` is a shape-parity placeholder and does NOT turn encryption on (driver.go:84-89 comment). | Set with plain `bolt://` → silently ignored. |
| `tls.cert-file` / `tls.key-file` | string | — | Mutual-TLS client certificate (static provider). ⚠ Both together. | Unreadable/invalid → boot error "neo4j: load client certificate". |
| `tls.server-name` | string | — | Override peer name verification. | — |
| `tls.insecure-skip-verify` | bool | false | Skip verification. | Convenience only; obvious risk. |

### 3.4 Instrumentation

There are **no instrumentation config keys** (no level, no skip list, no argument cap): the
helpers' observation is unconditional and emitted by module-local instrumentation
([observe.go]); trace/metric ride the OTel globals starter-otel installs
(`spring.observability.*`).

Reconciliation: the 14 `value:"..."` tags in the starter (Config fields) are exactly the keys
above — `grep -rhoE 'value:"[^"]+"'` matches both ways.

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components include "neo4j:graph", "neo4j:analytics"
docker stop starter-neo4j        # indicator runs VerifyConnectivity → component flips DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start starter-neo4j       # flips back UP on the next probe — no restart
```


### 4.2 Observability — what this starter actually emits

```bash
grep _app_neo4j_access app.log | tail -1
# system=neo4j op=query status ok duration=...  (only for StarterNeo4j.Query;
# success with Cypher at Debug, plain success at Info, failure at Warn)
curl -s :9090/metrics | grep -E 'db.client.(operation.duration|active_requests)'
# per-Query duration histogram + in-flight gauge, db.system=neo4j
# Jaeger (example-otel compose): span "query" with db.statement=<Cypher>
```

Counter-drill: call `neo4j.ExecuteQuery` directly — no span, no metric, no access log. That
silence is the un-intercepted path, not a broken pipeline (§2.2).

### 4.3 Resilience drill (example-cloudnative / example-load shape)

```properties
govern.enabled=true
govern.default.enabled=true
govern.default.rate-limit=5
```

```bash
go run ./example-cloudnative -manual   # self-asserts: burst of 15 → some admitted,
                                       # some rejected with resilience.ErrRateLimited
```

Fault injection (hot-reload, example-load): flip `govern.fault.enabled=true`,
`govern.fault.rate=0.5`, `govern.fault.error=timeout` in `conf/app.properties` while the load
binary runs — the error breakdown moves without restart.

### 4.4 Discovery drill

Register a backend (`discovery.RegisterDiscovery`, see example/discovery.go), set
`spring.neo4j.graph.service-name=neo4j-cluster` with dummy `uri=bolt://0.0.0.0:0`. Boot logs
`neo4j client initialized, uri=bolt://127.0.0.1:7687` — the spliced address, not the dummy.
Kill that instance: queries keep failing — the address was resolved once (§2.4); restart the
app (or let a platform do it) to re-resolve.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "failed to verify neo4j connectivity" | Server unreachable / wrong credentials / TLS mismatch | Fail-fast probe is unconditional [starter.go:109-119]; fix connectivity or auth. |
| Boot fails "neo4j driver not found: X" | `driver` names nothing registered | Register via `StarterNeo4j.RegisterDriver` in an init, or use `DefaultDriver`. |
| Boot fails "neo4j: resolve service X" | `service-name` set but no backend registered under `discovery` | Register the backend (example/discovery.go) or drop service-name. |
| Queries work but no spans/metrics/access log | Code calls `neo4j.ExecuteQuery` directly, bypassing the seam | Swap to `StarterNeo4j.Query` / wrap with `StartSpan` (§2.2); import starter-otel for real export. |
| No protection though governance is on | Session code not routed through `Query`/`RunWithResilience`, or a raw driver passed (type-assert misses) | Route through the helpers; always pass the `*Client` wrapper [command.go:111-116]. |
| TLS settings appear to do nothing | URI scheme is plain `bolt://`/`neo4j://` | Switch scheme to `neo4j+s://`/`bolt+s://`; tls.* only customizes trust for encrypted schemes [driver.go:84-89]. |
| Discovery endpoint changed, client still dials the old address | One-shot resolution — no dialer hook in the driver | Rebuild/restart the client (§2.4); or front with mesh/sidecar LB. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 14 instance keys + tls group (4) |
| Required | 1 (`uri`) |
| Quickstart external deps | 1 (Neo4j) |
| "Watch out" entries | 6 |

Design suspects (audit ledger — kept from the previous audit, plus new):

- Protection only via opt-in `Query`/`RunWithResilience` — direct session use is silently
  unguarded/unobserved. Audited again during the 2026-08-28 guard-unification pass and
  confirmed **SDK-blocked, not starter negligence**: `neo4j.SessionWithContext` contains
  unexported methods, so no wrapper outside the driver package can implement it (ruling out a
  guarded session from an overridden `NewSession`), and `neo4j.ExecuteQuery` is a package-level
  generic function (Go forbids type-parameterized methods, so it cannot be overridden on
  `*Client` either). The helpers are therefore the deepest reachable seam; governance applies
  automatically once they are used (no resilience flag). Covered by resilience_test.go.
- `Query` type-asserts its driver argument back to `*Client` to find the executor — a
  differently-typed custom driver silently loses the guard (command.go:111-116).
- `tls.enabled` is a dead placeholder key here (scheme owns encryption) — candidate for a
  slimmer tls shape (driver.go:84-89); one-shot discovery resolution (vs live re-resolution
  in every other client starter) is forced by the missing dialer hook (§2.4).
- Fail-fast probe reuses `socket-connect-timeout` as its bound — mixing "user's TCP budget"
  and "startup probe budget" in one key (starter.go:110,132-137).
