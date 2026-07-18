# starter-otel

[English](README.md) | [中文](README_CN.md)

`starter-otel` is Go-Spring's unified observability core. It defines the single,
framework-level observability configuration (`${spring.observability}`), builds
the shared OpenTelemetry `TracerProvider` / `MeterProvider`, and installs them as
the OTel process globals during startup.

Every instrumented component (gorm, and more to come) reads those globals through
its own OTel plugin, so you **configure observability once here** instead of
adapting each component. Import this starter and the whole chain lights up; leave
it out and the components fall back to OTel's no-op globals — zero overhead, no
errors.

## How It Works

```
component (e.g. gorm plugin)  ──reads──▶  OTel globals (otel.GetTracerProvider/GetMeterProvider)
                                               ▲
starter-otel  ──sets at startup───────────────┘  otel.SetTracerProvider / SetMeterProvider
```

- **Components depend on the OTel API, not on this starter.** They are decoupled
  through the OTel process globals.
- **Enablement is global and implicit.** Importing `starter-otel` installs real
  providers; omitting it leaves the no-op globals in place.
- **Timing is guaranteed.** Providers are built eagerly during module setup,
  which runs before any component bean is constructed, so a component always sees
  a live provider when it installs its plugin.

## Installation

```bash
go get go-spring.org/starter-otel
```

## Quick Start

### 1. Import the `starter-otel` Package

```go
import _ "go-spring.org/starter-otel"
```

Import an instrumented component alongside it — that is the entire wiring:

```go
import (
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-gorm-mysql"
)
```

### 2. Configure Observability

Add observability configuration in your project's configuration file, for example
exporting both traces and metrics to an OTel Collector over OTLP/gRPC:

```properties
spring.observability.enable=true
spring.observability.service-name=my-service

spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true

spring.observability.metrics.exporter=otlp-grpc
spring.observability.metrics.endpoint=127.0.0.1:4317
spring.observability.metrics.insecure=true
```

That is all. Any instrumented component you import now emits spans and metrics
through these providers.

## Built-in Exporters

The exporters below are compiled into `starter-otel`; you select one per signal
via the `exporter` key — no extra dependency or code is needed.

Trace (`spring.observability.trace.exporter`):

| Value | Backend | Notes |
| --- | --- | --- |
| `otlp-grpc` | OTLP over gRPC | Default. Send to a Collector / OTLP-native backend on `endpoint` (`:4317`). |
| `otlp-http` | OTLP over HTTP | Same as above over HTTP (`:4318`). |
| `stdout` | Standard output | Prints spans as JSON; handy for local debugging. |
| `none` | (disabled) | Builds no `TracerProvider`. |

Metrics (`spring.observability.metrics.exporter`):

