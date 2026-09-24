# scheduling

[English](README.md) | [中文](README_CN.md)

`scheduling` runs periodic and cron-scheduled background jobs — the Go-idiomatic
equivalent of Spring's `@Scheduled` / `TaskScheduler`. A `Job` (built by
`NewJob`) bundles a run function, a `Trigger` and the execution options; a
`Scheduler` drives it with graceful-shutdown drain. `starter-scheduler` wires
the same API into the IoC container.

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

This works until it doesn't: the real gap is 10s *plus* however long `doWork`
took, so the schedule slips each round; "every day at 03:00" cannot be written
this way at all (a sleep is a duration, not a time of day); and a task asleep
in `time.Sleep` cannot be woken for shutdown.

`scheduling` splits the question in two. A `Trigger` answers only *"when is the
next fire?"* — it does no waiting and keeps no state. The waiting is a loop per
task inside the `Scheduler`, and each turn does four things:

1. Ask the trigger for the next instant, passing the current `Now` and the
   `LastScheduled` it returned last time.
2. Arm a timer for that instant; wait on the timer *and* the shutdown signal.
3. When the timer fires, dispatch the run (inline, for fixed-delay).
4. Record the instant just scheduled as `LastScheduled`, then go to 1.

Drift dies in step 4: the trigger is fed the instant it *planned*, so
`FixedRate(d)` always schedules `last planned + d` — fire k lands on
`first fire + k*d`, error never compounds. When a slow run pushes past whole
intervals, the trigger jumps to the next slot on the *original* grid and drops
the missed ones in bulk — the phase is unchanged. (`FixedDelay` is the
deliberate exception: it anchors on completion, for work that must never
overlap.)

The split buys the rest too: cron is expressible at all (a time of day needs an
instant), shutdown is immediate (a waiting task wakes on the signal), and
overlap is an explicit `ConcurrencyPolicy` rather than whatever the loop
happens to do.

### How the wait works, and how precise it is

The wait is one one-shot `time.NewTimer(time.Until(next))` per turn — sleep
*until an instant*, not *for a duration* — no poll, no timing wheel, no tick
rate. Cost scales with the number of **tasks**, not fires: one in-flight timer
plus one parked loop goroutine per task, all in the runtime's shared timer
heap. At job-scale counts (tens to hundreds) it is negligible.

Precision is the Go runtime's: never *before* the planned instant, then within
a millisecond-scale wake-up delay (sensitive to `GOMAXPROCS` and load). That
lag is not drift — the next fire is still measured from the planned instant —
and it is exported as the `scheduling.lag` histogram. Cron is coarser by
construction: no seconds field, so a cron fire aligns to the minute at best.

### The runtime model

After `Start`, each task owns one *loop* goroutine that asks the trigger, waits,
and dispatches. A fire runs **inline** in the loop (fixed-delay — hence serial)
or in a dispatched *run* goroutine (fixed-rate / cron), bounded by the policy:

| Policy | In-flight runs | A fire while one runs |
| --- | --- | --- |
| `Skip` (default) | ≤ 1 | dropped (`skipped_policy`) |
| `Queue` | ≤ 1 running + 1 parked | first parks, the rest dropped |
| `Replace` | ≤ 1 (the newest) | cancels the in-flight run, starts the new one |

Contexts form one tree rooted at the `Start` ctx: loop ctx per task (cancelled
by the task's cancel func), run ctx per fire (plus `WithTimeout` / `Replace`
cancellation) — cancel the root and everything stops. Two mutexes, never
nested: the scheduler's for the registry and lifecycle, each task's for its
timing state, so managing tasks never contends with running them.

Lifecycle: `Schedule` works before or after `Start`; its cancel func removes
that one task. `Stop` is terminal — cancels the root, drains every loop and
run bounded by the caller's ctx, and reports a deadline error on timeout (the
remaining runs finish on their own).

## The API

