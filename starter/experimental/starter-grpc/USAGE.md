# starter-grpc Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `admission.go`, `balancer.go`, `extension.go`, `fault.go`,
`loadtest.go`, `metrics.go`, `recover.go`, `tracing.go`) and the runnable examples
([example/](example/), [example-lb/](example-lb/), [example-otel/](example-otel/)) — each ships a
self-asserting `check.sh`. **grpc-go's own semantics** (streaming, deadlines, keepalive, status
codes) are [grpc-go's documentation](https://grpc.io/docs/languages/go/) — everything here is
go-spring's increment: wiring, interceptor chain, observability, governance admission, client-side
load balancing.

**Activation**: the server bean exists only when `spring.grpc.server.addr` is set
(`gs.OnProperty("spring.grpc.server.addr")` in starter.go's init). That key is the on/off switch;
there is no `enabled` key and no default port (ports are never defaulted in go-spring).
Single-server model: one `grpc.Server`, one listener.

---

## 1. Complete worked project

A realistic Echo service with health probing, tracing/metrics and runtime fault injection, plus a
client that load-balances across instances through a discovery backend. File tree:

```
demo/
├── go.mod
├── main.go
├── server.go
├── client.go
├── idl/echo.proto
└── conf/app.properties
```

**go.mod** (deps that matter): `google.golang.org/grpc`, `go-spring.org/spring`,
`go-spring.org/starter-grpc`; optional `go-spring.org/starter-otel` (real trace/metric export) and
`go-spring.org/starter-governance` (governance center: admission + fault).

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**server.go** — service registration (proto codegen assumed at `demo/idl/proto`):

```go
package main

import (
    "context"

    "google.golang.org/grpc"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterGrpc "go-spring.org/starter-grpc"
    "demo/idl/proto"
)

func init() {
    // The application provides exactly ONE ServiceRegister bean. The starter owns
    // the *grpc.Server and its lifecycle; you register services on what it hands you.
    gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
        return func(svr *grpc.Server) {
            proto.RegisterEchoServiceServer(svr, &Controller{})
        }
    })
}

type Controller struct {
    proto.UnimplementedEchoServiceServer
}

func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
    // ctx already carries the trace span and the load-test marker (if any) —
    // branch on traffic.IsLoadTest(ctx) to degrade features under synthetic load.
    log.Infof(ctx, log.TagAppDef, "echo: %s", req.Message)
    return &proto.EchoResponse{Message: req.Message}, nil
}
```

**client.go** — dial through a discovery backend with a Go-Spring balancer (see §2.4):

```go
StarterGrpc.UseUnaryInterceptor(authGuard) // user guard, outermost of the whole chain

conn, err := grpc.NewClient(
    StarterGrpc.Scheme+":///echo-service", // gsdiscovery:///<service> via default backend
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultServiceConfig(StarterGrpc.LoadBalancingConfig(loadbalance.RoundRobin)),
)
// consistent-hash affinity / zone affinity per call:
ctx = StarterGrpc.WithHashKey(ctx, req.UserId)
ctx = StarterGrpc.WithZone(ctx, "zone-a")
```

**conf/app.properties** — the complete commented surface:

```properties
# --- grpc server --------------------------------------------------------------
# Let the gRPC server own the port; disable gs's built-in HTTP server.
spring.http.server.enabled=false
spring.grpc.server.addr=:9494

# Size/concurrency caps (0 keeps the grpc-go default).
spring.grpc.server.maxRecvMsgSize=4194304
spring.grpc.server.maxSendMsgSize=4194304
spring.grpc.server.maxConcurrentStreams=100

# Server-side keepalive enforcement (keepalive.ServerParameters).
spring.grpc.server.keepalive.time=2h
spring.grpc.server.keepalive.timeout=20s

# Standard grpc_health_v1 service (default on) with overall status SERVING.
spring.grpc.server.health.enabled=true

# Load-test identification off inbound metadata (default on).
spring.grpc.server.loadtest.enabled=true

# Built-in observability interceptors (both default on; no-ops without starter-otel).
spring.grpc.server.observer.tracing.enabled=true
spring.grpc.server.observer.metrics.enabled=true

# Access-log verbosity for the observe layer (level: brief|full|off).
spring.grpc.server.observability.level=brief
spring.grpc.server.observability.maxArgBytes=512

# --- observability (starter-otel; mirror of example-otel/conf) ----------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- governance (admission + fault; hot-reloaded from this file) --------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml** (fault drill in §4.3; admission under resilience):

```yaml
govern:
  enabled: true
  # Per-resource resilience rules; the resource label = "grpc:{addr}" (admission.go).
  rules:
    - resources: ["grpc::9494"]
      rate-limit: 100      # QPS cap; over → codes.ResourceExhausted
      max-concurrent: 50   # bulkhead; over → codes.ResourceExhausted
  fault:
    enabled: false         # flip to true to "set fire" without restart
    scope: loadtest        # only traffic marked x-loadtest is affected
    rules:
      - resources: ["grpc:/EchoService/Echo"]   # rule label = "grpc:{FullMethod}"
        rate: 0.2
        error: timeout
```

**Verify** (structurally identical to what the examples assert):

```bash
cd example && ./check.sh        # self-asserts: echo round-trip, x-handler response header,
                                # grpc_health_v1 = SERVING, then SIGTERM
cd example-lb && ./check.sh     # 3-phase LB smoke: even spread, eviction+recovery, kill
cd example-otel && docker compose up -d && ./check.sh   # traces verified via Jaeger API
```

grpcurl equivalents: `grpcurl -plaintext :9494 EchoService/Echo` and
`grpcurl -plaintext :9494 grpc.health.v1.Health/Check`.

---
## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-grpc
  └─ gs.Provide(NewSimpleGrpcServer)            [condition: spring.grpc.server.addr set]
        └─ .Export(gs.As[gs.Server]())
gs.Run()
  ├─ config bind: ${spring.grpc.server} → Config (value tags)
  ├─ bean wiring: ServiceRegister (non-nullable — missing bean fails the container)
  ├─ SimpleGrpcServer.Run(ctx, sig):
  │    ├─ buildOptions(): translate Config → []grpc.ServerOption, assemble chains
  │    ├─ grpc.NewServer(opts...)
  │    ├─ health: RegisterHealthServer + SetServingStatus("", SERVING)   [health.enabled]
  │    ├─ s.reg(svr): YOUR services register here
  │    ├─ net.Listen(addr)
  │    ├─ <-sig.TriggerAndWait()   ← readiness gates Serve
  │    └─ svr.Serve(listener)
  └─ on SIGTERM: StopContext → svr.GracefulStop()  (drains in-flight RPCs; ctx tags the log only)
```

User interceptors registered via `UseUnaryInterceptor`/`UseStreamInterceptor` must be called from
`init()` — extension.go snapshots them when `buildOptions` runs.

### 2.2 Interceptor chain — exact order and why

Unary chain (`buildOptions`, starter.go:165-209; composed with `grpc.ChainUnaryInterceptor`, never
the single-setter `UnaryInterceptor` — see the code comment on the historical shadowing bug):

```
user interceptors → LoadTest → Tracing → Metrics → Resilience(admission, unary only)
                  → Fault → Recover → handler
```

Stream chain is the same minus Resilience (admission covers unary only).

Rationale (from the source comments, verified):

- **User outermost** (extension.go): "an app guard sees the request before the built-in stack and
  can short-circuit before any work is observed" — mirrors starter-gin's EngineMiddleware.
- **LoadTest outermost of the built-ins** (starter.go:173): the marker is on the context before
  tracing, metrics, resilience or the handler run, so every downstream layer can branch on
  `traffic.IsLoadTest(ctx)`. No-op without the marker key.
- **Tracing before Metrics**: the span wraps the metrics observation too, so duration and status
  land on the same trace context.
- **Resilience before Fault/Recover**: admission control decides before work is attempted; its
  executor is wrapped with `resilobserve.WrapExecutor` so trips/rejects emit span + counter +
  histogram themselves.
- **Fault innermost-of-policy** (fault.go): "installed innermost so an injected error flows back
  through tracing/metrics/resilience and is observed" — you can observe the fire you set.
- **Recover innermost** (recover.go): "grpc-go recovers handler panics nowhere by itself"; the
  converted `codes.Internal` unwinds through every observer on its normal error path — the
  unified panic policy's request-side placement.

### 2.3 One unary RPC, layer by layer

`Echo(msg)` with metadata `x-loadtest: 1`, fault scope engaged, rate limit configured:

1. User interceptor(s) — auth/guard may reject before anything is observed.
2. LoadTest: `extractLoadTest` reads `x-loadtest` off incoming metadata → ctx tagged
   (`traffic.IsLoadTest(ctx) == true`).
3. Tracing: W3C context extracted from metadata (`extractTraceContext`), server span started,
   name = FullMethod, attributes `rpc.system=grpc`, `rpc.service`, `rpc.method`.
4. Metrics: in-flight +1 (`rpc.method` attr); duration clock starts.
5. Resilience: `exec.Execute(ctx, "grpc::9494", handler)` — over the rate cap → the handler never
   runs and `mapAdmissionError` maps to `ResourceExhausted`; breaker open → `Unavailable`.
6. Fault: `fault.Apply(ctx, InjectorFor(), "grpc:/EchoService/Echo", handler)` — marked traffic
   fails ~`rate` of the time or gains injected latency; unmarked/scope-off passes through.
7. Recover arms; the handler runs. A panic is reported via `goutil.ReportPanic` and converted to
   `codes.Internal`.
8. The response/error unwinds 6→5→4→3: duration recorded with `rpc.grpc.status_code`, count +1,
   in-flight −1, span gets status code + Error status + RecordError on failure, span ends.

### 2.4 Client side: the gsdiscovery resolver + balancers (balancer.go)

- Dial `gsdiscovery:///<service>` (default discovery backend) or `gsdiscovery://<backend>/<service>`
  (named backend). The resolver seeds an initial snapshot before watching, so the first RPC does
  not race an empty address list.
- Select a strategy purely via service config: `grpc.WithDefaultServiceConfig(
  StarterGrpc.LoadBalancingConfig(strategy))`. Balancer names are `gs_round_robin`, `gs_least_conn`,
  `gs_consistent_hash`, `gs_weighted`, `gs_zone_aware` (pre-registered in init with suspension
  defaults: 5 consecutive failures → evicted 30s, then half-open trial).
- Per-call hints: `WithHashKey` (consistent-hash affinity), `WithZone` (zone-aware preference).
- Weight=0 endpoints are filtered by the strategies themselves (pool-wide drain semantics);
  `RegisterBalancer(name, strategy, trackerConfig)` registers a custom name for isolated suspension
  state (panics on unknown strategy or duplicate name, matching grpc-go's own contract).
- example-lb/main.go is the executable proof: even spread, eviction + half-open readmission, and
  discovery-kill drop — all asserted in-process.

---

## 3. Per-key behavior reference

All keys under `spring.grpc.server.*`. Reconciled with
`grep -rhoE 'value:"[^"]+"' . --include='*.go' | sort -u`.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key** (required). Listen address; also the resilience resource suffix (`grpc:{addr}`) and the only instance discriminator. | Missing → whole starter silently inactive; ServiceRegister bean then fails the container. |
| `connectionTimeout` | duration | 0 | `grpc.ConnectionTimeout` when >0. | 0 = grpc-go default. |
| `maxRecvMsgSize` / `maxSendMsgSize` | int | 0 | Bytes; applied when >0. | Too low → `ResourceExhausted` (grpc's "received message larger than max") on large payloads, per-RPC not at boot. |
| `maxConcurrentStreams` | uint32 | 0 | Per-connection cap. | 0 = grpc-go default (unlimited-ish). |
| `keepalive.time` | duration | 0 | `keepalive.ServerParameters.Time`. ⚠ the whole block is applied only if ANY of the four values >0; a lone `keepalive.timeout` silently brings defaults for the rest. | Aggressive `time` + clients that ping less often → GOAWAY storms. |
| `keepalive.timeout` | duration | 0 | `ServerParameters.Timeout`. | |
| `keepalive.maxConnectionIdle` / `maxConnectionAge` | duration | 0 | Connection lifecycle bounds. | |
| `tls.enabled` | bool | false | `credentials.NewTLS(TLS.BuildServer())`. | Expecting plaintext on a TLS port → handshake failures. |
| `tls.cert-file` / `tls.key-file` | string | — | Key pair for the server cert. ⚠ both needed together once `tls.enabled`. | Boot error "build TLS" from `Run`. |
| `tls.ca-file` | string | — | **Server-side this means mTLS**: sets ClientCAs + `RequireAndVerifyClientCert` (tlsconf.BuildServer). | Setting it casually → all clients without certs rejected. |
| `tls.server-name` / `tls.insecure-skip-verify` | | — | Client-side knobs; **dead keys here** (ignored by BuildServer). | False confidence; no effect. |
| `health.enabled` | bool | true | Registers `grpc_health_v1`, overall status SERVING. | false → probes/LB health checks get Unimplemented. |
| `loadtest.enabled` | bool | true | Installs LoadTest interceptors reading `x-loadtest` metadata (lowercase — grpc metadata keys are lower-case). | false → load-test marker invisible; fault `scope: loadtest` never fires. |
| `observer.tracing.enabled` | bool | true | Installs tracing interceptors riding OTel globals; no-op without starter-otel (no warning). ⚠ example-otel's conf uses `interceptor.tracing.*` — a dead path; only the default-true saves it. | Prefix typo → silently the default. |
| `observer.metrics.enabled` | bool | true | Installs metrics interceptors; same no-op caveat. | Same. |
| `observability.level` | string | brief | cloud/observe ObserveConfig: `brief`/`full`/`off` verbosity for the observe-resilience access-log side. | |
| `observability.maxArgBytes` | int | 512 | Arg-capture cap in the observe layer. | |
| `observability.skipOps` | []string | — | Ops to skip in the observe access log. | |

---

## 4. Verification & fault drills

### 4.1 Basic round-trip + health (mirrors example/check.sh assertions)

```bash
go run ./example          # self-asserts: echo body round-trip, x-handler response header,
                          # grpc_health_v1 → SERVING; exits 0
grpcurl -plaintext -d '{"message":"hi"}' :9494 EchoService/Echo
grpcurl -plaintext :9494 grpc.health.v1.Health/Check   # {"status":"SERVING"}
```

### 4.2 Metrics names & attributes (metrics.go, meter `go-spring.org/starter-grpc`)

Unary: `rpc.server.request_count` (counter), `rpc.server.request.duration` (histogram, seconds,
explicit buckets 5ms…10s), `rpc.server.active_requests` (UpDownCounter, `rpc.method` only).
Streams: `rpc.server.stream_count`, `rpc.server.stream_duration`. Duration/count attributes:
`rpc.method` = FullMethod, `rpc.grpc.status_code` = e.g. `OK`, `ResourceExhausted`.

```bash
curl -s :9090/metrics | grep -E 'rpc_server_request_(duration|count)|active_requests'
```

Spans (tracing.go): name = FullMethod; `rpc.system=grpc`, `rpc.service` (from the path),
`rpc.method`; on error `rpc.grpc.status_code` + Error status + recorded error event.
example-otel verifies end-to-end against the Jaeger API
(`http://127.0.0.1:16686/api/traces?service=grpc-otel-example`).

### 4.3 Fault drill — hot toggle, no restart (fault.go)

1. Start with `govern.yaml` `fault.enabled: false`.
2. Baseline traffic → all OK.
3. Flip `fault.enabled: true` in the file — `fault.InjectorFor()` is resolved **per call**, so the
   change applies on the next RPC without restart.
4. Rule label is `grpc:{FullMethod}` (e.g. `grpc:/EchoService/Echo`) — you can burn a single
   method. With `scope: loadtest` only metadata-marked traffic burns:

```bash
grpcurl -plaintext -H 'x-loadtest: 1' -d '{"message":"x"}' :9494 EchoService/Echo  # ~20% fail
grpcurl -plaintext -d '{"message":"x"}' :9494 EchoService/Echo                      # 200s continue
```

5. Watch the fire: `rpc_server_request_count{rpc.grpc.status_code!="OK"}`, error spans, then flip
   back to false to extinguish.

### 4.4 Admission drill (admission.go)

Resource label is `grpc:{addr}` → `grpc::9494`. Add a rule (`govern.rules[n].resources=grpc::9494`,
`rate-limit=...`) below your traffic rate:
rejections surface as `codes.ResourceExhausted` (rate/bulkhead) or `codes.Unavailable` (open
breaker) — `mapAdmissionError` guarantees consumers can branch on the code. The wrapped
observe-resilience executor emits its own counters/histogram for trips and rejects. Do NOT
configure inbound retry: a handler that already produced side effects cannot be replayed
(admission.go's own warning).

### 4.5 Load-balancing drill

```bash
cd example-lb && ./check.sh    # asserts: even 3-way spread, eviction after 3 fails + 2s cooldown,
                               # readmission after recovery, drop of a killed instance
```

### 4.6 Panic path

A handler panic → `codes.Internal` "panic in {FullMethod}: ..." plus a structured report through
the shared goutil panic chain (`goutil.ReportPanic`) — visible in log and span; the process stays up.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Nothing listens; container fails on ServiceRegister | `spring.grpc.server.addr` missing — starter inactive, your bean dangles | Set it; it is the activation switch. |
| Tracing/metrics "enabled" but nothing exported | starter-otel not imported — OTel globals are silent no-ops | Add the import (example-otel pattern). |
| Config for interceptors ignored | Wrong prefix: it's `observer.*`, not `interceptor.*` (example-otel's conf has this dead key) | Use `spring.grpc.server.observer.tracing/metrics.enabled`. |
| Clients rejected at TLS handshake with cert errors | `tls.ca-file` set — that enables **mTLS** (`RequireAndVerifyClientCert`) | Remove it for one-way TLS, or issue client certs. |
| Everything works, no fault/admission effect | starter-governance not imported or `govern.source` not configured — seams yield pass-through | Import it and point `govern.source.file.path` at your file. |
| `ResourceExhausted` "received message larger than max" | `maxRecvMsgSize` below payload | Raise the cap. |
| Stream RPCs bypass rate limit | Admission is unary-only by design | Guard streams with a user interceptor (`UseStreamInterceptor`). |
| GOAWAY / connection churn | Aggressive `keepalive.time` vs client ping rate | grpc keepalive semantics; relax server params. |
| LB client: `ErrNoSubConnAvailable` at startup | Discovery backend missing / no healthy endpoints; or unknown balancer name in service config | Register the backend (`discovery.RegisterDiscovery`) and use `BalancerName`/`LoadBalancingConfig`. |
| LB stops updating after a discovery error | `watchLoop` returns permanently on a `WatchResult.Err` | Restart the client; tracked as a design suspect. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (leaf, incl. tls/observability) | 21 |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (collector only for full observability) |
| "Watch out" entries | 6 |

Design suspects (kept from the previous audit, updated):

1. README stale claim that interceptor composition requires handler-layer wrapping —
   `UseUnaryInterceptor`/`UseStreamInterceptor` + chained interceptors exist (example.go's
   `interceptedEchoServer` is legacy). *(still open — README text)*
2. example/conf comment claims a ":9494 default" — there is none; `addr` is required.
3. Resilience admission covers unary only; stream RPCs skip it (admission.go builds no stream
   interceptor).
4. `observability.*` binds cloud/observe config but only the observe-resilience wrapper consumes
   it; the access-log side is largely unused.
5. NEW: example-otel conf uses dead prefix `spring.grpc.server.interceptor.*` (works only because
   defaults are true) — config-surface trap.
6. NEW: balancer.go `watchLoop` exits permanently on a watch error (comment acknowledges the
   push path is best-effort); a transient discovery error freezes the address set.
