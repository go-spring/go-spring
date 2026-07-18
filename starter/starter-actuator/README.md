# starter-actuator

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-actuator` exposes operational HTTP endpoints — liveness, readiness, and
build info — on a dedicated management port managed by the Go-Spring IoC
container. It gives Go-Spring applications the entry points that Kubernetes
probes, registry health checks, and ops tooling expect.

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
curl http://127.0.0.1:9370/health      # liveness
curl http://127.0.0.1:9370/readiness   # readiness (aggregates health indicators)
curl http://127.0.0.1:9370/info        # build/version info
```

Map them to a Kubernetes pod spec directly:

```yaml
livenessProbe:
  httpGet: { path: /health, port: 9370 }
readinessProbe:
  httpGet: { path: /readiness, port: 9370 }
```

## Endpoints

| Endpoint | Method | Meaning |
| --- | --- | --- |
| `/health` | GET | Liveness. Returns `200 {"status":"UP"}` once the process is serving. It reflects that the process is up, **not** dependency health — a down database never trips a liveness restart. |
| `/readiness` | GET | Readiness. Returns `200 {"status":"UP"}` only after the app has crossed its readiness barrier **and** every registered health indicator passes; `503` otherwise (`OUT_OF_SERVICE` before ready, `DOWN` when a component fails). |
| `/info` | GET | Build/version metadata read from the binary's embedded build info (module path/version, Go toolchain, and the VCS revision/time when built from a checkout). |

## Health Indicators

`/readiness` aggregates health checks contributed by other beans. Any bean
exported as `health.Indicator` (from the zero-dependency `go-spring.org/stdlib/health`
package) is collected automatically — no per-component registration API and no
import of this starter:

```go
import "go-spring.org/stdlib/health"

type dbHealth struct{ db *sql.DB }

func (h *dbHealth) HealthName() string                    { return "mysql:orders" }
func (h *dbHealth) CheckHealth(ctx context.Context) error { return h.db.PingContext(ctx) }

// Register it as a bean exported as health.Indicator:
gs.Provide(&dbHealth{db}).Export(gs.As[health.Indicator]())
```

The failing components are listed under `components` in the `/readiness`
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

## Configuration

| Property | Default | Description |
| --- | --- | --- |
| `spring.actuator.enabled` | `true` | Enables or disables the actuator server. |
| `spring.actuator.addr` | `:9370` | Management listen address. Binds all interfaces by default so in-cluster probes can reach it. Distinct from the main HTTP server (`:9090`) and the pprof server (`127.0.0.1:9981`). |

## License

This project is licensed under the Apache License 2.0.
