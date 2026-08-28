# starter-admin-ui

[English](README.md) | [中文](README_CN.md)

> A lightweight, self-contained "Spring Boot Admin equivalent" for Go-Spring:
> a small HTML dashboard that polls the `starter-actuator` endpoints of a list
> of application instances and renders their aggregated status.

`starter-admin-ui` self-hosts a dedicated HTTP server (activated by configuring
`spring.admin-ui.addr`; no server is started without it) and
runs a background poller that periodically fetches `/health`, `/readiness`,
`/startup`, and `/info` from each configured instance. The dashboard renders
one row per instance with color-coded status pills, per-component health, and
build metadata.

## Positioning — when to use this

In a Kubernetes deployment, the standard aggregate-monitoring story is
**Prometheus + Grafana** (see `contrib/observability/` in this repo). That
stack gives you time-series history, alerting, and rich dashboards, and it is
the recommended default. This starter is deliberately narrow in scope; it is
useful when:

- Prometheus/Grafana is unavailable or overkill for the deployment.
- You want a single-page, at-a-glance view of a handful of pods on-prem.
- You are bringing up a new environment and want a quick sanity dashboard
  before wiring up the full observability stack.

This is not a feature-for-feature reimplementation of Spring Boot Admin; it
gives you the equivalent effect (poll actuator, render a status table) using
idiomatic Go — one HTML page, one poller goroutine, zero third-party deps.
See [DECISION.md](DECISION.md) for the reasoning in more depth.

## Installation

```bash
go get go-spring.org/starter-admin-ui
```

## Quick Start

### 1. Import the package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-admin-ui"
```

### 2. Configure the dashboard

Add settings to your project's [configuration file](example/conf/app.properties):

```properties
# Setting addr is what activates the starter (there is no enabled key).
spring.admin-ui.addr=:9280
spring.admin-ui.instances[0]=http://10.0.0.1:9370
spring.admin-ui.instances[1]=http://10.0.0.2:9370
spring.admin-ui.interval=10s
spring.admin-ui.timeout=3s
spring.admin-ui.title=Go-Spring Admin

# Optional auth (either token, or username+password). Recommended whenever
# addr binds a non-loopback interface; without it startup logs a warning.
# spring.admin-ui.token=s3cret
# spring.admin-ui.username=admin
# spring.admin-ui.password=pw
```

> Migration: earlier releases started the dashboard by default on `:9280`
> (`spring.admin-ui.enabled=true` with match-if-missing). If you relied on
> that, add `spring.admin-ui.addr=:9280`; the `enabled` key is no longer read.

Each `instances[i]` entry is a base URL — the UI appends `/health`,
`/readiness`, `/startup`, `/info` when polling. If `instances` is empty the
dashboard still serves and shows a "No instances configured" state.

### 3. Open the dashboard

```bash
open http://127.0.0.1:9280/
```

The page auto-refreshes every `spring.admin-ui.interval` via a
`<meta http-equiv="refresh">` tag. There is no CDN or external asset fetch;
the page is a single self-contained HTML document rendered from an embedded
template.

## Endpoints

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/` | GET | The HTML dashboard: aggregated status table with color-coded pills. |
| `/api/status` | GET | The same snapshot as JSON, for scripts and integrations. |

Example JSON payload:

```json
{
  "polled_at": "2026-07-19T09:45:14Z",
  "instances": [
    {
      "base": "http://10.0.0.1:9370",
      "health": "UP",
      "readiness": "UP",
      "startup": "UP",
      "components": [{ "name": "mysql:orders", "status": "UP" }],
      "go": "go1.26",
      "module": "example.com/orders",
      "version": "v1.2.3",
      "revision": "deadbeef",
      "build_time": "2026-07-19T00:00:00Z"
    }
  ]
}
```

## How it works

- A single background goroutine wakes on `spring.admin-ui.interval`.
- Each tick polls every instance in parallel, bounded by
  `spring.admin-ui.timeout` per HTTP call.
- Results are stored atomically as a snapshot behind an `RWMutex`; the HTTP
  handler renders that snapshot without ever blocking on live polls, so page
  loads stay fast regardless of instance health.
- Unreachable instances show up as `DOWN` with the underlying error, not as
  "server error" — a broken pod never breaks the dashboard.
- `/readiness` legitimately returns `503` when a component is down; the
  poller keeps the body and renders the `OUT_OF_SERVICE` / `DOWN` state
  correctly.

## Graceful shutdown

The starter participates in the framework's server lifecycle: `Stop()` closes
the poller's stop channel, waits for the sweep goroutine to exit, then calls
`http.Server.Shutdown`. It has no `PreStop` hook — the dashboard is an
operator tool, not a probe target, so there is nothing to flip on drain.

## Configuration

| Property | Default | Description |
| --- | --- | --- |
| `spring.admin-ui.addr` | *(none — required)* | Listen address; setting it activates the admin-ui server (no `enabled` key). Distinct from the main HTTP server (`:9090`), the actuator (`:9370`), and pprof (`:9981`). |
| `spring.admin-ui.token` | *(empty)* | When set, requires `Authorization: Bearer <token>` on every request. Takes precedence over username/password. |
| `spring.admin-ui.username` / `spring.admin-ui.password` | *(empty)* | When both set, require HTTP Basic auth. Without any credentials on a non-loopback addr, startup logs a warning. |
| `spring.admin-ui.instances` | *(empty)* | Comma-separated / indexed list of actuator base URLs to poll. |
| `spring.admin-ui.interval` | `10s` | Poll cadence; also drives the page's auto-refresh. |
| `spring.admin-ui.timeout` | `3s` | Per-request HTTP timeout when polling one endpoint on one instance. |
| `spring.admin-ui.title` | `Go-Spring Admin` | Dashboard title. Handy for labelling per-environment dashboards. |

## License

This project is licensed under the Apache License 2.0.
