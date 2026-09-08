# starter-kratos Usage — Reference

Umbrella usage reference covering the three sub-server packages (`http/`, `grpc/`, `ws/`) and
the internal log bridge. Overview: [README.md](README.md). Anchored to the runnable examples in
[contrib/go-kratos/](../../../contrib/go-kratos/) (provider/consumer pairs with
`scripts/smoke-test.sh`). **kratos' own semantics (transport, middleware, App lifecycle,
proto codegen) are [kratos' documentation](https://go-kratos.dev/docs/)** — everything below is
Go-Spring's increment: activation keys, bean wiring, etcd registration, observability, log
bridge.

**Activation**: each sub-server exists only when its addr key is set — the key is the on/off
switch; there is no `enabled` key:

- `spring.kratos.http.server.addr` → HTTP transport (`khttp`)
- `spring.kratos.grpc.server.addr` → gRPC transport (`kgrpc`)
- `spring.kratos.ws.server.addr` → WebSocket transport (`kws`, tx7do fork)

The three are independent and can coexist in one process (each builds its own `kratos.App`).
Activation also requires the application to provide that family's `ServiceRegister` bean —
the autowire is non-nullable, so a configured addr without a bean fails the container (and
vice versa: a bean without addr silently leaves the starter out).

---

## 1. Complete worked project

A realistic gRPC-flavor service (the shape is identical for http/ws): one proto service,
etcd publication, JSON logging, OTel metrics/tracing. File tree (mirrors
`contrib/go-kratos/grpc/provider/`):

```
demo/
├── go.mod
├── main.go
├── handler.go
├── idl/helloworld/v1/            # protoc-generated (greeter.pb.go, greeter_grpc.pb.go)
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/go-kratos/kratos/v2  v2.9.x
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-kratos    latest   // import path selects http/ grpc/ ws/
    go-spring.org/starter-otel      latest   // optional: real metric/trace export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-kratos/grpc" // side-effect import registers the server bean
    _ "go-spring.org/starter-otel"        // optional: lights up tracing/metrics export
)

func main() { gs.Run() }
```

**handler.go** — the application's entire gRPC surface:

```go
package main

import (
    "context"

    kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
    v1 "demo/idl/helloworld/v1"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    kratosgrpc "go-spring.org/starter-kratos/grpc"
)

func init() {
    // Provide exactly ONE ServiceRegister bean. The starter owns the kratos.App
    // and the transport server; you bind your generated RegisterXxxServer here.
    gs.Provide(func() kratosgrpc.ServiceRegister {
        return func(s *kgrpc.Server) error {
            v1.RegisterGreeterServer(s, &GreeterService{})
            return nil
        }
    })
}

type GreeterService struct{ v1.UnimplementedGreeterServer }

func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
    log.Infof(ctx, log.TagBizDef, "SayHello name=%s", in.Name)
    return &v1.HelloReply{Message: "Hello " + in.Name}, nil
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- gs built-in HTTP server ------------------------------------------------
# Disable it: the only server gs.Run() starts is the kratos transport below.
# Without this, gs.Run() binds an HTTP listener and complains that no handlers
# are registered.
spring.http.server.enabled=false

# --- kratos gRPC transport --------------------------------------------------
# Bound from ${spring.kratos.grpc.server} by starter-kratos/grpc.
spring.kratos.grpc.server.name=kratos-grpc
spring.kratos.grpc.server.addr=0.0.0.0:9000
spring.kratos.grpc.server.timeout=1s

# etcd registration: the App publishes {name, endpoints} here on startup and
# deregisters on stop. Leave empty for a plain direct-connect server.
spring.kratos.grpc.server.etcd.addr=127.0.0.1:2379

# Request metrics (default on). Recorded into the global OTel meter; exporter
# and scrape endpoint are owned by starter-otel, not this starter.
spring.kratos.grpc.server.metrics.enable=true

# --- logging ----------------------------------------------------------------
# go-spring's log module writes both business lines and kratos' framework logs
# (bridged by starter-kratos, tag _rpc_kratos) as structured JSON.
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=provider.log
logging.logger.root.layout.type=JSONLayout

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**Prerequisite**: an etcd reachable at `127.0.0.1:2379` (each contrib example ships a
`docker-compose.yml`), or leave `etcd.addr` empty for direct-connect (0 external deps).

**Verify** (same shape as `contrib/go-kratos/grpc/scripts/smoke-test.sh`):

```bash
# after gs.Run(): startup line appears in the log
grep 'kratos grpc server starting on 0.0.0.0:9000' ../logs/provider.log

# discovery call from any client with the same etcd (consumer example):
#   dial "discovery:///kratos-grpc" → SayHello → "Hello Kratos"
# service published in etcd:
ETCDCTL_API=3 etcdctl get --prefix /microservices/kratos-grpc/
```

For the HTTP flavor change the import to `starter-kratos/http`, the keys to
`spring.kratos.http.server.*` (default addr `0.0.0.0:8000`, name `kratos-http`), and register
with `v1.RegisterGreeterHTTPServer`. For WebSocket see §4.4.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

Per sub-server family (identical shape; shown for grpc):

```
import starter-kratos/grpc
  └─ init(): gs.Provide(NewGrpcServer, IndexArg(0, TagArg("${spring.kratos.grpc.server}")))
                .Export(gs.As[gs.Server]())
                .Condition(gs.OnProperty("spring.kratos.grpc.server.addr"))

gs.Run()
  ├─ condition gate: addr set? (no → bean never exists, starter inert)
  ├─ config bind: ${spring.kratos.grpc.server} → Config (value tags)
  ├─ bean wiring: NewGrpcServer(cfg, reg ServiceRegister)   ← non-nullable autowire
  ├─ Rooter Init → Run(ctx, sig):
  │     ├─ build middleware chain (§2.2), kgrpc.NewServer, reg(srv)
  │     ├─ optional etcd client (DialTimeout 5s) → kratos.Registrar(etcd.New(cli))
  │     ├─ kratos.New(Name, Logger(bridge), Server(srv), [Registrar])
  │     ├─ <-sig.TriggerAndWait()          ← parks until gs signals readiness
  │     ├─ "kratos grpc server starting on <addr>" log line
  │     └─ go app.Run()  (publishes into etcd, serves) / select on done|errCh
  └─ on SIGTERM: Stop → Stop closes done → Run calls app.Stop()
        (kratos deregisters from etcd, drains the transport, gs completes shutdown)
```

Design notes (from source comments, verified):

- `kratos.App.Run` blocks until `Stop`, so it runs in a goroutine while `Run` parks on the
  `done` channel; `Stop` closes `done` to hand control back to Go-Spring after tearing the
  App down.
- `app.Stop()` takes no context — `Stop`'s ctx only tags the shutdown log.
- If `app.Run()` returns an error on its own, `Run` surfaces it via
  `errutil.Explain(err, "kratos grpc app exited with error")`.

### 2.2 Middleware chain — exact order and why (http/grpc)

```
recovery.Recovery() → tracing.Server() → [kmetrics.Server() if metrics.enable]
→ your service handlers (registered via ServiceRegister)
```

Rationale (from the source comment in `Run`):

- **recovery outermost**: a panic anywhere downstream is recovered before it crosses the
  transport boundary.
- **tracing before metrics**: tracing starts the span, then metrics records under the
  active span/context. `tracing.Server()` reads the global OTel provider installed by
  starter-otel; absent that it is a no-op.
- **metrics innermost**: the counter/histogram observe the handler's outcome and duration.

There is no fault/governance, loadtest, request-id or access-log layer in this starter —
kratos' middleware system is yours to extend by adding `khttp.Middleware`/`kgrpc.Middleware`
is NOT exposed via config; the chain is fixed (see design suspects).

**ws has no middleware chain at all** — the tx7do transport is not instrumented: no tracing,
no metrics, no recovery beyond kratos-transport internals.

### 2.3 One request, layer by layer (grpc)

`SayHello("Kratos")` over the discovery endpoint:

1. Client dials `discovery:///kratos-grpc` → etcd watch resolves a live endpoint → gRPC
   connection to `0.0.0.0:9000`.
2. recovery arms (deferred recover).
3. tracing extracts the span context from gRPC metadata; starts the server span (no-op
   without starter-otel's provider).
4. metrics increments `server_requests_code_total` and starts the
   `server_requests_seconds` observation.
5. your handler runs; the reply unwinds 4→3: duration recorded, span ended.
6. kratos framework events on this path (transport errors etc.) flow through the log bridge
   (§3.3) into go-spring's log under tag `_rpc_kratos`.

---

## 3. Per-key behavior reference

### 3.1 `spring.kratos.http.server.*` (activation: `addr`) — 6 keys, 1 required

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** Presence registers the server bean. | Missing → starter silently inactive (handlers configured but never served). |
| `name` | string | `kratos-http` | Service name published into etcd; also the OTel **meter name** (`otel.Meter(cfg.Name)`), so metric series are scoped per server name. | Mismatch with consumers' `discovery:///<name>` → resolution failure at call time, not at boot. |
| `network` | string | `""` | Empty = tcp; e.g. `unix` for a unix-domain socket. | Wrong value → listen error at Run. |
| `timeout` | duration | `1s` | kratos server timeout (deadline propagated per request). `0` skips the option. | Too low → fast handlers still pass, slow ones get deadline-exceeded errors. |
| `etcd.addr` | string | `""` | Empty = direct-connect (no registration). Set → App publishes `{name, endpoints}` into etcd on Run and deregisters on stop. Client dials `etcd.DialTimeout` 5s. ⚠ unreach etcd → Run fails at startup with "failed to create etcd client" only if client construction fails; a reachable-but-wrong cluster surfaces later. | Wrong cluster → service invisible to discovery consumers. |
| `metrics.enable` | bool | `true` | Installs `kmetrics.Server()` (http/grpc only) recording into the **global** OTel meter; exporter/scrape are owned by starter-otel. | No starter-otel → metrics recorded into a no-op meter; nothing warns. |

### 3.2 `spring.kratos.grpc.server.*` (activation: `addr`) — 6 keys, 1 required

Identical key set to §3.1; `name` defaults to `kratos-grpc`. Middlewares, etcd, meter naming
behave exactly the same.

### 3.3 `spring.kratos.ws.server.*` (activation: `addr`) — 5 keys, 1 required

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** | Same as above. |
| `name` | string | `kratos-ws` | etcd service name. ⚠ kratos-transport's WS **client has no discovery hook** — consumers dial `ws://host:port/path` directly; registration is parity-only (per the example's config comment). | Registering into etcd does not make WS discoverable. |
| `network` | string | `""` | Empty = tcp. | Same as above. |
| `path` | string | `/` | WebSocket upgrade path. | Client URL path must match, else handshake 404. |
| `etcd.addr` | string | `""` | Same semantics as http/grpc. | Same as above. |

ws has **no** `timeout` and **no** `metrics.*` keys — the WebSocket transport has no
middleware chain and is not instrumented.

### 3.4 Wire format (ws, fixed — no key)

Every frame is `<4-byte little-endian uint32 messageType><JSON-encoded payload>` (binary
payload type, JSON codec). The message-type integer is an application contract agreed out of
band; payloads are plain JSON structs, typically NOT the protoc-generated types (see the ws
example's handler.go rationale). The `github.com/tx7do/kratos-transport` dependency is
pinned to v1.3.1: v1.3.4 broke session registration (wsHandler no longer registers with the
SessionManager, `SendMessage` fails "session not found"). Binary mode is chosen over text
mode because the pinned version's text mode is asymmetric (unwraps a `{"type","payload"}`
envelope on receive but replies without one).

### 3.5 Log bridge (internal/logger)

kratos' `log.Logger` signature carries no `context.Context`, so:

- every forwarded line is tagged `_rpc_kratos` (`log.RegisterRPCTag("kratos", "")`) —
  filter/route it via go-spring's logger tag config;
- trace-id propagation via `log.FieldsFromContext` is not available on this path;
- the recorded caller (file:line) points into the bridge, not the real emit site.

Level mapping is one-to-one over the five shared levels (kratos has no Trace/Panic). kratos'
`msg` keyval becomes the event message; the redundant `level` keyval is dropped; everything
else is forwarded as structured fields.

---

## 4. Beans & API

### 4.1 Beans provided (per activated family)

| Bean | Type | Notes |
|------|------|-------|
| HTTP server | `*StarterKratosHttp.HttpServer`, exported as `gs.Server` | Wraps the `kratos.App`; created only when `spring.kratos.http.server.addr` is set. |
| gRPC server | `*StarterKratosGrpc.GrpcServer`, exported as `gs.Server` | Same for the grpc prefix. |
| WS server | `*StarterKratosWs.WsServer`, exported as `gs.Server` | Same for the ws prefix. |

### 4.2 Bean you must provide (per family)

| Family | Bean type | Typical body |
|--------|-----------|--------------|
| http | `StarterKratosHttp.ServiceRegister = func(hs *khttp.Server) error` | `v1.RegisterGreeterHTTPServer(hs, svc)` |
| grpc | `StarterKratosGrpc.ServiceRegister = func(gs *kgrpc.Server) error` | `v1.RegisterGreeterServer(gs, svc)` |
| ws | `StarterKratosWs.ServiceRegister = func(ws *kws.Server) error` | `kws.RegisterServerMessageHandler(ws, msgType, handler)` |

The autowire is non-nullable: zero beans → container failure; two beans of the same type →
ambiguity failure. Exactly one per family. The starter never learns your service types —
registration is entirely in your bean.

### 4.3 Programmatic surface

Nothing beyond the two beans: no exported client helpers, no exported middleware builders, no
`ApplyMiddlewares` equivalent. The middleware chain is fixed at Run (§2.2). To add kratos
middleware you currently cannot — see design suspects.

### 4.4 Verification & fault drills

```bash
# HTTP: round trip + metrics via starter-otel's prometheus exporter
curl -i :8000/helloworld/kratos
curl -s :9464/metrics | grep -E 'server_requests_code_total|server_requests_seconds'

# gRPC: any client against the discovery endpoint; framework logs land under _rpc_kratos
grep _rpc_kratos ../logs/provider.log | head

# WS: hand-craft one frame (any language); reply uses the same framing
python3 - <<'EOF'
import socket,struct,json,base64,hashlib  # or use a ws client; framing shown for clarity
EOF
# 4-byte LE msgType=1 || {"name":"Kratos"} → reply 4-byte LE 1 || {"message":"Hello Kratos"}

# Shutdown drill: SIGTERM → log shows "kratos grpc server shutting down on <addr>",
# etcd key removed:
ETCDCTL_API=3 etcdctl get --prefix /microservices/kratos-grpc/   # empty after stop
```

Observables:

- **Metrics** (http/grpc, on by default): counter `server_requests_code_total` and histogram
  `server_requests_seconds`, created via `otel.Meter(cfg.Name)` — attributes follow kratos'
  metrics middleware defaults (`operation`, `code`); instrumented per server `name`.
- **Tracing**: span per request from `tracing.Server()`; check your collector after traffic.
- **Logs**: framework lines under `_rpc_kratos`; business lines under your own tags; startup
  and shutdown lines under the app-def tag.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Nothing starts, no kratos log lines | addr key missing | Set `spring.kratos.<family>.server.addr` — it is the activation switch. |
| Container fails: ServiceRegister missing | addr set but no bean provided | Provide exactly one `ServiceRegister` bean for that family. |
| Container fails: ambiguous bean | two `ServiceRegister` beans of one type | Keep exactly one. |
| gs complains "no handlers registered" on HTTP | built-in HTTP server still enabled | `spring.http.server.enabled=false`. |
| Discovery consumers can't find the service | `etcd.addr` empty/wrong, or `name` mismatch | Set `etcd.addr`; align `name` with the client's `discovery:///<name>`. |
| Run fails: "failed to create etcd client" | etcd unreachable at construction | Start etcd (example docker-compose) or leave `etcd.addr` empty. |
| Everything works, no traces/metrics | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| WS replies never arrive / "session not found" | dependency drifted past the v1.3.1 pin, or text-mode framing assumed | Keep the pin; speak binary framing `<4B LE type><JSON>`. |
| WS client 404 on handshake | client path ≠ `path` key | Align the URL path (default `/`). |
| kratos logs lack trace ids / wrong file:line | bridge limitation: kratos `Log` has no ctx | Accepted trade-off (see §3.5); use `_rpc_kratos` filtering instead. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 (http) + 6 (grpc) + 5 (ws) |
| Required | 1 per family (`addr`) + 1 bean |
| Quickstart external deps | 0 direct-connect; 1 (etcd) with registration |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger):

- metrics flag named `enable` here but `enabled` in goframe/go-zero families
  (cross-family inconsistency).
- ws transport pinned to a fork at v1.3.1 with no upstream fix; text-mode asymmetry forces
  binary framing with no config escape.
- no health indicator for any sub-server (nothing feeds gs actuator readiness beyond the
  generic gs.Server signal).
- middleware chain is hard-coded in Run — no config or bean seam to append kratos middleware
  (no fault/governance/request-id/access-log parity with starter-echo/gin).
- ws etcd registration is parity-only: the WS client has no discovery hook, so the key
  advertises a service nothing can discover over WS.
- log bridge loses context (no trace ids, caller points into the bridge) — inherent to
  kratos' `log.Logger` signature, but worth revisiting if kratos grows a ctx-aware variant.
