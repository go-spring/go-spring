# starter-milvus Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the starter source
(`starter.go`, `config.go`, `client.go`, `guard.go`, `observe.go`, `health.go`) and the runnable [example/](example/)
— file:line spot-checks in brackets below. **Milvus semantics and the milvus-sdk-go v2 API are
[Milvus's own documentation](https://milvus.io/docs/install-go.md)
([SDK](https://github.com/milvus-io/milvus-sdk-go))** — everything below is go-spring's increment.

**Activation**: any `spring.milvus.instances.*` key (the module is `OnProperty("spring.milvus")`, a
prefix check [starter.go:36]). Each `spring.milvus.instances.<name>` entry creates one
`*StarterMilvus.Client` bean named `<name>`, plus a health indicator named `milvus:<name>`.
**Honest scope note**: since the per-RPC governance guard landed (guard.go — gRPC client
interceptors on the SDK dial options), every Milvus RPC is transparently protected
(rate-limit/breaker/bulkhead/retry/timeout + fault injection) with no opt-in at the call
site. The starter only **declares** what each RPC is (`observe.go`: span name, the
`db.client` metric prefix, the `db.system`/`db.operation` labels, the full method path as
span/log detail); the **resilience layer emits** — the one point on the executor chain
inside the injected executor that sees a whole call, retries included — so every call
reports a call-level `db.client.operation.duration` histogram, an attempt-level
`db.client.attempt.duration` histogram and an access log tagged `_app_milvus_access`.
There is no self-built per-RPC span or instrument in this starter, and no separate
per-RPC trace layer for unguarded traffic — when governance is off the executor is a
transparent no-op and nothing is emitted. The health indicator remains the declared
liveness signal (§2.2, §6) — contributed unless the instance sets `health=false`.

---

## 1. Complete worked project

One service, one Milvus instance, health via actuator. File tree:

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
    github.com/milvus-io/milvus-sdk-go/v2 v2.4.2
    go-spring.org/spring           v1.3.x
    go-spring.org/log              v0.1.x
    go-spring.org/starter-milvus   latest
    go-spring.org/starter-actuator latest   // optional: readiness endpoint
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-milvus"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper, run a real vector round trip (same shape as the
smoke-tested [example/main.go](example/main.go)):

```go
package service

import (
    "context"

    "github.com/milvus-io/milvus-sdk-go/v2/entity"
    "go-spring.org/spring/gs"
    StarterMilvus "go-spring.org/starter-milvus"
)

type Service struct {
    // Always the wrapper type *StarterMilvus.Client. It embeds the SDK's
    // client.Client interface, so NewCollection/Insert/Search/Flush/... are all
    // promoted unchanged.
    Client *StarterMilvus.Client `autowire:"a"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            _ = s.Client.NewCollection(ctx, "docs", 8) // exists-already error: see §5
            _, err := s.Client.Insert(ctx, "docs", "",
                entity.NewColumnInt64("id", []int64{1}),
                entity.NewColumnFloatVector("vector", 8, [][]float32{
                    {0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
                }))
            if err != nil {
                return err
            }
            _ = s.Client.Flush(ctx, "docs", false)
            _ = s.Client.LoadCollection(ctx, "docs", false)
            sp, _ := entity.NewIndexFlatSearchParam()
            res, err := s.Client.Search(ctx, "docs", nil, "", []string{"id"},
                []entity.Vector{entity.FloatVector{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}},
                "vector", entity.L2, 1, sp)
            if err != nil {
                return err
            }
            _ = res // results[0].IDs carries the top-1 hit (example asserts id=1)
            return nil
        }
    })
}
```

**conf/app.properties** — the complete surface (copy of
[example/conf/app.properties](example/conf/app.properties), plus actuator):

```properties
# --- milvus instance "a" ----------------------------------------------------
spring.milvus.instances.a.addr=127.0.0.1:19530
spring.milvus.instances.a.database=default
# Auth keys, only when the cluster has auth on:
#spring.milvus.instances.a.username=root
#spring.milvus.instances.a.password=Milvus

# --- actuator (readiness folds in milvus:a) ---------------------------------
spring.actuator.addr=:9370
```

**Verify** (start Milvus first — [example's docker-compose.yml](example/docker-compose.yml)
brings up etcd + minio + milvus standalone):

```bash
docker compose -p gs-milvus-demo up -d   # milvus boot is slow (~60s)
go run .                                 # boot fails fast if 19530 is unreachable (needs ping=true; off by default)
curl -s :9370/readyz | grep milvus       # component "milvus:a" UP
grep -E 'round trip|Milvus' app.log      # the search above returned
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-milvus
  └─ gs.Module(OnProperty("spring.milvus")) fires when any spring.milvus.instances.* key exists
        └─ conf.BindEach(p, "${spring.milvus}") → one Config per <name> entry
              ├─ addr validated non-empty by the expr tag at bind time [config.go:26]
              ├─ Provide(newClient).Name(<name>).Destroy((*Client).Destroy) [starter.go:38-44]
              └─ Provide health.Indicator named "milvus:<name>", TagArg(<name>) picks the
                   right *Client, Export(gs.As[health.Indicator]()) [starter.go:47-49]

