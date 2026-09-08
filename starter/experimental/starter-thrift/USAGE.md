# starter-thrift Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `middleware.go` — note the package is `StarterThrift`,
an experimental/ module, i.e. **unreviewed marker, not a quality tier**) and the runnable
[example/](example/) (self-asserting via `example/check.sh`) and [example-otel/](example-otel/).
**Thrift's own semantics** (IDL, code generation, protocol/transport wire rules, TSimpleServer
behavior) are [apache/thrift's documentation](https://thrift.apache.org/) — everything below is
go-spring's increment: wiring, lifecycle, and the observability wrapper. Per project policy,
go-spring does not aim to perfect thrift protocol support; protocol concerns defer to mature
frameworks, and this starter is deliberately the thinnest of the RPC family.

**Activation**: the server bean exists only when `spring.thrift.server.addr` is set
(`gs.OnProperty("spring.thrift.server.addr")` in `starter.go` init) — that key is the on/off
switch; there is no `enabled` key and **no default port** (you must configure one; the port is
a startup condition). The server is apache/thrift's `TSimpleServer` (single accept loop,
goroutine per connection) — the only server model the Go Thrift library ships.

---

## 1. Complete worked project

A realistic Thrift service: IDL → generated processor → wrapped (middleware) processor →
server, plus tracing/metrics export. File tree (mirrors `example/`):

```
demo/
├── go.mod
├── main.go
├── handler.go
├── middleware.go
├── idl/
│   └── echo.thrift
└── conf/
    └── app.properties
```

