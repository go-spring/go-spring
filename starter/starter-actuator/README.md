# starter-actuator

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

Give your service an ops port: Kubernetes liveness/readiness/startup probes, build
info, and (if you use starter-otel) Prometheus `/metrics` — all on one port that is
separate from your business port. Import the package, set one config key, done.

One thing is unusual on purpose: this server starts answering requests **before**
your app finishes starting. K8s needs that — a readiness probe must see the
"not ready → ready" flip, and a liveness probe must keep answering during a slow
boot so the pod is not restarted for no reason.

## Installation

```bash
go get go-spring.org/starter-actuator
```

## Quick Start

### 1. Import the package

See [example.go](example/example.go).

```go
import _ "go-spring.org/starter-actuator"
```

### 2. Turn it on

Add one key to your [config file](example/conf/app.properties):

```properties
spring.actuator.addr=:9370
# Optional: require a token on the whole port. If you bind a non-loopback
# address without any auth, startup logs a WARN.
# spring.actuator.token=s3cret
```

No `addr`, no actuator — that key is the switch.

### 3. Probe it

```bash
curl http://127.0.0.1:9370/healthz     # is the process alive?
curl http://127.0.0.1:9370/readyz      # can it take traffic?
curl http://127.0.0.1:9370/startupz    # has it finished starting?
curl http://127.0.0.1:9370/info        # what build is this?
```

`/health`, `/readiness`, `/startup` work too — same endpoints, older names.

Wire it straight into a pod spec:

```yaml
startupProbe:
  httpGet: { path: /startupz, port: 9370 }
livenessProbe:
  httpGet: { path: /healthz, port: 9370 }
readinessProbe:
  httpGet: { path: /readyz, port: 9370 }
```

## Endpoints

| Endpoint | What it answers |
| --- | --- |
| `/healthz` (alias `/health`) | `200` while the process is up. It deliberately does **not** check dependencies — a dead database should drain traffic, not restart the pod. |
| `/readyz` (alias `/readiness`) | `200` only when the app is up **and** every critical health indicator passes. `503` before startup finishes, while shutting down, or when a critical dependency is down. If only non-critical indicators fail: `200` with status `DEGRADED` — still serving, problem visible in the body. |
| `/startupz` (alias `/startup`) | `503` until the app has finished starting, then `200`. This is what lets a slow boot survive: K8s retries this probe instead of killing the pod on liveness. |
| `/info` | Version info compiled into the binary (module path/version, Go version, git revision/time when built from a checkout). |
| `/metrics` | Not served by this starter. If you import `starter-otel` with the Prometheus exporter, its scrape handler is mounted here — same port, one thing to monitor. |

Health indicators are just beans: export anything as `health.Indicator` (a redis
client, a db pool) and it is automatically folded into `/readyz`. No wiring, no
import of this starter from your component. See the [example](example/example.go).

### Metrics mounting

To put `/metrics` on this port, configure `starter-otel`:

```properties
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # no separate metrics server, actuator only
```

### Prometheus discovery

Pod annotation:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9370"
    prometheus.io/path: "/metrics"
```

Or expose the port on a Service and select it with a `ServiceMonitor`:

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

| Property | Default | What it does |
| --- | --- | --- |
| `spring.actuator.addr` | — | Listen address. Required — setting it is what turns the starter on. `:9370` binds all interfaces so in-cluster probes can reach it; keep it separate from your business port (`:9090`) and pprof (`127.0.0.1:9981`). |
| `spring.actuator.endpoints.include` | `""` | Endpoint whitelist, comma-separated paths (`/info,/metrics`). Empty = default set (`/info` + probes + contributed endpoints). Non-empty = only what you list. Probes are never filtered. An endpoint marked `Sensitive` by its contributor only registers when listed here. |
| `spring.actuator.token` | `""` | Require `Authorization: Bearer <token>` on the whole port. Wins over Basic. |
| `spring.actuator.username` / `spring.actuator.password` | `""` | Require HTTP Basic (both must be set). No auth at all + non-loopback address → startup WARN. |

## License

This project is licensed under the Apache License 2.0.
