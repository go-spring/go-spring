# starter-elasticsearch Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health/health.go`, `resilience_test.go`) and the runnable examples
([example/](example/) — self-asserting smoke; [example-otel/](example-otel/),
[example-cloudnative/](example-cloudnative/), [example-load/](example-load/)). Elasticsearch
semantics and the go-elasticsearch v8 API are
[the client's own documentation](https://www.elastic.co/guide/en/elasticsearch/client/go-api/current/index.html)
— everything below is go-spring's increment.

**Activation**: any `spring.elasticsearch.instances.*` key (the module is `gs.Module(gs.OnProperty("spring.elasticsearch"))`,
a prefix check). Each `spring.elasticsearch.instances.<name>` entry creates one `*StarterElasticsearch.Client`
bean named `<name>`, plus a health indicator named `elasticsearch:<name>`.

---

## 1. Complete worked project

One service with a direct instance, a discovery-backed instance, and actuator + otel wired in.
File tree (mirrors example/ + example-otel/): `go.mod`, `main.go`, `discovery.go`,
`service.go`, `conf/app.properties`.


**go.mod** (deps that matter — versions as in example/go.mod):

```
require (
    github.com/elastic/go-elasticsearch/v8 v8.19.6
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-elasticsearch latest
    go-spring.org/starter-actuator     latest   // optional: readiness on :9370
    go-spring.org/starter-otel         latest   // optional: real trace/metric export
    go-spring.org/starter-governance-file   latest   // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    StarterElasticsearch "go-spring.org/starter-elasticsearch"
    _ "demo/service"
)

func main() { gs.Run() }
```

**discovery.go** — registers the backend a `service-name` instance resolves through (a real
deployment points Consul/Nacos/k8s here; see example/discovery.go):

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(    discovery.Endpoint{Addr: "127.0.0.1:9200", Healthy: true}), nil
    }).Name("default")
}
```

**service.go** — inject the wrapper, run real index/get/search operations:

```go
package service

import (
    "context"
    "strings"

    "go-spring.org/spring/gs"
    StarterElasticsearch "go-spring.org/starter-elasticsearch"
)

const indexName = "demo-docs"

type Service struct {
    // Always the wrapper type *StarterElasticsearch.Client. It embeds the raw
    // *elasticsearch.Client, so the API tree (Index/Get/Search/Info, ...) and
    // the lifecycle methods are all promoted.
    Main *StarterElasticsearch.Client `autowire:"main"`
    Disc *StarterElasticsearch.Client `autowire:"disc"` // discovery-resolved nodes
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // Readiness probe: one Info request against the cluster.
            if err := StarterElasticsearch.HealthCheck(ctx, s.Main); err != nil {
                panic(err) // unreachable cluster — fail loudly
            }
            body := `{"title":"hello","views":1}`
            _, _ = s.Main.Index(indexName, strings.NewReader(body),
                s.Main.Index.WithDocumentID("1"), s.Main.Index.WithRefresh("true"))
        }
    }).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** — the complete surface actually used above (style copied from
example/conf/app.properties + example-otel/conf/app.properties):

```properties
# --- direct instance --------------------------------------------------------
spring.elasticsearch.instances.main.addresses=http://127.0.0.1:9200

# --- discovery-backed instance ---------------------------------------------
# `addresses` is required by validation even when service-name overrides it —
# the dummy below is deliberately non-resolvable to prove discovery wins.
spring.elasticsearch.instances.disc.addresses=http://nonexistent.invalid:9200
spring.elasticsearch.instances.disc.service-name=es-cluster
spring.elasticsearch.instances.disc.discovery-scheme=http

# --- actuator + otel (example-otel style) ----------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**Verify** (start ES first — see example/docker-compose.yml; ES 8.13 single-node, security off,
takes up to 120 s on first boot, and Jaeger via example-otel/docker-compose.yml if tracing):

```bash
go run .                          # boot fails fast if the cluster is unreachable
curl -s :9370/readyz | jq .       # components include elasticsearch:main and elasticsearch:disc
curl -s :9090/metrics | grep -E 'db.client.*elasticsearch'   # duration + in-flight gauges
curl "http://127.0.0.1:16686/api/traces?service=demo&limit=1" # spans in Jaeger (example-otel check)
grep _app_elasticsearch_access app.log | tail -3              # one record per request
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-elasticsearch
  └─ gs.Module(OnProperty("spring.elasticsearch")) fires when any spring.elasticsearch.instances.* key exists
        └─ conf.BindEach("${spring.elasticsearch}") → one Config per <name> entry
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver)).Name(<name>)
              │      .Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator named "elasticsearch:<name>"
                     .Export(gs.As[health.Indicator]())

