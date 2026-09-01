# starter-mongodb Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health/health.go`) and the runnable examples ([example/](example/),
[example-otel/](example-otel/), [example-cloudnative/](example-cloudnative/),
[example-load/](example-load/) — file:line spot-checks in brackets below). **MongoDB semantics
and the mongo-driver v2 API are [the driver's own
documentation](https://www.mongodb.com/docs/languages/go/go-driver/current/)** — everything
below is go-spring's increment.

**Activation**: any `spring.mongodb.*` key (the module is `OnProperty("spring.mongodb")`, a
prefix check). Each `spring.mongodb.<name>` entry creates one `*StarterMongoDB.Client` bean
named `<name>` (it embeds `*mongo.Client`), plus a health indicator named `mongo:<name>`.

---

## 1. Complete worked project

Two instances in one service — one dialed directly from its URI, one resolved through
discovery — plus health/actuator, otel, and governance. File tree:

```
demo/
├── go.mod
├── discovery.go
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    go.mongodb.org/mongo-driver/v2  v2.6.0
    go-spring.org/spring            v1.3.x
    go-spring.org/cloud             latest
    go-spring.org/starter-mongodb   latest
    go-spring.org/starter-actuator  latest   // optional: readiness + health
    go-spring.org/starter-governance latest  // optional: resilience/fault policy
    go-spring.org/starter-otel      latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-mongodb"
    _ "demo/service"
)

func main() { gs.Run() }
```

**discovery.go** — a company adapter registers its naming service once; here a static backend
keeps the demo self-contained (copied from [example/discovery.go](example/discovery.go)):

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default", discovery.NewStaticDiscovery(
        discovery.Endpoint{Addr: "127.0.0.1:27017", Healthy: true},
    ))
}
```

**service.go** — inject the wrapper, use the full mongo-driver surface:

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterMongoDB "go-spring.org/starter-mongodb"
    "go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
    // Always the wrapper type *StarterMongoDB.Client. It embeds
    // *mongo.Client, so Database/Collection/StartSession/ping promote
    // unchanged. It is NOT injectable as a bare *mongo.Client.
    Main *StarterMongoDB.Client `autowire:"a"`   // direct URI
    Disc *StarterMongoDB.Client `autowire:"disc"` // discovery (service-name)
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            coll := s.Main.Database("test").Collection("kv")
            _, _ = coll.InsertOne(ctx, bson.M{"key": "k", "value": "v"})
            // The discovery client round-trips the same way; its address
            // came from the backend, not the (dummy) URI host.
            _ = s.Disc.Database("test").RunCommand(ctx, bson.M{"ping": 1}).Err()
        }
    })
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- direct instance --------------------------------------------------------
spring.mongodb.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.a.max-pool-size=100
spring.mongodb.a.min-pool-size=1
spring.mongodb.a.max-conn-idle-time=5m
spring.mongodb.a.server-selection-timeout=10s

# --- discovery instance -----------------------------------------------------
# The uri host is a non-resolvable dummy ON PURPOSE: service-name takes over
# addressing, so a successful connection proves discovery is wired.
# directConnection=true keeps the driver on the dialed seed instead of doing
# its own replica-set topology discovery (see §3.1 ⚠ note).
spring.mongodb.disc.uri=mongodb://nonexistent.invalid:27017/?directConnection=true
spring.mongodb.disc.service-name=mongo-cluster
spring.mongodb.disc.server-selection-timeout=10s

# --- governance: policy for the dial seam (rate-limit makes the dial
#     protection observable; breaker/retry/timeout also apply) ---------------
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5

# --- actuator + otel --------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**Verify** (start MongoDB first: `docker run -d -p 27017:27017 mongo:7`, plus a Jaeger/OTLP
collector on :4317 if you enable the trace exporter):

```bash
go run .                          # boot fails fast if the server is unreachable (startup ping)
curl -s :9370/readyz | jq .       # components include "mongo:a" and "mongo:disc"
curl -s :9090/metrics | grep db.client   # db.client.operation.duration / db.client.active_requests
grep _app_mongodb_access app.log | tail -3   # one access record per command
# Jaeger UI (http://localhost:16686): spans named insert/find/ping, attr db.system=mongodb
```

The [example-cloudnative](example-cloudnative/) flagship self-asserts this same set (health,
direct + discovery round-trip, rate-limited dial burst, hot-reloaded config) and exits non-zero
on failure — run its `check.sh` as the executable version of this section.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-mongodb
  └─ gs.Module(OnProperty("spring.mongodb")) fires when any spring.mongodb.* key exists
        └─ conf.BindEach("${spring.mongodb}") → one Config per <name> entry
              ├─ Provide(newClient).Name(<name>).Init((*Client).Init).Destroy((*Client).Destroy)
              └─ Provide health.Indicator named "mongo:<name>", exported As[health.Indicator]

gs.Run()
  ├─ ctor newClient [starter.go:80]: ApplyURI → timeouts/pool/auth → tls.Build
  │   → SetMonitor(command monitor, lazy observer) [starter.go:118]
  │   → newLiveResolver (discovery watch when service-name set, mesh off) [starter.go:121]
  │   → SetDialer(shared dialerWrapper) [starter.go:147]
  │   → mongo.Connect → fail-fast Ping bounded by connect-timeout (10s fallback)
  │     [starter.go:160-169] — a dead server fails the BOOT, not the first query
  ├─ gs field-injects Client.Observability (${observability:=} — a top-level key, see §3.3 ⚠)
  ├─ Init [client.go:97]: NewDB("mongodb", ...) observer → resource label
  │   → fault.WrapExecutor(resilience.ExecutorFor(resource)) → resilience.WrapExecutor
  │   → swap dialerWrapper.dial = resilience.NewDialer(base, exec, resource)
  ├─ readiness: mongo:<name> indicator runs client.Ping against the live server
  └─ SIGTERM → Destroy [client.go:112]: exec.Close → resolver.Stop → client.Disconnect
```

