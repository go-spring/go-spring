# starter-influxdb Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `observe.go`, `driver.go`,
`health.go`) and the runnable [example/](example/) — file:line spot-checks in brackets
below. **InfluxDB semantics and the influxdb-client-go API are [the client's own
documentation](https://docs.influxdata.com/influxdb/v2/api-guide/client-libraries/go/)** —
everything below is go-spring's increment.

**Activation**: any `spring.influxdb.instances.*` key (the module is `OnProperty("spring.influxdb")`, a
prefix check [starter.go:41]). Each `spring.influxdb.instances.<name>` entry creates one
`*StarterInfluxdb.Client` bean named `<name>`, plus a health indicator named
`influxdb:<name>` (skip it with `health=false`); the startup probe is opt-in via `ping=true`.

---

## 1. Complete worked project

Two instances against one server (the example's own topology), real write + Flux query, plus
actuator/otel. File tree:

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
    github.com/influxdata/influxdb-client-go/v2 v2.14.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-influxdb   latest
    go-spring.org/starter-actuator   latest   // optional: readiness + /metrics
    go-spring.org/starter-otel       latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-influxdb"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper, write through the guarded path, read back with a Flux
query, and demonstrate the managed async writer:

```go
package service

import (
    "context"

    influxdb2 "github.com/influxdata/influxdb-client-go/v2"
    "go-spring.org/spring/gs"
    StarterInfluxdb "go-spring.org/starter-influxdb"
)

type Service struct {
    // Always the wrapper type *StarterInfluxdb.Client. It embeds the
    // influxdb2.Client interface, so QueryAPI/WriteAPI/DeleteAPI/... are
    // promoted unchanged.
    Main *StarterInfluxdb.Client `autowire:"a"` // org+bucket configured
    Raw  *StarterInfluxdb.Client `autowire:"b"` // url+token only (query-only use)
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 1. Blocking write — resilience-guarded, configured org/bucket.
            p := influxdb2.NewPointWithMeasurement("cpu").
                AddTag("host", "server-01").
                AddField("usage_idle", 42.5)
            if err := s.Main.WritePoints(ctx, p); err != nil {
                panic(err)
            }

            // 2. Flux query through the embedded API (still declared and
            //    executor-gated at the transport layer — see §2.3).
            raw, err := s.Main.QueryAPI(s.Main.Org()).QueryRaw(ctx,
                `from(bucket:"example") |> range(start: -1m) |> filter(fn: (r) => r._measurement == "cpu")`,
                influxdb2.DefaultDialect())
            _ = raw // CSV; contains usage_idle,42.5

            // 3. Async buffered write — errors drain into go-spring's log.
            w := s.Main.ManagedWriteAPI()
            w.WritePoint(ctx, influxdb2.NewPointWithMeasurement("mem").
                AddField("used_percent", 61.0))
            _ = err
        }
    })
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- instance "a": write+query client --------------------------------------
spring.influxdb.instances.a.server-url=http://127.0.0.1:8086
spring.influxdb.instances.a.auth-token=go-spring-example-token
spring.influxdb.instances.a.org=go-spring
spring.influxdb.instances.a.bucket=example

# --- instance "b": second client, same server (query-only for us) ----------
spring.influxdb.instances.b.server-url=http://127.0.0.1:8086
spring.influxdb.instances.b.auth-token=go-spring-example-token

# --- actuator + otel --------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**Verify** (start InfluxDB first — reuse the example's self-initializing compose file:

```bash
cd starter-influxdb/example && docker compose -p demo up -d
# wait for the one-shot setup (admin/org/bucket/token) to report pass:
curl -fsS http://127.0.0.1:8086/health | grep '"pass"'

go run .                          # boot fails fast if the server is unreachable (needs ping=true; off by default)
curl -s :9370/readyz | jq .      # components include influxdb:a and influxdb:b
curl -s :9370/metrics | grep -E 'db.client'   # call+attempt duration histograms + active gauge
grep _app_influxdb_access app.log | tail -3   # one access record per call
docker exec influxdb-example influx query \
  'from(bucket:"example") |> range(start:-1m) |> filter(fn:(r) => r._measurement=="cpu")' \
  --org go-spring --token go-spring-example-token   # usage_idle=42.5
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-influxdb
  └─ gs.Module(OnProperty("spring.influxdb")) fires when any spring.influxdb.instances.* key exists
        └─ conf.BindEach("${spring.influxdb}") → one Config per <name> entry
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │         IndexArg(2, ?Driver))    // optional Driver bean; several
              │                                    // coexist → ${..driver} names one
              │     .Name(<name>).Destroy((*Client).Destroy)
              └─ Provide health.Indicator named "influxdb:<name>"
                    (injects the client by name via TagArg, exported as health.Indicator)

gs.Run()
  ├─ ctor newClient [starter.go:79]:
  │    1. optional Driver bean — when none is present the starter falls back
  │       to the bundled DefaultDriver (d == nil [starter.go:83-85]); when
  │       several Driver beans coexist, the entry selects one by name:
  │       spring.influxdb.instances.<name>.driver = <bean-name> (empty = the single
  │       Driver bean by type; naming a missing bean fails startup)
  │    2. the ctor bundles the governance beans into
  │       cloud.ClientParams{Resilience: mgr, Fault: inj} and hands it to
  │       driver.CreateClient [driver.go:67]
  │    3. driver.CreateClient → NewClient [client.go:102]: builds the executor
  │       exec = params.ExecutorFor("influxdb", "influxdb:<server-url>") from the
  │       governance bundle, then installs the chain in the dynamicTransport the
  │       raw client rides — dyn.Swap(declareTransport{base:
  │       resilience.NewRoundTripper(http.DefaultTransport, exec)}) — so the
  │       declaration+governance chain is live the moment the driver returns
  │    4. ping=true only: fail-fast probe HealthCheck(ctx, w) → /health must report
  │       "pass", otherwise the just-assembled client is torn down (executor +
  │       connection) and the boot fails [starter.go:99-102]
  ├─ readiness: indicators flip UP (each probe = one /health round trip)
  └─ SIGTERM → Destroy [client.go:119]: Client.Close() — flushes the async
       writer's pending batches — then exec.Close()
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface,
`driver.go:52`). A company/umbrella starter may provide its own `Driver` as an **optional
container bean** (a `gs.Provide(func() StarterInfluxdb.Driver{...})`, so it can inject config
bound from the properties file at wiring time); every instance under `spring.influxdb` is then
built through it. The driver receives the `cloud.ClientParams` bundle along with the config
and returns the module's `*Client` — it builds the raw client and hands it to `NewClient`
together with `params`, which applies governance while the client is built; a custom driver that
installs no `dynamicTransport` gets a client with no transport-level declaration/governance.
When no such bean exists the starter falls back to the bundled `DefaultDriver`
(`driver.go:67`) inside assembly (`starter.go:83-85`). There is no per-config `driver` key.