| API | What it does |
| --- | --- |
| `NewScheduler()` | Creates a scheduler; nothing runs until `Start`. Every fire is reported through the package's built-in metrics and logs (see below). |
| `NewJob(name, trigger, run, opts...)` | Builds a [Job]: the registration bundle of name, run function, trigger, execution options, and (via `.WithLock`) the distributed lock. Returns an error on an empty name, nil run or nil trigger. |
| `Schedule(job)` | Registers a [Job] (before or after `Start`); returns a cancel that stops and removes it. Rejects a duplicate name or a stopped scheduler. |
| `FixedRate(d, opts...)` | Fire every d, anchored on the last **scheduled** time so drift does not accumulate. |
| `FixedDelay(d, opts...)` | Fire d after the previous run **completes**; never overlaps, no concurrency policy applies. |
| `After(d)` | Fire exactly once, d after the schedule starts, then stop the job — deferred one-shot work. |
| `WithInitialDelay(d)` | Trigger option for `FixedRate`/`FixedDelay`: delay the first fire by d; later fires follow the normal cadence. |
| `WithJitter(d)` | Trigger option for `FixedRate`/`FixedDelay`: delay each fire by a random [0, d) so shared cadences don't stampede a downstream. The delay feeds the next anchor, so the average interval becomes d + jitter/2. |
| `DailyWindow(start, end, tr)` | Fire tr's schedule only inside the daily window [start, end); fires outside it move to the next window's start. |
| `ParseCron(expr)` | Standard 5-field cron; returns `(Trigger, error)`. `*`, ranges `a-b`, steps `*/n`, lists; the classic day-of-month / day-of-week OR rule. |
| `WithConcurrencyPolicy(p)` | `Skip` (default, K8s "Forbid"), `Queue` (one may wait), `Replace` (cancel in-flight, K8s "Replace") — for fixed-rate/cron overlaps. |
| `WithTimeout(d)` | Cancels the job's context after the deadline, per run. |
| `WithLock(l, key, ttl)` | Multi-replica de-duplication: a fire only runs when the lock is taken. Takes the `cloud/lock.Locker` directly — the type the lock backends and starter-lock-* beans already are. |
| `Start(ctx)` / `Stop(ctx)` | Lifecycle. `Stop` cancels, waits for every loop and in-flight run; if the caller's ctx ends first it returns `ctx.Err()` while runs finish on their own. A stopped scheduler cannot be restarted — build a new one. |

## Usage

### 1. Schedule a job

```go
sch := scheduling.NewScheduler()

job := scheduling.NewJob("heartbeat", scheduling.FixedRate(10*time.Second),
    func(ctx context.Context) error {
        log.Println("tick")
        return nil
    },
    scheduling.WithTimeout(3*time.Second),
)
if _, err := sch.Schedule(job); err != nil {
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
- `After` — a single delayed fire ("do this once, a bit later").
- `DailyWindow` — wrap any of the above to fire only within, say, 9:00–18:00.
- `ParseCron("*/5 * * * *")` — wall-clock schedule; evaluated in the location of the
  reference time. No seconds field — sub-minute granularity is what
  `FixedRate`/`FixedDelay` are for.

Both fixed triggers take `WithInitialDelay(d)` when the first fire should wait
for a warm-up instead of one interval, and `WithJitter(d)` when many jobs share
a cadence and must not fire in lockstep.

### 3. Overlapping fires

A fixed-rate/cron job due to fire while its previous run is still executing:

- `Skip` (default): drop the new fire.
- `Queue`: let one fire wait; further ones are dropped.
- `Replace`: cancel the in-flight run's context and start the new one.

### 4. Observing what happened

Observability is built in: every fire reports `scheduling.runs{job,status}`
plus, for runs, `scheduling.run.duration{job,status}` and `scheduling.lag{job}`,
and one log line carrying the same `job`/`status` keys. The statuses are
exclusive — `ok | error | panic | skipped_policy | skipped_lock` — so the sum
over the dimension is the number of fires. A lock held by another replica is
`skipped_lock` with no error; a real locker backend failure is also
`skipped_lock` but its log line carries the error. Runs additionally open a
span on the global otel pipeline (`scheduler.job <name>`); a swallowed fire has
no run to trace.

## Rules the package guarantees

- A panicking job cannot kill the loop: the panic becomes an error reported via
  the fire's instrumentation, wrapped in `ErrJobPanicked` so `errors.Is` tells it apart from a
  run that merely returned an error.
- `Stop` drains deterministically: loops return, in-flight runs complete, only
  then is the scheduler stopped.
- Fail-fast on misconfiguration: `NewJob` returns an error on an empty name,
  nil run or nil trigger; `Schedule` rejects duplicate names;
  `FixedRate`/`FixedDelay`/`After`/`WithInitialDelay`/`WithJitter` panic on
  non-positive durations; `DailyWindow` panics on a nil trigger or a window
  outside one day.
- Each process's scheduler is independent — this is not a distributed scheduler;
  replica coordination is what the `WithLock` layer adds.
