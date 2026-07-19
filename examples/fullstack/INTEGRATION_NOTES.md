# Integration Notes (协同踩坑记录)

What surfaced when this many starters were assembled into one request path. Each
entry is either **resolved in the sample** (how) or a **gap that feeds back** to
a starter. This is the reference app's primary output — a regression baseline and
a to-do list for the ecosystem.

## 1. Multi-process management-port collision — *resolved*

Every service blank-imports `starter-actuator`, which binds `:9370` by default,
and each also wants a main/business port. Running three services on one host
means three processes competing for the same `:9370` (and the built-in HTTP
`:9090`). This is invisible in a single-service example.

**Resolved** by assigning distinct ports per process:
`gateway 9440/9370`, `order 8081/9371`, `inventory 8082/9372`. There is no
auto-offset; the ecosystem assumes one service per pod, so a multi-service host
must allocate explicitly. Documented in each `conf/app.properties`.

## 2. Consul has no client-side discovery starter — *gap feeds back*

`starter-registry-consul` is register-side only (it advertises an instance). The
gateway's `lb://order` and order→inventory need a *client-side*
`discovery.Discovery`, and only `starter-discovery-k8s` ships one.

**Bridged in the sample** via `internal/consuldisc`, a Consul-catalog-backed
`discovery.Discovery` registered with `discovery.Register("consul", …)`. It
proves the `stdlib/discovery` abstraction is sufficient to close the gap, but a
first-class `starter-discovery-consul` would remove this per-app code. **Feeds
back**: candidate new starter.

## 3. `lb://` resolves through a global registry, not a bean — *ordering note*

The gateway resolves an `lb://` upstream via `discovery.MustGet(name)` — the
process-global `stdlib/discovery` registry, not the IoC container. So
`discovery.Register(...)` must run in `init()` (before route compilation), and it
is a global side effect, not an injectable bean. Mixing a global registry with
bean-based wiring is easy to get wrong; the sample keeps the `Register` call in
each `main`'s `init` next to the blank imports so the ordering is obvious.

## 4. Trace does not survive the gateway hop — *gap feeds back*

`starter-otel` sets the global W3C propagator, so order→inventory continuity
works with a manual `Inject`/`Extract` around the HTTP call. But the **gateway
does not propagate the trace** to `order`: `httputil.ReverseProxy` (inside
`starter-gateway`) does not inject `traceparent`, and the gateway adds no client
instrumentation on the forward hop. Result: the gateway has its own trace and
`order` starts a fresh root — the end-to-end trace is split at the edge.

**Feeds back**: `starter-gateway` should contribute otel propagation (and a
server/client span) on the forward hop, the same way the services do manually.
Until then, "one request = one trace" holds only from `order` inward.

## 5. Inbound spans are manual — double-instrumentation risk — *watch item*

Neither `starter-gin` nor the built-in HTTP server starts a server span, so each
service adds its own span middleware (`traceMiddleware` / `traceServer`) to get a
`trace_id` into logs. That is correct **today**: `starter-otel` is provider-only
and adds no auto-middleware, so there is exactly one span per request. If a future
release adds automatic inbound instrumentation to those starters, this manual
layer would produce **two spans per request** — remove it in lock step with any
such change.

The two copies of the extract-then-start boilerplate were **de-duplicated** into
`StarterOTel.StartServerSpan(ctx, header, tracer, name)`
(`starter/starter-otel/serverspan.go`) — the inbound counterpart to the outbound
`discovery.SetTraceInjector` seam this starter already fills. Both services now
call it, so if inbound auto-instrumentation ever lands, there is one place to
retire, not two. See the Task 07.9 convergence log below.

## 6. Auth placed at the resource server, not the gateway — *design decision*

`Authenticator.Wrap` fits the gateway `Filter` seam, so JWT *could* be enforced
at the edge. The sample instead enforces it at `order` (the resource server) and
lets the gateway forward `Authorization` untouched. Rationale: the service that
owns the protected resource should decide authorization, and this keeps the
gateway a pure router. Both are valid; the choice is documented so it is not
mistaken for a limitation.

## 7. Nacos `optional:` import is required for cold start — *resolved*

`order` imports its charge-fail flag from Nacos. Without the `optional:` prefix
the service fails fast when the data id does not yet exist (first run, before any
publish). With `optional:` it starts with the default and refreshes live once the
value is published. Confirmed the `gs.Dync` field updates with no restart.

## 8. `Rooter __default__` collision — *avoided by construction*

The known `starter-config-file` pitfall (a `Rooter` bridge colliding on
`__default__`) does **not** arise here because the sample uses `starter-config-nacos`,
whose provider needs no `Rooter` bridge. Recorded so a future switch to
`starter-config-file` in this app re-checks it.

---

## Task 07.9 convergence log

The five conflict fronts the parallel-development plan flagged for convergence,
each driven to a verdict against the code above:

1. **Config-prefix collisions — non-issue.** Every capability owns a distinct
   top-level prefix (`spring.gateway`, `spring.observability`, `spring.actuator`,
   `spring.security.jwt`), and same-capability families deliberately share one
   (`spring.registry`, `spring.config`, `spring.transaction`, `spring.redis`) so
   swapping implementation is import-only. No unrelated starters read the same
   key. The only real per-host clash — ports — is solved by item 1 above.
2. **Multi-server drain order — verified correct.** The actuator server is a
   `PreStopper`, so on SIGTERM readiness flips first, the `pre-stop-delay` window
   elapses with every server (including actuator) still up, then all stop
   concurrently. No ordering guarantee between servers is needed because nothing
   stops until draining is done (`spring/gs/internal/gs_app/app.go`).
3. **OTel double instrumentation — non-issue.** `starter-otel` is provider-only;
   `starter-gin`/`starter-gateway` add no otel middleware; no provider is set
   twice. Exactly one span per request (see item 5).
4. **`Rooter __default__` collision — guarded.** Every config-source starter names
   its refresh bridge distinctly and the container fail-fasts on duplicate bean
   ids (`resolving.go` `checkDuplicateBeans`), so two config sources cannot
   silently collide. Not exercised here (Nacos-only), but safe if switched.
5. **Duplicate trace helper — extracted.** The inbound extract-then-start dance,
   copied into both services, moved to `StarterOTel.StartServerSpan` — the
   inbound twin of the existing outbound injector seam. Kept out of `stdlib`
   (zero-third-party rule; this needs OTel). Rejected as not worth extracting:
   `extractField` (single use), the `os.Chdir` workdir init (example
   boilerplate), and the LiveDialer `http.Client` (its reusable core
   `discovery.NewLiveDialer` already exists).

**Still open (feeds back, out of dedup scope):** item 4 above — the trace does not
survive the gateway hop because `starter-gateway` is intentionally otel-free and
starts no span. The seams to close it now exist without coupling the gateway to
OTel: wrap the proxy transport with `discovery.TraceRoundTripper` (forward hop
injection, gateway already imports `discovery`) and give the gateway an inbound
span. Tracked as a `starter-gateway` enhancement, not a consolidation fix.

---



- `POST /api/orders` without a token → **401** at `order`.
- `POST /api/orders` with a token → **200 committed**, request crossing
  gateway → order → inventory, discovered via Consul.
- Publish `fullstack.order.charge-fail=true` to Nacos → next order **409
  compensated**, and `inventory` logs a matching `/release` — cross-service
  rollback triggered by a live config change.
- SIGTERM → all three services drain and exit.

See [中文版](INTEGRATION_NOTES_CN.md).
