# starter-otel Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-otel` is a Global/infrastructure-archetype starter
(`starter/DESIGN.md` §2.4). It builds the process-wide OpenTelemetry
`TracerProvider` / `MeterProvider` from `${spring.observability}` and
installs them as OTel globals so any instrumented component (starter-
gorm-*, starter-mesh, http/gRPC middlewares, ...) is wired without
per-component adaptation.

## 1. Responsibilities & Boundaries

- Owns the shared trace + metrics providers, propagator, resource
  attributes, and log-trace correlation hook.
- Wiring model is **implicit-global**, not per-bean: importing the
  starter is the opt-in; every OTel-aware library reads the globals.
- Exports the providers as beans **only** for shutdown ordering. Callers
  do not autowire them.
- Optionally contributes a Prometheus `/metrics` handler as an
  `endpoint.Endpoint`, so `starter-actuator` — if present — serves it on
  the shared management port without any cross-starter import.

## 2. Key Abstractions & Seams

- **Eager provider construction in `gs.Module`.** The module body runs
  during `applyModules` (RefreshPrepare), i.e. before any bean's
  constructor. Setting `otel.SetTracerProvider` / `SetMeterProvider`
  here guarantees a downstream `db.Use` / `otelhttp.NewHandler` observes
  live providers. Constructing lazily in a bean would break ordering.
- **Endpoint contribution seam.** With a pull (Prometheus) exporter and
  `metrics.port=0`, the starter exposes the scrape handler as an
  `endpoint.Endpoint` bean. `starter-actuator` collects every
  `endpoint.Endpoint` and mounts them on `:9370` — one scrape port for
  operators. With `metrics.port>0` a dedicated `http.Server` is used.
- **`log.FieldsFromContext` seam.** Trace-log correlation installs a
  process-wide hook that stamps every record with the active span's
  `trace_id`/`span_id`. Only one installer wins by design; otel is that
  installer here.
- **Runtime metrics as an opt-in extra.** `runtime` instrumentation is
  registered against the same MeterProvider so `mp.Shutdown` tears it
  down — no separate stop hook to manage.
- **Enable=false is a full no-op.** Left as SDK no-op providers; an
  imported-but-disabled starter has no runtime effect.

## 3. Constraints

- **One TracerProvider and one MeterProvider per process.** Globals are
  singletons; a second `starter-otel`-like installer would race.
- **Exporter `none` disables the pillar.** Same effect as `enable:
  false` but per-pillar (e.g. traces on, metrics off).
- **Prometheus exporter is pull-based.** Choose port=0 (mount via
  actuator) or port>0 (dedicated server). Both cannot be used.
- **Shutdown order matters.** Providers are registered as beans with
  `Destroy` hooks so pending spans/metrics flush before other beans
  (which may still emit) tear down.

## 4. Trade-offs / Alternatives Rejected

- **Per-component bean injection of a tracer — rejected.** Making every
  component depend on a `trace.Tracer` bean forces cross-starter imports
  and defeats the OTel global model. The point of an ecosystem-standard
  API is that libraries read globals.
- **Bundling exporters/pillars into their own starters — rejected.**
  Splitting would duplicate resource assembly and force operators to
  synchronise many enable flags. One starter, one config tree.
