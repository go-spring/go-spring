# starter-actuator

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-actuator` exposes operational HTTP endpoints — health probes, build
info, and runtime introspection (loggers, environment, thread dump) — on a
dedicated management port managed by the Go-Spring IoC container. It gives
Go-Spring applications the entry points that Kubernetes probes, registry health
checks, and ops tooling expect.

Unlike the application's main HTTP server, the actuator starts serving the
moment its listener is bound. This is deliberate: a readiness probe must be able
to reach the endpoint *before* the app is ready so it can observe the
`OUT_OF_SERVICE` → `UP` transition, and a liveness probe must answer throughout
a long startup so the pod is not restarted prematurely.

## Installation

```bash
go get go-spring.org/starter-actuator
```

## Quick Start

### 1. Import the `starter-actuator` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-actuator"
```

### 2. Configure the Actuator Server

Add actuator configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.actuator.enabled=true
spring.actuator.addr=:9370
```

### 3. Access the Endpoints

```bash
curl http://127.0.0.1:9370/healthz     # liveness
curl http://127.0.0.1:9370/readyz      # readiness (aggregates health indicators)
curl http://127.0.0.1:9370/startupz    # startup probe (503 until started, then 200)
curl http://127.0.0.1:9370/info        # build/version info
curl http://127.0.0.1:9370/loggers     # configured loggers and their levels
curl http://127.0.0.1:9370/env         # merged configuration (secrets masked)
curl http://127.0.0.1:9370/threaddump  # goroutine stack dump
```

The legacy paths `/health`, `/readiness`, and `/startup` remain as aliases of
`/healthz`, `/readyz`, and `/startupz`.

Map the probes to a Kubernetes pod spec directly:

```yaml
startupProbe:
  httpGet: { path: /startupz, port: 9370 }
livenessProbe:
  httpGet: { path: /healthz, port: 9370 }
readinessProbe:
  httpGet: { path: /readyz, port: 9370 }
```

## Endpoints

The three probe endpoints map to the Kubernetes container probes. The z-suffixed
paths are canonical; the older names are kept as aliases.

| Endpoint | Method | Meaning |
| --- | --- | --- |
| `/healthz` (alias `/health`) | GET | **Liveness.** Returns `200 {"status":"UP"}` as long as the process is serving. Consults only indicators that declare the `liveness` group (usually none), so a down dependency never trips a liveness restart. |
| `/readyz` (alias `/readiness`) | GET | **Readiness.** Returns `200 {"status":"UP"}` only after the app has crossed its readiness barrier **and** every `readiness`-group indicator passes; `503` otherwise (`OUT_OF_SERVICE` before ready or while draining on shutdown, `DOWN` when a component fails). |
| `/startupz` (alias `/startup`) | GET | **Startup probe.** Returns `503 OUT_OF_SERVICE` until the app has finished starting **and** every `startup`-group indicator passes, then `200`. Unaffected by drain, so once startup succeeds the kubelet hands off to the liveness probe and a slow boot is not killed. |
| `/info` | GET | Build/version metadata read from the binary's embedded build info (module path/version, Go toolchain, and the VCS revision/time when built from a checkout). |
| `/loggers` | GET | Configured loggers with their effective levels, plus the selectable level names. The Go analogue of Spring Boot's `/actuator/loggers`. |
| `/loggers/{name}` | POST | Override a logger's level at runtime. Body `{"configuredLevel":"DEBUG"}`; use `root` for the root logger. `204` on success, `400` for an invalid level, `404` for an unknown logger. |
| `/env` | GET | Merged configuration properties as a flat property source. Secret-named keys (`password`, `token`, `secret`, ...) and `ENC(...)` values are masked. |
| `/configprops` | GET | Merged configuration as a nested tree (the Go analogue of `/actuator/configprops`), with the same masking as `/env`. |
| `/threaddump` | GET | Goroutine stack dump as `text/plain` — the Go analogue of a JVM thread dump. |
| `/metrics` | GET | Prometheus scrape endpoint. Present only when `starter-otel` is imported with `spring.observability.metrics.exporter=prometheus` — otel contributes its scrape handler and the actuator mounts it here (see *Metrics & Kubernetes Scraping*). |

### Runtime log levels

`GET /loggers` reports each configured logger and the levels you can set:

```json
{
  "levels": ["TRACE","DEBUG","INFO","WARN","ERROR","PANIC","FATAL"],
  "loggers": { "root": { "configuredLevel": "INFO" } }
}
```

Raise a logger to `DEBUG` at runtime without a restart, then restore it:

```bash
curl -X POST http://127.0.0.1:9370/loggers/root \
  -H 'Content-Type: application/json' -d '{"configuredLevel":"DEBUG"}'
