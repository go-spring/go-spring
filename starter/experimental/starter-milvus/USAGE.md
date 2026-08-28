# starter-milvus Usage — Reference

Detailed usage reference. Overview: [example/README.md](example/README.md) (the starter README
currently lives under example/). All behavior claims are verified against the starter source
(`starter.go`, `config.go`, `client.go`, `health/health.go`) and the runnable [example/](example/)
— file:line spot-checks in brackets below. **Milvus semantics and the milvus-sdk-go v2 API are
[Milvus's own documentation](https://milvus.io/docs/install-go.md)
([SDK](https://github.com/milvus-io/milvus-sdk-go))** — everything below is go-spring's increment.

**Activation**: any `spring.milvus.*` key (the module is `OnProperty("spring.milvus")`, a
prefix check [starter.go:29]). Each `spring.milvus.<name>` entry creates one
`*StarterMilvus.Client` bean named `<name>`, plus a health indicator named `milvus:<name>`.
**Honest scope note**: since the per-RPC governance guard landed (guard.go — gRPC client
interceptors on the SDK dial options), every Milvus RPC is transparently protected
(rate-limit/breaker/bulkhead/retry/timeout + fault injection) with no opt-in at the call
site, and the instance's `observability.*` block drives the guard execution's observation
(spans + outcome metrics + access log). There is no separate per-RPC trace layer for
unguarded traffic — when governance is off the executor is a transparent no-op. The health
indicator remains the always-on liveness signal (§2.2, §6).

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
smoke-tested [example/example.go](example/example.go)):

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
    // client.Client interface, so NewCollection/Insert/Search/Flush/... all
    // promote unchanged.
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
spring.milvus.a.addr=127.0.0.1:19530
spring.milvus.a.database=default
# Auth keys, only when the cluster has auth on:
#spring.milvus.a.username=root
#spring.milvus.a.password=Milvus

# --- actuator (readiness folds in milvus:a) ---------------------------------
spring.actuator.addr=:9370
```

**Verify** (start Milvus first — [example's docker-compose.yml](example/docker-compose.yml)
brings up etcd + minio + milvus standalone):

```bash
docker compose -p gs-milvus-demo up -d   # milvus boot is slow (~60s)
go run .                                 # boot fails fast if 19530 is unreachable
curl -s :9370/readyz | grep milvus       # component "milvus:a" UP
grep -E 'round trip|Milvus' app.log      # the search above returned
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-milvus
  └─ gs.Module(OnProperty("spring.milvus")) fires when any spring.milvus.* key exists
        └─ conf.BindEach(p, "${spring.milvus}") → one Config per <name> entry
              ├─ addr validated non-empty by the expr tag at bind time [config.go:26]
              ├─ Provide(newClient).Name(<name>).Init((*Client).Init)
              │       .Destroy((*Client).Destroy) [starter.go:31-33]
              └─ Provide health.Indicator named "milvus:<name>", TagArg(<name>) picks the
                   right *Client, Export(gs.As[health.Indicator]()) [starter.go:35-37]

gs.Run()
  ├─ ctor newClient [client.go]: client.NewClient(ctx, {Address, Username, Password,
  │   DBName, DialOptions: guardDialOptions(slot)}) — gRPC dial with the guard
  │   interceptors installed (inert until Init arms them); then fail-fast probe:
  │   ListCollections once, error → cl.Close() + boot fails — a wrong address or bad
  │   credential never reaches "serving"
  ├─ Init [client.go]: resource = ResourceLabel("milvus", addr) →
  │   fault.WrapExecutor(resilience.ExecutorFor(resource)) →
  │   resilobserve.WrapExecutor(exec, "milvus", Observability) → slot.arm — the
  │   interceptors guard every RPC from here on; governance off → no-op executor
  ├─ readiness: indicator repeats the same ListCollections probe periodically
  └─ SIGTERM → Destroy [client.go]: exec.Close() then o.Client.Close() — closes the gRPC conn
```

`Init` exists purely to arm the guard (the executor needs the field-injected
`observability` block); dial + probe still happen in the constructor, so a failed probe
means the bean is never created and dependent beans never wire against a dead client.

### 2.2 One operation through the ACTUAL layers

`Search(ctx, "docs", ...)` passes through exactly **two** layers:

1. The wrapper struct — `Client` embeds `client.Client` [client.go] and adds no
   interception at the method level; the guard rides below it, at the gRPC layer.
2. milvus-sdk-go → gRPC client → **guard interceptors** (executor: limiter/breaker/
   bulkhead/retry/timeout + fault, wrapped by the resilience observer) → server.

That is the whole story, plus the guard. **The guard is a gRPC interceptor chain**
[guard.go]: `newClient` passes custom dial options, so the SDK's own `DefaultGrpcOpts`
(keepalive, connect backoff, 2GB recv limit) are re-added first and the guard
interceptors (unary + stream-open) appended — additive, not a replacement. The
interceptors read a per-client slot that `Init` arms with
`fault.WrapExecutor(resilience.ExecutorFor("milvus:<addr>"))` wrapped in
`resilobserve.WrapExecutor` (governance off → no-op, passthrough; the fail-fast probe in
`newClient` runs pre-Init and relies on that passthrough). Every RPC — collections,
indexes, search, insert — rides it with zero call-site changes, the same transparent
per-request stance as the other NoSQL starters. What observability also exists: the
health indicator. `milvus:<name>` is always registered, its probe is the wrapper's own
`Health(ctx)` [client.go] — one `ListCollections` round trip verifying reachability AND
auth [health/health.go].

---

## 3. Per-key behavior reference

All keys live under `spring.milvus.<name>.` — bound per-instance via `conf.BindEach`
(NOT the absolute-property starter-Pool rule). Verified against
`grep -rhoE 'value:"[^"]+"'` — the table covers every tag, both directions.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **required**, validated non-empty by the expr tag `$ != ''` [config.go:26]. Milvus gRPC endpoint `host:19530`. ⚠ TLS is expressed inside the address scheme by the SDK — this starter exposes no TLS block. | Missing/empty → bind-time error before any dial. Wrong host/port → ctor's fail-fast probe fails, boot aborts. |
| `database` | string | `default` | Passed as `DBName` to `client.NewClient` [client.go:45]. | Nonexistent DB → fail-fast probe (ListCollections) errors at boot. |
| `username` | string | `""` | Auth credential; both halves must be set together when the cluster has auth on. ⚠ `username` without `password` (or vice versa) is silently half-sent. | Wrong pair → fail-fast probe fails at boot with the server's auth error. |
| `password` | string | `""` | See `username`. | See `username`. |
| `observability` | group | empty | Field-injected onto the wrapper and read by `Init` to observe the guard execution (`resilobserve.WrapExecutor`): spans, outcome metrics (`resilience.*`), access log for every guarded RPC. `off` silences the log signal only. | Expecting per-RPC spans of *unguarded* traffic (governance off) → nothing is emitted; the executor is a no-op. |

No `driver` registry, no `mode` (one topology: standalone/cluster is server-side), no
discovery, no otel keys — governance (resilience + fault) arrives via the shared
`govern.*` rules consumed by the per-RPC guard, not a milvus-specific key.

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

### 4.2 Fail-fast probe (server down at boot)

```bash
docker compose -p gs-milvus-demo down && go run .
# exits non-zero from the ctor (ListCollections on a dead port) — never reaches "serving"
```

### 4.3 Confirm the guard's silence (honest drill, governance off)

```bash
curl -s :9370/metrics | grep -i milvus   # nothing from this starter with governance off
grep _app_ app.log | grep -i milvus      # no access-log tag exists with governance off
```

With governance off (no starter-governance / no `govern.*` rules) expect empty output for
both: the guard executor is a no-op and the only signal this starter emits is the health
component. Turn governance on and the `resilience.*` outcome metrics and guard access log
appear. Milvus's own server metrics live on the server's `:9091` (exposed by the example
compose), not through this client.

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
| Search returns empty / no results | Forgot `Flush` + `LoadCollection` before searching (SDK semantics) | Flush then load, as in example/example.go:80-85. |
| Health DOWN though queries work | Indicator's `ListCollections` needs the same DB/auth as the client | Inspect the component error body in /readiness. |
| No traces/metrics/access log for Milvus ops | Expected — there is no instrumentation (§2.2) | None at starter level; watch health + server-side :9091 metrics. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 tags (4 effective + 1 dead group) |
| Required | 1 (`addr`) |
| Quickstart external deps | 3 in compose (etcd + minio + milvus standalone) |
| "Watch out" entries | 3 (auth pair, TLS-in-address, no instrumentation) |

Design suspects (audit ledger — kept and extended):

- `observability.*` binds but is never read — remove the key or wire it.
- Starter-level README/DESIGN/schema.json live under example/ rather than the module root
  (family asymmetry: other starters keep them at the root).
- Health indicator has no disable switch (`health.enabled`-style key absent; family asymmetry
  vs redigo).
- No TLS key — SDK TLS is expressed in the address; nothing in the starter documents how a
  user would even do it without forking `newClient`.
- Missing instrumentation is itself a suspect: the build-time-only gRPC dial options are cited
  [client.go:18-20] as the blocker, but no dial options are passed at all today — a
  `DialOptions` escape hatch on Config would unlock interceptors (auth tokens, observability)
  without a fork.
- errutil import is kept alive by a dummy `var _ = errutil.Explain` [starter.go:43-45]
  "for future driver dispatch" — speculative API residue.
