# starter-scheduler Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` is a **global / infrastructure** starter (see
[starter/DESIGN.md](../DESIGN.md) §2.4) that drives periodic and
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
- **A job's schedule is declared where the job is constructed.**
  `scheduling.NewJob(name, trigger, run, opts...)` takes the cloud
  package's own types — the work is a `scheduling.Job`, the trigger a
  `scheduling.Trigger` (FixedRate / FixedDelay / ParseCron), the options
  `scheduling.Option`s — so what a job declares reads exactly as
  cloud/scheduling documents it, and user code depends on cloud types;
  the starter adds only the bean and the lock-by-bean-name bridge. The
  earlier design keyed
  `${spring.scheduler.jobs.<name>.*}` against bean names across two
  sources, and validated asymmetrically — config→bean was an error,
  bean→config only a warning, so a typo on the bean side left a job that
  silently never fired. What remains in config is process-level only:
  `spring.scheduler.enabled`.
- **One form: the concrete bean.** A job is a `*scheduling.Job`, built by
  `NewJob` from the work (usually a bound method of the author's own
  struct) and registered with `gs.Provide(...).Name(name)` — no Export,
  since there is no interface to index. Because registration goes through
  the container, the constructor's dependencies — other beans, or config
  bound onto a value-tagged struct — are resolved like any bean's; a form
  that runs before the container could never reach either. An empty name,
  nil run or nil trigger is rejected by `NewJob` with an error — at wiring —
  rather than being re-checked later.
- **Locks passed straight through.**
  A job attaches its lock at construction — the `WithLock(lk, key, ttl)`
  option —
  with the `lock.Locker` bean injected into the job's constructor, so the
  reference cannot dangle. WithLock takes the cloud/lock type directly; no
  adapter and no intermediate interface stands between the caller and the
  lock. (The earlier design keyed lockers by bean name and
  resolved at start, since
  the bean does not exist yet at registration time. `cloud/scheduling` defines its
  own minimal `Locker` / `Lock` interfaces to stay zero-dep, so a
  `lockerAdapter` in the starter bridges `lock.Locker` and bakes
  TTL / renewal options into the adapter.
- **Instrumentation is built into `cloud/scheduling`.** Per the repo-wide
  "local instrumentation" direction (the observe kit was dissolved), the
  cloud package reports every fire itself — `scheduling.runs{job,status}`,
  `scheduling.run.duration`, `scheduling.lag`, one log line with the same
  keys, and a span per run on the global OTel pipeline. This starter
  carries no observability code; it only wires jobs into the lifecycle.
  Panics wrap `ErrJobPanicked`, so a panic is a countable outcome
  (`status=panic`) instead of an error string to match on.
- **Drain is bounded by the shutdown context, not a knob.**
  `Stop` drains within the framework's shutdown ctx —
  the orchestrator's kill deadline (K8s terminationGracePeriod, systemd
  TimeoutStopSec) is the real bound and force-kills past it, so a second
  in-process timeout would only ever fire in environments without one.

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
