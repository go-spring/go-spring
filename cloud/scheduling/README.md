# scheduling

[English](README.md) | [中文](README_CN.md)

`scheduling` runs periodic and cron-scheduled background jobs — the Go-idiomatic
equivalent of Spring's `@Scheduled` / `TaskScheduler`. A `Job` is a plain
function bound to a `Trigger`, driven by a `Scheduler` with graceful-shutdown
drain. `starter-scheduler` wires the same API into the IoC container.

The scheduler is single-process by design. On a multi-replica deployment
`WithLock` de-duplicates fires (only the lock holder runs) — it adds no
sharding, failover, or task orchestration. For those, use an external job
platform instead (e.g. `starter-xxl-job`); reach for `scheduling` when the
job is in-process work (cache refresh, heartbeats, cleanup) that does not
justify operating a scheduler center.

## The API

| API | What it does |
| --- | --- |
| `NewScheduler(opts...)` | Creates a scheduler; nothing runs until `Start`. `WithObserver(fn)` hooks every fire (run or skip) for metrics/logs. |
| `Schedule(name, trigger, job, opts...)` | Registers a task (before or after `Start`); returns a cancel that stops and removes it. Rejects nil trigger/job or a duplicate name. |
| `FixedRate(d)` | Fire every d, anchored on the last **scheduled** time so drift does not accumulate. |
| `FixedDelay(d)` | Fire d after the previous run **completes**; never overlaps, no concurrency policy applies. |
| `Cron(expr)` / `ParseCron(expr)` | Standard 5-field cron (`Cron` panics on a bad expression, `ParseCron` returns the error). `*`, ranges `a-b`, steps `*/n`, lists; the classic dom/dow OR rule. |
| `WithConcurrencyPolicy(p)` | `Skip` (default, K8s "Forbid"), `Queue` (one may wait), `Replace` (cancel in-flight, K8s "Replace") — for fixed-rate/cron overlaps. |
| `WithTimeout(d)` | Cancels the job's context after the deadline, per run. |
| `WithLock(locker, key)` | Multi-replica de-duplication: a fire only runs when the lock is taken. The `Locker` interface is minimal and local; `starter-scheduler` adapts `cloud/lock.Locker` (TTL/renew baked into the adapter). |
| `Start(ctx)` / `Stop(ctx)` | Lifecycle. `Stop` cancels, waits for every loop and in-flight run; if the caller's ctx ends first it returns `ctx.Err()` while runs finish on their own. A stopped scheduler cannot be restarted — build a new one. |

## Usage

### 1. Schedule a job

```go
sch := scheduling.NewScheduler(scheduling.WithObserver(func(e scheduling.Event) {
    if e.Err != nil {
        log.Printf("job %s failed: %v", e.Name, e.Err)
    }
}))

_, err := sch.Schedule("heartbeat", scheduling.FixedRate(10*time.Second),
    func(ctx context.Context) error {
        log.Println("tick")
        return nil
    },
    scheduling.WithTimeout(3*time.Second),
)
if err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithCancel(context.Background())
_ = sch.Start(ctx)
defer func() { cancel(); _ = sch.Stop(context.Background()) }()
```

### 2. Pick the right trigger

- `FixedRate` — steady cadence regardless of run duration (may overlap; see the
  concurrency policy).
- `FixedDelay` — spacing measured from completion; inherently serial.
- `Cron("*/5 * * * *")` — wall-clock schedule; evaluated in the location of the
  reference time. No seconds field — sub-minute granularity is what
  `FixedRate`/`FixedDelay` are for.

### 3. Overlapping fires

A fixed-rate/cron job due to fire while its previous run is still executing:

- `Skip` (default): drop the new fire.
- `Queue`: let one fire wait; further ones are dropped.
- `Replace`: cancel the in-flight run's context and start the new one.

### 4. Observing what happened

`Event` reports every fire: `Name`, `Scheduled`, `Start`/`Duration`/`Err` for
runs; `Skipped=true` with `Reason` `"policy"` or `"lock"` for the two ways a
fire can be swallowed. A lock held by another replica is not an error
(`Err=nil`); a real backend error reports `Err` — the paths differ
intentionally. The observer must not block.

## Rules the package guarantees

- A panicking job cannot kill the loop: the panic becomes an error reported via
  the observer.
- `Stop` drains deterministically: loops return, in-flight runs complete, only
  then is the scheduler stopped.
- Fail-fast on misconfiguration: `Schedule` rejects nil trigger/job and
  duplicate names; `FixedRate`/`FixedDelay` panic on non-positive durations.
- Each process's scheduler is independent — this is not a distributed scheduler;
  replica coordination is what the `WithLock` layer adds.