| Value | Backend | Notes |
| --- | --- | --- |
| `otlp-grpc` | OTLP over gRPC | Default. Push to a Collector on `endpoint` (`:4317`) every `interval`. |
| `otlp-http` | OTLP over HTTP | Same as above over HTTP (`:4318`). |
| `prometheus` | Prometheus (pull) | Serves a `/metrics` scrape endpoint. On a standalone server on `port`, and — when `starter-actuator` is present — also on its management port (see [Metrics via the Actuator](#metrics-via-the-actuator)). |
| `stdout` | Standard output | Prints metrics as JSON every `interval`. |
| `none` | (disabled) | Builds no `MeterProvider`. |

To reach a backend not listed here, keep `otlp-grpc`/`otlp-http` and let an
OpenTelemetry Collector translate/route to it — see
[Connecting Multiple Backends](#connecting-multiple-backends).

## Configuration Reference

All keys live under `${spring.observability}`.

| Key | Default | Description |
| --- | --- | --- |
| `enable` | `true` | Master switch; when `false` the starter installs nothing. |
| `service-name` | `${spring.application.name:=go-spring-app}` | `service.name` resource attribute. |

Trace, under `${spring.observability.trace}`:

| Key | Default | Description |
| --- | --- | --- |
| `enable` | `true` | Enable the shared `TracerProvider`. |
| `exporter` | `otlp-grpc` | `otlp-grpc` \| `otlp-http` \| `stdout` \| `none`. |
| `endpoint` | (empty) | Collector address; required for the otlp exporters. |
| `insecure` | `true` | Disable TLS for the otlp exporters. |
| `sampler-ratio` | `1.0` | ParentBased ratio sampler (`>=1` always, `<=0` never). |
| `propagator` | `w3c` | `w3c` (TraceContext + Baggage) \| `none`. |

Metrics, under `${spring.observability.metrics}`:

| Key | Default | Description |
| --- | --- | --- |
| `enable` | `true` | Enable the shared `MeterProvider`. |
| `exporter` | `otlp-grpc` | `otlp-grpc` \| `otlp-http` \| `prometheus` \| `stdout` \| `none`. |
| `endpoint` | (empty) | Collector address; required for the otlp exporters. |
| `insecure` | `true` | Disable TLS for the otlp exporters. |
| `port` | `9090` | Port of the standalone `/metrics` server (prometheus exporter). Set to `0` to skip the standalone server and serve `/metrics` **only** through the actuator management port (see [Metrics via the Actuator](#metrics-via-the-actuator)). |
| `path` | `/metrics` | Path of the prometheus scrape endpoint (used by both the standalone server and the actuator mount). |
| `interval` | `10s` | Push interval for the otlp/stdout readers. |

## Metrics via the Actuator

When `starter-actuator` is also imported, the Prometheus exporter's scrape
handler is mounted on the actuator management port (`:9370`) in addition to — or
instead of — its own standalone server. Operators then scrape a **single port**
for probes *and* metrics.

This works through the zero-dependency `go-spring.org/stdlib/endpoint` seam:
`starter-otel` exports its scrape handler as an `endpoint.Endpoint` bean, and the
actuator mounts every such bean on its management server. Neither starter imports
the other.

```go
import (
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-actuator"
)
```

```properties
# Serve /metrics only through the actuator (no standalone metrics server):
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics
```

```bash
curl http://127.0.0.1:9370/metrics
```

Leaving `port` at a non-zero value keeps both the standalone server and the
actuator mount active. See the actuator README's *Metrics & Kubernetes Scraping*
section for Pod-annotation and `ServiceMonitor` scrape samples.

## Trace ↔ Log Correlation

When tracing is enabled, `starter-otel` installs a `log.FieldsFromContext` hook
that lifts the active span's `trace_id` and `span_id` off the context onto every
log record written with that context. A request's logs are then joined to its
trace with no per-call-site code:

```go
log.Infof(ctx, tag, "handling request")
// → ...||trace_id=29b84b8a2f4eaa447120bf01282c3930||span_id=bc6627138bf150f7||msg=handling request
```

The hook is a no-op when the context carries no valid span, so uninstrumented
code paths pay nothing.

## Connecting Multiple Backends

Your application always exports to a **single** OTLP endpoint — typically an
OpenTelemetry Collector. Fan-out to multiple backends is the Collector's job, not
the application's:

```
app (starter-otel) ──OTLP──▶ Collector ──┬─▶ Jaeger / Tempo   (traces)
                                          ├─▶ Prometheus       (metrics)
                                          └─▶ Loki / ES        (logs)
```

Adding or swapping a backend only changes the Collector config; your application
code and properties stay untouched. When a backend natively ingests OTLP (e.g.
Jaeger on `:4317`) you may point `endpoint` at it directly; for Prometheus use
`exporter=prometheus` to expose a scrape endpoint on `port`.

## Example

See [example](example) for a runnable, self-asserting smoke test: importing
`starter-otel` + `starter-actuator` and configuring `${spring.observability}`
once is enough to (1) correlate a request's logs to its trace via `trace_id` and
(2) serve otel's Prometheus `/metrics` on the actuator management port — with no
external services (tracing uses the stdout exporter, metrics are scraped
in-process).

See [contrib/observability-gorm](../../contrib/observability-gorm) for an
end-to-end example that adds an instrumented component (`starter-gorm-mysql`):
GORM query spans and connection-pool metrics reach the Collector with no
per-component instrumentation code.

## Graceful Shutdown

The providers are registered as beans with destroy hooks, so on shutdown the
buffered spans and metrics are flushed and the exporters are closed cleanly.
