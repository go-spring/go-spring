# starter-scheduler Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` is a **global / infrastructure** starter (see
[starter/DESIGN.md](../../DESIGN.md) §2.4) that drives periodic and
cron-scheduled background jobs as part of the Go-Spring server lifecycle.
Triggers and concurrency primitives come from the zero-dep
`cloud/scheduling` package; this starter is the thin integration layer.

## 1. Responsibilities & Boundaries

- **In scope:** collect `Job` beans, read each one's schedule off the bean, hand
  it to `cloud/scheduling`, participate in graceful drain.
- **Out of scope:** the trigger algorithms, concurrency policies, and
  lock semantics (all in `cloud/scheduling`); locker backends
  (`starter-lock-*`).

## 2. Key Decisions

- **Scheduler is a `gs.Server`, not a `gs.Runner`.** A Runner's `Run`
  must return quickly; the scheduler needs to run for the app's
  lifetime and drain in-flight fires on `SIGTERM` — that fits Server.
  This is a deliberate deviation from the design doc's "Runner"
  wording.
- **`serialTrigger` marker for `fixed-delay`.** Fixed-delay is
  intrinsically serial: `cloud/scheduling` implements it as a
  synchronous next-fire anchored on `LastCompletion`, distinct from
  `fixed-rate` / `cron` which dispatch asynchronously and are governed
  by a `ConcurrencyPolicy` (`skip` / `queue` / `replace`).
- **A job's schedule is declared where the job is registered.**
  `scheduler.Provide(name, fn, opts...)` takes the trigger ([Every] /
  [After] / [Cron]) and the execution options as `JobOption`s, so a job
  reads in one place and its validity is settled at startup rather than
  by a name-keyed lookup. The earlier design keyed
  `${spring.scheduler.jobs.<name>.*}` against bean names across two
  sources, and validated asymmetrically — config→bean was an error,
  bean→config only a warning, so a typo on the bean side left a job that
  silently never fired. What remains in config is process-level only:
  `spring.scheduler.enabled` and `drain-timeout`.
- **Registration sugar `scheduler.Provide(name, fn, opts...)`.** One call
  does `gs.Provide` + `Name(name)` + `Export(gs.As[Job]())`. Naked
  `NewJob` is not collected because the container only indexes exported
  interfaces (see the `gs export interface index` note in project
  memory). A missing or duplicated trigger panics here — at registration,
  i.e. during startup — rather than being re-checked later.
- **Locks by bean name, adapted at the boundary.**
  `Lockers map[string]lock.Locker autowire:"?"` collects every
  contributed locker keyed by its bean name; a job names one with
  `WithLock`, and the name is resolved when the scheduler starts, since
  the bean does not exist yet at registration time. `cloud/scheduling` defines its
  own minimal `Locker` / `Lock` interfaces to stay zero-dep, so a
  `lockerAdapter` in the starter bridges `lock.Locker` and bakes
  TTL / renewal options into the adapter.
- **Instrumentation lives in the starter, not in `cloud/scheduling`.**
  The zero-dep reasoning that keeps `Locker` minimal applies to
  telemetry too: the cloud package exposes the `Observer` / `Event` seam
  and nothing else, while this starter turns each `Event` into a span,
  three metrics and a log line (`observe.go`). Putting the OTel bridge
  in the cloud package — the way `lock` does with `WrapLocker` — would
  force it to import OTel, which is exactly what its zero-dependency
  claim rules out. One core addition was unavoidable: panics now wrap
  `ErrJobPanicked`, so a panic is a countable outcome instead of an
  error string the observer would have to match on.
- **Drain is the scheduler's own bound on shutdown.**
  `spring.scheduler.drain-timeout` (default `30s`) bounds `Stop` —
  the scheduler stops accepting new fires immediately and waits for the
  in-flight set.

## 3. Constraints

- **Cron is 5-field.** 5-field expressions (`min hour dom month dow`)
  with 1-minute smallest granularity; the example intentionally does
  not exercise cron inside its smoke window (smoke only verifies wiring).
- **Overlap policy has no effect on `fixed-delay`.** Serial by
  construction.
- **Multi-replica de-dup is strict "max concurrency = 1".** The
  stdlib test `TestWithLockDeduplicates` uses shared `inFlight` /
  `maxSeen` atomics to assert the property (not "one replica always
  wins" — every fire retries the lock).

## 4. Trade-offs / Alternatives Rejected

- **A goroutine per job with `time.Ticker` — rejected.** Cron, drain,
  overlap policies, and lock-guarded fires need a proper scheduler
  loop; `cloud/scheduling` centralizes it.
- **Auto-derive job names from function pointers — rejected.**
  Function-pointer names are compiler-dependent; an explicit name is what
  the scheduler's task name and the default lock key are built from.
- **Schedules in configuration — rejected.** A retunable cadence is an ops
  knob, and ops-managed schedules are what a job platform
  (`starter-xxl-job`) is for. For the in-process work this starter targets,
  the cadence is part of what the job is, and naming it in code removes the
  name-keyed lookup between two sources.
- **Reuse `cloud/lock.Locker` directly in `cloud/scheduling` —
  rejected.** Would drag the whole locker abstraction into the
  zero-dep scheduling package; the adapter at the starter boundary
  keeps both packages clean.