When the instance sets `ping=true`, an unreachable/uninitialized server fails the boot — the
process never reaches "serving" with a dead InfluxDB; with the default `ping=false` the boot
succeeds and the first request fails instead. Note the OSS server reports differently before its
one-time setup finishes, so bootstrap-order races surface at boot (with `ping=true`), not as
intermittent write failures (see the README's design notes).

### 2.2 Request chain — exact order and why

Every HTTP request the SDK makes goes through the swapped transport:

```
influxdb-client-go → declareTransport (declares the Operation on the ctx) →
resilience round-tripper (exec.Execute: opens the call span, records the call- and
attempt-level duration metrics + access log) → http.DefaultTransport → network
```

Rationale (source comments [client.go:102-113], [observe.go]):

- **Declaration outermost, resilience inside it**: `declareTransport` puts the request's
  Operation on the context and delegates to the resilience round-tripper, which reads it at
  `Execute` entry. The declaration MUST sit *outside* the executor — a declaration made per
  attempt (as the round-tripper's `base`) would be read by nobody, since the emitter only
  reads the Operation once, at call entry.
- **Resilience emits, the starter declares**: the executor permit (rate limiter / breaker /
  injected fault, scoped to the service key `influxdb:<server-url>` — one scope per
  instance, not per operation) is checked first; the resilience layer then owns span +
  duration metrics + access log. influxdb-client-go ships no OTel instrumentation of its
  own, but that is no longer this starter's concern: it emits nothing.
- The span name is `"<METHOD> <path>"`, e.g. `POST /api/v2/write`, while the metric label is
  bounded: `db.operation=<method>` (`post`/`get`/`delete`…); the URL path — which can carry
  the org/bucket/measurement — rides as `db.statement` in the span and log only, never a
  label [observe.go].
- HTTP 5xx responses are mapped to retryable failures inside the round-tripper, and request
  bodies are rewound per retry attempt (`GetBody`); requests without a rewindable body
  simply run once (resilience/roundtripper.go:83-95).

### 2.3 One write and one query through the chain

**`WritePoints` (blocking)** [client.go:132]:

1. org/bucket guard — empty config returns the pointed error
   `influxdb: write helpers need org and bucket` without touching the network.
2. `WriteAPIBlocking(org, bucket)` is created and the whole write runs inside a **second**
   executor run (`o.exec.Execute`) — the per-call guard on the overload-sensitive path.
3. The SDK issues `POST /api/v2/write`; that request passes through declareTransport and the
   transport-level executor, so a single WritePoints crosses the executor twice (both
   layers share the same service key and therefore the same limiter/breaker state —
   rejections at the inner layer count in the outer one's view).

**`QueryAPI(org).QueryRaw` (embedded SDK method)**: no per-call guard is added
(query-time resilience was deliberately left out — see the README's design notes; a
GuardedQuery would be additive). The request is still declared and executor-gated at the transport layer, because
every request is.

**`ManagedWriteAPI` (async)** [client.go:152]: background batching, **not** routed through
any executor — the SDK's own batching retries govern it; guarding per point would
double-count (see the README's design notes). Failed batches land on the `Errors()` channel, which the
wrapper drains into go-spring's log (`influxdb: async write failed: ...`) — an undrained
channel would block the writer on its first failure. Destroy's `Client.Close()` flushes
pending batches.

### 2.4 Why a dynamicTransport at all

The SDK fixes the `*http.Client` at construction (`Options.SetHTTPClient`), but the
declaration/governance chain is built from beans the driver must not depend on. DefaultDriver
therefore installs a pass-through `dynamicTransport` [driver.go:68-72] and hands it to
`NewClient`, which the driver calls with the `cloud.ClientParams` bundle; `NewClient` then
swaps the real chain in while it builds the client [client.go:102-113] — so there is no window
between construction and governance: the chain is live the moment the driver returns. A custom
driver that passes no such transport gets a client with no transport-level
declaration/governance — resilience is then unavailable for that client [client.go:102-113];
the per-call `WritePoints` executor still works.

---

## 3. Per-key behavior reference

All keys live under `spring.influxdb.instances.<name>.` — per-instance prefix binding via
`conf.BindEach`. Complete list (reconciled with
`grep -rhoE 'value:"[^"]+"' --include='*.go'`, both directions):

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `server-url` | string | — | InfluxDB base URL; also seeds the resilience service key `influxdb:<server-url>`. ⚠ HTTPS is expressed by the URL scheme — there is no `tls.*` block. | Empty → BindEach fails the boot (`expr:"$ != ''"` [config.go:26]); wrong host → error on first use (or the fail-fast /health probe fails the boot when `ping=true`). |
| `auth-token` | string | — | API token passed to the SDK. | Empty → boot error; wrong token → writes/queries fail per request (the /health probe may still pass — it does not authenticate). |
| `org` | string | `""` | Default org for `WritePoints`/`ManagedWriteAPI` and `Org()`. ⚠ Required **at call time**, not at wiring: a client without org/bucket still serves Query/Delete APIs. | Missing → `WritePoints` returns an error, `ManagedWriteAPI` **panics** (inconsistent failure modes — design suspect). |
| `bucket` | string | `""` | Default destination bucket for the write helpers. ⚠ Same call-time rule as `org`. | Same as `org`. |
| `ping` | bool | `false` | Startup connectivity probe: when true the ctor runs `HealthCheck` (a `/health` round trip) once and fails the boot if it errors, restoring fail-fast. Off by default so a server that is not up yet does not block startup. | `ping=true` against a down server → boot error `failed to reach influxdb server …`. |
| `health` | bool | `true` | Whether this instance contributes a `health.Indicator` (name `influxdb:<name>`) for the actuator's readiness/startup probes. Set false to keep the instance out of the aggregated health report. | `health=false` → no `influxdb:<name>` component in `/readyz`. |

No `driver` key: client assembly is owned by an optional `Driver` bean (see §2.1) or the
bundled `DefaultDriver`. No `tls.*` group, no `service-name`/discovery, no timeout keys —
everything not listed is the SDK's own defaults.

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components: influxdb:a, influxdb:b
docker stop influxdb-example     # probe = one /health round trip per check
curl -s :9370/readyz             # 503 OUT_OF_SERVICE, message from the server
docker start influxdb-example
```

The probe maps only `status=pass` to healthy; `fail` carries the server's message
(`influxdb: health status fail: <msg>`) [health.go:49-57].

### 4.2 What observability actually emits

The starter **declares** each request's identity; the **resilience layer emits** the signals
(the single emitter on the executor chain, so retries fold into one call). What you see:

| Signal | Name / shape |
|--------|--------------|
| Span | name = `"<METHOD> <path>"`, e.g. `POST /api/v2/write`; kind = internal (the emitter's call span); attributes `db.system=influxdb`, `db.operation=<method>`, `db.statement=<path>` (path truncated to 512 bytes) |
| Metric (call level) | `db.client.operation.duration` (histogram, s) + `db.client.active_requests` (UpDownCounter) — shared vocabulary with every DB-family starter; labels `db.system`, `db.operation`, `status` (the path is never a label) |
| Metric (attempt level) | `db.client.attempt.duration` (histogram, s) — one record per downstream attempt; a rejected call (rate-limited / open circuit) records none, since the downstream was never touched |
| Access log | tag `_app_influxdb_access`, one record per call at the log package's native levels: error → Warn; success with a request path → Debug (lazy); plain success → Info |
| Async-write failures | log tag `influxdb` (app tag), `influxdb: async write failed: <err>` [client.go:172] — the async batch never crosses the executor, so this line is the only failure signal the emitter cannot produce for it |

```bash
curl -s :9370/metrics | grep db.client
grep _app_influxdb_access app.log | tail -2
```

### 4.3 Server-down drill

```bash
docker stop influxdb-example
go run .   # boot fails: "failed to reach influxdb server http://..." — the probe
           # requires pass, not merely reachable
```

With the server dying *after* boot: readiness flips DOWN (§4.1); blocking writes return the
executor/transport error; the breaker (if a governance policy targets
`influxdb:<server-url>`) opens after its threshold and rejects without dialing.

### 4.4 Governance drill

With a governance rules source configured, a limiter/breaker policy on service
`influxdb:http://127.0.0.1:8086` applies to **every** request (write, query, health probe)
through the transport executor — hammer `WritePoints` and watch rejections surface as
`_app_influxdb_access` records and in the resilience layer's outcome counters. Flip the
policy at runtime; the executor hot-reloads without restart. Injected faults
(`spring.governance.client.fault.*`) strike at the same seam.

### 4.5 Async-writer drain

Stop the container, write via `ManagedWriteAPI`, restart — the SDK's batching retries
exhaust and the failure appears as `influxdb: async write failed` log lines, while the
process keeps running (async failures never become caller errors).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `failed to reach influxdb server ...` | Server down, wrong server-url, or OSS server not yet set up (first-boot migration) | Wait for `curl :8086/health` to report `pass`; check the URL scheme/host. |
| `panic: influxdb: write helpers need org and bucket` | `ManagedWriteAPI` with empty org/bucket | Set `spring.influxdb.instances.<name>.org/.bucket` — or use the embedded `WriteAPI(org, bucket)` directly. |
| `WritePoints` returns the org/bucket error | Same call-time gap, non-panicking shape | Same as above. |
| Writes fail but boot and health are green | Wrong `auth-token` — /health does not authenticate | Verify the token with `influx query --token ...`. |
| No spans/metrics despite requests flowing | starter-otel not imported | The emitter rides the OTel globals; import starter-otel (the access log still emits without it). |
| Queries return nothing right after a write | Bucket write-path settling (eventual visibility) | Retry window — the example itself polls up to 15s [example/main.go:81-91]. |
| Async writes vanish silently | Wrong mental model: `ManagedWriteAPI` failures are log lines, not errors | Grep `influxdb: async write failed`; use `WritePoints` when you need the error. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 instance keys |
| Required | 2 at wiring (`server-url`, `auth-token`) + 2 at call time (`org`, `bucket`) |
| Quickstart external deps | 1 (InfluxDB 2.x) |
| "Watch out" entries | 5 |

Design suspects (audit ledger — carried over plus new):

- `org`/`bucket` validated only at call time; `WritePoints` errors but `ManagedWriteAPI`
  **panics** — inconsistent failure modes for the same gap.
- Health indicator is per instance (`health=false` opts out); the startup probe is opt-in
  (`ping=true`) — the `health`/`ping` pair replaces redigo's `health.enabled`/`startup-ping`.
  The health probe also rides the declaring transport, adding an access-log record per readiness check.
- Embedded-client methods other than `WritePoints` get no per-call governance (transport
  layer only); `ManagedWriteAPI` gets none at all — two guardedness tiers that are invisible
  at the call site.
- `WritePoints` crosses the executor twice (per-call + transport) over one shared service
  key — breaker counts are amplified, mirroring the http-client bug family fixed 2026-08.
- No `tls.*` / `service-name` unlike sibling starters — HTTPS-only-via-scheme is a smaller
  surface but an asymmetry to document.
