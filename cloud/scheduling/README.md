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

## How it works

The obvious way to run something on a timer is a loop:

```go
for {
    doWork()
    time.Sleep(10 * time.Second)
}
```

This works until it doesn't. The gap between runs is really 10 seconds *plus*
however long `doWork` took, so the schedule slips a little further each round.
"Every day at 03:00" cannot be written this way at all — a sleep is a length of
time, not a time of day. And a task asleep in `time.Sleep` cannot be woken, so
shutting down means waiting out the rest of the sleep.

`scheduling` splits the question in two. A `Trigger` answers only *"when is the
next fire?"*: given the current time and what happened before, it returns the
next instant. It does no waiting and keeps no state. The waiting is a loop per
task inside the `Scheduler`, and each turn of it does four things:

1. Ask the trigger for the next instant, passing the *current* `Now` and the
   `LastScheduled` the trigger returned last time.
2. Set a timer for that instant and wait on the timer *and* the shutdown signal
   together.
3. When the timer fires, dispatch the run (or run it inline, for fixed-delay).
4. Record the instant just scheduled as `LastScheduled`, then go to 1.

Drift dies in step 4: what goes back into the trigger is the instant it
*planned*, not when the run actually started or ended. So `FixedRate(d)` always
schedules `last planned instant + d`, and fire k lands on `first fire + k*d` —
the error never compounds. Anchor on "now, after the run finished" instead and
the schedule re-anchors every round, which is precisely the drift. Falling
behind is what the *current* `Now` in step 1 catches: once a slow run pushes
past whole intervals, the trigger does not fall back to `Now + d` but jumps to
the next slot on the *original* grid, dropping the missed slots in bulk rather
than firing them in a burst — the phase is unchanged. (`FixedDelay` is the
deliberate exception: it anchors on completion, for work that must never
overlap.)

The split pays for the rest too:

- **Cron is expressible at all.** "Daily at 03:00" needs a time of day, which
  only an instant can express.
- **Shutdown is immediate.** A waiting task wakes on the shutdown signal, no
  matter how far off the next fire was.
- **Overlap is a choice.** Whether a fire is skipped, queued, or replaces the
  run in flight is an explicit `ConcurrencyPolicy` — not whatever the loop
  happens to do.

### How the wait works, and how precise it is

The wait is not a poll and not a timing wheel — it is a sleep, just `sleep until
an instant` rather than `sleep for a duration`: each turn arms exactly one
`time.NewTimer(time.Until(next))` for the fire it is waiting for — a one-shot
timer, discarded once it fires and rebuilt on the next turn. There is no tick
rate to tune and no minimum sleep granularity; waiting three hours is one
three-hour wait.

Cost scales with the number of **tasks**, not the number of fires: a task has at
most one timer in flight, plus one loop goroutine parked in the `select`. The
timer is dropped once it fires and rebuilt on the next turn. The runtime keeps
them all in a single timer heap that the scheduler itself checks — not one OS
timer or thread per timer. At job-scale task counts (tens to hundreds) the cost
is negligible.

Precision is the Go runtime's timer precision: it does not fire *before* the
planned instant, and fires within the delay it takes the runtime to wake it — a
millisecond-scale order, sensitive to `GOMAXPROCS` and current load. That lag is
not drift: the next fire is measured from the planned instant. In an `Event`,
`Scheduled` is the planned instant and `Start` the actual one; their difference
is that fire's wake-up lag.

Cron has a coarser floor by construction: the expression has no seconds field
and `Next` zeroes seconds and nanoseconds, so a cron fire can only be aligned to
the minute.

## The API

| API | What it does |
| --- | --- |
| `NewScheduler(opts...)` | Creates a scheduler; nothing runs until `Start`. `WithObserver(fn)` hooks every fire (run or skip) for metrics/logs. |
| `Schedule(name, trigger, job, opts...)` | Registers a task (before or after `Start`); returns a cancel that stops and removes it. Rejects nil trigger/job, a duplicate name, or a stopped scheduler. |
| `FixedRate(d)` | Fire every d, anchored on the last **scheduled** time so drift does not accumulate. |
| `FixedDelay(d)` | Fire d after the previous run **completes**; never overlaps, no concurrency policy applies. |
| `ParseCron(expr)` | Standard 5-field cron; returns `(Trigger, error)`. `*`, ranges `a-b`, steps `*/n`, lists; the classic day-of-month / day-of-week OR rule. |
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
- `ParseCron("*/5 * * * *")` — wall-clock schedule; evaluated in the location of the
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
  the observer, wrapped in `ErrJobPanicked` so `errors.Is` tells it apart from a
  run that merely returned an error.
- `Stop` drains deterministically: loops return, in-flight runs complete, only
  then is the scheduler stopped.
- Fail-fast on misconfiguration: `Schedule` rejects nil trigger/job and
  duplicate names; `FixedRate`/`FixedDelay` panic on non-positive durations.
- Each process's scheduler is independent — this is not a distributed scheduler;
  replica coordination is what the `WithLock` layer adds.
