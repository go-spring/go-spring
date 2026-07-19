# starter-admin-ui Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-admin-ui` is a Server-archetype starter (`starter/DESIGN.md`
§2.1): a lightweight self-contained dashboard that periodically polls
the `starter-actuator` endpoints of a configured list of application
instances and renders an aggregated status table.

## 1. Responsibilities & Boundaries

- Runs its own `http.Server` on `:9280` by default; distinct from the
  actuator (`:9370`), pprof (`:9981`) and the app's main server.
- Polls each configured instance's `/health`, `/readiness`, `/startup`,
  `/info` on a fixed cadence and serves the aggregate as both HTML and
  `GET /api/status` JSON.
- Deliberately narrow in scope. Prometheus + Grafana is the recommended
  aggregate-monitoring story; this starter targets the case where that
  stack is unavailable (on-prem, ad-hoc bring-up, local inspection).
  Framed as "equivalent effect via idiomatic Go", not a
  feature-for-feature reimplementation of Spring Boot Admin.

## 2. Key Abstractions & Seams

- **Zero third-party dependency.** One HTML template embedded as a Go
  string constant, one poller goroutine, `net/http`. Air-gapped
  deployments run it unmodified; no CDN fetches, no external assets.
- **Read-optimised snapshot.** The poller is the sole writer under a
  RWMutex; page handlers copy the snapshot and never wait on a live
  poll. A stale snapshot beats a slow page load.
- **Bounded sweep = interval.** Each refresh runs on a `context` capped
  at the poll interval, so a wedged instance never lets consecutive
  sweeps overlap.
- **Serves during startup.** Like the actuator, it does not block on
  `sig.TriggerAndWait` — the dashboard is reachable during startup so
  operators can watch the aggregate transition.
- **Partial-failure rendering.** `/info` failures are silently ignored
  (nice-to-have); only `/health` unreachable is surfaced as the row's
  `Error`. Non-2xx from `/readiness` (503 with a body) is decoded so
  the status pill still renders correctly.

## 3. Constraints

- **`Instances` is a static list.** No discovery integration — the
  target set is `spring.admin-ui.instances` (comma-separated actuator
  base URLs); this UI is for a known small fleet, not a fleet-wide
  scanner.
- **Default port `:9280`.** Configurable via `spring.admin-ui.addr`.
- **Auto-refresh minimum 1s.** Sub-second poll interval is clamped to
  the HTML `meta refresh` floor.
- **Sorted rows.** Rows are sorted by base URL so the table does not
  shuffle between refreshes.

## 4. Trade-offs / Alternatives Rejected

- **Full Spring Boot Admin reimplementation — rejected.** SBA carries
  self-registration, JMX, notifiers, journaled history — features
  either duplicated by Prometheus + Grafana in a K8s stack or worth
  building only for a large Java fleet.
- **Sourcing instances from a discovery backend — rejected for v1.**
  The intended niche is bring-up / on-prem where a discovery backend
  may itself be the thing being verified. A future extension can layer
  a discovery mode on top of the same rendering.
- **Server-Sent Events / WebSocket push — rejected.** HTML `meta
  refresh` on a 10s cadence is enough for a status table and keeps the
  whole starter to a few hundred lines of Go.
