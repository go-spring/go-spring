# starter-gateway Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`gateway.go`, `server.go`, `compile.go`, `route.go`, `predicate.go`,
`filter.go`, `proxy.go`, `metrics.go`, `otel_tracing.go`) and the runnable [example/](example/)
(`example/check.sh` is the self-asserting smoke test). The Route/Predicate/Filter model follows
[Spring Cloud Gateway](https://docs.spring-cloud-spring-cloud-gateway/reference/) — everything
below is Go-Spring's increment.

**Activation**: the `gatewayServer` bean is registered only when `spring.gateway.server.addr`
is set (gateway.go:40, `gs.OnProperty("spring.gateway.server.addr")`) — that key is the on/off
switch. Route table + metrics + health beans exist unconditionally; without the server key
routes bind but are never served.

---

## 1. Complete worked project

A realistic gateway fronting a real upstream service with two routes (path + method predicates,
header injection, prefix stripping), health probe, metrics, tracing and rate limiting. File tree:

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-gateway   latest
    go-spring.org/starter-actuator  latest   // optional: probes + management port
    go-spring.org/starter-otel      latest   // optional: real trace export
)
```

**main.go** — the application's entire gateway surface; routes are pure config:

```go
package main

import (
	_ "demo/conf" // via gs config or -D flags; plain properties work too

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-gateway"
	_ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

There is no mandatory application bean: a blank import of `starter-gateway` plus config is a
working gateway (this is exactly what [example/](example/) does). Application code enters only
through the extension seams in §3.3.

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- gateway listen port (activation key; also fails fast on port clash) ------
spring.gateway.server.addr=:9440

# --- route table (hot-reloadable through any refresh-capable config source) --
# Route "orders": /api/orders/** -> strip the /api prefix, stamp X-From, forward.
spring.gateway.routes.orders.predicates.path=/api/orders/**
spring.gateway.routes.orders.predicates.methods=GET,POST
spring.gateway.routes.orders.filters=stripPrefix(1),addRequestHeader(X-From,gw),requestId()
spring.gateway.routes.orders.upstream.target=http://127.0.0.1:19000

# Route "users": POST-only, per-IP rate limit of 50 rps, header must be present.
spring.gateway.routes.users.predicates.path=/api/users/**
spring.gateway.routes.users.predicates.methods=POST
spring.gateway.routes.users.predicates.headers=X-Client-Id:demo
spring.gateway.routes.users.filters=rateLimit(rate=50,key=ip)
spring.gateway.routes.users.upstream.target=http://127.0.0.1:19000

# --- resilience: name registry only; policy values come from ${govern} -------
spring.gateway.resilience.orders=

# --- observability ------------------------------------------------------------
spring.gateway.tracing.enabled=true     # default; spans need starter-otel

spring.actuator.addr=:9370
spring.observability.service-name=demo-gateway
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
```

**Verify** (with any HTTP upstream on `:19000`, e.g. `python3 -m http.server 19000`, or the
in-process backend from example/example.go):

```bash
go run .
curl -i :9440/api/orders/42                 # 200; upstream sees /orders/42, X-From: gw
curl -i :9440/api/users/1 -X POST           # 429 above 50 rps per client IP
curl -i :9440/nope                           # 404 Not Found (gateway itself)
curl -s :9370/gateway/metrics | grep gateway_requests_total
curl -i :9370/healthz                        # gateway indicator UP once table compiled
```

Prerequisites: none for direct `http(s)://` targets. `lb://` routes additionally need a
discovery backend (starter-registry-nacos etc.); cross-replica `rateLimit` a redis limiter
driver; tracing an OTel collector (see example-otel/conf).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gateway
  ├─ gs.Provide(newMetrics)                                   // shared counters
  ├─ gs.Provide(newRouteTable).Destroy(RouteTable.Destroy)    // compiled route table
  ├─ gs.Provide(newGatewayServer).Name("gatewayServer")
  │      .Export(gs.As[gs.Server]())
  │      .Condition(gs.OnProperty("spring.gateway.server.addr"))
  ├─ gs.Provide(newMetricsEndpoint).Export(endpoint.Endpoint) // GET /gateway/metrics
  └─ gs.Provide(newGatewayHealth).Export(health.Indicator)    // "gateway"
        │
gs.Run()
  ├─ config bind: ${spring.gateway} → gatewayConfig (routes via gs.Dync, resilience map,
  │                discovery name, tracing toggle)
  ├─ field injection INTO the route table bean: Wrappers map[string]FilterWrapper
  │                (autowire "?", compile.go:78) — bean-backed filters resolve here
  ├─ GatewayServer.Run (server.go:79):
  │    1. tbl.warmup() — first compile; a bad initial config FAILS STARTUP
  │    2. tlsConfig() — BuildServer(); bad cert files fail startup
  │    3. net.Listen — port clash fails startup
  │    4. <-sig.TriggerAndWait() — serves only after app readiness
  ├─ readiness: health flips UP (route table compiled, metrics.go:150-157)
  └─ on SIGTERM: StopContext → http.Server.Shutdown drains in-flight;
                 RouteTable.Destroy stops every discovery watch (proxy.go:139)
```

Design reasons (source comments): compilation is deferred to warmup because `Wrappers` is
field-injected after construction (compile.go:76-78); the server "listens early, then serves
after readiness" to align with graceful-drain orchestration (server.go:44-46).

### 2.2 Route compile pipeline — exact order per route (compile.go:221-263)

For each route id, in **priority-descending order, ties by ascending id** (compile.go —
deterministic matching; `priority` is an optional per-route key, unset keeps the historical
id-sorted order):

1. **Predicate compile** (`buildPredicates`, predicate.go:29) — each non-empty literal becomes a
   `Predicate`; malformed `headers`/`queries` pairs or a non-RFC3339 `after` return a parse
   error → whole recompile aborted.
2. **Upstream parse** (`parseUpstream`, compile.go:377) — `lb://svc` becomes a discovery-backed
   `Upstream`; anything else must parse as `http(s)://host` or it errors.
3. **Executor resolve** (compile.go:232) — a non-empty `resilience.policy` must name a key of
   `spring.gateway.resilience`; unknown name errors. Executors come from
   `resilience.ExecutorFor("gateway:<name>")` (compile.go:211) — the governance center owns all
   policy values and hot-reloads them; governance off yields a transparent no-op.
4. **Proxy handler** (`newProxyHandler`, proxy.go:159) — see §2.3.
5. **Filter DSL parse** (`buildFilters` → `splitFilters`, compile.go:268-373) — tokens split on
   commas at paren depth 0; unknown filter name, bad args, or a `jwt-auth`/`lua` token whose
   bean name is not in `Wrappers` error out.
6. **Chain assembly** (compile.go:253-260) — filters wrap the proxy outermost-first in
   declaration order, then `instrument` (route-id context stamp + metrics), then the tracing
   span wrapper if enabled.

On any stage error the compiled table is left untouched (keep-last-good, §4.2).

### 2.3 One proxied request, layer by layer

`GET /api/orders/42` on route `orders` (filters `stripPrefix(1),addRequestHeader,requestId`):

1. **Match** — `GatewayServer.ServeHTTP` (server.go:60) walks routes in priority-then-id order; first
   route whose predicates ALL accept wins (`Route.match`, route.go:78 — AND-combined). No match
   → `404 Not Found` from the gateway itself, upstream never contacted.
2. **Tracing wrapper** — extracts incoming trace context, starts server span `gateway orders`
   and child client span `proxy orders`, injects context into outbound headers
   (otel_tracing.go:38-79). No-op without starter-otel.
3. **instrument** — in-flight gauge +1, route id stamped into the request context (metrics.go:86).
4. **Filters, outermost first** — `stripPrefix(1)` rewrites the path to `/orders/42`;
   `addRequestHeader` sets `X-From: gw`; `requestId` generates an `X-Request-Id` if absent.
5. **Proxy handler** (proxy.go:184) — the target is picked ONCE here, not in the Director, so a
   pick failure (no live instance behind `lb://`) is a clean `503` instead of a dial to an empty
   host. For direct upstreams the target is the fixed route URL.
6. **Forward** — `httputil.ReverseProxy` over a clone of `http.DefaultTransport` wrapped by
   `resilience.NewRoundTripper` (proxy.go:165-166): retry/breaker apply to the forwarding hop;
   a resilience retry reuses the same target. Director applies the chosen scheme/host (and
   keeps the inbound Host when `preserveHostHeader` marked the context).
7. **Response** — status captured by `statusWriter`; forwarded 5xx is reported to the
   load-balancer done-callback as failure (outlier accounting, proxy.go:194-198); a transport
   error renders `502 Bad Gateway` (proxy.go:178-181). The unwind records the status-class
   counter and closes both spans.

---

## 3. Per-key behavior reference

### 3.1 Top level — `spring.gateway.*`

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `routes` | map `id→RouteRaw` | empty | **Hot-reloadable** (`gs.Dync`, compile.go:52). Each `<id>` is a route; ids are map keys so uniqueness is structural. | Empty → gateway serves 404 for everything (table compiles to zero routes, still UP). |
| `resilience` | map name→(ignored) | empty | **Name registry only**: keys name the executors routes may reference; the value is an empty struct — policy values live under `${govern}` keyed `gateway:<name>` (route.go:114-128). | Sub-keys under `resilience.<name>.*` are silently ignored by the binder (legacy compat). |
| `discovery` | string | "" | Default discovery backend for `lb://` routes lacking `upstream.discovery`. ⚠ An `lb://` route with neither → compile error (proxy.go:98). | Routes fail to compile (startup error / reload keeps old table). |
| `tracing.enabled` | bool | true | Wraps every matched request in gateway server+client spans. | Needs starter-otel for real export; without it a silent no-op. |

### 3.2 Per route — `spring.gateway.routes.<id>.*`

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `predicates.path` | string | "" | Ant-style: `*` one segment (also inside a segment, `v*`→`v1`), `**` any segments; `/api/**` matches prefix + descendants (predicate.go:65-225). | No predicate at all → catch-all route. |
| `predicates.methods` | string | "" | Comma list, case-insensitive (`GET,POST`). | |
| `predicates.host` | string | "" | Exact host, or `*.suffix` (also matches bare `suffix`); port stripped. | |
| `predicates.headers` | string | "" | `K:V;K2:V2` — **all** pairs must match exactly. Malformed pair (no separator) → parse error. | Parse error at compile time. |
| `predicates.queries` | string | "" | `k=v;k2=v2` — all must match exactly. | Same. |
| `predicates.after` | string | "" | RFC3339; route matches only at/after that instant. Non-RFC3339 → parse error. | Same. |
| `priority` | int | 0 | Matching order: larger priority is checked first; ties (incl. all-unset) fall back to ascending id — the historical default. Hot-reloadable with the rest of the route. | Overlapping paths resolve to the higher priority (then lower id); renaming an id no longer changes precedence among prioritized routes. |
| `filters` | string | "" | Filter DSL, §3.3; applied outermost-first in declaration order. | Parse errors at compile time, not bind time. |
| `upstream.target` | string | — | **Required in practice**: `lb://<service>` or `http(s)://host[:port]`. Missing/malformed → `parseError` (compile.go:379-392). | Route fails to compile. |
| `upstream.balancer` | string | `round_robin` | One of `round_robin`, `least_conn`, `consistent_hash`, `weighted` (cloud/loadbalance). Unknown name → compile error. | Per-upstream; two routes to one service may differ while sharing the discovery watch. |
| `upstream.discovery` | string | "" | Per-route backend override of top-level `discovery`. | |
| `upstream.suspend-threshold` | int | 0 (off) | Consecutive failures before an lb:// upstream instance is suspended (outlier suspension, cloud/loadbalance `Tracker`); 0 disables. | A zombie instance (up but failing) stops receiving traffic for the cool-down instead of yielding periodic 502s. |
| `upstream.suspend-for` | string | `""` | Go duration (e.g. `30s`); how long a suspended instance stays out before a half-open trial. Empty/0 keeps the tracker's 5s default. | Malformed value → compile error at reload, previous table kept. |
| `resilience.policy` | string | "" | Must name an existing `spring.gateway.resilience.<name>` key. ⚠ Coupling: unknown name → `unknown resilience policy` compile error (compile.go:236). | Startup failure / reload keeps old table. |

⚠ **Precedence coupling**: when no route sets `priority`, matching order is sorted by route id
(compile.go). Renaming an id changes precedence; overlapping paths resolve to the
alphabetically-first id. Set `priority` to make precedence explicit and id-independent.

### 3.3 Filter DSL reference (parser: compile.go:309-373)

One comma-separated string; commas inside parentheses are argument separators — split happens
at paren depth 0, so `addRequestHeader(X-Foo,gw)` survives. `name(a,b)` args are trimmed.
Argument values cannot contain `,`, `(` or `)` — the DSL deliberately has **no escape syntax**;
such a value fails route compilation with an error pointing at the alternatives: resolve the
value from a `${...}` config placeholder at bind time, or carry it in a custom filter
registered via `RegisterFilter`. Registered built-ins (filter.go:73-86):

| Filter | Args | Behavior |
|--------|------|----------|
| `stripPrefix(n)` | n ≥ 0 | Drop the first n path segments (`/api/orders/42` →n=1→ `/orders/42`); n beyond depth → `/`. |
| `prefixPath(p)` | prefix | Prepend `p` (slashes normalized). |
| `addRequestHeader(k,v)` / `setRequestHeader(k,v)` / `removeRequestHeader(k)` | 2 / 2 / 1 | Add/Set/Del on the request headers before forwarding. |
| `addResponseHeader(k,v)` / `setResponseHeader(k,v)` / `removeResponseHeader(k)` | 2 / 2 / 1 | Same on the response headers (applied before the proxy writes them). |
| `rewriteHost(h)` | host | Override outbound Host. |
| `preserveHostHeader()` | — | Keep inbound Host instead of the upstream's; sets a marker the Director honors (proxy.go:174). |
| `requestId([header])` | optional | Ensure an `X-Request-Id` (or the given header) exists; generates a random 128-bit hex id when absent. |
| `rateLimit(k=v,…)` | see below | `rate` (req/s, **required** >0), `burst`, `driver` (default `default`; a redis driver gives cross-replica budgets), `algorithm` (`token-bucket`/`sliding-window`), `key` (`route` default / `ip`). Reject → `429 Too Many Requests`; **fails open** on limiter backend error with a Warn log (filter.go:269-277). |

Bean-backed tokens (resolved from the injected `Wrappers` map, NOT the registry):

| Token | Behavior |
|-------|----------|
| `jwt-auth(beanName)` | Calls `beanName.Wrap` on a bean exported as `gateway.FilterWrapper` (e.g. starter-security-jwt's Authenticator). Exactly one non-empty arg. Unknown bean name → compile error telling you to export the bean (compile.go:283). |
| `lua(beanName)` | Same seam, for starter-lua-filter's Filter. |

Extension seams:

- `gateway.RegisterFilter(name, factory)` — global registry for self-contained factories;
  panics on empty name / nil / duplicate (filter.go:51-64).
- `gateway.FilterWrapper` — export any `Wrap(next http.Handler) http.Handler` bean under that
  type; reference by bean name in the DSL.

### 3.4 Server — `spring.gateway.server.*`

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key** (gateway.go:40); port clash fails at `net.Listen` (server.go:92). | Missing → routes bind but nothing serves; no warning is emitted. |
| `tls.enabled` | bool | false | Enables TLS listen. | |
| `tls.cert-file` / `tls.key-file` | string | "" | Server certificate; built via `tlsconf.BuildServer` — missing/unreadable files fail startup. | |
| `tls.ca-file` | string | "" | **mTLS switch**: presence → `RequireAndVerifyClientCert` (server.go:69-75). ⚠ unlike the echo starter, this IS wired for mTLS. | |
| `tls.server-name` / `tls.insecure-skip-verify` | string/bool | — | Client-side keys of the shared tlsconf struct — **bound but dead** on a server. | Silent no-op. |

---

## 4. Verification & fault drills

All drills assume the §1 project running with `go run .` and an upstream on `:19000`
(`python3 -m http.server 19000` or any HTTP server).

### 4.1 Routing, predicates, filters

```bash
curl -i :9440/api/orders/42            # 200, upstream log shows GET /orders/42
curl -i :9440/api/orders/42 -X POST    # still 200 (methods=GET,POST)
curl -i :9440/api/users/1 -X POST      # 200 only with header:
curl -i -H 'X-Client-Id: demo' :9440/api/users/1 -X POST
curl -i :9440/nope                     # 404 Not Found — no route matched
```

### 4.2 Hot route edit (no restart)

`spring.gateway.routes` is a `gs.Dync` field (compile.go:52): any refresh-capable config
source (starter-config-file volume watch, starter-config-nacos, ...) swaps the map, and the
**next request** lazily recompiles the table (`RouteTable.current`, compile.go:116 — the fast
path is one atomic load + pointer compare).

1. Add a route to the live config source:
   `spring.gateway.routes.ping.predicates.path=/ping/**` +
   `spring.gateway.routes.ping.upstream.target=http://127.0.0.1:19000`.
2. Trigger the source's refresh (file watcher mtime / nacos push). No restart, no log line on
   success — the new table simply takes effect on the next request:
   `curl -i :9440/ping/anything` → 200 from the upstream.
3. Modify an existing route (e.g. change `stripPrefix(1)` to `stripPrefix(2)`) and repeat —
   the upstream path changes accordingly.

Note: a plain static `app.properties` without a refresh source never swaps the map — hot reload
requires one of the dynamic config starters.

### 4.3 Reload error keeps the old table

Inject a broken route (parse error), e.g. `filters=stripPrefix(abc)` or
`upstream.target=ftp://x` or `resilience.policy=nonexistent`:

1. Old table still serving: `curl -i :9440/api/orders/42` → 200 as before.
2. New/broken route absent: `curl -i :9440/ping/x` → 404.
3. Error surfaced loudly — grep the app log:
   `grep 'route reload failed, keeping previous table'` (compile.go:123, Error level, tag AppDef).
4. Metric incremented:
   `curl -s :9370/gateway/metrics | grep gateway_route_reload_errors_total` → 1.
5. The broken map's pointer is adopted (compile.go:125) so the same broken edit is NOT retried
   every request; fixing the config and refreshing again recompiles normally.

Contrast: the FIRST compile at startup fails the whole application (server.go:82) — only
post-startup reloads keep-last-good.

### 4.4 Rate limit drill

```bash
for i in $(seq 1 100); do curl -s -o /dev/null -w '%{http_code}\n' -X POST :9440/api/users/1 -H 'X-Client-Id: demo'; done | sort | uniq -c
# with rate=50: mix of 200 and 429; from a second source IP the budget is independent (key=ip)
```

### 4.5 Observability

- Metrics (actuator `GET /gateway/metrics`, Prometheus text, metrics.go:110-139):
  `gateway_requests_total{route="orders",status="2xx"}`, `gateway_in_flight_requests`,
  `gateway_route_reload_errors_total`.
- Health: `curl :9370/healthz` — the `gateway` indicator is UP iff the route table compiled
  (metrics.go:150-157). An `lb://` route with zero live instances stays UP by design — that is
  a per-route concern (503s + Warn logs), not a gateway-wide failure.
- Traces (with starter-otel): server span `gateway <routeId>` with `gateway.route` attribute +
  child client span `proxy <routeId>`; 5xx marks the client span Error. Filter by route id in
  your tracing backend.

### 4.6 Upstream failure drill

Stop the upstream → each request gets `502 Bad Gateway` (proxy.go:180) with a Warn log
`route %q upstream error`. For `lb://` routes with no live instance → `503 Service Unavailable`
("no upstream", proxy.go:187-189).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Gateway doesn't listen | `spring.gateway.server.addr` missing | Set it — it is the activation key. Routes configured without it now log a startup WARN naming the orphaned route(s) (RouteTable.Init, compile.go) — they are never served. |
| App fails at startup with `route %q: gateway: invalid …` | Initial route table has a parse error (predicate/upstream/filter) | Fix the literal; first compile is fatal by design (server.go:82). |
| `route reload failed, keeping previous table` in logs | Hot edit broken; old table still serving | Fix the literal and refresh again; watch `gateway_route_reload_errors_total`. |
| Always 404 | No route's predicates match (path typo, methods/host/headers predicate rejecting) | Remember first-match in priority-then-id order; a more specific route with a later id never wins over an overlapping earlier id — set `priority` to override. |
| `unknown resilience policy` | `resilience.policy` names no key under `spring.gateway.resilience` | Add the name as a (value-less) key; policy VALUES come from `${govern}` under `gateway:<name>`. |
| Legacy `resilience.<name>.max-retries` etc. have no effect | By design — value is an empty struct; binder ignores sub-keys (route.go:121-128) | Move policy to the governance center, resource label `gateway:<name>`. |
| `no FilterWrapper bean named …` | `jwt-auth(x)`/`lua(x)` references a bean not exported as `gateway.FilterWrapper` | Export the bean with `.Export(gs.As[gateway.FilterWrapper]())` before startup (wrappers inject pre-warmup). |
| `lb:// upstream cannot resolve … mesh mode active` | lb route with no discovery backend, or mesh mode on | Set `upstream.discovery`/`spring.gateway.discovery`; in mesh mode route to the service's stable address instead (proxy.go:115). |
| 429s you didn't ask for / limiter not shared across replicas | `rateLimit` key defaults to route id; driver defaults to `default` (local) | Use `key=ip` for per-client, a `redis` driver for cross-replica budgets. |
| Upstream sees wrong path/Host | stripPrefix count off; Host rewritten by default | Tune `stripPrefix(n)`; add `preserveHostHeader()` or `rewriteHost(h)`. |
| No traces despite `tracing.enabled=true` | starter-otel not imported | Add it; the OTel globals are otherwise silent no-ops. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (live) | 22 (4 top + 12 route + 2 server + 4 tls live; 2 tls keys dead) |
| Required | 2 (`server.addr`, `upstream.target`) |
| Quickstart external deps | 0 (discovery/redis/collector optional) |
| "Watch out" entries | 7 |

Design suspects (for the audit ledger):

1. ~~17 dead legacy policy keys bind but are never read~~ — evolved: `policyRaw` is now an empty
   struct (route.go:128), so legacy sub-keys do not even bind (silently ignored). Residual risk:
   users porting old config get zero feedback that policy moved to `${govern}`.
2. ~~Filter DSL is a string grammar — no escaping for `,`/`()`~~ — resolved 2026-08-28 by
   design: no escape syntax is added; values containing `,`/`(`/`)` are rejected at route
   compile time with an error pointing at `${...}` placeholder / `RegisterFilter` alternatives.
3. ~~Route config without `spring.gateway.server.addr` silently does nothing~~ — resolved
   2026-08-28: `RouteTable.Init` logs a startup WARN naming the orphaned route(s).
4. ~~Stale example-otel tracing key~~ — FIXED 2026-08-27. Additionally the main example's smoke
   ran before `gs.Run()` (server not yet up) — also fixed via a background gs.Runner.
5. ~~Route precedence implicit via sorted id — no explicit order key~~ — resolved 2026-08-28:
   optional per-route `priority` key; larger priority matches first, ties keep id order.
6. ~~`schema.json` is a stub; example-otel has config but no main.go~~ — resolved 2026-08-28:
   schema.json now mirrors the full config surface; example-otel/main.go makes it runnable.
7. `tls.server-name` / `tls.insecure-skip-verify` are client-side keys of the shared tlsconf
   struct — bound but dead on the server side (same pattern as other starters).