### 2.2 The two seams — and the asymmetry that shapes them

The mongo driver v2 exposes **no per-operation hook comparable to go-redis's ProcessHook**
([client.go:44-47], [command.go:18-21]). The starter therefore splits what other client
startners do in one hook chain into two seams:

- **Observation rides the command monitor** ([command.go:53]): `event.CommandMonitor`
  Started/Succeeded/Failed events, correlated by (connection id, request id). Every command —
  insert, find, ping, hello — opens a span + bumps the in-flight gauge in Started and closes
  them in Succeeded/Failed. The monitor reads the observer lazily via an atomic pointer
  because it is installed in the ctor, before gs field-injects `Observability`; the nil guard
  covers the startup ping path ([command.go:41-45]).
- **Resilience/fault ride the dial layer**: `Init` wraps the dial function with
  `resilience.NewDialer` — breaker/limiter/bulkhead/timeout/fault apply **per new connection**,
  not per command. Already-open pooled connections run at full speed ([client.go:47-51]).
  This is the honest statement of the family asymmetry: there is **no per-request
  instrumentation for resilience here** — a breaker trips on connection failures and caps
  connection churn; slow-but-successful commands never touch it.

The dial swap works without rebuilding the client because the ctor hands the driver a shared
`dialerWrapper` whose `dial` field `Init` later mutates ([starter.go:143-147], [client.go:104-106]).

Why hand-rolled instead of otelmongo: the official instrumentation targets the v1 driver and
its CommandMonitor type is incompatible with v2; the bridge here delegates to the shared
observe kit so MongoDB emits the same vocabulary as every other client starter
([command.go:47-52]).

### 2.3 One command through the layers: `FindOne` on the direct instance

1. `coll.FindOne(ctx, ...)` runs on the embedded `*mongo.Client` — no wrapper interception;
   every driver method promotes unchanged.
2. If the pool has no idle connection, the driver calls `dialerWrapper.DialContext` →
   resilience executor asks for a permit (resource label `mongodb:<service-name or uri>` —
   per instance, [client.go:99]); over the rate limit the dial is rejected and the operation
   surfaces `resilience.ErrRateLimited`. With service-name set, the base dial first asks the
   discovery Resolver to pick a live endpoint and ignores the URI address ([starter.go:130-137]).
3. The driver sends the `find` command; the command monitor's `Started` fires:
   `obs.Start(ctx, "find", "test")` — span name = command name, argument = database name.
4. The reply fires `Succeeded` (or `Failed`): the span ends, `db.client.operation.duration`
   is recorded, the in-flight gauge is balanced, and one `_app_mongodb_access` log record is
   emitted at Info (Warn + `error` field on failure).
5. A **reused pooled connection skips step 2 entirely** — that is the dial-only resilience
   seam in action.

---

## 3. Per-key behavior reference

Instance keys live under `spring.mongodb.<name>.` (bound via `conf.BindEach`).
Exception: the observability block is a **top-level** key (see §3.3 ⚠).