gs.Run()
  ├─ ctor newClient:
  │    ├─ service-name set && !mesh.Enabled() → resolveAddresses(c, backend):
  │    │      read snapshot → "scheme://host:port" overrides c.Addresses
  │    │      (fails fast: no backend bean cited, or no endpoints for the service)
  │    ├─ optional Driver bean (none → bundled DefaultDriver; several coexist →
  │    │      the entry selects one by name: spring.elasticsearch.instances.<name>.driver =
  │    │      <bean-name>, empty = the family-wide `spring.elasticsearch.default.driver`, then the single Driver bean by type, naming a
  │    │      missing bean fails startup) → driver.CreateClient(ctx, c, backend, params):
  │    │      DefaultDriver installs the dynamicTransport indirection and returns
  │    │      the *Client wrapper; NewClient fixes identity, derives the service
  │    │      label and sets exec = params.ExecutorFor("elasticsearch", service)
  │    │      (fault(governed executor), or observe-only Unmanaged for the zero params),
  │    │      then installs the transport: declaration outermost, resilience inside
  │    └─ fail-fast probe: HealthCheck(ctx, client) — one Info straight to the raw client;
  │        failure releases the client (Destroy) and aborts boot
  ├─ readiness: indicator flips UP (runs the raw client.Info via HealthCheck)
  └─ SIGTERM → Destroy: exec.Close → client.Close
```

A dead cluster, or a service-name with no endpoints fails the boot — the process never reaches
"serving" with a broken ES connection.

### 2.2 The transport chain — exact order and why

Every request passes through the layers below. Order as built:

```
elasticsearch API (Index/Get/...)
  → elastictransport retry loop (MaxRetries / DisableRetry)
  → dynamicTransport (RWMutex indirection; http.DefaultTransport until the wrapper swaps)
  → declareTransport: puts the operation (db.system/db.operation + URL path) on the request
      context — OUTSIDE the executor, so the resilience layer reads it at Execute entry
  → resilience roundTripper: executor = fault(emit(limiter/breaker/bulkhead/retry)):
      the injected fault injector outermost, the executor's own emit layer (span + call-level
      and attempt-level duration histograms + outcome counter + access log per Execute)
      inside it, the governed core innermost
  → http.DefaultTransport → network
