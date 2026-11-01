# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` holds the rate-limit counting algorithms shared by the
`go-spring.org/cloud` modules — the resilience stage
([cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience/)) and any
process-local limiter (the gateway's memory counters, e.g.). The counting code
is dependency-free — nothing beyond the standard library (plus
`stdlib/timeutil` for ctx-aware sleeping) — so callers pull in no resilience
semantics to count with it; `starter.go` only contributes the built-in provider
bean to the container.

A rule names the **algorithm** to count with — a bean name such as
`token-bucket` (built in, process local) or `redis:token-bucket` (a shared
store). A [Factory] creates the
counter for one scope; a [Limiter] is that counter.

All counters are safe for concurrent use.

## Algorithms

One file per algorithm; each trades how evenly it admits for how much state
it keeps:

| Algorithm | File | Admits | Smoothness | State | Notes |
| --- | --- | --- | --- | --- | --- |
| `TokenBucket` | `token_bucket.go` | up to `burst` instantly, then at `rate`/s | full (continuous refill) | 4 numbers | the default; `rate-limit-warm-up-duration` ramps capacity in from `burst/coldFactor` (sentinel-style cold start); an over-limit charge is reserved up front and slept once up to a deadline, refusing `n > burst` immediately |
| `SlidingWindow` | `sliding_window.go` | up to `limit` per rolling `window` | windowed (weighted estimate) | 2 counts | blends the previous window by overlap; no timestamp per event |
| `FixedWindow` | `fixed_window.go` | up to `limit` per ALIGNED window | boundary burst up to 2x | 1 count | window boundaries fall on clock multiples, so budgets align across instances without coordination |

Rules of thumb:

- Default to `TokenBucket` (it is what `resilience`'s `token-bucket`
  algorithm runs on, warm-up included).
- Need a hard per-window cap instead of a continuously refilled budget?
  `SlidingWindow`; if a boundary burst is acceptable and the smallest possible
  state matters, `FixedWindow`.

## Installation

```
go get go-spring.org/cloud
```

## Usage

Wire the providers as beans; the container's `ratelimit.Factory` collects them.
A rule's `limiter` key names the algorithm to create:

```go
// factory: the container's Factory — the process-local provider plus any store
// the wiring contributed
lim, err := factory.Create("token-bucket", "checkout") // or "sliding-window", "fixed-window", "redis:token-bucket"
ok, err := lim.Allow(ctx, ratelimit.Policy{RateLimit: 100, Burst: 200}, 1)
```

## Design

- **One call shape, two state locations.** A [Provider] builds the [Limiter] for
  one algorithm and scope, and that limiter takes `Allow(ctx, policy, n)`: a
  process-local provider keeps its state in memory, a shared store keeps it
  outside the process. The rule travels with every call, so a configuration
  change updates the counter in place instead of rebuilding it.
- **Each provider exports the algorithms it answers.** `Provider.Algorithms()` is that
  set: the process-local provider answers the plain algorithm names, a store adds
  its own prefix (`redis:token-bucket`). `Factory` keeps the collected
  providers and, on `Create`, scans them for the one exporting the requested
  algorithm — there is no separate registry to keep in sync. The factory is a
  REQUIRED dependency of both directions. A rule naming an algorithm no provider
  exports is rejected when the document is applied.
- **A process-local counter is bound to one algorithm.** The algorithm is fixed
  by the process-local provider's `New`; the state is created from the first
  call's `Policy`. A numeric change updates it in place; only a sliding window's
  width change starts fresh state.
- **One file per algorithm**, with `factory.go` (the algorithm names, `Limiter`,
  the `Provider` seam and the process-local provider) and `policy.go` beside
  them; tests beside each — the same layout convention as the
  rest of `cloud/`.
- **Invalid configuration is rejected at the boundary.** The policy fields
  carry their own `expr` bounds, so `conf.Bind` refuses negative rates/bursts/
  windows; a programmatic caller that builds a `Policy` vouches for its values.
- **Time is faked in tests, not in production.** The counters call `time.Now`
  and `timeutil.Sleep` directly; the suite runs them inside `testing/synctest`,
  where time jumps instead of being waited out.
- **A rule's `limiter` knob** resolves `token-bucket`, `sliding-window` and
  `fixed-window`. Those are the process-local algorithms; an algorithm without a
  consumer does not belong in the package.
