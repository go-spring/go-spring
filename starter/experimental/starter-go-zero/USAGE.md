# starter-go-zero Usage — Reference

Umbrella usage reference covering the two sub-server packages (`rest/`, `zrpc/`) and the internal
log bridge. Overview: [README.md](README.md). Anchored to the runnable examples
[rest/example/](rest/example/example.go), [zrpc/example/](zrpc/example/example.go) and the
Jaeger-backed observability examples `rest/example-otel/`, `zrpc/example-otel/` (each has a
`docker-compose.yml`). **go-zero's own semantics (rest routing, zrpc governance, logx, DevServer)
are [go-zero's documentation](https://go-zero.dev/docs/)** — everything below is Go-Spring's
increment: activation keys, bean wiring, etcd publication, log bridge, graceful shutdown.

**Activation**: each sub-server exists only when its key is set — the key is the on/off switch;
there is no `enabled` key:

- `spring.go-zero.rest.server.port` → HTTP/API server (`rest.Server`)
- `spring.go-zero.zrpc.server.listen-on` → gRPC server (`zrpc.RpcServer`)

The two are independent and can coexist in one process (§1 shows exactly that, including the
metrics-port collision workaround). Activation also requires the application to provide that
family's register bean — the autowire is non-nullable, so a configured key without a bean fails
the container (and vice versa: a bean without the key silently leaves the starter out).

---

## 1. Complete worked project

A dual-protocol service — one REST API for humans, one gRPC service for internal callers — in a
single Go-Spring process. File tree (mirrors `rest/example/` + `zrpc/example/`):

```
demo/
├── go.mod
├── main.go
├── rest_handlers.go
├── rpc_services.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/zeromicro/go-zero   v1.10.1
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-go-zero  latest   // import path selects rest/ zrpc/
    go-spring.org/starter-otel     latest   // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-zero/rest" // side-effect import registers the REST bean
    _ "go-spring.org/starter-go-zero/zrpc" // side-effect import registers the gRPC bean
    _ "go-spring.org/starter-otel"         // optional: installs the global OTel provider
)

func main() { gs.Run() }
```

**rest_handlers.go** — your entire HTTP surface:

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/zeromicro/go-zero/rest"
    "go-spring.org/spring/gs"
    gozerorest "go-spring.org/starter-go-zero/rest"
)

func init() {
    // Provide exactly ONE HandlerRegister bean. The starter owns the rest.Server;
    // you only attach routes onto it (AddRoute / rest.WithJwt / ... — go-zero API).
    gs.Provide(func() gozerorest.HandlerRegister {
        return func(server *rest.Server) {
            server.AddRoute(rest.Route{
                Method:  http.MethodGet,
                Path:    "/greet",
                Handler: func(w http.ResponseWriter, r *http.Request) {
                    name := r.URL.Query().Get("name")
                    w.Header().Set("Content-Type", "application/json")
                    _ = json.NewEncoder(w).Encode(map[string]string{"message": "Hi, " + name})
                },
            })
        }
    })
}
```

**rpc_services.go** — your entire gRPC surface (the example uses grpc's built-in health service
so it needs no protoc step; swap for `pb.RegisterGreeterServer(s, ...)` in a real project):

```go
package main

import (
    "go-spring.org/spring/gs"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    gozerozrpc "go-spring.org/starter-go-zero/zrpc"
)