```

Rationale (source comments, [command.go] and [client.go]):

- **The starter DECLARES, the resilience layer EMITS.** There is exactly one emitter on the
  chain. The declaration transport ([command.go]) turns the method + URL path into an
  `observability.Operation` and puts it on the request context; the executor reads it at
  Execute entry and emits the one span, both duration histograms, the in-flight gauge and the
  access log.
- **The declaration sits OUTSIDE the executor, not as its base.** The executor reads the
  operation *before* the protected call runs, so a declaration made inside it would be read by
  nobody. The stack is therefore declaration-outermost, wrapping the resilience round-tripper,
  which wraps `http.DefaultTransport` — the reverse of the old layering, where the emitting
  transport was the executor's base and ran per attempt.
- **The elastic transport's own OTel instrumentation is NOT enabled.**
  `elasticsearch.Config.Instrumentation` used to carry `NewOtelInstrumentation`, which opened a
  call-level client span per request from the esapi layer — a second span for the same call,
  duplicating the one the resilience layer now emits. It is removed so the resilience layer
  stays the single emitter. (The same call as starter-go-redis, where redisotel's per-command
  span was dropped and only its non-per-call pool metrics kept; here the instrumentation is
  per-call only, so nothing of it remains.)
- **dynamicTransport instead of a fixed transport**: the ES transport is fixed at construction
  and cannot be swapped on the client afterwards; the indirection keeps the resilience policy
  hot-reloadable (Dync) even though the transport instance is not. The slot
  is guarded by RWMutex, not atomic.Value, because the active round-tripper is one of several
  distinct concrete types — the atomic.Value version panicked on the second Swap, which is
  exactly what resilience_test.go pins as a regression test.
- **Custom drivers may install none of this**: only clients built by DefaultDriver are handed
  a dynamicTransport; a custom driver's own transport silently bypasses the swap — the
  declaration and resilience layers are then unavailable for that instance.

### 2.3 One request through the chain: `Search` with a match query

1. The generated API builds `POST /demo-docs/_search` and hands it to the transport chain.
2. The retry loop (up to `max-retries`, default 3) hands the request to dynamicTransport.
3. declareTransport (outside the executor) puts the operation on the request context: the span
   name and `db.statement` are `POST /demo-docs/_search` (its URL path), `db.operation` is
   `POST` (the bounded part), `db.system` is `elasticsearch` [command.go].
4. The resilience executor asks for a permit scoped to the service label, e.g.
   `elasticsearch:es-cluster` or `elasticsearch:http://127.0.0.1:9200` (first address;
   derived via `resilience.ServiceLabel` in `NewClient`) — per cluster, not per request.
   With governance off, the executor is the observe-only `resilience.Unmanaged`.
5. On completion the executor emits the one span, the call-level and attempt-level duration
   histograms and the access log. The caller sees exactly what plain go-elasticsearch returns —
   including 4xx `res.IsError()` bodies; only transport-level errors (5xx are mapped to a
   breaker-tripping, retryable error by the resilience round-tripper,
   cloud/resilience/roundtripper.go:99-104).

**Pass a context on every call.** It carries the call's cancellation and deadline, and the
operation span the resilience layer opens inherits it — `HealthCheck` and the health indicator
always pass one explicitly [starter.go, health.go]; user code should use
`es.Search.WithContext(ctx)` etc.

### 2.4 Discovery addressing — seed plus a live node set

When `service-name` is set and mesh mode is off, the endpoints are resolved once in the ctor and
baked into `c.Addresses`; the loader is a pure snapshot function with no resources and no
background watch, so nothing is kept alive and nothing needs stopping on shutdown
[starter.go, driver.go]. That read is the **fail-fast gate and the seed** — the
live feed is a custom `ConnectionPoolFunc` the driver installs over the same resolver
[pool.go]: the transport's node set is re-read from the naming service, so a node joining or
leaving the cluster becomes usable without a restart. The propagation budget is one second
(`refreshInterval`, checked on the request path), and a registry hiccup or an empty snapshot
keeps the last good node set rather than black-holing traffic that a moment ago worked.

Node selection is untouched: the live pool keeps a library-built inner pool with the transport's
own selector and live/dead bookkeeping, and rebuilds it only when the node set actually changes
(address-sorted, so a reordered snapshot does not look like a change). In mesh mode the sidecar
owns discovery+LB and the static Addresses (or CloudID) are used unchanged.

---

## 3. Per-key behavior reference

