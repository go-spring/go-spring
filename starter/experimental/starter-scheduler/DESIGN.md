# starter-scheduler Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` is a **global / infrastructure** starter (see
[starter/DESIGN.md](../DESIGN.md) §2.4) that drives periodic and
cron-scheduled background jobs as part of the Go-Spring server lifecycle.
Triggers and concurrency primitives come from the zero-dep
`spring/scheduling` package; this starter is the thin integration layer.

## 1. Responsibilities & Boundaries

- **In scope:** collect `Job` beans, match them to `spring.scheduler.jobs.<name>`
  config, hand each to `spring/scheduling`, participate in graceful drain.
- **Out of scope:** the trigger algorithms, concurrency policies, and
  lock semantics (all in `spring/scheduling`); locker backends
  (`starter-lock-*`).

## 2. Key Decisions

- **Scheduler is a `gs.Server`, not a `gs.Runner`.** A Runner's `Run`
  must return quickly; the scheduler needs to run for the app's
  lifetime and drain in-flight fires on `SIGTERM` — that fits Server.
  This is a deliberate deviation from the design doc's "Runner"
  wording.
- **`serialTrigger` marker for `fixed-delay`.** Fixed-delay is
  intrinsically serial: `spring/scheduling` implements it as a
  synchronous next-fire anchored on `LastCompletion`, distinct from
  `fixed-rate` / `cron` which dispatch asynchronously and are governed
  by a `ConcurrencyPolicy` (`skip` / `queue` / `replace`).
- **Registration sugar `scheduler.Provide(name, fn)`.** One call does
  `gs.Provide` + `Name(name)` + `Export(gs.As[Job]())`. Naked
  `NewJob` is not collected because the container only indexes exported
  interfaces (see the `gs export interface index` note in project
  memory).
- **Config-defined jobs are fail-fast.** A configured entry with no
  matching `Job` bean, or a `Job` bean whose trigger is missing /
  ambiguous, is a boot error. A typo cannot silently produce a job
  that never fires.
- **Locks by bean name, adapted at the boundary.**
  `Lockers map[string]lock.Locker autowire:"?"` collects every
  contributed locker keyed by its bean name; the scheduler resolves
  each job's `lock` field to a name. `spring/scheduling` defines its
  own minimal `Locker` / `Lock` interfaces to stay zero-dep, so a
  `lockerAdapter` in the starter bridges `lock.Locker` and bakes
  TTL / renewal options into the adapter.
- **Drain participates in the framework drain.**
  `spring.scheduler.drain-timeout` (default `30s`) bounds `Stop`;
  it is a safety net on top of `app.shutdown.timeout` — the scheduler
  stops accepting new fires immediately and waits for the in-flight
  set.

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
  loop; `spring/scheduling` centralizes it.
- **Auto-derive job names from function pointers — rejected.**
  Function-pointer names are compiler-dependent; explicit bean names
  keep configuration stable.
- **Reuse `spring/lock.Locker` directly in `spring/scheduling` —
  rejected.** Would drag the whole locker abstraction into the
  zero-dep scheduling package; the adapter at the starter boundary
  keeps both packages clean.
