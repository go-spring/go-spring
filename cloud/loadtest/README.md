# loadtest
[English](README.md) | [中文](README_CN.md)

`loadtest` is the load-test harness shared by the client starters'
example-load binaries. It drives an `Op` under a pluggable scheduling
`Driver` for a bounded duration, records latency and a classified error
breakdown, then runs health `Assert`ions so a run reaches a verdict — "the
protection stack held and signals were not lost" — rather than merely
producing numbers. It is example-grade tooling, not production code: when a
run needs statistical rigor, reach for a real load tool.

A runnable, self-verifying standalone demo lives in
[example](example/example.go) (`./example/check.sh` exits 0).

## Quick start

The one-liner (closed-loop, N workers):

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

`Op` is `func(ctx) error` — nil on success; the starter-specific main
supplies the client call. Every op's context is tagged as load-test traffic
(`traffic.WithLoadTest`) so downstream clients recognize the synthetic load
(shadow-table routing, fault-injection scope, and metrics labeling in
`cloud/governance/traffic` all key off it).

## Choosing a driver

`Driver` decides WHEN ops fire; the runner already times and records each
op, so a driver is pure scheduling.

| Driver | Shape |
|---|---|
| `ClosedLoop{Concurrency: N}` | N workers looping as fast as op returns; throughput self-limits ("N concurrent users") |
| `OpenLoop(rps, maxConcurrent)` | fixed arrival rate independent of latency; exposes tail behavior closed-loop self-throttling hides |
| `Ramp(from, to, over, max)` | RPS ramping linearly, then holding |
| `Staircase(max, steps...)` | step function of `Step{RPS, Duration}` |
| `Scheduled(schedule, max)` | arbitrary `Schedule` (`func(elapsed) float64`) for bursty / diurnal / replay shapes; `<= 0` pauses dispatch |

`Schedule` building blocks: `ConstantSchedule`, `LinearRamp`,
`StepSchedule`. A custom `Driver` models any other shape; the contract is
two clauses — invoke once per scheduled op, and do not return until every
started invoke finished (no lost tail latency).

## Making a verdict

`Assert` is `func(ctx, *Result) error`; nil = pass. Built-ins:
`AssertMinQPS`, `AssertErrorRateBelow`, `AssertP99Below`,
`AssertGCPauseAvgBelow` (needs `Runner.CaptureGC(true)`). Write your own for
anything `Result` or the runtime exposes — "breaker opened", "metric
registry gained samples", "no goroutine leak".

A run with no assertions has nothing to fail (`Passed()` is true).

## Error buckets

`Result.Buckets` maps each error label to its count. Built-in labels (fixed
display order in `Result.Print` so reports stay diffable):

| Constant | Label | Meaning |
|---|---|---|
| `BucketCircuit` | `circuit` | `resilience.ErrCircuitOpen` |
| `BucketRateLimited` | `rate-limited` | `resilience.ErrRateLimited` |
| `BucketBulkhead` | `bulkhead` | `resilience.ErrBulkheadFull` |
| `BucketInjected` | `fault-injected` | `fault.IsInjected` |
| `BucketOther` | `other` | everything else |

`Classify` (`Runner.Classify`) replaces the default (`DefaultClassify`) to
add custom buckets — e.g. split "other" into per-status breakdowns; custom
labels print sorted after the built-ins. `""` counts under `other`, so a
custom classifier can never lose an error.

## Limits

Single-mutex recorder, `time.Sleep` pacing, pass/fail thresholds instead of
confidence intervals — the right trade at example scale, not a benchmarking
tool.

## Installation

```
go get go-spring.org/cloud
```
