# starter-bigcache Example

In-process hot cache for starter-bigcache.

## Features

The example is a self-asserting program: `runTest` in [main.go](main.go) walks these in
order and exits non-zero on the first failure.

- **SET/GET**: write entries with `Set` and read them back with `Get`.
- **DELETE + miss**: a deleted key reads back as `ErrEntryNotFound`.
- **Instance isolation**: a key written to one named instance is invisible through another.
- **Hit/miss statistics**: `stats-enabled` is on, so `Stats()` reports the counters.
- **Cache abstraction**: the same `hot` instance injected as a `*cache.Cache` bean
  (`bigcache:hot`) — JSON-encoded values, a miss as `cache.ErrMiss`, and a demonstration that
  the per-call TTL is **ignored** (BigCache expires by the instance's `life-window` alone).
- **Eviction under a hard cap**: overflowing the `evict` instance drops the oldest entries, and
  the assertion covers both sides — fewer entries resident than were written, **and** the
  eviction hook has fired.
- **A custom Driver**: [driver.go](driver.go) registers one Driver bean, selected by
  `spring.bigcache.default.driver=hook`. It owns the assembly so it can reach
  `bigcache.Config.OnRemove` — the one thing the starter does not bind, and what makes the
  eviction assertion above possible.
- **Inherited configuration**: `spring.bigcache.default.*` holds the keys every instance shares,
  so `hot` overrides only its `life-window`.
- **The cache surface beyond Get/Set/Delete**: `Len`, `Iterator` and `Reset`.
- **What `life-window` does**: two instances share a 1s window and differ only in `clean-window`.
  After waiting past the window, one has stopped serving the stale entry and the other still
  serves it — because `Get` never checks age and only the cleaner removes an entry.
- **HTTP endpoints**: `/get` and `/set` against the `hot` instance — driven and asserted by
  `runTest`, not merely documented.

> There is deliberately no TTL-expiry feature. BigCache has no per-entry TTL, so the
> abstraction's `ttlSeconds` is documented as ignored rather than demonstrated as working; the
> feature above shows the opposite outcome on purpose.

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-bigcache/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://127.0.0.1:9090/get
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.