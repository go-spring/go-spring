# starter-actuator Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-actuator` is a Server-archetype starter (`starter/DESIGN.md`
§2.1). It runs a management HTTP server on its own port that exposes
Kubernetes probes, build info, and runtime introspection — the operational
counterpart to the application's business server.

## 1. Responsibilities & Boundaries

- Serves `/healthz`, `/readyz`, `/startupz` (K8s liveness / readiness /
  startup probes; legacy `/health`, `/readiness`, `/startup` kept as
  aliases) plus `/info`, `/loggers`, `/env`, `/configprops`,
  `/threaddump`.
- Collects every bean exported as `health.Indicator` and folds their
  status into `/readyz`; the actuator does not know any concrete backend
  (a Redis client, a GORM pool, ...) — the seam is the stdlib interface.
- Collects every `endpoint.Endpoint` bean and mounts each on this server
  so one management port covers actuator + otel `/metrics` + future
  contributors, without cross-starter imports.
- Coexists with the app's main HTTP server (via a distinct bean `Name`)
  and with `starter-pprof` and `starter-admin-ui` — one port each.

## 2. Key Abstractions & Seams

- **Serves during startup, not after readiness.** The server binds and
  serves immediately; it calls `sig.TriggerAndWait` only to observe the
  overall aggregate. This is intentional: a readiness probe must be able
  to see the OUT_OF_SERVICE→UP transition, and a liveness probe must
  answer throughout a long boot so the pod is not killed prematurely.
- **`health.Indicator` in stdlib.** The interface lives in
  `spring/health`, not in the starter — the container only matches by
  explicit `.Export(As[Iface]())`, and the interface must be reachable
  to every contributor (redis, gorm, ...) without importing this
  starter.
- **`PreStop` flips readiness.** `PreStop` sets `draining=true` so
  `/readyz` returns 503 OUT_OF_SERVICE while other servers keep serving
  in-flight requests; K8s endpoint controller removes the pod, then the
  drain delay elapses, then servers stop.
- **Endpoint contribution.** Built-in patterns are registered first;
  contributor endpoints (e.g. otel `/metrics`) are mounted after. A
  duplicate pattern panics `ServeMux` at boot — the surface for
  detecting misconfiguration.
- **Per-sweep timeout on `/readyz`.** A single readiness sweep is
  capped by `checkTimeout` so one slow indicator cannot stall the probe
  past the K8s probe timeout.

## 3. Constraints

- **Fixed default port `:9370`.** Distinct from the main HTTP server
  (`:9090`) and pprof (`127.0.0.1:9981`); binds all interfaces so
  in-cluster probes can reach it.
- **JSON body limits.** POST `/loggers/{name}` decodes up to 64 KiB with
  `DisallowUnknownFields` — a malformed or oversized POST cannot
  exhaust memory or silently ignore typos.
- **Indicators / Endpoints / Env are optional (`autowire:"?"`).** An app
  with no indicators still gets liveness/readiness/info.
- **Group semantics.** Liveness ignores non-liveness-group indicators;
  readiness AND startup each require their group to pass — a degraded
  dependency must not restart the pod, only bleed traffic.

## 4. Trade-offs / Alternatives Rejected

- **Reusing the app's main HTTP mux — rejected.** Probes must answer
  during startup and drain, so they need a listener whose lifecycle is
  decoupled from the app's readiness gate.
- **Pushing indicators from starters instead of pulling — rejected.**
  Push forces every backend starter to import the actuator; pull via
  interface export lets the actuator work with or without any backend.
