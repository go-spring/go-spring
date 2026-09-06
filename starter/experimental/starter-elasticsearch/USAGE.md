# starter-elasticsearch Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health/health.go`, `resilience_test.go`) and the runnable examples
([example/](example/) — self-asserting smoke; [example-otel/](example-otel/),
[example-cloudnative/](example-cloudnative/), [example-load/](example-load/)). Elasticsearch
semantics and the go-elasticsearch v8 API are
[the client's own documentation](https://www.elastic.co/guide/en/elasticsearch/client/go-api/current/index.html)
— everything below is go-spring's increment.

**Activation**: any `spring.elasticsearch.*` key (the module is `gs.Module(gs.OnProperty("spring.elasticsearch"))`,
a prefix check). Each `spring.elasticsearch.<name>` entry creates one `*StarterElasticsearch.Client`
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
    go-spring.org/starter-governance   latest   // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:9200", Healthy: true}))
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
    // Always the wrapper type *StarterElasticsearch.Client. It embeds
    // *elasticsearch.Client, so Index/Get/Search/Info promote unchanged.
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
spring.elasticsearch.main.addresses=http://127.0.0.1:9200

# --- discovery-backed instance ---------------------------------------------
# `addresses` is required by validation even when service-name overrides it —
# the dummy below is deliberately non-resolvable to prove discovery wins.
spring.elasticsearch.disc.addresses=http://nonexistent.invalid:9200
spring.elasticsearch.disc.service-name=es-cluster
spring.elasticsearch.disc.discovery-scheme=http

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
  └─ gs.Module(OnProperty("spring.elasticsearch")) fires when any spring.elasticsearch.* key exists
        └─ conf.BindEach("${spring.elasticsearch}") → one Config per <name> entry
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver)).Name(<name>)
              │      .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator named "elasticsearch:<name>"
                     .Export(gs.As[health.Indicator]())   [starter.go:41-71]

gs.Run()
  ├─ ctor newClient [starter.go:73]:
  │    ├─ service-name set && !mesh.Enabled() → resolveAddresses:
  │    │      discovery.NewLoader → read snapshot → "scheme://host:port" overrides c.Addresses
  │    │      (fails fast: no backend registered, or no endpoints for the service)
  │    ├─ optional Driver bean (none → bundled DefaultDriver) → driver.CreateClient:
  │    │      DefaultDriver installs dynamicTransport + OTel instrumentation
  │    │      and records it in dynamicTransports; newClient picks it up for Init
  │    └─ HealthCheck (Info request) — unconditional fail-fast probe; failure closes the
  │        client and aborts boot
  ├─ Init [client.go:78]: newDBObserver("elasticsearch") → module-local observer
  │    → obsTransport (metric + access log, no span)
  │    → fault.WrapExecutor(resilience.ExecutorFor(resource)) — governance seams
  │    → resilience.WrapExecutor(exec, "elasticsearch") — outcome counter + resilience span
  │    → dyn.Swap(resilience.NewRoundTripper(obsTransport, exec, →resource)) [client.go:86-92]
  ├─ readiness: indicator flips UP (runs client.Info)
  └─ SIGTERM → Destroy [client.go:98]: exec.Close → stop discovery watch → client.Close
```

A dead cluster, or a service-name with no endpoints fails the boot — the process never reaches
"serving" with a broken ES connection.

### 2.2 The transport chain — exact order and why

Every request passes through the layers below. Order as built:

```
elasticsearch API (Index/Get/...)
  → elastictransport: OTel instrumentation span (NewOtelInstrumentation(nil,...) — rides the
      OTel global TracerProvider starter-otel installs; no-op otherwise)
  → elastictransport retry loop (MaxRetries / DisableRetry)
  → dynamicTransport (RWMutex indirection; http.DefaultTransport until Init swaps)
  → resilience roundTripper: executor = fault.InjectorFor → limiter/breaker/bulkhead/retry,
      itself wrapped by resilience (span + outcome counter + access log per Execute)
  → obsTransport: db.client.operation.duration histogram + db.client.active_requests gauge
      + _app_elasticsearch_access log (module-local observer emits no span — no duplicate)
  → http.DefaultTransport → network