gs.Run()
  ├─ ctor newClient [starter.go:69]: NewClient(ctx, c, cloud.ClientParams{...})
  ├─ NewClient [client.go:82] owns the whole assembly — dial + guard + governance:
  │   client.NewClient(ctx, {Address, Username, Password, DBName,
  │   DialOptions: guardDialOptions(slot)}) — gRPC dial with the guard
  │   interceptors installed (inert until governance is applied)
  ├─ ...then fixes identity and applies governance:
  │   service = ServiceLabel("milvus", addr) →
  │   params.ExecutorFor("milvus", service) — the injected *resilience.Manager / *fault.Injector
  │   bundled by the ctor → (fault-wrapped mgr.ClientExecutorFor) → slot.apply — the
  │   interceptors guard every RPC from here on; zero params → observed-only Unmanaged executor
  ├─ ping=true only: fail-fast probe HealthCheck (ListCollections once), error → Destroy()+boot fails
  │   [starter.go:89] — a wrong address or bad credential never reaches "serving"
  ├─ readiness: indicator repeats HealthCheck (the same ListCollections probe) periodically
  └─ SIGTERM → Destroy [client.go:94]: exec.Close() then o.client.Close() — closes the gRPC conn
```

There is no `Init` hook: `newClient` assembles the client (dial + guard + governance) and only
then probes it, so a failed probe means the bean is never created and dependent beans never wire
against a dead client.

### 2.2 One operation through the ACTUAL layers

`Search(ctx, "docs", ...)` passes through exactly **two** layers:

1. The wrapper struct — `Client` embeds the raw `client.Client` [client.go], so its full
   method set is promoted and it intercepts nothing at the method level; the guard rides
   below it, at the gRPC layer.
2. milvus-sdk-go → gRPC client → **guard interceptors** (executor: limiter/breaker/
   bulkhead/retry/timeout wrapped by the resilience observer, with fault injection outermost) → server.

That is the whole story, plus the guard. **The guard is a gRPC interceptor chain**
[guard.go]: `NewClient` passes custom dial options, so the SDK's own `DefaultGrpcOpts`
(keepalive, connect backoff, 2GB recv limit) are re-added first and the guard
interceptors (unary + stream-open) appended — additive, not a replacement. The
interceptors read a per-client slot that `NewClient` creates, applies with
`params.ExecutorFor("milvus", "milvus:<addr>")`, and keeps private — the slot cannot appear
in an exported signature, so the constructor owns both the dial and the governing step.
The executor is built from the `*resilience.Manager` and
`*fault.Injector` the ctor bundles into a `cloud.ClientParams`
(governance off → observed-only `Unmanaged` executor, passthrough;
governance is applied as the client is built, so the fail-fast probe in
`newClient` runs *after* the slot is applied and on a governance-on boot the probe itself rides the
guard). The interceptor is also the **declaration seam**: it puts the RPC's identity on the
caller's context before `exec.Execute` runs (`observability.WithOperation(ctx, operation(method))`),
and the emitter inside the executor reads it at Execute entry — the one place that sees the
whole call, retries included — to name the span, the `db.client.*` metrics and the access log.
Declaring inside the executor's fn would be read by nobody (per attempt), so it is placed here,
outside. Every RPC — collections, indexes, search, insert — rides it with zero call-site
changes, the same transparent per-request stance as the other NoSQL starters. The other signal
this starter emits itself is the health indicator (not per-call). `milvus:<name>` is registered
unless the instance sets `health=false`; its probe goes straight to the raw client —
`HealthCheck`'s one `ListCollections` round trip verifying reachability AND auth [health.go].

---

## 3. Per-key behavior reference

All keys live under `spring.milvus.instances.<name>.` — bound per-instance via `conf.BindEach`
(NOT the absolute-property starter-Pool rule). Verified against
`grep -rhoE 'value:"[^"]+"'` — the table covers every tag, both directions.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **required**, validated non-empty by the expr tag `$ != ''` [config.go:26]. Milvus gRPC endpoint `host:19530`. ⚠ TLS is expressed inside the address scheme by the SDK — this starter exposes no TLS block. | Missing/empty → bind-time error before any dial. Wrong host/port → error on first use (or the ctor's fail-fast probe aborting boot when `ping=true`). |
| `database` | string | `default` | Passed as `DBName` to `client.NewClient` [starter.go:65]. | Nonexistent DB → error on first use (or the ListCollections probe at boot when `ping=true`). |
| `username` | string | `""` | Auth credential; both halves must be set together when the cluster has auth on. ⚠ `username` without `password` (or vice versa) is silently half-sent. | Wrong pair → auth error on first use (or the probe at boot when `ping=true`). |
| `password` | string | `""` | See `username`. | See `username`. |
| `ping` | bool | `false` | Startup connectivity probe: when true the ctor runs `HealthCheck` (a `ListCollections`) once and fails the boot if it errors, restoring fail-fast. Off by default so a server that is not up yet does not block startup. | `ping=true` against a down server → boot error `milvus: startup probe failed`. |
| `health` | bool | `true` | Whether this instance contributes a `health.Indicator` (name `milvus:<name>`) for the actuator's readiness/startup probes. Set false to keep the instance out of the aggregated health report. | `health=false` → no `milvus:<name>` component in `/readyz`. |

No `driver` registry, no `mode` (one topology: standalone/cluster is server-side), no
discovery, no otel keys — governance (resilience + fault) arrives via the shared
`spring.governance.*` rules consumed by the per-RPC guard, not a milvus-specific key.

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | grep milvus       # "milvus:a": UP
docker stop gs-milvus-demo-milvus-standalone-1
curl -s :9370/readyz                    # flips DOWN (503) on the next probe cycle
```

