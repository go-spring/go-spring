# starter-bigcache OTel Example

Surfaces BigCache statistics as OpenTelemetry gauges and scrapes them via the
Prometheus pull exporter.

## What it shows

- **Statistics gauges**: `bigcache.hits`, `bigcache.misses`, `bigcache.delete_hits`,
  `bigcache.delete_misses`, `bigcache.collisions`, `bigcache.entries`, `bigcache.capacity`, each
  labeled with `instance`.
- **Per-operation signals**: the `bigcache.operation.total` counter, labeled
  `operation` × `status` × `instance`.
- `starter-otel` serves all of it at the in-process Prometheus endpoint (`:9090/metrics`).
  No external collector is required.

The example drives `hot` (20 SET/GET hits + 5 GETs on absent keys), scrapes `/metrics`, and
asserts:

- `bigcache_hits{instance="hot"}` reads at least the 20 hits driven, i.e. the gauge is wired
  to `Stats()` and not a never-updated zero;
- the per-operation counter is present for `get` and `set`, a miss is counted as
  `status="ok"` rather than an error, and no cache key appears as a metric label.

**No spans.** The wrapper deliberately opens none: a trace is worth what its edges say, and an
in-process microsecond call has no edge. The counter and the gauges are the whole story.

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