### 3.1 Connection & addressing

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `uri` | string | — | **Required** (expr `$ != ''`). Parsed by `ApplyURI`; driver options in the URI win unless overridden below. | Missing/empty → boot error at binding. |
| `username` | string | — | When non-empty, sets a `options.Credential` from username/password/auth-source/auth-mechanism [starter.go:97-104]. Empty → credentials come solely from the URI. | Setting username but forgetting password/auth-source → auth failure at the startup ping. |
| `password` | string | — | Part of the credential above. ⚠ Only effective together with `username`. | — |
| `auth-source` | string | — | Credential verification database, e.g. `admin`. ⚠ Only with `username`. | Wrong db → "Authentication failed" at boot ping. |
| `auth-mechanism` | string | — | e.g. `SCRAM-SHA-256`; empty = driver negotiates. ⚠ Only with `username`. | Unsupported mechanism → boot ping error. |
| `connect-timeout` | duration | `10s` | Passed to the driver AND bounds the fail-fast startup ping (0 → 10s fallback, [starter.go:182-187]). | Too small → boot ping spuriously times out on slow networks. |
| `server-selection-timeout` | duration | `0` | 0 = driver default (30s). How long the driver waits for a suitable server. | Too small + discovery latency → "server selection timeout". |
| `max-pool-size` | uint64 | `100` | Max connections per server. 0 would mean "use default" — but the starter passes 100 explicitly when unset. | Too small → ops queue waiting for a pool slot. |
| `min-pool-size` | uint64 | `0` | Min pooled connections (always applied, even 0). | — |
| `max-conn-idle-time` | duration | `0` | 0 = no limit; e.g. `5m` prunes idle conns. ⚠ With `service-name`, a finite value recycles connections onto updated endpoints without a restart. | `0` + discovery → conns linger on a removed endpoint until they break. |
| `service-name` | string | — | Resolve addressing via the registered discovery backend; a Resolver-backed dialer replaces the URI hosts per connection [starter.go:126-137]. ⚠ **Bypasses MongoDB's own topology discovery** (replica set / mongos) — the driver dials whatever the naming service hands out; pair with `directConnection=true` in the URI ([config.go:80-84]). Ignored in mesh mode (sidecar owns discovery+LB). | Without `directConnection=true` on a replica-set URI → "no such host"/topology errors; the dummy-URI trick only proves discovery when the resolver is actually consulted. |
| `scheme` | string | — | Narrows discovery endpoints to one transport scheme (e.g. `tls`). Only consulted when service-name is set. | — |
| `discovery` | string | `default` | Which registered discovery backend resolves service-name. | Unregistered backend → boot error from discovery.NewResolver. |
| `tls.*` | group | off | Shared `tlsconf` block (enabled/ca-file/cert-file/key-file/server-name/insecure-skip-verify); `tls.Build` error fails the boot [starter.go:105-112]. Enabled=false → no TLS unless the URI itself requests it (`mongodbs://` / `tls=true`). | Partial config → boot error "mongodb: build TLS". |

### 3.2 Resilience / fault (govern.*, not under the instance prefix)

