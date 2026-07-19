# scheduling Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`scheduling` is the zero-dependency stdlib abstraction for running periodic
and cron-scheduled background jobs. It provides the substrate `starter-
scheduler` wires into the IoC container as a `gs.Server`, so scheduled work
participates in graceful shutdown.

## 1. Responsibilities & Boundaries

- Compute the next fire time from a `Trigger` and run a `Job` against it,
  under a `ConcurrencyPolicy` and optional per-run timeout / distributed lock.
- Not a queue, not a distributed scheduler, not a workflow engine. Each
  process's scheduler is independent; multi-replica coordination is layered
  by attaching a `Locker` via `WithLock`.
- Cron parsing lives here on purpose. Pulling a third-party cron library
  would break stdlib's zero-dependency contract; the built-in parser handles
  the standard 5-field form and the day-of-month / day-of-week OR rule.

## 2. Key Abstractions & Seams

- `Trigger.Next(TriggerContext) time.Time`. Returning zero means "never fire
  again". `FixedRate` anchors on `LastScheduled` so drift does not accumulate;
  `FixedDelay` anchors on `LastCompletion`; `Cron` walks the parsed spec.
- **`serialTrigger` marker**: `fixedDelay` implements an unexported `serial()`
  method. The scheduler detects it via type-assert and runs those tasks
  **synchronously in the loop** so the next fire is measured from completion;
  everything else is dispatched off the loop under the concurrency policy.
- `Locker` / `Lock` are declared **locally in this package** with a minimal
  shape ("TryAcquire returns local `Lock`") so the package stays dependency-
  free. `stdlib/lock.Locker` does not satisfy it directly — the integration
  layer (`starter-scheduler`) adapts one, baking TTL / renew choices into
  the adapter.
- `Observer` fires after every run **and** every skip; `Skipped=true` with
  `Reason="policy"` or `"lock"` — the two ways a fire can be swallowed.
- Registration: `Schedule(name, trigger, job, opts...)` returns a `cancel`
  that stops the loop and removes the task. Scheduling before / after
  `Start` are both supported.

## 3. Constraints (do not break)

- **Fail-fast on misconfiguration**. `Schedule` rejects a nil trigger, nil
  job, or duplicate name; `FixedRate` / `FixedDelay` panic on non-positive
  duration; `Cron` panics on a malformed expression (`ParseCron` returns
  error).
- **Lock skip is not an error**. `Locker.TryAcquire` returning `ok=false`
  emits a `Skipped:"lock"` event with `Err=nil`; a real backend error emits
  `Skipped:"lock"` with `Err=err` — the two paths differ intentionally.
- **`safeRun` panic → error**. A panicking job cannot kill the loop; it is
  converted into an error and reported via the `Observer`.
- **`Stop` waits for in-flight runs**. It cancels the scheduler ctx, waits
  for every task loop to return, then waits for every in-flight run's
  `runWg`; only then does it consider drain complete. If the caller's ctx
  ends first, `Stop` returns `ctx.Err()` — the runs continue to complete on
  their own.
- **After `Stop` the scheduler cannot be restarted**. Build a new one.

## 4. Trade-offs / Alternatives Rejected

- **Scheduler is a `gs.Server`, not a `gs.Runner`**. Runners must return
  quickly; the scheduler must live for the process lifetime and drain on
  SIGTERM. Server semantics fit — a deliberate deviation from any "Runner"
  wording elsewhere.
- **`ConcurrencyPolicy` only applies to non-serial triggers**. Fixed-delay
  cannot overlap by construction; adding a policy there is a footgun.
- **No cron seconds field**. Standard 5-field expressions are the least
  surprising; scheduling at sub-minute granularity is better served by
  `FixedRate` / `FixedDelay`.
- **Lock adapter, not direct import**. Keeping `stdlib/lock` out of this
  package preserves layer independence and lets a caller supply any
  minimal `Locker` (in-memory, test double, ...) without pulling the lock
  abstraction.