The component error body carries the gRPC error verbatim — that distinguishes reachability
(`Unavailable`) from auth (`Unauthenticated`).

### 4.2 Fail-fast probe (server down at boot, `ping=true`)

```bash
docker compose -p gs-milvus-demo down && go run .   # with the instance's ping=true
# exits non-zero from the ctor (ListCollections on a dead port) — never reaches "serving"
# with the default ping=false the boot succeeds and the first RPC fails instead
```

### 4.3 Confirm the guard's silence (honest drill, governance off)

```bash
curl -s :9370/metrics | grep -i milvus   # nothing from this starter with governance off
grep _app_ app.log | grep -i milvus      # the access log; empty with governance off
```

With governance off (no `cloud/governance` beans in the container / no `spring.governance.*` rules) expect empty output for
both: the guard executor is a no-op and the only signal this starter emits is the health
component. Turn governance on and the declared-identity signals appear — the `db.client.operation.duration`
and `db.client.attempt.duration` histograms plus the `_app_milvus_access` access log — alongside
the `resilience.*` outcome metrics. Milvus's own server metrics live on the server's `:9091`
(exposed by the example compose), not through this client.

### 4.4 Round-trip smoke (same shape as example/check.sh)

```bash
grep "round trip" app.log    # the marker check.sh greps ("Milvus round trip OK: hit id= [1]")
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails in ctor, gRPC `Unavailable`/timeout | Milvus not up yet (boot is slow) or wrong `addr` | Wait for port 19530 (`(exec 3<>/dev/tcp/127.0.0.1/19530)`), fix `addr`. |
| Boot fails with `Unauthenticated` | Auth on but `username`/`password` missing or wrong | Set both halves; ⚠ half a pair is silently ignored. |
| Boot fails listing collections, server reachable | `database` does not exist | Point `database` at an existing DB (Milvus ≥2.3). |
| `NewCollection` fails on restart | Collection already exists from a previous run | Drop it first or tolerate the error (example's check.sh uses a fixed name). |
| Search returns empty / no results | Forgot `Flush` + `LoadCollection` before searching (SDK semantics) | Flush then load, as in example/main.go:80-85. |
| Health DOWN though queries work | Indicator's `ListCollections` needs the same DB/auth as the client | Inspect the component error body in /readiness. |
| No traces/metrics/access log for Milvus ops | Governance is off (the executor is a no-op) | Turn governance on (a configured rules source + `spring.governance.*` rules); then `db.client.*` metrics + `_app_milvus_access` appear. Server-side :9091 metrics never go through this client. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 tags (all effective) |
| Required | 1 (`addr`) |
| Quickstart external deps | 3 in compose (etcd + minio + milvus standalone) |
| "Watch out" entries | 3 (auth pair, TLS-in-address, TLS/auth dial-option escape hatch) |

Design suspects (audit ledger — kept and extended):

- `schema.json` lives under example/ rather than the module root
  (family asymmetry: other starters keep it at the root).
- Health indicator is per instance (`health=false` opts out); the startup probe is opt-in
  (`ping=true`) — the `health`/`ping` pair replaces redigo's `health.enabled`/`startup-ping`.
- No TLS key — SDK TLS is expressed in the address; nothing in the starter documents how a
  user would even do it without forking `newClient`.
- ~~Missing instrumentation~~ resolved: `observe.go` declares each RPC's identity and the
  resilience layer emits the `db.client.*` signals + access log from the guard interceptors.
  Still open: a TLS/auth `DialOptions` escape hatch on Config — an app wanting to add its
  own interceptor (auth tokens, custom tracing) must fork `newClient`.