All keys live under `spring.elasticsearch.instances.<name>.` — per-instance prefix binding via
`conf.BindEach` (the ctor's Config arg). There are no observability keys — observation is
unconditional (see §3.4).

### 3.1 Addressing & discovery

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addresses` | list | — | Node URLs, e.g. `http://127.0.0.1:9200` (comma-separated). Validated non-empty (`len($) > 0`). ⚠ Required even when `service-name` overrides it — the example carries a non-resolvable dummy on purpose. ⚠ Ignored when `cloud-id` is set (client-side precedence). | Empty → BindEach error; unreachable first probe → boot error "failed to reach elasticsearch cluster". |
| `service-name` | string | — | Resolve node addresses via a registered discovery backend, and keep the transport's node set following it (pool.go); overrides `addresses`. Ignored in mesh mode. ⚠ Pairs with `scheme`/`discovery`/`discovery-scheme`. | Service has no endpoints at boot → boot error `discovery %q returned no endpoints`; empty at runtime → last good set kept. |
| `scheme` | string | — | Narrows discovery to endpoints of one transport scheme. Only consulted when `service-name` is set. | — |
| `discovery` | string | — | Which registered discovery backend resolves `service-name`. Falls back to `${spring.elasticsearch.default.discovery}` when unset. | Both unset or an unregistered name while service-name is set → boot error. |
| `discovery-scheme` | string | `http` | URL scheme stamped onto discovered `host:port` endpoints (`http`/`https`). | Wrong scheme → first probe fails at boot. |
| `cloud-id` | string | — | Elastic Cloud deployment ID; when set the client prefers it over `addresses`. | — |

### 3.2 Auth & TLS

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `username` / `password` | string | — | HTTP Basic auth. | Wrong → boot error at the startup Info probe. |
| `api-key` | string | — | Base64 API key; takes precedence over username/password when set. | Combined with basic auth → API key silently wins. |
| `service-token` | string | — | Service-account token auth. | — |
| `certificate-fingerprint` | string | — | SHA256 hex of the CA cert — pins self-signed HTTPS without a CA file. ⚠ Requires `https://` addresses (or CloudID); there is **no `tls.*` block** in this starter. | Mismatched fingerprint → TLS error at boot. |

### 3.3 Transport behavior

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `max-retries` | int | 3 | elastictransport retry count. ⚠ Interacts with the governance executor's retry (`spring.governance.client.default.max-retries`) — two stacked retry loops multiply attempts and latency. | Large + governance retry → multiplied attempts. |
| `disable-retry` | bool | false | Disables the client retry loop entirely. | — |
| `compress-request-body` | bool | false | gzip request bodies. | — |
| `enable-metrics` | bool | true | Client's built-in elastictransport metrics switch (⚠ schema.json wrongly says default false — code is the truth). | — |
| `enable-debug-logger` | bool | false | elastictransport debug logging. | On → very chatty logs incl. request/response bodies. |

### 3.4 Instrumentation

There are **no instrumentation config keys** (no level, no skip list): the starter DECLARES
each request's identity unconditionally ([observe.go]), and the resilience layer EMITS every
signal from that declaration. All ride the OTel globals starter-otel installs
(`spring.observability.*`).

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .           # components include "elasticsearch:main"
docker stop starter-elasticsearch     # indicator runs client.Info → component flips DOWN
curl -s :9370/readyz                  # 503 OUT_OF_SERVICE
docker start starter-elasticsearch
```

### 4.2 What observability actually emits

- **Metrics** (OTel, via starter-otel's globals), emitted by the resilience layer: the
  call-level `db.client.operation.duration` and attempt-level `db.client.attempt.duration`
  histograms plus the in-flight `db.client.active_requests` gauge — attributes
  `db.system=elasticsearch`, `db.operation=<METHOD>` (bounded), `status` = ok/error, none
  of them carrying the URL path. The same layer emits the `resilience.client.calls` counter
  with `status` ∈ {ok, error}, plus `resilience.outcome` ∈ {rate_limited, circuit_open,
  bulkhead_full, retry_budget_exceeded, timeout} when protection refused the call.
- **Spans**: one span per request, opened by the resilience layer (internal kind), named like
  `POST /demo-docs/_search`, with `db.system`/`db.operation` attributes plus `db.statement` =
  the URL path. Verify: `curl "http://127.0.0.1:16686/api/traces?service=demo&limit=1"`
  and grep for `data":[{` (same check as example-otel's self-test).
- **Access log**: one line per request, written by the resilience layer under tag
  `_app_elasticsearch_access` (`log.RegisterAppTag("elasticsearch", "access")`) at native
  levels — error → Warn; success with the captured URL path (truncated to 512 bytes) → Debug;
  plain success → Info.

```bash
curl -s "127.0.0.1:9090/search" >/dev/null  # or drive via the app
curl -s :9090/metrics | grep -E 'db.client.operation.duration|active_requests'
grep _app_elasticsearch_access app.log | tail -1
```

### 4.3 Resilience / fault drill (example-load style)

```properties
# NOTE: governance RULES go in conf/governance.properties, referenced by spring.governance.source.file.path in app.properties (see starter-governance-file USAGE).
spring.governance.enabled=true
spring.governance.driver=default
spring.governance.client.default.rate-limit=5          # burst > 5 concurrent → ErrRateLimited rejections
spring.governance.client.default.error-threshold=20
spring.governance.client.default.open-duration=5s
spring.governance.client.fault.enabled=false           # flip to true + rate=0.5 + error=timeout to "set fire"
```

Run [example-load/](example-load/) (`go run . -concurrency=16 -duration=5s`): the printed
error breakdown shows rate-limited / circuit-open outcomes; the resilience outcome counter and
breaker state-change log lines appear without restart. Policy is hot-reloadable (Dync) — edit
spring.governance.* and the executor picks it up.

### 4.4 Discovery wiring

```properties
spring.elasticsearch.instances.disc.addresses=http://nonexistent.invalid:9200  # dummy, overridden
spring.elasticsearch.instances.disc.service-name=es-cluster
```

A successful boot + `HealthCheck` proves the addresses came from the discovery backend, not
config (example/ feature 5). Remember the resolution is one-shot: moving the instance later
requires a restart (§2.4).

### 4.5 Server-down behavior

- At boot: fail-fast — process exits with "failed to reach elasticsearch cluster".
- At runtime: requests return transport errors through the full chain (span + access log +
  breaker counting); `/readyz` flips DOWN within one probe interval.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "failed to reach elasticsearch cluster" | Unreachable address / wrong credentials / fingerprint mismatch / ES still booting (up to 120 s) | Fix connectivity; wait for `curl http://127.0.0.1:9200` to answer, restart. |
| Boot fails `discovery ... returned no endpoints` | service-name unknown to the backend, or backend not registered | Register the backend bean (a named discovery.Discovery bean) before gs.Run; check the service name. |
| Panic on a nil context inside a request | `http.Request.WithContext` panics on a nil context | Pass `WithContext(ctx)` on every call; never use the no-context API variant. |
| Boot fails on `addresses` validation though service-name is set | `addresses` is required unconditionally (`len($) > 0`) | Keep a dummy address (the example's pattern) — it is overridden. |
| No spans/metrics though code is correct | starter-otel not imported | The resilience layer's signals ride the OTel globals; import starter-otel and configure exporters. |
| No access log lines | log tag filtered by logger config | Check logger config for `_app_elasticsearch_access`. |
| Custom driver instance has no breaker/metrics | Only DefaultDriver installs dynamicTransport | Use DefaultDriver, or install the declaration+resilience transport yourself in the custom driver. |
| Retries seem multiplied | Client `max-retries` + governance `max-retries` both > 0 | Set one of them to 0 / disable-retry. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 16 instance keys |
| Required | 1 (`addresses`, validated non-empty) |
| Quickstart external deps | 1 (Elasticsearch) |
| "Watch out" entries | 5 |

Design suspects (audit ledger; first three carried over from the previous doc):

- `addresses` silently ignored when `service-name` (or `cloud-id`) is set, yet still required
  by validation → the dummy-address pattern is a workaround, not a design (candidate: relax
  the expr when service-name/cloud-id present).
- No `tls.*` block unlike sibling starters — TLS lives in three different keys plus the URL
  scheme (`https://` addresses, `cloud-id`, `certificate-fingerprint`).
- Custom drivers silently lose the governance + resilience declaration transport swap — no warning, no hook.
- schema.json `enable-metrics` default (`false`) disagrees with the code (`true`) — schema is
  not generated, so it drifts.
- Health indicator has no opt-out key (same family asymmetry as go-redis; redigo has
  `health.enabled`).