```

Rationale (source comments, [command.go:17-41] and [client.go:56-95]):

- **Trace comes from elastictransport, not the module observer.** The client exposes
  `elasticsearch.Config.Instrumentation`, so the span covers retries too; the module-local
  observer in [observe.go] emits no span and only fills the metric+log gap.
- **Resilience OUTSIDE the observe transport** — deliberate difference from go-redis
  (where the access log wraps the breaker). Here the resilience executor itself is wrapped by
  `resilience.WrapExecutor(exec, "elasticsearch")`, so breaker trips / rate-limit rejections
  get their *own* span + outcome counter, while obsTransport records the HTTP outcome inside
  the protected call.
- **dynamicTransport instead of a fixed transport**: the ES transport is fixed at construction
  and cannot be swapped on the client afterwards; the indirection keeps the resilience policy
  hot-reloadable (Dync) even though the transport instance is not [client.go:31-44]. The slot
  is guarded by RWMutex, not atomic.Value, because the active round-tripper is one of several
  distinct concrete types — the atomic.Value version panicked on the second Swap, which is
  exactly what resilience_test.go pins as a regression test.
- **Custom drivers may install none of this**: only clients built by DefaultDriver appear in
  `dynamicTransports`; a custom driver's own transport silently bypasses the Init-time swap —
  resilience and the observe transport are then unavailable for that instance.

### 2.3 One request through the chain: `Search` with a match query

1. The generated API builds `POST /demo-docs/_search`; elastictransport opens the client span
   (no-op without starter-otel's globals).
2. The retry loop (up to `max-retries`, default 3) hands the request to dynamicTransport.
3. The resilience executor asks for a permit scoped to the resource label, e.g.
   `elasticsearch:es-cluster` or `elasticsearch:http://127.0.0.1:9200` (first address;
   derived via `resilience.ResourceLabel` [client.go:114-122]) — per cluster, not per request.
   With governance off, the executor is a transparent no-op.
4. obsTransport derives the operation `POST /demo-docs/_search` from method + URL path, bumps
   the in-flight gauge, and emits the duration histogram + access log on completion
   [command.go:44-51].
5. The caller sees exactly what plain go-elasticsearch returns — including 4xx `res.IsError()`
   bodies; only transport-level errors (5xx are mapped to a breaker-tripping, retryable error
   by the resilience round-tripper, cloud/governance/resilience/roundtripper.go:99-104).

**Context is mandatory on every call.** The OTel instrumentation derives its span from the
request context and panics on a nil parent — `HealthCheck` and the health indicator always pass
one explicitly [starter.go:120-124, health/health.go:19-31]; user code should use
`es.Search.WithContext(ctx)` etc.

### 2.4 Discovery addressing — one-shot at startup

When `service-name` is set and mesh mode is off, the endpoints are resolved **once** in the
ctor and baked into `c.Addresses`; the loader is a pure snapshot function with no resources and
no background watch, so nothing is kept alive and nothing needs stopping on shutdown
[starter.go:55-70, driver.go:96-125]. No re-resolution at runtime:
ES cluster addresses are typically stable VIPs. In mesh mode the sidecar owns discovery+LB and
the static Addresses (or CloudID) are used unchanged.

---

## 3. Per-key behavior reference

