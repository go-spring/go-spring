# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` holds the rate-limit counting algorithms shared by the
`go-spring.org/cloud` modules — the resilience stage
([cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience/)) and any
process-local limiter (the gateway's memory counters, e.g.). The counting code
is dependency-free. A rule names the **algorithm** to count with; a [Factory]
builds the counter for one scope; a [Limiter] is that counter. All counters are
safe for concurrent use.

## Algorithms

### TokenBucket — the default

`token_bucket.go`, a port of Guava's `SmoothRateLimiter`. Admits at
`rate-limit`/s with continuous smoothing: each charge is granted at the head of
a time-axis queue and its cost is charged to whoever comes next, so the actual
pace converges on the rate without window edges.

- **`rate-limit`** — sustained budget, units per second. The only required
  knob; 0 disables the stage.
- **`burst`** — free stockpile on top of the pace, spent before queueing.
  Starts empty and regrows only while idle. Default: about one second's worth
  of the rate. Set it to the size of spike you want absorbed without waiting.
- **`rate-limit-max-wait`** — how long a charge may queue for its turn. 0
  refuses instead of waiting (pure rejection); a positive value turns bursts
  into bounded waits (traffic shaping). Pair it with the caller's real latency
  budget.
- **`rate-limit-warm-up-duration`** — cold-start protection (Guava
  `SmoothWarmingUp`): the bucket starts full and cold, and while the stockpile
  sits above the warm threshold permits cost up to `warm-up-cold-factor` (default 3)
  times their steady price, ramping down as traffic drains the stockpile.
  Enable it when a freshly restarted or long-idle process must not hit a cold
  downstream (connection pool, database, peer service) at full quota. Capacity
  is derived from the period; `burst` is ignored while warm-up is set.

**When to choose it:** almost always. It is the only one of the three that
smooths instead of capping, and the only one with burst tolerance and warm-up.

### SlidingWindow — a hard cap per rolling window

`sliding_window.go`. Up to `rate-limit × window` units per rolling `window`,
estimated by blending the previous window by its remaining overlap. No
timestamp per event — just two counts.

- **`rate-limit`** with **`window`** (default 1s) — the cap is
  `rate-limit × window`; e.g. 100/s over a 10s window = 1000 per rolling 10s.
- The estimate assumes events were spread evenly, so counts right after a
  boundary can over- or under-shoot slightly.

**When to choose it:** when you want "no more than N per window" as a hard
budget (quota accounting) rather than a smoothed pace.

### FixedWindow — the cheapest cap

`fixed_window.go`. Up to `rate-limit × window` per ALIGNED window whose
boundaries fall on clock multiples, so budgets align across instances without
coordination. One count.

- Same knobs as SlidingWindow. The weakness is the boundary burst: a client
  can spend the whole cap at the end of one window and again at the start of
  the next — up to 2x the cap in a near-instant.

**When to choose it:** aligned per-window caps where a boundary burst is
acceptable and state must be minimal.

## Usage

```go
// factory: the container's Factory — the process-local provider plus any store
// the wiring contributed
lim, err := factory.NewLimiter("token-bucket", "checkout") // or "sliding-window", "fixed-window", "redis:token-bucket"
ok, err := lim.Allow(ctx, ratelimit.Policy{RateLimit: 100, Burst: 200}, 1)
```

Invalid configuration is rejected at the boundary: the `Policy` fields carry
their own `expr` bounds, so `conf.Bind` refuses negative rates/bursts/windows;
a rule naming an algorithm no provider exports is rejected when the document is
applied.

Design notes live in the code comments — the provider seam in `factory.go`,
the queue model in `token_bucket.go`.
