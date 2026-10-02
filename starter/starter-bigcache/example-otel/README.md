# starter-bigcache OTel Example

Surfaces BigCache statistics as OpenTelemetry gauges and scrapes them via the
Prometheus pull exporter.

## What it shows

- **Statistics gauges**: `bigcache.hits`, `bigcache.misses`, `bigcache.delete_hits`,
  `bigcache.delete_misses`, `bigcache.collisions`, `bigcache.entries`, `bigcache.capacity`, each
  labeled with `cache.name`.
- **Per-operation signals**: the `bigcache.operation.total` counter and the
  `bigcache.operation.duration` histogram, labeled `operation` × `status` × `cache.name`.
- `starter-otel` serves all of it at the in-process Prometheus endpoint (`:9090/metrics`).
  No external collector is required.

The example drives `hot` (20 SET/GET hits + 5 GETs on absent keys), scrapes `/metrics`, and
asserts:

- `bigcache_hits{cache_name="hot"}` reads at least the 20 hits driven, i.e. the gauge is wired
  to `Stats()` and not a never-updated zero;
- the per-operation counter and histogram are present for `get` and `set`, a miss is counted as
  `status="ok"` rather than an error, and no cache key appears as a metric label.

**Spans.** The wrapper opens one span per operation. They go to the built-in `stdout` exporter
(`spring.observability.trace.exporter=stdout`), so they print alongside the example's output —
read them rather than take the attribute names on trust. Swap in `otlp-grpc` plus an endpoint to
ship them to a collector instead.

## Run

```bash
cd starter-bigcache/example-otel
go run .
```

## Manual mode

```bash
go run . -manual
# then, in another terminal:
curl http://localhost:9090/metrics | grep bigcache
```

## Smoke test

```bash
./check.sh
```