func init() {
    // Provide exactly ONE ServiceRegister bean; it receives the *grpc.Server that
    // zrpc builds and registers your pb-generated services onto it.
    gs.Provide(func() gozerozrpc.ServiceRegister {
        return func(s *grpc.Server) {
            healthpb.RegisterHealthServer(s, health.NewServer())
        }
    })
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- gs built-in HTTP server ------------------------------------------------
# Disable it: the only HTTP listeners gs.Run() starts are the go-zero servers
# below. Without this, gs.Run() binds its own HTTP listener and complains that
# no handlers are registered.
spring.http.server.enabled=false

# --- go-zero REST server ----------------------------------------------------
# Bound from ${spring.go-zero.rest.server} by starter-go-zero/rest.
# `port` is the activation key: absent -> the whole sub-server stays out.
spring.go-zero.rest.server.name=greet
spring.go-zero.rest.server.host=0.0.0.0
spring.go-zero.rest.server.port=8888

# --- go-zero gRPC server ----------------------------------------------------
# `listen-on` is the activation key.
spring.go-zero.zrpc.server.name=greet-rpc
spring.go-zero.zrpc.server.listen-on=0.0.0.0:8081

# Optional etcd publication of the ListenOn endpoint for discovery consumers.
# Leave both empty for direct-connect (0 external deps to boot).
#spring.go-zero.zrpc.server.etcd.addr=127.0.0.1:2379
#spring.go-zero.zrpc.server.etcd.key=greet.rpc

# --- metrics: the 6060 collision workaround (REQUIRED when both active) -----
# rest and zrpc each start a go-zero DevServer on metrics.port, and BOTH default
# to 6060 — the second one fails to bind. Give them distinct ports:
spring.go-zero.rest.server.metrics.enabled=true
spring.go-zero.rest.server.metrics.port=6060
spring.go-zero.zrpc.server.metrics.enabled=true
spring.go-zero.zrpc.server.metrics.port=6061

# --- tracing -----------------------------------------------------------------
# Default (tracing.disabled=true) defers to starter-otel's global provider —
# nothing to configure here. Only flip disabled=false for go-zero's native
# OTLP export (then set endpoint/sampler/batcher per sub-server).

# --- logging -----------------------------------------------------------------
# go-spring's log module writes both business lines and go-zero framework logs
# (bridged, tag _rpc_gozero). log.level (below) is the only logx-side knob:
#spring.go-zero.rest.server.log.level=info
#spring.go-zero.zrpc.server.log.level=info

# --- observability (starter-otel, optional) ----------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**Prerequisite**: nothing for direct-connect. With `etcd.addr` set, an etcd at that address
(zrpc example pattern: `docker run -d -p 2379:2379 quay.io/coreos/etcd`). For the otel flavor,
Jaeger via `docker compose up -d` in `example-otel/` (all-in-one:1.60, OTLP :4317, UI :16686).

**Verify** (same shape as the examples' `check.sh`):

```bash
go run .
# startup lines: "go-zero rest server starting on 0.0.0.0:8888"
#                "go-zero zrpc server starting on 0.0.0.0:8081"

curl -i 'http://127.0.0.1:8888/greet?name=world'        # {"message":"Hi, world"}
grpcurl -plaintext 127.0.0.1:8081 grpc.health.v1.Health/Check   # status: SERVING
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

Per sub-server family (identical shape; shown for zrpc, rest differs only in keys):

```
import starter-go-zero/zrpc
  └─ init(): gs.Provide(NewZrpcServer, IndexArg(0, TagArg("${spring.go-zero.zrpc.server}")))
                .Export(gs.As[gs.Server]())
                .Condition(gs.OnProperty("spring.go-zero.zrpc.server.listen-on"))

gs.Run()
  ├─ condition gate: listen-on set? (no → bean never exists, starter inert)
  ├─ config bind: ${spring.go-zero.zrpc.server} → Config (value tags, prefix-relative)
  ├─ bean wiring: NewZrpcServer(cfg, reg ServiceRegister)   ← non-nullable autowire
  ├─ Rooter Init → Run(ctx, sig):
  │     ├─ build zrpc.RpcServerConf (Name/Log.Level/Telemetry/DevServer/ListenOn)
  │     ├─ etcd.addr != "" → conf.Etcd = discov.EtcdConf{Hosts, Key}
  │     ├─ zrpc.MustNewServer(conf, func(g){ reg(g) })
  │     ├─ logx.SetWriter(logger.NewWriter())   ← AFTER MustNewServer (see §3.3)
  │     ├─ <-sig.TriggerAndWait()               ← parks until gs signals readiness
  │     ├─ "go-zero zrpc server starting on <listen-on>" log line
  │     └─ go svr.Start() (binds listener, registers under Etcd.Key, blocks)
  │         / select on errCh | done
  └─ on SIGTERM: StopContext closes done → Run calls svr.Stop()
        (zrpc deregisters from etcd, drains the transport, gs completes shutdown)
```

Design notes (from source comments, verified):

- `Start` blocks internally (rest comment: "Start binds the listener and blocks until Stop is
  called"), so it runs in a goroutine while `Run` parks on the `done` channel; `Stop` closes
  `done` to hand control back to Go-Spring after tearing the server down.
- `svr.Stop()` takes no context — `StopContext`'s ctx only tags the shutdown log.
- If `Start` returns on its own, `Run` surfaces nil (errCh receives nil); real Start failures
  surface as listener-bind panics/errors inside go-zero — see §5.

### 2.2 Middleware / interceptor walk

Go-Spring adds **no middleware of its own** — you get go-zero's stock chain, nothing more:

- **rest**: `rest.MustNewServer(rc)` with no extra options → go-zero's built-in handler chain
  (recovery, logging, metrics, trace middleware as go-zero's `RestConf` defaults provide;
  [go-zero rest docs](https://go-zero.dev/docs/micro-service/restful-api)). No
  fault/governance, request-id or access-log layer is injected by this starter.
- **zrpc**: `zrpc.MustNewServer(conf, registerFn)` → go-zero's stock server interceptors
  (tracing, stat, breaker, prometheus — governed by `RpcServerConf`;
  [go-zero zrpc docs](https://go-zero.dev/docs/micro-service/rpc-call)). The register function
  the starter passes is exactly your `ServiceRegister` bean.

Trace middleware behavior (rest/starter.go Config comment, verified): with the default
`tracing.disabled=true` go-zero does **not** start its own trace agent, so the middleware emits
spans through whatever **global** OTel TracerProvider is installed — starter-otel's when
imported, a no-op otherwise. Flip `disabled=false` for go-zero's native OTLP export.

### 2.3 One request, layer by layer

REST `GET /greet?name=world`:

1. net/http listener on `host:port` accepts; go-zero's rest handler chain runs
   (recovery → logging → metrics → trace → route match).
2. trace middleware extracts/starts a span via the global provider (no-op without starter-otel).
3. your `HandlerRegister`-attached handler runs; reply `{"message":"Hi, world"}`.
4. go-zero framework events on this path flow through the log bridge (§4.3) under tag
   `_rpc_gozero`.

gRPC `grpc.health.v1.Health/Check` (direct dial):

1. Client dials `listen-on` with plaintext creds.
2. go-zero's server interceptor chain (stat/tracing/breaker/prometheus per RpcServerConf).
3. your `ServiceRegister`-registered Health service answers `SERVING`.
4. With `etcd.addr` set, consumers resolve `Etcd.Key` via go-zero's discov instead of dialing
   directly; deregistration happens on Stop.

---

## 3. Per-key behavior reference

### 3.1 `spring.go-zero.rest.server.*` (activation: `port`) — 11 keys, 1 required

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `port` | int | — | **Activation key.** Presence registers the server bean. | Missing → starter silently inactive. |
| `host` | string | `0.0.0.0` | Listen host of the rest server. | Unbindable host → listen error at Run. |
| `name` | string | `go-zero` | Service name (`RestConf.Name`); also logx `ServiceName`. | Purely cosmetic; no discovery coupling in rest. |
| `tracing.disabled` | bool | `true` | true = defer to the global OTel provider (starter-otel). ⚠ `endpoint`/`sampler`/`batcher` are **dead keys** unless this is false. | Flip to true-less setups: native OTLP silently idle. |
| `tracing.endpoint` | string | `""` | go-zero native OTLP collector addr; only when `disabled=false`. | Set without flipping disabled → ignored. |
| `tracing.sampler` | float64 | `1.0` | Native OTLP sampling ratio; only when `disabled=false`. | Same dead-key rule. |
| `tracing.batcher` | string | `otlpgrpc` | Native batcher kind; only when `disabled=false`. | Same dead-key rule. |
| `metrics.enabled` | bool | `true` | Starts go-zero's DevServer (Prometheus). ⚠ distinct from starter-otel's metrics pipeline — the two never merge. | Unintended extra listener on 6060. |
| `metrics.port` | int | `6060` | DevServer listen port. ⚠ **collides with zrpc's identical default** when both sub-servers are active — the second DevServer fails to bind. | Process crash / DevServer error when rest+zrpc coexist on defaults (see §5). |
| `metrics.path` | string | `/metrics` | Prometheus scrape path on the DevServer. | Scraper pointed elsewhere → 404. |
| `log.level` | string | `info` | The **only** logx field honored after the bridge: logx filters by level before delegating. `debug`/`info`/`error`/`severe` (go-zero values). | Wrong casing/value → logx default. |

DevServer `EnablePprof` is hard-coded false in the starter — there is no key for it.

### 3.2 `spring.go-zero.zrpc.server.*` (activation: `listen-on`) — 12 keys, 1 required

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `listen-on` | string | — | **Activation key**; also the gRPC listen address (`host:port`). | Missing → starter silently inactive. |
| `name` | string | `go-zero` | Service name; also logx `ServiceName`. | Cosmetic. |
| `etcd.addr` | string | `""` | Empty = direct-connect. Set → publish `ListenOn` under `etcd.key` on Start, deregister on Stop. ⚠ single-host only — the starter builds `discov.EtcdConf{Hosts: []string{addr}}`; no cluster list. | Unreachable etcd → zrpc start error at Run. |
| `etcd.key` | string | `""` | Discovery key consumers resolve. ⚠ meaningless without `etcd.addr`. | Empty key with addr set → go-zero discov default behavior. |
| `tracing.disabled` | bool | `true` | Same policy as rest §3.1 (zrpc trace interceptor uses the ambient global provider). | Same as rest. |
| `tracing.endpoint` / `.sampler` / `.batcher` | string/float/string | `""`/`1.0`/`otlpgrpc` | Native OTLP knobs; dead unless `disabled=false`. | Same dead-key rule. |
| `metrics.enabled` | bool | `true` | Same DevServer as rest. ⚠ same 6060 collision. | Same as rest. |
| `metrics.port` | int | `6060` | ⚠ set a **distinct** port when rest is also active (e.g. 6061). | Port-in-use failure at startup. |
| `metrics.path` | string | `/metrics` | Scrape path. | 404 on mismatch. |
| `log.level` | string | `info` | Same as rest §3.1 — filters the logx side of the bridge. | Same as rest. |

### 3.3 Log bridge (internal/logger)

go-zero's `logx.Writer` methods carry no `context.Context`, so (internal/logger/logger.go,
verified):

- every forwarded line is tagged `_rpc_gozero` (`log.RegisterRPCTag("gozero", "")`) — filter or
  route it via go-spring's logger tag config;
- trace-id propagation via `log.FieldsFromContext` cannot fire, but go-zero injects
  trace/span ids via `logx.WithContext` fields, which are forwarded as structured fields;
- the recorded caller (file:line) points into the bridge, not the real emit site;
- **install timing matters**: `MustNewServer` runs `ServiceConf.SetUp()` → `logx.SetUp()` which
  installs logx's own writer, so each sub-starter re-installs the bridge via `logx.SetWriter`
  **after** building its server. Consequence: with both sub-servers active, the logx level that
  applies is whichever sub-server's `Run` executed last (process-global writer + level) — keep
  `log.level` identical across the two prefixes.

Level mapping: Debug/Info/Error 1:1; `Alert`→Error, `Severe`→Fatal (level signal only — the
bridge never calls os.Exit), `Slow`→Warn, `Stat`→Info, `Stack`→Error.

### 3.4 Wrapper tags are prefix-relative (verified)

The nested struct tags `value:"${tracing}"` / `${metrics}` / `${log}` / `${etcd}` bind under the
ctor-arg prefix (`gs.IndexArg(0, gs.TagArg("${spring.go-zero.<family>.server}"))`), i.e. they
are **not** top-level absolute references. No `${observability:=}`-style wrapper field exists in
this starter (grep-verified against all value tags).

---

## 4. Beans, observability & drills

### 4.1 Beans provided (per activated family)

| Bean | Type | Notes |
|------|------|-------|
| REST server | `*StarterGoZeroRest.RestServer`, exported as `gs.Server` | Exists only when `...rest.server.port` is set. |
| gRPC server | `*StarterGoZeroZrpc.ZrpcServer`, exported as `gs.Server` | Exists only when `...zrpc.server.listen-on` is set. |

### 4.2 Bean you must provide (per family)

| Family | Bean type | Typical body |
|--------|-----------|--------------|
| rest | `StarterGoZeroRest.HandlerRegister = func(*rest.Server)` | `server.AddRoute(rest.Route{...})` |
| zrpc | `StarterGoZeroZrpc.ServiceRegister = func(*grpc.Server)` | `pb.RegisterGreeterServer(s, svc)` |

Non-nullable autowire: zero beans → container failure; two of one type → ambiguity failure.
The starter never learns your route table or pb types.

### 4.3 Observability surface

- **Logs**: framework lines under `_rpc_gozero`; business lines under your own tags; the
  startup/shutdown lines under the app-def tag.
- **Tracing**: rides starter-otel's global provider by default — zero go-zero config. The
  example-otel pair proves the path end-to-end (20 requests → Jaeger at
  `http://127.0.0.1:16686`, service `gozero-rest-otel-example` / `gozero-zrpc-otel-example`).
- **Metrics**: go-zero-native Prometheus only, on the DevServer (`metrics.port`), independent
  of starter-otel's metrics pipeline.

### 4.4 Verification & fault drills

```bash
# REST round trip (same assertion as rest/example/check.sh):
curl -i 'http://127.0.0.1:8888/greet?name=world'    # expect {"message":"Hi, world"}

# gRPC round trip (grpcurl, same as zrpc/example's health check):
grpcurl -plaintext 127.0.0.1:8081 grpc.health.v1.Health/Check   # status: SERVING

# DevServer metrics (after some traffic):
curl -s :6060/metrics | grep -E 'prometheus|metric' | head

# The 6060 collision drill: comment out one of the distinct metrics.port lines in
# conf/app.properties so both default to 6060, then `go run .` → the second
# DevServer fails to bind (listen :6060 in use). Restore the split ports.

# Framework logs flow through the bridge:
grep _rpc_gozero <your log file> | head

# SIGTERM graceful shutdown drill:
kill -TERM <pid>   # expect BOTH lines:
                   # "go-zero rest server shutting down on 0.0.0.0:8888"
                   # "go-zero zrpc server shutting down on 0.0.0.0:8081"
# with etcd set, the key disappears after stop:
ETCDCTL_API=3 etcdctl get --prefix ''   # no greet.rpc entry

# Tracing drill (example-otel pattern):
docker compose -f rest/example-otel/docker-compose.yml up -d
go run ./rest/example-otel   # sends 20 reqs, asserts Jaeger has the service, self-exits
```

Observables: startup/shutdown log lines under the app-def tag; spans named per go-zero's
rest/zrpc middleware defaults; Prometheus series from go-zero's DevServer registry.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Nothing starts, no go-zero log lines | activation key missing (`port` / `listen-on`) | Set it — it is the on/off switch. |
| Container fails: register bean missing | key set but no `HandlerRegister`/`ServiceRegister` bean | Provide exactly one for that family. |
| Container fails: ambiguous bean | two register beans of one type | Keep exactly one. |
| gs complains "no handlers registered" on HTTP | built-in HTTP server still enabled | `spring.http.server.enabled=false`. |
| Startup fails binding :6060 | rest+zrpc both active, both default `metrics.port=6060` | Set distinct ports (6060/6061) or disable one via `metrics.enabled=false`. |
| zrpc start error with etcd configured | `etcd.addr` unreachable | Start etcd or leave `etcd.addr` empty (direct-connect). |
| Everything works, no traces | starter-otel not imported (global provider is a no-op) | Add it; or flip `tracing.disabled=false` + endpoint for native OTLP. |
| `tracing.endpoint` seems ignored | dead key while `disabled=true` (the default) | Flip `tracing.disabled=false` first. |
| go-zero logs lack trace ids / wrong file:line | bridge limitation: logx.Writer has no ctx | Correlation still rides trace/span fields; filter via `_rpc_gozero` (§3.3). |
| Conflicting log levels between the two families | `logx.SetWriter` + level are process-global; last `Run` wins | Keep `log.level` identical in both prefixes. |
| zrpc silently absent though "port" configured | wrong key: zrpc binds `listen-on`, not `port` | Use `spring.go-zero.zrpc.server.listen-on=host:port`. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 11 (rest) + 12 (zrpc) |
| Required | 1 per family (`port` / `listen-on`) + 1 bean |
| Quickstart external deps | 0 direct-connect; 1 (etcd) with registration; +1 (Jaeger) for the otel drill |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger):

- **port collision**: rest and zrpc both default `metrics.port=6060` — importing both with
  defaults double-binds the DevServer port; consider per-family defaults (6060/6061) or a
  shared singleton DevServer.
- **process-global mutations**: `logx.SetWriter` and the tracing globals are process-wide, so
  two sub-servers cannot disagree on log/trace setup; the bridge is also re-installed per
  sub-server `Run` (last one wins).
- **log.level-only**: after the bridge, `log.level` is the only logx knob honored — logx's
  rich `LogConf` (encoding, rotation, stat intervals) is not exposed and partly overridden.
- DevServer's pprof switch is hard-disabled with no config escape.
- etcd config is single-host (`Hosts: []string{addr}`) — no cluster list, no TLS.
- no health indicator bean for either sub-server (readiness is only the generic gs.Server
  signal); zrpc's grpc health service must be registered manually by the app.
- cross-family inconsistency: metrics flag is `enabled` here but `enable` in the kratos family.
- stale doc risk: both plain example configs' comments claim metrics stay off by default, but
  the code default is `enabled=true`; zrpc/example-otel config sets
  `spring.go-zero.zrpc.server.port` (a nonexistent key for zrpc — activation requires
  `listen-on`), so that example's server likely never starts as configured.