Policy keys live at the top level under `govern.*` (starter-governance's governance center);
this starter resolves `resilience.ExecutorFor("mongodb:<service-name|uri>")` and
`fault.InjectorFor` in `Init` [client.go:97-102]. Relevant keys (see starter-governance USAGE
for the full set): `govern.enabled`, `govern.driver`, `govern.<driver>.rate-limit` /
`error-threshold` / `open-duration` / `max-retries` / `timeout`, and the `govern.fault.*`
injection block (enable/rate/error). ⚠ Remember the seam is the **dial layer**: a breaker
policy manifests as rejected *connections*; a fault injection fires per dial, not per command.

### 3.3 Observability

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `observability.level` | string | `brief` | Access log: `off` / `brief` / `detailed` (detailed appends the argument = database name). ⚠ This is bound as `Client.Observability` with `${observability:=}` — a **top-level absolute property** (`observability.level=...`), NOT `spring.mongodb.<name>.observability.level`; one setting is shared by all instances. | Setting it under the instance prefix → silently ignored, log stays `brief`. |
| `observability.maxArgBytes` | int | `512` | Bound of the captured argument in detailed mode. ⚠ Same top-level rule. | — |
| `observability.skipOps` | list | — | Suppresses span + metric + log together for listed command names (`find`, `insert`, `ping`, ...). ⚠ Same top-level rule. | — |

Trace and metric themselves have **no per-instance switch** here: they ride the OTel globals
that starter-otel installs (`spring.observability.*`); without starter-otel they are no-ops.

There is no `driver` key — this starter has no driver registry ([driver.go:17-21]).

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components include "mongo:a", "mongo:disc"
docker stop <mongo>              # indicator runs client.Ping → component flips DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <mongo>
```

The indicator is unconditional — no disable switch ([starter.go:55-57]).

### 4.2 What observation actually emits

```bash
curl -s :9090/metrics | grep db.client
# db.client.operation_duration_seconds...{db.operation="find",db.system="mongodb",status="ok"}
# db.client.active_requests{db.operation=...,db.system="mongodb"}
grep _app_mongodb_access app.log | tail -1
# brief: db.operation=find status=ok duration_ms=1.2 ; failure adds error=... at Warn
```

- Spans: named after the MongoDB command (`find`, `insert`, `ping`), attribute
  `db.system=mongodb`, `db.operation=<command>`, `db.statement=<database>` in detailed mode.
- The startup ping is also observed — unless the observer is still nil (pre-Init), which the
  monitor's nil guard covers ([command.go:44-45]).

### 4.3 Dial-layer resilience drill (from example-cloudnative)

```properties
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5
spring.mongodb.a.max-pool-size=100   # room for the burst to force fresh dials
```

Fire 40 concurrent `InsertOne` on a cold pool: dials beyond the limit fail with
`resilience.ErrRateLimited` surfaced as the operation's connection error; admitted ops succeed
([example-cloudnative/example.go:197-225]). Then note the flip side: once the pool is warm,
the same burst sails through — protection is connection-level. Flip `govern.*` at runtime —
the executor hot-reloads without restart (governance center).

### 4.4 Fault injection + load drill (example-load)

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic    # or: timeout / reset
```

Run [example-load](example-load/): the upsert/FindOne closed loop prints throughput, latency
percentiles and an error breakdown; faults fire at the dial seam (fresh connections), so
`max-conn-idle-time` shortens the fault exposure window.

### 4.5 Discovery address recycling

Scale/move the backing instance; every **new** connection asks the Resolver for a live
endpoint. With `max-conn-idle-time=5m` the pool fully migrates within that window without a
restart. Verify via `_app_mongodb_access` records or by stopping the old endpoint and watching
`mongo:<name>` health stay UP.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `mongodb: ping <uri>: ...` | Unreachable server / wrong credentials / TLS mismatch — the fail-fast ping is unconditional | Fix connectivity/credentials; `connect-timeout` bounds the probe. |
| Boot fails at binding on `uri` | `uri` empty — it is expr-validated non-empty | Set `spring.mongodb.<name>.uri`. |
| Boot fails "build TLS" / "build discovery resolver" | Partial `tls.*` config; `discovery` names nothing registered | Complete the tlsconf block; register the backend via `discovery.RegisterDiscovery`. |
| Discovery client errors "no such host" / topology errors | `service-name` bypasses driver topology discovery | Add `directConnection=true` to the URI, or drop service-name for replica-set/mongos URIs. |
| Ops fail with `ErrRateLimited` under burst | Governance rate-limit on the dial seam | Raise `govern.<driver>.rate-limit` or `max-pool-size`/`min-pool-size` (warm pool skips dials). |
| Breaker never opens despite slow queries | By design — resilience is dial-layer only; slow-but-connected commands are invisible to it | Alert on `db.client.operation.duration` instead; see §2.2. |
| No spans/metrics though commands work | starter-otel not imported — the monitor rides the OTel globals | `_ "go-spring.org/starter-otel"` + `spring.observability.*`. |
| `observability.level=detailed` has no effect | Key set under the instance prefix; binding is top-level | Use `observability.level=detailed` at the top level (§3.3 ⚠). |
| Access log too chatty (every `ping`/`find`) | brief level logs every command | `observability.skipOps=ping,hello` or `observability.level=off` (log only). |
| Injecting `*mongo.Client` fails | The bean is the wrapper `*StarterMongoDB.Client` | Autowire the wrapper type; driver methods promote unchanged. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 14 instance keys + tls group + 3 top-level observability |
| Required | 1 (`uri`) |
| Quickstart external deps | 1 (MongoDB) |
| "Watch out" entries | 6 (directConnection, top-level observability ×3 counted once, warm-pool bypass, username-gated credential) |

Design suspects (audit ledger; kept from the previous edition, additions marked NEW):

- `service-name` silently disables driver topology discovery and needs the user's
  `directConnection=true` cooperation — the starter cannot inject it itself (URI is opaque).
- Health indicator has no disable switch (family asymmetry: redigo has `health.enabled`).
- Per-instance `observability` binds as a **top-level** key shared by all instances rather
  than under the instance prefix — surprising and undocumented in config.go.
- Resilience is dial-layer only; users expecting per-command breaker semantics (as in
  starter-go-redis) get silent non-protection for warm-pool command failures. Re-audited in
  the 2026-08-28 guard-unification pass and confirmed **SDK-blocked at the command level**:
  the v2 driver's only per-command hook is `event.CommandMonitor`, which is observe-only
  (events carry no reject capability), the internal `driver.Deployment` seam is not
  constructible outside the driver, and `ClientOptions` exposes no command-executor
  override. The dial layer plus the command monitor (observation) is therefore the deepest
  reachable seam; revisit if the v2 driver grows a reject-capable hook.
- (NEW) `command.go` bridges to the observe kit with span arg = database name only; command
  documents (filter/update) are never captured even in detailed mode — deliberate (payload
  safety) but makes `detailed` barely differ from `brief` for MongoDB.
- (NEW) The health indicator Provide relies on `gs.TagArg(name)` to fetch the wrapper by bean
  name — correct today, but a second Client-typed bean family would make the tag ambiguous.
