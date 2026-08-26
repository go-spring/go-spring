# loadtest
[English](README.md) | [中文](README_CN.md)

`loadtest` is the load-test harness shared by the client starters'
example-load binaries. It drives an `Op` under a pluggable scheduling
`Driver` for a bounded duration, records latency and a classified error
breakdown, then runs health `Assert`ions so a run reaches a verdict — "the
protection stack held and signals were not lost" — rather than merely
producing numbers. It is example-grade tooling, not production code.

## Quick start

The legacy one-liner (closed-loop, N workers):

```go
res := loadtest.Run(ctx, loadtest.Config{Concurrency: 50, Duration: 30 * time.Second}, op)
res.Print(os.Stdout)
```

The full-featured path is the `Runner` builder:

```go
res := loadtest.New().
    Driver(loadtest.Ramp(100, 1000, time.Minute, 500)). // open-loop ramp
    Duration(2 * time.Minute).
    CaptureGC(true).
    Assert("qps-floor", loadtest.AssertMinQPS(500)).
    Assert("error-ceiling", loadtest.AssertErrorRateBelow(0.01)).
    Assert("p99", loadtest.AssertP99Below(200*time.Millisecond)).
    Run(ctx, op)
if !res.Passed() { /* verdict FAILED */ }
res.Print(os.Stdout)
```

## The four seams

- **`Driver` decides WHEN ops fire.** `Drive` receives an `invoke` that
  already times and records each op; the driver only owns timing and
  concurrency, and must not return until every invoke it started has
  completed. Built-ins:
  - `ClosedLoop{Concurrency: N}` — N workers looping as fast as op
    returns; throughput self-limits ("N concurrent users").
  - `OpenLoop(rps, maxConcurrent)` — fixed arrival rate independent of
    latency; exposes tail behavior closed-loop self-throttling hides.
  - `Ramp(from, to, over, max)` — RPS ramping linearly, then holding.
  - `Staircase(max, steps...)` — step function of `Step{RPS, Duration}`.
  - `Scheduled(schedule, max)` — arbitrary `Schedule`
    (`func(elapsed) float64`) for bursty / diurnal / replay shapes.
    Returning `<= 0` pauses dispatch.
  - Supply your own `Driver` to model any traffic shape; implement the
    unexported namer or the `%T` type name is used in the report.
- **`Schedule`** is the pacing function behind the open-loop family; see
  `ConstantSchedule`, `LinearRamp`, `StepSchedule`.
- **`Assert` decides what "held up" means.** `func(ctx, *Result) error`,
  nil = pass. Built-ins: `AssertMinQPS`, `AssertErrorRateBelow`,
  `AssertP99Below`, `AssertGCPauseAvgBelow` (needs
  `Runner.CaptureGC(true)`). Write your own for anything `Result` or the
  runtime exposes — "breaker opened", "metric registry gained samples",
  "no goroutine leak".
- **`Classify` decides how errors map to buckets.**
  `func(error) string`; the default (`DefaultClassify`) uses the
  resilience + fault taxonomy. A classifier returning `""` counts under
  `other`.

## Error buckets

`Result.Buckets` maps each error label to its count. The built-in labels
(constants, fixed display order in `Result.Print` so reports stay
diffable):

| Constant | Label | Meaning |
|---|---|---|
| `BucketCircuit` | `circuit` | `resilience.ErrCircuitOpen` |
| `BucketRateLimited` | `rate-limited` | `resilience.ErrRateLimited` |
| `BucketBulkhead` | `bulkhead` | `resilience.ErrBulkheadFull` |
| `BucketInjected` | `fault-injected` | `fault.IsInjected` |
| `BucketOther` | `other` | everything else |

Custom `Classify` labels land in the same map; `Print` outputs the five
built-ins in the listed order, then custom labels sorted alphabetically.

## Load-test traffic tagging

Every op's context is tagged via `traffic.WithLoadTest` before dispatch, so
downstream clients across the whole chain recognize the synthetic load
(`traffic.IsLoadTest`) — shadow-table routing, fault injection scope, and
metrics labeling all key off it.

## Installation

```
go get go-spring.org/cloud
```