**idl/echo.thrift** (generate with the Apache Thrift compiler, `thrift -r --gen go:package_prefix=demo/idl/ idl/echo.thrift`;
generated code lands under `idl/gen-go/`, vendored like example's `idl/proto/`):

```thrift
namespace go proto

struct EchoRequest {
1: required string message
}

struct EchoResponse {
1: required string message
}

service EchoService {
  EchoResponse echo(1: EchoRequest req)
}
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"    // optional: real trace/metric export
    _ "go-spring.org/starter-thrift"
)

func main() { gs.Run() }
```

**handler.go** — the service implementation plus the processor bean:

```go
package main

import (
    "context"

    "github.com/apache/thrift/lib/go/thrift"
    "go-spring.org/spring/gs"

    "demo/idl/proto" // your generated package
)

func init() {
    // The application provides any bean implementing thrift.TProcessor.
    // Exactly one processor bean per app (see §3).
    gs.Provide(&Controller{})
    gs.Provide(func(c *Controller) thrift.TProcessor {
        return newLoggingProcessor(proto.NewEchoServiceProcessor(c))
    })
}

// Controller satisfies proto.EchoService.
type Controller struct{}

func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
    return &proto.EchoResponse{Message: req.Message}, nil
}
```

**middleware.go** — the extension point: a TProcessor decorator. ⚠ You cannot naively
"log then call inner.Process": the input protocol's message header is single-shot, so the
decorator must re-implement the tiny dispatch loop against the inner processor's
`ProcessorMap` — this is exactly what example's `loggingProcessor` does:

```go
package main

import (
    "context"

    "github.com/apache/thrift/lib/go/thrift"
    "go-spring.org/log"
)

type loggingProcessor struct{ inner thrift.TProcessor }

func newLoggingProcessor(inner thrift.TProcessor) *loggingProcessor {
    return &loggingProcessor{inner: inner}
}

func (p *loggingProcessor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (bool, thrift.TException) {
    name, _, seqId, err := iprot.ReadMessageBegin(ctx)
    if err != nil {
        return false, thrift.WrapTException(err)
    }
    log.Infof(ctx, log.TagAppDef, "thrift middleware: method=%s seq=%d", name, seqId)
    if fn, ok := p.inner.ProcessorMap()[name]; ok {
        return fn.Process(ctx, seqId, iprot, oprot)
    }
    // Unknown method: mirror generated behaviour to keep the wire well-formed.
    _ = iprot.Skip(ctx, thrift.STRUCT)
    _ = iprot.ReadMessageEnd(ctx)
    x := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "Unknown function "+name)
    _ = oprot.WriteMessageBegin(ctx, name, thrift.EXCEPTION, seqId)
    _ = x.Write(ctx, oprot)
    _ = oprot.WriteMessageEnd(ctx)
    _ = oprot.Flush(ctx)
    return false, x
}

func (p *loggingProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
    return p.inner.ProcessorMap()
}
func (p *loggingProcessor) AddToProcessorMap(name string, fn thrift.TProcessorFunction) {
    p.inner.AddToProcessorMap(name, fn)
}
```

**conf/app.properties** — the complete, commented surface actually used above (copied from
example/conf):

```properties
# Disable gs's built-in HTTP server so only the Thrift endpoint is exposed.
spring.http.server.enabled=false

# Thrift server bind address. REQUIRED — also the activation switch; no default.
spring.thrift.server.addr=:9292

# Per-connection socket timeout on the server socket (0 = no timeout).
spring.thrift.server.clientTimeout=30s

# Wire protocol: binary (default) / compact / json. The CLIENT must use the
# matching protocol factory (example uses compact to exercise non-default).
spring.thrift.server.protocol=compact

# Transport wrapper: none (raw socket, default) / buffered / framed. The
# CLIENT must use the matching transport (framed: client wraps TSocket in
# TFramedTransport).
spring.thrift.server.transport=framed

# Buffer size (buffered) / max frame size (framed), bytes.
spring.thrift.server.bufferSize=4096

# Observer (tracing/metrics) is on by default; shown for discoverability:
spring.thrift.server.observer.tracing.enabled=true
spring.thrift.server.observer.metrics.enabled=true

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# TLS (left off so the plaintext client can connect):
# spring.thrift.server.tls.enabled=true
# spring.thrift.server.tls.cert-file=server.crt
# spring.thrift.server.tls.key-file=server.key
```

**Client** (in-process in the example; standalone is identical) — must match the server's
`protocol=compact` + `transport=framed`:

```go
socket := thrift.NewTSocketConf(":9292", nil)
transport := thrift.NewTFramedTransportConf(socket, nil)
defer transport.Close()
client := proto.NewEchoServiceClientFactory(transport, thrift.NewTCompactProtocolFactoryConf(nil))
if err := transport.Open(); err != nil { /* ... */ }
resp, err := client.Echo(ctx, &proto.EchoRequest{Message: "Hello, Thrift!"})
```

**Verify** (structurally identical to what the example asserts):

```bash
cd example && ./check.sh          # self-asserting: echo round-trip x2 + middleware count == 2, exit 0
# or manually:
cd example && go run .            # prints "Response from server: Hello, Thrift!" etc., self-SIGTERMs
cd example && go run . -manual    # server stays up for interactive probing
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-thrift
  └─ init(): gs.Provide(NewSimpleThriftServer, IndexArg(0, TagArg("${spring.thrift.server}")))
             .Export(As[gs.Server]()) .Condition(OnProperty("spring.thrift.server.addr"))
gs.Run()
  ├─ config bind: ${spring.thrift.server} → Config (value tags; NewSimpleThriftServer logs
  │   "thrift server created addr=… protocol=… transport=…" at Debug)
  ├─ bean wiring: your thrift.TProcessor bean autowired into NewSimpleThriftServer
  │   (missing/duplicate processor bean → container fails)
  ├─ gs.Server phase — SimpleThriftServer.Run(ctx, sig):
  │   ├─ newTransport(): listen (TLS via tlsconf.BuildClient() when tls.enabled)  ← socket opens HERE
  │   ├─ protocolFactory()/transportFactory(): validated — bad protocol/transport name fails boot
  │   ├─ WrapProcessor(proc) when tracing or metrics enabled — OUTERMOST processor layer
  │   ├─ <-sig.TriggerAndWait()  ← waits for readiness before serving
  │   └─ svr.Serve() blocks; "thrift server starting on :9292" logged (Info, TagAppDef)
  └─ on SIGTERM: Stop → "thrift server shutting down" → s.svr.Stop()
```

### 2.2 Processor chain — exact order and why

```
TSimpleServer → WrapProcessor (observedProcessor) → your decorator → generated processor
```

- **WrapProcessor outermost** (applied in `Run`, around whatever bean you provided): every
  call — including calls your decorator short-circuits or mangles — is counted, timed and
  spanned. Metric instruments are created once at `WrapProcessor` time and stored on the
  struct ("avoiding any lazy init on the hot path", per the source comment on
  `observedProcessor`).
- **Your decorator inner**: it consumes the message header to learn the method name, then
  dispatches via `ProcessorMap()` (see §1 for why it must re-implement dispatch).
- The starter deliberately has **no built-in middleware chain** — no recovery, no request-id,
  no access log, no admission, no fault injection. The decorator is the only cross-cutting
  seam (design stance: no shared interceptor protocol; each family carries a minimal seam).

### 2.3 One call, layer by layer

`client.Echo(ctx, req)` with server `protocol=compact transport=framed`:

1. Client writes a framed compact message to :9292.
2. `TSimpleServer` accept loop hands the connection to a goroutine.
3. `observedProcessor.Process`: starts OTel server span `thrift.process`
   (`rpc.system=thrift`; a **new root** — see §4.2), increments
   `rpc.server.active_requests`, starts the clock.
4. Your decorator: `ReadMessageBegin` → logs method/seq → dispatches `Echo` via ProcessorMap.
5. Generated processor decodes args, calls `Controller.Echo`, encodes the reply.
6. Unwind: `observeEnd` records `rpc.server.request_count` +1 and
   `rpc.server.request.duration` (seconds; buckets 5ms…10s) with
   `rpc.thrift.status_code=ok|error`; in-flight −1; span ends — on TException the span also
   gets `thrift.error_code` (numeric `TExceptionType`) and Error status.

---

## 3. Per-key behavior reference

All keys under `spring.thrift.server.*`. Verified against
`grep -rhoE 'value:"[^"]+"' … | sort -u` (10 tags; tls sub-keys from cloud/tlsconf).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key + required.** Presence registers the server bean (`OnProperty`). Also the listen address; TLS path swaps in `NewTSSLServerSocketTimeout`. | Missing → starter silently inactive. Bad address → `Run` fails at listen ("failed to listen on …"). |
| `clientTimeout` | duration | 0 | Socket timeout passed to `NewTServerSocketTimeout`/SSL variant — bounds reads/writes **per connection**. 0 = no timeout. | Too low → slow clients/requests killed mid-call. |
| `protocol` | string | binary | Enum: `binary` / `compact` / `json` / `header` (empty string = binary). Mapped in `protocolFactory()`. `header` (THeaderProtocol) self-frames, carries per-message headers — the only mode with W3C trace propagation (§4.2); pair it with `transport=none`. ⚠ the client protocol factory **must** match. | Unknown value → boot fails (`unknown thrift protocol %q`). Mismatched client → corrupted reads / deadlock, not a clean error. |
| `transport` | string | none | Enum: `none` (raw socket, identity factory — historical default) / `buffered` / `framed` (framed sets `MaxFrameSize = bufferSize`). Mapped in `transportFactory()`. ⚠ the client transport must match; cross-language clients commonly require `framed`. | Unknown value → boot fails. Mismatch → hang/corruption. `framed` with too-small `bufferSize` → frames rejected. |
| `bufferSize` | int | 4096 | Only meaningful for `buffered` (buffer size) and `framed` (max frame size). ⚠ dead key when `transport=none`. | Too small vs payload → framed transport errors at runtime. |
| `tls.enabled` | bool | false | Switches `newTransport()` to the SSL server socket. | Plaintext server where TLS expected. |
| `tls.cert-file` / `tls.key-file` | string | — | ⚠ Required together when `tls.enabled=true` (`tlsconf.BuildClient()` errors otherwise → listen fails). | Boot-time listen error "thrift: build TLS". |
| `tls.ca-file` / `tls.server-name` / `tls.insecure-skip-verify` | — | — | **Dead on the server side**: these are client-verification keys in tlsconf; server path calls `Build()` and never verifies client certs (no mTLS). | Expecting mTLS → silently absent. |
| `observer.tracing.enabled` | bool | true | Wraps the processor with the OTel span layer. No-op without starter-otel's providers (silent). ⚠ **not** hot-reloadable — evaluated once in `Run`. | — |
| `observer.metrics.enabled` | bool | true | Wraps the processor with the metrics layer (same caveats). ⚠ example-otel's conf historically used `…server.interceptor.*` keys — those are dead; it only worked because the defaults are `true`. Use `observer.*`. | Wrong prefix (`interceptor.*`) → silently ignored, observer still on. |

---

## 4. Verification & fault drills

### 4.1 Wiring + decorator

```bash
cd example && ./check.sh    # asserts: echo body round-trips twice, decorator fired exactly 2x
```

The example self-SIGTERMs on success, exercising the shutdown path (`Stop`).

### 4.2 Observability reading

With starter-otel + Prometheus exporter (`example-otel/conf/app.properties`):

```bash
cd example-otel && docker compose up -d      # Jaeger (OTLP :4317, UI :16686)
go run .                                     # sends 20 Echo RPCs, verifies Jaeger API, exits 0
curl -s :9090/metrics | grep -E 'rpc_server_request_(count|duration)|rpc_server_active_requests'
```

- **Spans**: name `thrift.process`, kind server, attribute `rpc.system=thrift`; on error also
  `thrift.error_code` + Error status. Trace propagation depends on the protocol:
  - `protocol=header` (THeaderProtocol): W3C trace-context propagation **works** — the client
    injects `traceparent` into the message headers (e.g. Go client: `THeaderProtocol.SetWriteHeader`),
    and the server extracts it via the global OTel propagator and parents `thrift.process` to the
    remote span (implemented in `middleware.go` `observedProcessor.Process`; no-op without
    starter-otel, same global-first pattern as starter-kitex).
  - `binary` / `compact` / `json`: no header channel exists on the wire — every span is a **new
    root**, and cross-service traces show up disconnected. Adding one would require a custom
    protocol envelope, i.e. a breaking wire change — ruled out (see §6).
- **Metrics**: `rpc.server.request_count` (counter), `rpc.server.request.duration`
  (seconds histogram, explicit buckets 0.005…10), `rpc.server.active_requests` (up-down
  gauge), all with `rpc.system=thrift`; count/duration add `rpc.thrift.status_code=ok|error`.
- **Logs**: only the app-def lines the starter emits (`thrift server created/starting/
  shutting down`, tag `app-def`) plus whatever your decorator logs — there is no access log.

### 4.3 Fault drill

**None available — and intentional.** This starter has no fault injection, no resilience
admission, no loadtest identification and no governance hook, and none will be added: the
thrift starter is deliberately the thinnest protocol family in the repo. For production-grade
thrift with governance, use mature frameworks (e.g. kitex via contrib/kitex). The only failure
drills you can run are protocol/transport mismatches (§5) and kill -9.

### 4.4 Shutdown

`SIGTERM` → `Stop` logs and calls `thrift.TSimpleServer.Stop()`, which closes the
server transport and interrupts the accept loop. thrift's `TSimpleServer.Stop` does not wait
for per-connection goroutines — there is **no graceful drain**; treat shutdown as "stop
accepting, drop stragglers". This is a declared boundary, not a bug to fix here: for
connection-draining thrift servers use mature frameworks.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Starter completely inactive, no thrift logs | `spring.thrift.server.addr` missing | Set it — it is both the port and the activation switch (no default). |
| Container fails at wiring | zero or multiple `thrift.TProcessor` beans provided | Provide exactly one (the non-nullable ctor arg in `NewSimpleThriftServer`). |
| Boot fails: `unknown thrift protocol/transport %q` | typo in `protocol`/`transport` | Use binary/compact/json/header and none/buffered/framed. |
| Client connects then hangs / garbage errors | protocol or transport mismatch client↔server | Align factories: framed↔TFramedTransport, compact↔TCompactProtocol (example pins both sides). |
| Framed transport frame errors at runtime | `bufferSize` < actual frame size | Raise `spring.thrift.server.bufferSize`. |
| Boot fails: `thrift: build TLS` | `tls.enabled=true` without cert/key pair (or bad files) | Provide `tls.cert-file` + `tls.key-file`. |
| Expected mTLS, clients not verified | server path uses `tlsconf.BuildClient()`; `ca-file` etc. are dead keys | Not supported; raise as design issue if needed. |
| No traces/metrics but observer "on" | starter-otel not imported, or wrong key prefix (`interceptor.*`) | Import starter-otel; use `observer.tracing.enabled`/`observer.metrics.enabled`. |
| Traces appear as disconnected roots | using binary/compact/json protocol, which has no propagation carrier | Expected; switch both peers to `protocol=header` for W3C propagation, or correlate by timing/service name. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 starter-side (+3 dead tls client keys) |
| Required | 1 (`addr`; `tls.cert-file`/`key-file` conditionally) |
| Quickstart external deps | 0 (Jaeger only for the observability example) |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger):

1. example/conf comment claims a ":9292 default" — there is none (still present; conf file,
   not Go source).
2. ~~Trace propagation absent~~ **Resolved**: with `protocol=header` the server now extracts
   W3C trace context from THeaderProtocol message headers and parents `thrift.process` to the
   remote span (middleware.go). For binary/compact/json this stays impossible without a
   custom protocol envelope (breaking wire change) — declared boundary, won't fix.
3. ~~No graceful drain~~ **Declared boundary**: `TSimpleServer.Stop` only closes the
   transport; no drain will be built here — use mature frameworks for production thrift.
4. No fault injection / resilience admission / health service — asymmetric with starter-grpc
   and gin, now settled as intentional ("thinnest protocol family"; rely on mature frameworks
   for production thrift).
5. example-otel conf uses dead `spring.thrift.server.interceptor.*` keys — only works because
   observer defaults are true.
6. tlsconf client-side keys (`ca-file`/`server-name`/`insecure-skip-verify`) bind but are
   dead on the server; no mTLS posture.