All keys live under `spring.elasticsearch.<name>.` — per-instance prefix binding via
`conf.BindEach` (the ctor's Config arg). There are no observability keys — observation is
unconditional (see §3.4).

### 3.1 Addressing & discovery

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addresses` | list | — | Node URLs, e.g. `http://127.0.0.1:9200` (comma-separated). Validated non-empty (`len($) > 0`). ⚠ Required even when `service-name` overrides it — the example carries a non-resolvable dummy on purpose. ⚠ Ignored when `cloud-id` is set (client-side precedence). | Empty → BindEach error; unreachable first probe → boot error "failed to reach elasticsearch cluster". |
| `service-name` | string | — | Resolve node addresses via a registered discovery backend, once at startup; overrides `addresses`. Ignored in mesh mode. ⚠ Pairs with `scheme`/`discovery`/`discovery-scheme`. | Service has no endpoints → boot error `discovery %q returned no endpoints`. |
| `scheme` | string | — | Narrows discovery to endpoints of one transport scheme. Only consulted when `service-name` is set. | — |
| `discovery` | string | `default` | Which registered discovery backend resolves `service-name`. | Unregistered backend → boot error at NewLoader. |
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
| `max-retries` | int | 3 | elastictransport retry count. ⚠ Interacts with the governance executor's retry (`govern.default.max-retries`) — two stacked retry loops multiply attempts and latency. | Large + governance retry → multiplied attempts. |
| `disable-retry` | bool | false | Disables the client retry loop entirely. | — |
| `compress-request-body` | bool | false | gzip request bodies. | — |
| `enable-metrics` | bool | true | Client's built-in elastictransport metrics switch (⚠ schema.json wrongly says default false — code is the truth). | — |
| `enable-debug-logger` | bool | false | elastictransport debug logging. | On → very chatty logs incl. request/response bodies. |

### 3.4 Instrumentation

There are **no instrumentation config keys** (no level, no skip list, no argument cap):
metrics and the access log are emitted unconditionally by module-local instrumentation
([observe.go]); the trace span comes from the elastic transport's own OTel instrumentation.
Both ride the OTel globals starter-otel installs (`spring.observability.*`).

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

- **Metrics** (OTel, via starter-otel's globals): `db.client.operation.duration` histogram
  and `db.client.active_requests` gauge, attributes `db.system=elasticsearch`,
  `db.operation` = `"<METHOD> <path>"`, `status` = ok/error. Resilience layer adds
  `resilience.calls` counter with `resilience.outcome` ∈
  {success, rate_limited, circuit_open, bulkhead_full, timeout, error}.
- **Spans**: client span per request from elastictransport instrumentation (name like
  `POST /demo-docs/_search`); one internal span per resilience Execute
  (`resilience.resource` attribute). Verify: `curl "http://127.0.0.1:16686/api/traces?service=demo&limit=1"`
  and grep for `data":[{` (same check as example-otel's self-test).
- **Access log**: one line per request under tag `_app_elasticsearch_access`
  (`log.RegisterAppTag("elasticsearch", "access")`) at native levels — error → Warn; success
  with the captured URL path (truncated to 512 bytes) → Debug; plain success → Info.

```bash
curl -s "127.0.0.1:9090/search" >/dev/null  # or drive via the app
curl -s :9090/metrics | grep -E 'db.client.operation.duration|active_requests'
grep _app_elasticsearch_access app.log | tail -1
```

### 4.3 Resilience / fault drill (example-load style)

```properties
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5          # burst > 5 concurrent → ErrRateLimited rejections
govern.default.error-threshold=20
govern.default.open-duration=5s
govern.fault.enabled=false           # flip to true + rate=0.5 + error=timeout to "set fire"
```

Run [example-load/](example-load/) (`go run . -concurrency=16 -duration=5s`): the printed
error breakdown shows rate-limited / circuit-open outcomes; the resilience outcome counter and
breaker state-change log lines appear without restart. Policy is hot-reloadable (Dync) — edit
govern.* and the executor picks it up.

### 4.4 Discovery wiring

```properties
spring.elasticsearch.disc.addresses=http://nonexistent.invalid:9200  # dummy, overridden
spring.elasticsearch.disc.service-name=es-cluster
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
| Boot fails `discovery ... returned no endpoints` | service-name unknown to the backend, or backend not registered | Register the backend (discovery.RegisterDiscovery) before gs.Run; check the service name. |
| Panic with nil context inside a request | OTel instrumentation derives the span from the request context | Pass `WithContext(ctx)` on every call; never use the no-context API variant. |
| Boot fails on `addresses` validation though service-name is set | `addresses` is required unconditionally (`len($) > 0`) | Keep a dummy address (the example's pattern) — it is overridden. |
| No spans/metrics though code is correct | starter-otel not imported | Instrumentation rides the OTel globals; import starter-otel and configure exporters. |
| No access log lines | log tag filtered by logger config | Check logger config for `_app_elasticsearch_access`. |
| Custom driver instance has no breaker/metrics | Only DefaultDriver installs dynamicTransport | Use DefaultDriver, or install the observe+resilience transport yourself in the custom driver. |
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
- Custom drivers silently lose the governance/resilience observe transport swap — no warning, no hook.
- schema.json `enable-metrics` default (`false`) disagrees with the code (`true`) — schema is
  not generated, so it drifts.
- Health indicator has no opt-out key (same family asymmetry as go-redis; redigo has
  `health.enabled`).
- Resilience sits OUTSIDE the observe transport here but INSIDE for go-redis — per-family
  layering differs; the resilience bridge compensates but doubles span/metric emission
  points across families.
