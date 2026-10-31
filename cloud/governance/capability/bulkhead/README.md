# bulkhead

[English](README.md) | [中文](README_CN.md)

`bulkhead` is a concurrency-isolation stage: it bounds how many callers may hold
a slot at once. It depends only on the standard library and works for both
inbound requests and outbound calls.

## Principles

It protects against slowness, not failures. A circuit breaker decides *whether*
to call; a bulkhead decides *how many callers may be in flight*. A rate limit
caps *how many per second*; a bulkhead caps *how many at once*. When a
dependency slows down, in-flight requests grow linearly — a rate limit cannot
stop that, only a concurrency bound can.

One `Bulkhead` protects one unit and lives as long as that unit. The unit is
not a code module but **the group of calls that fails together**: cut along
fault-propagation paths, not along code structure. Two downstream services
sharing a bulkhead are not isolated at all. Never create one per request — it
isolates nothing and merely measures concurrency.

`Acquire` takes a slot and the caller releases it exactly once when the work
settles; the slot represents one in-flight call. A full bulkhead refuses by
default; pass `WithQueueTimeout` to allow a bounded wait instead. Queueing turns
overload into latency, so it is off by default.

## Best practices

- **Granularity**: one compartment per downstream service by default. Cutting
  per method is the classic Hystrix-era mistake — dozens of compartments of ten
  slots each reject requests while total concurrency is plenty. A handful of
  compartments per service is normal; past twenty or so, merge.
- **Lifetime**: build it at wiring time and follow configuration generations.
  The admission engine builds one per server instance and rebuilds only when a
  policy generation changes.
- **Capacity**: `maxConcurrent` is the caller's budget, not the downstream's.
  Core paths get the lion's share, edge dependencies a small compartment;
  compartments may sum past the total, but the core one must be exclusive.
- **Queueing**: a coarse self-protective compartment queues briefly (10–100ms)
  to shave peaks; a fine isolating one does not queue, so failures reach the
  breaker quickly.
- **Scope key**: bulkhead, breaker and rate limit must partition on the same
  key, or a breaker keyed by service while the bulkhead is keyed by method
  leaves a seam where the circuit is open yet slots are still consumed.
- **Placement**: acquire as early as possible, release as late as possible; the
  slot covers the handler, downstream calls and every retry. A refusal from an
  inner stage must release the outer slot.

The admission engine orders its stages this way — bulkhead first, then rate
limit, then the breaker gate, then fault injection — with every later refusal
releasing the bulkhead slot and the final `Done` returning it:

```go
release, ok := bulk.Acquire(ctx)          // 1. outermost: bound concurrency
if !ok {
    return refused(ErrBulkheadFull, bulk.QueueTimeout())
}
if !limiter.Allow(ctx) {                  // 2. budget the request
    release()
    return refused(ErrRateLimited, 0)
}
permit, ok := breaker.Allow()             // 3. gate on the circuit
if !ok {
    release()
    return refused(ErrCircuitOpen, 0)
}
done := func(outcome error) {             // 4. release when the work settles
    breaker.Record(permit, outcome == nil, elapsed)
    release()
}
```

A client-side middleware over one downstream dependency has the same shape:
acquire before the first attempt, hold across every retry, release when the
operation settles.

## Usage

```go
b := bulkhead.New(maxConcurrent) // fail fast: a full caller is refused at once
// Alternative: give a full caller a short window to wait for a slot.
// b := bulkhead.New(maxConcurrent, bulkhead.WithQueueTimeout(queueTimeout))

release, ok := b.Acquire(ctx)
if !ok {
    return ErrBulkheadFull // refused; QueueTimeout() is the retry-after hint
}
defer release()
err := doWork()
```

- `New(maxConcurrent, opts...)` panics on `maxConcurrent <= 0`: "no isolation"
  is a nil `*Bulkhead` at the holder, never a disabled instance, and an
  unconfigured policy keeps nil.
- Fail fast is the default; `WithQueueTimeout(queueTimeout)` lets a caller wait
  up to `queueTimeout` for a slot.
- `Acquire` returns the release function; a refusal consumes nothing and its
  release is a no-op. A successful acquisition and its release are a pair the
  caller owns: the release must run **exactly once**. Releasing twice gives back
  a slot this caller does not hold, so capacity drops by one for good; skipping
  the release pins the slot for good. Neither is detectable inside the
  bulkhead, so exact pairing is the contract, not a courtesy.
- While queued, the caller is bounded by both its `ctx` and the queue time;
  whichever ends first returns `ok == false`. A `ctx` that is already done when
  `Acquire` is called is refused before a slot is taken.
- `QueueTimeout()` reports the queue time, for an adapter to surface as a
  retry-after hint.
- Waiting callers are not strictly FIFO, but the bound itself is never
  exceeded.