```

### Secret masking (`/env`, `/configprops`)

Values are redacted to `******` when the key matches `password`, `passwd`,
`secret`, `token`, `credential`, `apikey`/`api-key`, `private-key`, or
`access-key` (case-insensitive), or when the value is an `ENC(...)` placeholder
produced by config encryption. All other values pass through unchanged.

## Graceful Shutdown (Drain)

On `SIGTERM`, the framework runs a drain sequence before stopping servers: the
actuator flips `/readyz` to `503 OUT_OF_SERVICE` (via a `PreStop` hook) while
`/healthz` and in-flight requests stay up, then waits `app.shutdown.pre-stop-delay`
so the Kubernetes endpoint controller can remove the pod from Service endpoints
before it stops accepting new traffic. This is what makes a rolling update
lossless. Servers are then stopped, bounded by `app.shutdown.timeout`.

```properties
# Wait this long after readiness flips false before stopping servers.
app.shutdown.pre-stop-delay=5s
# Optional cap on how long to wait for servers to stop (0 = wait indefinitely).
app.shutdown.timeout=30s
```

Both settings are framework-level (they apply to every server, not just the
actuator) and default to `0`, which disables the drain wait and preserves
immediate shutdown.

## Health Indicators

The probes aggregate health checks contributed by other beans. Any bean
exported as `health.Indicator` (from the zero-dependency `go-spring.org/spring/health`
package) is collected automatically — no per-component registration API and no
import of this starter:

```go
import "go-spring.org/spring/health"

type dbHealth struct{ db *sql.DB }

func (h *dbHealth) HealthName() string                    { return "mysql:orders" }
func (h *dbHealth) CheckHealth(ctx context.Context) error { return h.db.PingContext(ctx) }

// Register it as a bean exported as health.Indicator:
gs.Provide(&dbHealth{db}).Export(gs.As[health.Indicator]())
```

By default an indicator contributes to the `readiness` and `startup` groups
(never `liveness`, so a dependency check can never restart the pod). An
indicator that implements the optional `health.Grouped` interface can override
which probe groups it belongs to:

```go
func (h *dbHealth) HealthGroups() []health.Group {
    return []health.Group{health.GroupReadiness}
}
```

The failing components are listed under `components` in the `/readyz`
response so a probe failure is easy to attribute:

```json
{
  "status": "DOWN",
  "components": {
    "mysql:orders": { "status": "DOWN", "error": "dial tcp ...: connection refused" }
  }
}
```

Client starters that ship a health indicator (e.g. `starter-go-redis`) are
folded in automatically once both starters are imported.

## Metrics & Kubernetes Scraping

The actuator can also serve the Prometheus `/metrics` endpoint, so operators
scrape a **single management port** for probes *and* metrics instead of the
metrics exporter running its own server. This is opt-in through `starter-otel`:
any bean exported as `endpoint.Endpoint` (from the zero-dependency
`go-spring.org/spring/endpoint` package) is mounted on the management port, and
`starter-otel`'s Prometheus exporter contributes exactly such a bean — no import
of otel by this starter and no extra wiring:

```go
import (
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
)
```

```properties
# Serve /metrics through the actuator only (no dedicated metrics server):
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics
```

```bash
curl http://127.0.0.1:9370/metrics
```

### Pod annotation scraping

For a Prometheus server using pod-annotation discovery, point it at the
management port:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9370"
    prometheus.io/path: "/metrics"
```

### ServiceMonitor (Prometheus Operator)

Expose the management port on the Service, then select it with a
`ServiceMonitor`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
    - port: management   # the Service port that maps to 9370
      path: /metrics
      interval: 15s
```

## Configuration

| Property | Default | Description |
| --- | --- | --- |
| `spring.actuator.enabled` | `true` | Enables or disables the actuator server. |
| `spring.actuator.addr` | `:9370` | Management listen address. Binds all interfaces by default so in-cluster probes can reach it. Distinct from the main HTTP server (`:9090`) and the pprof server (`127.0.0.1:9981`). |

## License

This project is licensed under the Apache License 2.0.
