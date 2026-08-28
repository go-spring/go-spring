# starter-dubbo Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`config.go`, `client.go`, `server.go`, `dync.go`, `fault.go`, `loadtest.go`,
`internal/logger`, `internal/mapconfig`) and the runnable [example/](example/). **Dubbo/tribble
semantics (protocols, registry behavior, cluster/loadbalance strategies, serialization, filter
semantics) are [dubbo-go's documentation](https://dubbo-go.github.io/)** — everything below is
go-spring's increment: binding surface, bean wiring, lifecycle, hot reload, governance and
observability integration.

**Activation**: everything is gated on the literal presence of the `spring.dubbo.registries`
property (config.go:52). A typo there silently disables the whole starter — no error, no bean.
On top of that: the server bean additionally requires `spring.dubbo.provider.enabled` to be
unset-or-`true` (server.go:33-34) **and** at least one `ServiceRegister` bean; the client bean
follows the `Instance` (client.go:29).

---

## 1. Complete worked project

A provider + consumer in one process (the shape example/ uses), with metrics, tracing and runtime
fault injection. External prerequisite: a registry — the example uses etcd on `127.0.0.1:2379`
(nacos / zookeeper / polaris also work).

```
demo/
├── go.mod
├── main.go
├── provider.go
├── consumer.go
├── idl/greet.proto              # source of the generated stubs
├── idl/proto/greet.pb.go        # protoc output
├── idl/proto/greet.triple.go    # protoc-gen-go-triple output
└── conf/app.properties
```

**idl/greet.proto**:

```protobuf
syntax = "proto3";
package greet;
option go_package = "demo/idl/proto;greet";

message GreetRequest  { string name = 1; }
message GreetResponse { string greeting = 1; }

service GreetService {
  rpc Greet(GreetRequest) returns (GreetResponse);
}
```

Generate with `protoc --go_out=. --go-triple_out=. idl/greet.proto` (dubbo-go's Triple plugin —
see the [dubbo-go protobuf guide](https://dubbo-go.github.io/)). Commit the generated files.

**main.go**:

```go
package main

import (
    _ "demo/consumer"
    _ "demo/provider"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-dubbo" // REQUIRED: blank import activates the starter
)

func main() { gs.Run() }
```

**provider.go** — note both registration rules: called directly at the top level of `init()`,
and the explicit interface conversion (Go will not infer a type parameter as an interface):

```go
package provider

import (
    "context"

    _ "dubbo.apache.org/dubbo-go/v3/imports" // REQUIRED side-effect import, in USER code
    greet "demo/idl/proto"
    StarterDubbo "go-spring.org/starter-dubbo"
)

type GreetProvider struct{}

func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
    return &greet.GreetResponse{Greeting: req.Name}, nil
}

func init() {
    // name "greet" is the key under ${spring.dubbo.provider.services.greet}.
    // Call DIRECTLY from init(): the bean's file:line debug info is captured
    // from the caller frame (see §2.2) — do not wrap this in a helper.
    StarterDubbo.RegisterService("greet", greet.RegisterGreetServiceHandler,
        greet.GreetServiceHandler(&GreetProvider{})) // explicit conversion, not inferred
}
```

**consumer.go** — inject the typed stub anywhere:

```go
package consumer

import (
    "context"

    _ "dubbo.apache.org/dubbo-go/v3/imports"
    greet "demo/idl/proto"
    StarterDubbo "go-spring.org/starter-dubbo"
    "go-spring.org/spring/gs"
)

func init() {
    // Binds ${spring.dubbo.consumer.references.greet} and provides a *greet.GreetService bean.
    StarterDubbo.RegisterReference("greet", greet.NewGreetService)
}

type Caller struct {
    Greet *greet.GreetService `autowire:"?"` // named bean; omit "?" to make it required
}
```

**conf/app.properties** — the complete surface used above:

```properties
# gs's built-in HTTP server off; dubbo owns the ports.
spring.http.server.enabled=false

# --- application (required) ---------------------------------------------------
spring.dubbo.application.name=demo-app

# --- registries (required; the module's activation key) -----------------------
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379

# --- protocols (global; omit all to fall back to Triple on :20000) -----------
spring.dubbo.protocols.tri.name=tri
spring.dubbo.protocols.tri.port=20000

# --- provider side ------------------------------------------------------------
spring.dubbo.provider.services.greet.interface=greet.GreetService
spring.dubbo.provider.services.greet.version=1.0.0
spring.dubbo.provider.services.greet.filter=loadtest,fault   # starter filters (§2.4)

# --- consumer side ------------------------------------------------------------
spring.dubbo.consumer.references.greet.interface=greet.GreetService
spring.dubbo.consumer.references.greet.version=1.0.0
spring.dubbo.consumer.references.greet.timeout=3s

# --- observability (defaults shown; see §3 for gotchas) -----------------------
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.tracing.enable=false        # default is true with exporter=stdout — see §5
```

**Verify** (in-process, no registry needed for a pure-URL smoke — this is what example/ does):

```bash
cd example && ./check.sh           # 40s watchdog; server self-tests and SIGTERMs itself
# or manually:
go run . -manual                   # terminal 1
go run check_client.go             # terminal 2 → "OK: Dubbo RPC verified"
curl -s 127.0.0.1:9090/metrics | grep dubbo   # dubbo-go Prometheus metrics
```

Note: example/conf/app.properties points at etcd, so even the "in-process" smoke needs a live
registry (Design Health suspect 10).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-dubbo
  ├─ init(): dubbo-go log bridge installed (internal/logger, both facades)
  ├─ init(): mapconfig installed as dubbo-go DynamicConfiguration (config.go:47)
  ├─ gs.Provide(NewInstance)  — condition: OnProperty("spring.dubbo.registries")   [config.go:49-52]
  ├─ gs.Provide(NewClient)    — condition: OnBean[*Instance]                       [client.go:27-30]
  ├─ gs.Module(provider.enabled=true-by-default) → gs.Provide(NewSimpleDubboServer)
  │     exported as gs.Server — conditions: OnBean[ServiceRegister] + OnBean[*Instance]  [server.go:32-44]
  ├─ gs.Provide(newDyncPoller) — unconditional; bound to ${spring.dubbo.application}  [dync.go:40-42]
  └─ init(): "fault" and "loadtest" filters registered in dubbo-go's filter registry

gs.Run()
  ├─ bind ${spring.dubbo} once, statically → DubboConfig → NewInstance
  │     (mounts protocols, metrics, tracing, shutdown on one *dubbo.Instance;
  │      config.go:391-513; application.name blank or 0 registries → fail fast)
  ├─ RegisterReference beans bind ${spring.dubbo.consumer.references.<n>} → typed stubs via NewClient
  ├─ wiring order note: Rooters (dyncPoller) are wired BEFORE Runners (governance engine)
  │     — see dync.go:90-94 comment; governance.OnReady re-polls to close the gap
  ├─ SimpleDubboServer.Run: buildOptions(provider, protocols, registries) → d.NewServer()
  │     → regAll(): every ServiceRegister bean invoked → <-sig.TriggerAndWait() → svr.Serve()  [server.go:297-327]
  ├─ readiness: ready signal fires after Run's trigger; in-flight RPCs drain on Stop
  └─ SIGTERM: StopContext closes the done channel → Run returns → gs completes shutdown
        (dubbo-go's own graceful-shutdown timings come from ${spring.dubbo.shutdown.*})
```

### 2.2 The caller-frame rule (RegisterService / RegisterReference)

`gs.Provide` captures file:line debug info via `runtime.Caller(skip)` → `SetFileLine`
(spring/gs/internal/gs_bean/bean.go:372-376). Both helpers pin that capture with `.Caller(2)`
(server.go:360, client.go:175): frame 0 = `Caller`, frame 1 = the helper itself, frame 2 = **you**.
Call them directly at the top level of your own `init()`/package code; wrapping them in a helper
function or calling from deep inside a closure shifts the stack and the bean's debug info points
at the wrong place. This affects only diagnostics (bean descriptions, error attribution) — not
binding — but wrong frames make container dumps useless.

### 2.3 The Instance model (2026-07-24 refactor)

There is exactly **one** static bind of `${spring.dubbo}` into a `DubboConfig`, held by the
`Instance` facade (config.go:51, 351-354). `NewClient` (client.go:34) and `NewSimpleDubboServer`
(server.go:190) both consume from `Instance` — `Registries()`, `Protocols()`, `Consumer()`,
`Provider()`, `NewServer()`, `NewClient()` (config.go:357-386). Registries and protocols are
therefore process-global: define them once at the top level, select per role by
`registry-ids` / `protocol-ids` (empty = all; unknown ID fails fast — config.go:592-605).

### 2.4 Filter chain — what the starter registers, and where

Two dubbo-go filters are registered at init time; both are **opt-in per service** by adding the
name to the provider's `filter` key (comma-separated, dubbo-go semantics for the rest):

- **`loadtest`** (loadtest.go:38): reads the load-test marker from the inbound dubbo attachment
  (string or []byte, both handled) and tags the context so `traffic.IsLoadTest(ctx)` works in
  later filters and in your service impl. Put it **first** in the chain (source comment,
  loadtest.go:31-34) so the marker lands before anything else branches on it.
- **`fault`** (fault.go:40): resolves `fault.InjectorFor()` from the governance seam **on every
  call** — hot-toggleable, transparent pass-through when governance is absent (fault.go:50-65).
  An injected failure surfaces as `result.RPCResult{Err: fault.ErrInjected}`.

Filters are **frozen at Refer/export time** (dync.go:60) — changing a `filter` key needs a
restart; timeout/retries do not (§4.2).

### 2.5 One call, layer by layer (consumer → provider)

1. Your code calls the typed stub (`*greet.GreetService`), which was built by
   `RegisterReference` from the shared `*client.Client` + the reference's `ReferenceOption`s
   (client.go:172-176).
2. dubbo-go's consumer filter chain (whatever `consumer.filter` /
   `consumer.references.<n>.filter` installed) runs; outbound load-test marking is done by
   cloud/governance/traffic's carrier injection — the starter's `loadtest` filter is the
   inbound companion (loadtest.go:44-47 comment).
3. Cluster strategy / loadbalance pick an instance (dubbo-go semantics; URL params — this is
   exactly the set the hot-reload path can change at runtime, §4.2).
4. Triple (or dubbo/jsonrpc) transport delivers the invocation to the provider port.
5. Server filter chain: `loadtest` tags ctx, `fault` may inject, then the handler registered by
   your `RegisterService` call runs; the response unwinds through the same filters.

---

## 3. Per-key behavior reference

169 `value` tags / 103 unique key spellings (~159 leaves) under `spring.dubbo.*`. What each key
**means for dubbo-go** is one line at most — follow the
[dubbo-go config docs](https://dubbo-go.github.io/). Below is our binding surface: prefix,
defaults, validation, couplings. Map ids (`registries.<id>`, `protocols.<id>`,
`provider.services.<n>`, `consumer.references.<n>`, `...methods.<m>`) are free-form, validated
`^[_a-zA-Z][a-zA-Z\d_-]*$`. All durations are **strings** (`"3s"`, `"10m"`); an unparseable or
non-positive duration is **silently dropped**, never an error (e.g. config.go:565-569).

### 3.1 application (8 keys) — required node for `name`

| Key | Default | Binding behavior |
|-----|---------|------------------|
| `name` | `dubbo.io` | The only hard-required key: blank → `NewInstance` fails fast (config.go:393-395). Also the app-level governance label and the dyncPoller's override key (dync.go:77-83). |
| `metadata-type` | `local` | `remote` flips `dubbo.WithRemoteMetadata` (config.go:419-421). |
| `organization`/`module`/`owner` | `dubbo-go`/`sample`/`dubbo-go` | Only forwarded when non-empty. |
| `group`/`version`/`environment` | — | Forwarded when non-empty. |

### 3.2 registries.<id> (13 keys) — **module activation node**

| Key | Default | Binding behavior |
|-----|---------|------------------|
| *(node presence)* | — | Literal `spring.dubbo.registries` property gates the whole starter (config.go:52). ≥1 entry or `NewInstance` fails (config.go:396-398). |
| `protocol` | — | nacos/etcdv3/polaris/xds/zookeeper/service-discovery-registry; empty falls back to the map-key id (config.go:544-547). |
| `address` | — | Required in practice (registry needs one). |
| `timeout` / `ttl` | 5s / 10s | Duration strings; invalid → dropped silently. |
| `weight` | 100 | Always passed (`>=0`, config.go:571-573). |
| `simplified` / `preferred` / `zone` | false / false / — | Registry-side knobs. |
| `group` / `namespace` / `username` / `password` / `params` | — | Forwarded when set. |

### 3.3 protocols.<id> (4 keys)

| Key | Default | Binding behavior |
|-----|---------|------------------|
| `name` | `dubbo` | dubbo/rest/grpc/filter/jsonrpc/tri/registry; empty falls back to the id (config.go:517-520). |
| `port` | 0 | 0 lets dubbo-go pick. |
| `ip` / `params` | — | `params` is `map[string]string` — a value-typed map, not `map[string]any` (binder limitation, config.go:123-129 comment). |

⚠ No protocols at all → server falls back to a single `tri` listener on `:20000` (server.go:276-282).

### 3.4 metadata-report — removed

Formerly bound (protocol/address/username/password/group/namespace/timeout) but never translated
into any dubbo option — dead config, deleted (2026-08). Setting `spring.dubbo.metadata-report.*`
now simply has no binding target; use `application.metadata-type=remote` if you need remote
metadata (that routes through the registries block).

### 3.5 provider (27 keys) + provider.services.<n> (26) + methods.<m> (12)

Provider-wide defaults every exported service inherits; per-service fields override them; both
translate to `server.ServerOption` / `ServiceOption` in server.go:50-132 and 196-294. Only
non-zero/non-empty values are forwarded (dubbo-go defaults fill the rest).

Notables: `registry-ids`/`protocol-ids` must reference existing map keys (fails fast);
`retries` default is `-1` = unset (keep dubbo-go's own default), `0` = no retry attempts,
`>0` = that many retries — the SAME semantics at every level (provider/service/consumer/
reference/method); values below `-1` fail startup with a validation error;
`warmup` is a duration string; `not-register=true` exports without publishing;
`adaptive-service(-verbose)` map to their ServerOptions.

### 3.6 consumer (19 keys) + consumer.references.<n> (19) + methods.<m> (12)

Same structure on the client side (client.go:34-98 for consumer level, 101-168 for references).
This tree is bound **twice** — statically for the client bean and as `gs.Dync` for hot reload
(§4.2).

| Gotcha | Detail |
|--------|--------|
| `check` | Both consumer and reference levels default **true** (fail fast on missing providers). Migration: references that relied on the old reference-level default **false** must set `spring.dubbo.consumer.check=false` — dubbo-go v3 has no per-reference "no check" option, so `references.<n>.check=false` combined with consumer check=true only WARNs at startup (it cannot be honored). |
| `protocol` | consumer accepts tri/triple/jsonrpc/dubbo (client.go:39-46); reference accepts whatever dubbo-go takes. |
| `url` | Direct-connection mode — bypasses the registry for that reference. |
| Separator mix | provider/consumer level uses dashes (`tps-limit-rate`), service/reference/method level uses dots (`tps.limit.rate`, `force.tag`) — same concept, two spellings. **Canonical by design** (kept for compatibility): the dotted spellings at service/reference/method level intentionally mirror dubbo's own URL-param names that the hot-reload path publishes (dync.go pushes `tps.limit.rate` etc. as URL params); the dashed spellings at provider/consumer level follow the starter's own key style. Exact-match binding only (no relaxed binding), so both forms must stay as documented. |

### 3.7 metrics (6 keys) / tracing (12 keys) — ⚠ on by default

| Key | Default | Binding behavior |
|-----|---------|------------------|
| `metrics.enable` | **true** | `true` mounts dubbo-go's Prometheus exporter. |
| `metrics.port` / `metrics.path` | **9090** / `/metrics` | A second metrics endpoint alongside actuator — plan ports accordingly (memory: server ports must be explicit). |
| `metrics.push-gateway-address` | — | Enables the pushgateway path when set. |
| `metrics.mode` / `metrics.namespace` | — | Removed (2026-08): bound but dead, no v3 metrics.Option existed for them. |
| `tracing.enable` | **true** | Mounts dubbo-go's OTel tracing with exporter default **stdout** — see §5 before relying on it. |
| `tracing.exporter` / `endpoint` / `propagator` / `mode` / `ratio` / `insecure` | stdout / — / w3c / — / 1.0 / false | Forwarded to dubbo-go `trace.Option`s (config.go:453-472). |
| `tracing.name`/`serviceName`/`address`/`use-agent` | — | Removed (2026-08): legacy jaeger fields, never translated, no v3 Option. |

### 3.8 shutdown (6 keys)

Duration strings translated to `graceful_shutdown.Option`s **only when at least one field is
set** (config.go:475-506, `anySet` at 536-540). `reject-handler`: any non-empty value merely
turns rejection on — the value itself is ignored (config.go:497-499). `internal-signal=true`
(default) lets dubbo-go react to signals itself.

### 3.9 Wrapper-field note (absolute vs prefixed keys)

Some starters expose `value:"${observability:=}"`-style wrapper fields that resolve as
**top-level absolute** keys regardless of the instance prefix. starter-dubbo has **no such
wrapper**: every key above is prefixed under `spring.dubbo.*` and bound relative to it (the only
top-level refs are the literal tag expressions `spring.dubbo`, `spring.dubbo.consumer`,
`spring.dubbo.application` in config.go:51, dync.go:41, dync.go:67).

---

## 4. Verification & fault drills

### 4.1 Verify the assembly

```bash
grep -ri "dubbo server starting" logs/    # SimpleDubboServer.Run reached (server.go:314)
curl -s 127.0.0.1:9090/metrics | head     # dubbo-go Prometheus metrics (if enabled)
grep -ri "_rpc_dubbo" logs/ | head        # dubbo-go framework logs via the bridge (§4.4)
```

### 4.2 Hot reload drill (timeout/retries, no restart)

`${spring.dubbo}` is bound once statically for `Instance` — fields consumed at build/refer time
are **frozen**: protocols, registries, filters (dync.go:60), serialization, interface/group
routing (they define the override key). `${spring.dubbo.consumer}` is bound **again** as
`gs.Dync[DubboConsumer]` (dync.go:67) and the `dyncPoller` pushes the dynamically-applicable
subset into an in-memory config center (mapconfig) as flat dubbo URL params: `timeout`,
`retries`, `loadbalance`, `cluster`, `group`, `version`, `serialization`, `sticky`,
`force.tag`, `weight`, and per-method `methods.<m>.{timeout,retries,loadbalance,weight,sticky,
tps.limit.*,execute.limit*}` (dync.go:57-59, 154-228).

Chain (DESIGN.md, confirmed in code):

```
property change (file/nacos/env) → gs RefreshProperties → gs.Dync swap
  → dyncPoller.OnChanged → poll() → diff vs last snapshot (no-op refreshes skipped)
  → mapconfig.RefreshOverrideRules → dubbo-go consumerConfigurationListener /
    referenceConfigurationListener → invoker URL updated → next call uses new params
```

Drill:

1. Start with `spring.dubbo.consumer.references.greet.timeout=3s`.
2. Change it to `100ms` and trigger a refresh (e.g. `curl -X POST :9370/actuator/refresh` if
   wired, or your source's refresh path).
3. Observe slow calls now fail fast; the override landed under key
   `greet.GreetService:1.0.0:.configurators` — see §4.3 for the key format.

### 4.3 Override key format (verify before drilling)

Consumer-level defaults publish as `<application.name>.configurators` (app-level listener);
each reference with a non-empty `interface` publishes as
`<interface>:<version>:<group>.configurators` — built by `colonSeparatedKey`
(dync.go:245-257): version is omitted when empty or the `0.0.0` sentinel **but its `:` is always
written**; likewise group. A bare interface name never matches. Consequence: for the override to
land, the reference's `version`/`group` must match what the provider exported.

### 4.4 Governance merge path (dynamic timeout from the center)

Optional; active only when starter-govern is imported and `govern.enabled=true`. The poller
subscribes to governance policies under two resource labels (dync.go:263-264):

- `dubbo:<application.name>` — consumer-level defaults
- `dubbo:<interface>:<version>:<group>` — per reference (same colon-separated key as §4.3)

`Policy.Timeout` (ms) and `Policy.MaxRetries` override the `timeout`/`retries` params when > 0
(dync.go:288-295); note MaxRetries maps to dubbo's **cluster** retries, not resilience-layer
retry. Ordering is handled: Rooters wire before Runners, so `governance.OnReady` re-polls once
the engine is live (dync.go:90-99). Drill: flip `govern.*` timeout in the center's source, watch
the reference override re-push without restart.

### 4.5 Fault drill (provider side, no restart)

1. Add `fault` to the service's filter chain: `...services.greet.filter=loadtest,fault`.
2. Import starter-governance; configure `govern.fault.*` (rate/error/scope) via a hot source.
3. With `scope: loadtest`, only invocations carrying the load-test marker burn — mark them by
   injecting the outbound carrier (cloud/governance/traffic) from a marked upstream, or use
   `scope: real` in a dedicated environment.
4. Watch: consumer receives the injected error; `traffic.IsLoadTest(ctx)` returns true inside
   the impl for marked calls (loadtest filter ordered first).

### 4.6 Observing the log bridge

Importing the starter installs one adapter under **both** dubbo-go logger facades (including
gost/getty) — internal/logger/logger.go init(). Every dubbo-go framework line is re-emitted
through go-spring's log with tag `_rpc_dubbo` (registered via `log.RegisterRPCTag("dubbo", "")`,
internal/logger/logger.go:41). Configure it like any logger tag under `${logging.logger}`.
Known trade-off: no ctx on this path, so no trace-id enrichment, and caller file:line points
into the bridge.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Nothing happens; no dubbo beans at all | `spring.dubbo.registries` misspelled/absent — module gate (config.go:52) | Set it; this failure mode is silent by design (covered in wiring_test.go). |
| Boot fails: `${spring.dubbo.application.name} is required` | blank name | Set `spring.dubbo.application.name` (config.go:393-395). |
| Boot fails: registry id "x" is not defined | `registry-ids`/`protocol-ids` references an unknown map key | Align ids under `registries`/`protocols` (config.go:592-605). |
| Server never starts, client works | no `ServiceRegister` bean, or `provider.enabled=false` | Call `RegisterService` (at top level) or re-enable. |
| Startup panic mentioning OTel/stdout exporter | `tracing.enable=true` (default) with exporter `stdout` and no provider wired | Set `spring.dubbo.tracing.enable=false` or wire starter-otel / a real exporter. |
| Port 9090 conflict | dubbo-go metrics defaults on (`:9090/metrics`) | `spring.dubbo.metrics.enable=false` or set an explicit port. |
| Hot reload of timeout doesn't land | version/group mismatch → wrong colon-separated key; or `filter`-type field (frozen) | Make the reference's `version`/`group` match the provider's; only URL-param fields hot-reload (§4.2). |
| Duration key silently ignored | unparseable or non-positive value | Use Go duration strings (`3s`, `10m`) — config drops bad values without error. |
| Bean debug info points at a helper file | `RegisterService`/`RegisterReference` wrapped in a function | Call them directly from your frame (§2.2). |
| Dubbo logs don't reach the app sink | no go-spring logger configured | Bridge only re-routes; configure `${logging.logger}`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (value tags) | 169 tags / 103 unique spellings (~159 leaves) |
| Required | 1 hard (`application.name`) + `registries` ≥1 entry as the module gate |
| Quickstart external deps | 1 (a registry) |
| "Watch out" entries | 10 |

Design suspects (audit ledger):

1. Config surface too large to document exhaustively — itself a signal.
2. Duplicate binding of the same tree (static `Instance` + `Dync` consumer) with a frozen-field
   list only visible in code (dync.go:57-60).
3. ~~Dash-vs-dot key separators differ by nesting level~~ — resolved as documented-canonical
   (see §3.6 "Separator mix"): dotted keys mirror dubbo URL params on dynamic levels, dashed
   keys are the starter's own style on static levels; kept for compatibility, exact-match only.
4. ~~`check` default differs consumer (true) vs reference (false); `retries=0` semantics differ
   by level~~ — both fixed (2026-08): `check` now defaults true at both levels (unhonorable
   per-reference opt-outs WARN), and `retries` is unified as -1=unset / 0=no retries / >0=count
   at every level, with startup validation rejecting values below -1.
5. ~~Dead config: `metadata-report.*`, `provider.proxy`, `consumer.proxy`, `max_message_size`,
   `metrics.mode/namespace`, legacy jaeger tracing fields~~ — all deleted (2026-08); binding
   and docs removed. `shutdown.reject-handler` remains: any non-empty value enables rejection
   (the value itself is ignored — that IS the documented behavior).
6. Tracing/metrics enabled by default with surprising side effects (stdout exporter, port 9090).
7. Module activation gated on the literal presence of `spring.dubbo.registries` — typo = silent
   no-op.
8. Caller-frame sensitivity of `RegisterService`/`RegisterReference` — no wrapping allowed.
9. Reference path has no end-to-end example coverage (example dials direct URL, no registry
   discovery).
10. `check.sh` claims "no external service" but example/conf/app.properties requires etcd.
11. ~~wiring_test.go "KNOWN BUG" comment is stale (fixed `map[string]string`), protocols block
    still untested.~~ — the `map[string]any` binder failure is fixed (config.go:123-129
    documents the constraint); protocols block remains untested in wiring_test.go.
12. ~~DESIGN.md is stale relative to the code~~ — DESIGN.md refreshed (2026-08) to match
    config.go/dync.go; this USAGE and the source remain authoritative for details.
