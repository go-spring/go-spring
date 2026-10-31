# circuitbreaker

[English](README.md) | [中文](README_CN.md)

`circuitbreaker` is the **direction-neutral circuit breaker**: the open /
half-open / closed state machine, without any direction's policy.

The outbound fault-tolerance engine
([outbound/resilience](../../dataplane/outbound/resilience/)) consumes it today; the state
machine stays in capability/ because it is a mechanism, not a direction's policy.
A caller assembles a [`Spec`](#spec) from its own policy, and nothing here knows
whether the protected unit is a call or a request.

The package is a leaf: it depends on the standard library only.

## How it works

A failing dependency gets worse when callers keep hammering it: each new call
ties up a caller thread, a connection and a timeout that the dependency cannot
serve, which turns one slow component into a cascading outage. A circuit
breaker cuts that feedback loop by failing fast: once its strategy decides the
unit is unhealthy, it stops letting calls through, so the dependency gets quiet
time to recover and callers get an immediate, cheap error instead of an
expensive timeout.

It is a three-state machine:

```
Closed ──threshold reached──▶ Open ──cool-down elapsed──▶ HalfOpen
   ▲                             │                          │
   └──── all trials succeed ─────┘──── first trial fails ────┘
                              (re-opens, cool-down restarts)
```

- **Closed** — business as usual: calls proceed and every outcome feeds the
  counting strategy (`StrategyConsecutive`, `StrategyErrorRate`, `StrategySlowCall`, `StrategyErrorCount`).
- **Open** — the tripped state: calls are refused immediately. It lasts
  `Spec.OpenDuration`; outcomes from calls admitted before the trip are ignored — they
  describe a state that no longer exists.
- **HalfOpen** — the probe: after the cool-down, a bounded number of trial
  calls (`Spec.HalfOpenTrials`) checks whether the dependency recovered. All trials
  must succeed to close the circuit; the first failure re-opens it and the
  cool-down restarts. The round waits for its trials' outcomes instead of
  closing on a timer: low traffic keeps the circuit half-open and every arriving
  call is another trial (the resilience4j / gobreaker behavior). Set
  `Spec.HalfOpenTimeout` to re-open a round that never settles.

## Spec

`Spec` is the breaker's own vocabulary — the knobs one breaker is built from.
The consuming engine assembles it from its own policy, so the state machine
never sees that policy's vocabulary.

Structural defaults when a field is unset: `MinSamples` 1, `Window` 1s,
`OpenDuration` 5s, `HalfOpenTrials` 1, `HalfOpenTimeout` unlimited. A policy
layer may apply its own mainstream defaults before calling `New`.

### Strategies

Exactly **one** counting strategy runs per breaker (`Strategy` is a single
value, not a list). The one de-facto combination: a `StrategyErrorRate` breaker with
a slow-call pair set (`SlowThreshold` + `SlowRate`) trips on **error ratio OR
slow ratio** — see `StrategyErrorRate` below.

This follows the mainstream: Resilience4j likewise runs one strategy per
breaker with the error-or-slow pair built into its rate breaker, and gobreaker
offers only consecutive counting. Frameworks that do combine strategies
(Sentinel, Envoy) stack *separate breaker instances* over the same protected
unit rather than stacking strategies inside one breaker — a policy-layer
option that needs no change here.

#### `StrategyConsecutive` — N failures in a row

Counts failures since the last success; trips when `FailureThreshold` failures land
back to back. Any single success resets the count to zero.

- Knobs: `FailureThreshold`.
- Use when: the dependency is healthy until it abruptly dies (process crash,
  connection refused) — the simplest and most common choice.
- Watch out: one lucky success masks an ongoing problem; interleaved
  `fail, ok, fail, ok` never trips even at a 50% failure rate.

```go
circuitbreaker.Spec{Strategy: circuitbreaker.StrategyConsecutive, FailureThreshold: 5}
```

#### `StrategyErrorRate` — bad/total ratio over a window

Maintains a rolling estimate of the bad/total ratio over `Window`; trips once
the window's estimate holds at least `MinSamples` samples and the ratio reaches
`FailureRate`. A slow-call pair adds a second, independent arm: any call whose
duration is **strictly above** `SlowThreshold` — successful or failed — feeds the
slow ratio, and the breaker trips when that ratio reaches `SlowRate`. This
protects against a dependency that "works" but only after hanging.

- Knobs: `FailureRate`, `Window`, `MinSamples`, optionally `SlowThreshold`
  + `SlowRate`.
- Use when: traffic is steady and you care about the failure *proportion*, or
  when slow calls are as harmful as failures (user-facing
  latency budgets).
- Watch out: with low traffic the window holds few samples, so `MinSamples`
  matters — the capability default is 1 (the first failure trips), while the policy
  layer defaults it to 5.

```go
circuitbreaker.Spec{
    Strategy:      circuitbreaker.StrategyErrorRate,
    FailureRate:   0.5, MinSamples: 20, Window: time.Minute,
    SlowThreshold: 2 * time.Second, SlowRate: 0.8, // trip on errors OR slowness
}
```

#### `StrategySlowCall` — slow ratio over a window

Like `StrategyErrorRate`, but slowness is the **only** trip signal: it trips when
the ratio of slow calls (duration **strictly above** `SlowThreshold`) over
`Window` reaches `SlowRate`. Any call above the threshold counts as slow,
successful or failed; fast failures only sit in the denominator and can dilute
the ratio but never trip it.

- Knobs: `SlowThreshold` + `SlowRate`, `Window`, `MinSamples`.
- Use when: the failure mode you are protecting against is degradation, not
  errors — a dependency that succeeds but takes seconds.
- Watch out: a dependency that starts failing *fast* (connection refused in
  1ms) never trips a slow-call breaker; pair it with an error-based breaker
  if both failure modes are possible.

```go
circuitbreaker.Spec{
    Strategy:      circuitbreaker.StrategySlowCall,
    SlowThreshold: time.Second, SlowRate: 0.5, Window: time.Minute, MinSamples: 10,
}
```

#### `StrategyErrorCount` — failure count over a window

Trips when the *number* of failures (not the ratio) within `Window` reaches
`FailureThreshold`, successes interleaved or not.

- Knobs: `FailureThreshold`, `Window`.
- Use when: the absolute number of failures is what hurts (each failure leaks
  a resource, sends an alert, or costs money), regardless of the failure ratio.
- Watch out: under high traffic a healthy 0.1% error rate can still produce
  `FailureThreshold` raw failures — the ratio strategies are usually the better
  default.

```go
circuitbreaker.Spec{
    Strategy: circuitbreaker.StrategyErrorCount, FailureThreshold: 100, Window: time.Minute,
}
```

### Choosing a strategy

`Strategy` must be set; the capability does not infer one from threshold values and
rejects an empty value. The policy layers default an omitted setting to
`StrategyConsecutive` and may fill a missing trip condition with their own
mainstream defaults before calling `New`. In particular, a slow-call pair alone
does not switch the breaker to `StrategyErrorRate` — set the strategy
explicitly.

| Symptom you are protecting against | Strategy |
| --- | --- |
| Dependency abruptly dies | `StrategyConsecutive` |
| Elevated failure proportion, or errors + hangs | `StrategyErrorRate` (+ slow-call pair) |
| Degraded latency, errors are fine | `StrategySlowCall` |
| Absolute failure volume matters | `StrategyErrorCount` |

## Usage

A `Breaker` is built per protected unit (a service or an instance) and is safe
for concurrent use:

```go
spec := circuitbreaker.Spec{
    Strategy:     circuitbreaker.StrategyErrorRate,
    FailureRate:  0.5,
    MinSamples:   20,
    Window:       time.Minute,
    OpenDuration: 10 * time.Second, // cool-down after a trip
}
b, err := circuitbreaker.New(spec)
if err != nil {
    return err
}
```

Around each protected call:

```go
permit, ok := b.Allow()
if !ok {
    // Open (or half-open with the trial budget spent): fail fast here —
    // return a fallback, a cached copy, or a "try later" error. Do NOT
    // busy-retry: the point of the open circuit is to leave the
    // dependency alone.
    return ErrCircuitOpen
}
start := time.Now()
err := doWork()
b.Record(permit, err == nil, time.Since(start)) // duration feeds the slow-call logic
```

`Allow` returns a `Permit` that ties the outcome to the state that admitted the
call. Pass it back to `Record` exactly once. A permit issued in a state the
breaker has since left — a call admitted before the trip, or a trial from a
round that was abandoned — is ignored: it can neither close, re-open, nor
extend the cool-down, nor feed a stale window.

An outcome that carries no verdict — the caller was cancelled before the unit's
result was known — goes to `RecordIgnored` instead: it feeds neither counter, and
in HalfOpen it returns the trial permit so the round keeps waiting for a result
that actually decides.

What happens after the trip:

- **During Open** every `Allow` is refused for `Spec.OpenDuration`.
- **In HalfOpen** up to `HalfOpenTrials` trials are admitted. All must succeed to
  close the circuit; the first failure re-opens it and the cool-down restarts.
  There is no timer for closing: successes accumulate at whatever rate traffic
  arrives, and `HalfOpenTimeout` (unlimited by default) only re-opens a round
  whose trials never settle.
- A turned-off breaker is a **nil `*Breaker` at the holder** — the same OFF
  convention as `*Bulkhead`: the holder checks nil and admits the call itself, so
  no instance is built and an off switch costs nothing at runtime. A breaker that
  does exist is enabled and must be complete: a strategy with a missing threshold
  is a construction error, not a silent off switch. Build one with `New`; the
  zero value is not usable.

## State reporting

The breaker emits nothing itself. `Stats()` returns a read-only snapshot under
the breaker's own lock: the current state and the monotonic counts of entries
into Open, HalfOpen and Closed. A governor that
holds the breaker reads that snapshot on its own collection path and publishes
it from its own package — this package never imports an observability SDK,
keeping the capability leaf-only.
