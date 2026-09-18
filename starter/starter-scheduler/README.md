# starter-scheduler

[English](README.md) | [中文](README_CN.md)

`starter-scheduler` runs periodic and cron-scheduled background jobs as part of
the Go-Spring application lifecycle. Blank-import it, register a `Job` per unit
of work, and declare each job's trigger in configuration — the starter drives
them, participates in graceful shutdown, and can de-duplicate jobs across
replicas via a distributed lock.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it opens no network port. It exports a
`gs.Server` so the scheduler joins the server lifecycle — jobs start firing once
the application is ready and, on `SIGTERM`, in-flight runs drain before the
process exits.

The trigger and concurrency primitives come from the zero-dependency
[`cloud/scheduling`](../../cloud/scheduling) package; this starter is the thin
integration layer that binds the IoC container to it. A job's schedule is part of
the job, so it is declared where the job is registered — the only configuration
is process-level (`enabled`, `drain-timeout`).

## Installation

```bash
go get go-spring.org/starter-scheduler
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. Register a Job with its schedule

`scheduler.Provide` names the bean after the job, exports it as `Job` so the
scheduler collects it, and takes the schedule as an option — the cadence is read
beside the work it describes.

```go
import scheduler "go-spring.org/starter-scheduler"

func main() {
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return svc.Cleanup(ctx)
    }, scheduler.Every(5*time.Minute))

    scheduler.Provide("nightly", svc.Prune, scheduler.Cron("0 3 * * *"))
    gs.Run()
}
```

A job with no trigger — or two — panics at registration, which is during startup:
a mistake surfaces at boot rather than as a job that silently never fires.

## Triggers

Each job declares **exactly one** trigger:

| Option                | Meaning                                                          |
|-----------------------|------------------------------------------------------------------|
| `scheduler.Cron(expr)`| Standard 5-field cron expression (`min hour dom month dow`).     |
| `scheduler.Every(d)`  | Fire every `d`, measured from each scheduled fire time.          |
| `scheduler.After(d)`  | Fire `d` *after the previous run finishes*; never overlaps.      |

## Per-job options

Applied at registration alongside the trigger:

| Option                        | Default          | Meaning                                                            |
|-------------------------------|------------------|--------------------------------------------------------------------|
| `scheduler.WithTimeout(d)`    | none             | When positive, the run's context is cancelled after `d`.           |
| `scheduler.WithConcurrency(p)`| `scheduling.Skip`| Overlap policy for `Every`/`Cron`: `Skip`, `Queue` or `Replace`.   |
| `scheduler.WithLock(bean)`    | —                | Name of a `lock.Locker` bean; only the holder runs each fire.      |
| `scheduler.WithLockKey(k)`    | job name         | Key acquired on the locker.                                        |
| `scheduler.WithLockTTL(d)`    | locker's default | Lease duration; auto-renewed while the job holds it.               |

`WithConcurrency` has no effect on `After` jobs, which are serial by
construction.

## Multi-replica de-duplication

To ensure a job runs on only one replica at a time, point its `WithLock` at a
`lock.Locker` bean from `starter-lock-{redis,etcd,consul}`. Each fire
acquires the lock; the loser skips.

```go
scheduler.Provide("nightly", svc.Prune, scheduler.Cron("0 2 * * *"),
    scheduler.WithLock("jobs"),                // a lock.Locker bean named "jobs"
    scheduler.WithLockTTL(5*time.Minute))
```

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"   // contributes the "jobs" locker
    _ "go-spring.org/starter-scheduler"
)
```

## Graceful shutdown

On `SIGTERM` the scheduler stops firing and waits for in-flight runs to finish,
bounded by `spring.scheduler.drain-timeout` (default `30s`) — the scheduler's
own bound on its graceful shutdown.

## Observability

With `starter-otel` (or any SDK provider) imported, every job run opens a span
(`scheduler.job <name>`, with a `status` attribute) and every fire — run or
skipped — feeds three metrics: `scheduling.runs{job,status}`,
`scheduling.run.duration{job,status}` and `scheduling.lag{job}`. The status
vocabulary is `ok`, `error`, `panic`, `skipped_policy`, `skipped_lock`.

Each fire also writes one log line under its own tag `_app_scheduler_access`
(`log.RegisterAppTag("scheduler", "access")`) instead of the default app tag;
the lifecycle lines (starting / started / drain) stay on the default tag. The
line carries the same `job` and `status` the metrics record, plus `reason`
(skips), `duration_ms` and, on failure, `error` — so a series can be joined to
the line that explains it.

`lag` is the distance between a fire's scheduled instant and when its run
actually started: a rising one means runs are starting later and later, which no
other instrument would show. Without an SDK provider all of this is a no-op.
See [USAGE §1.1](USAGE.md#11-observability-variant-example-otel) for the full
instrument reference.

## Configuration reference

| Key                              | Default | Description                                  |
|----------------------------------|---------|----------------------------------------------|
| `spring.scheduler.enabled`       | `true`  | Enable the scheduler (active once ≥1 Job).   |
| `spring.scheduler.drain-timeout` | `30s`   | Max time `Stop` waits for in-flight runs.    |

## Example

See [`example/`](example) for a runnable demo exercising `fixed-rate`,
`fixed-delay`, `cron` and a lock-guarded job (backed by an in-process
`MemoryLocker`, so no docker is required):

```bash
cd example && ./check.sh
```
