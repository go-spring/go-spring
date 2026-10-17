# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` holds the rate-limit counting algorithms shared by the
`go-spring.org/cloud` modules — the resilience stage
([cloud/runtime/dataplane/resilience](../resilience/)) and any process-local
`resilience.ClusterLimiter` store (the gateway's memory counters, e.g.). It is a
leaf package: nothing beyond the standard library (plus
`stdlib/timeutil` for ctx-aware sleeping), so callers pull in no resilience
semantics to count with them.

All counters are safe for concurrent use.

## Algorithms

One file per algorithm; each trades smoothness for state:

| Algorithm | File | Admits | Smoothness | State | Notes |
| --- | --- | --- | --- | --- | --- |
| `TokenBucket` | `token_bucket.go` | up to `burst` instantly, then at `rate`/s | full (continuous refill) | 4 numbers | the default; `WithWarmUp` ramps capacity in from `burst/coldFactor` (sentinel-style cold start); `WaitN` queues up to a deadline |
| `SlidingWindow` | `sliding_window.go` | up to `limit` per rolling `window` | windowed (weighted estimate) | 2 counts | blends the previous window by overlap; no timestamp per event |
| `FixedWindow` | `fixed_window.go` | up to `limit` per ALIGNED window | boundary burst up to 2x | 1 count | window boundaries fall on clock multiples, so budgets align across instances without coordination |
| `LeakyBucket` | `leaky_bucket.go` | while `capacity` units fit alongside a constant drain | smooth outflow | 2 numbers | the token bucket's dual: bounds by drain, rejects rather than queues |
| `GCRA` | `gcra.go` | at `rate`/s with `burst` tolerance | full (virtual scheduling) | 1 timestamp | the token bucket as one theoretical-arrival-time — the shape that ports to an atomic store script (Redis Lua) with the least per-key state |

Rules of thumb:

- Default to `TokenBucket` (it is what `resilience`'s `token-bucket`
  algorithm runs on, warm-up included).
- Need a hard per-window cap? `SlidingWindow`; if a boundary burst is
  acceptable and state matters, `FixedWindow`.
- Need strict constant outflow or a store script with one number per key?
  `LeakyBucket` / `GCRA` respectively.

## Installation

```
go get go-spring.org/cloud
```

## Usage

```go
import "go-spring.org/cloud/runtime/dataplane/ratelimit"

// a budget of 100/s, bursting to 200, warming up over 30s
b := ratelimit.NewTokenBucket(100, 200).WithWarmUp(30*time.Second, 3)
if !b.AllowN(1) {
    // over budget — reject now
}

// the queueing form: wait up to 50ms for a token before rejecting
ok := b.WaitN(ctx, 1, 50*time.Millisecond)
```

A hard per-second cap:

```go
w := ratelimit.NewSlidingWindow(100, time.Second)
if !w.AllowN(1) { /* over the cap */ }
```

## Design

- **Leaf package, no interface.** The counters' method sets deliberately
  differ (`TokenBucket.AllowN(float64)` + `WaitN`, the windows'
  `AllowN(int)`); every consumer today picks the concrete type at
  construction time (resilience's `newRateState`, the gateway's
  `scopeState`), so a shared interface would only flatten that difference
  away. If a polymorphic caller appears, it declares the one-method
  interface it needs where it uses it.
- **One file per algorithm** (plus `doc.go`), tests beside each — the same
  layout convention as the rest of `cloud/`.
- **resilience's `Algorithm` policy knob** currently wires only
  `token-bucket` and `sliding-window`; `FixedWindow`, `LeakyBucket` and
  `GCRA` are package capabilities held in reserve, not yet executor stages.
