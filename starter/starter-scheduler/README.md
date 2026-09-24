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
is process-level (`enabled`); the shutdown drain is bounded by the
orchestrator's kill deadline, not a config value.

## Installation

```bash
go get go-spring.org/starter-scheduler
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. Define a Job bean

A job is the cloud type `scheduling.Job` (no interface — nothing can
implement it by accident, and navigation lands on the struct), built by
`scheduling.NewJob` in the work's constructor, typically from a bound method of
your own struct so the job's dependencies ride in that struct and are resolved
by the container. No `Export` is needed: the scheduler collects beans of
exactly this type.

```go
import scheduling "go-spring.org/cloud/scheduling"

type CleanupService struct {
    db *gormcore.DB // any bean the container knows
}

func (s *CleanupService) Cleanup(ctx context.Context) error { ... }

func NewCleanupJob(db *gormcore.DB) *scheduling.Job {
    return scheduling.NewJob("cleanup", scheduling.FixedRate(5 * time.Minute), svc.Cleanup)
}

func main() {
    gs.Provide(NewCleanupJob)
    gs.Run()
}
```

A job with an empty name, a nil run function or a nil trigger is rejected
by `NewJob` with an error, which is during wiring: a mistake surfaces at boot
rather than as a job that silently never fires. Trigger-side mistakes (a non-positive duration,
an unparsable cron) fail in the cloud package — in the constructor, so equally
at boot.

## Triggers and options

Triggers and per-job options are the cloud package's own types — see
[cloud/scheduling](../../../cloud/scheduling/README.md) for the full contract.
The three triggers: `scheduling.FixedRate(d)` (every `d`, anchored on each
planned fire time), `scheduling.FixedDelay(d)` (`d` after the previous run
finishes; never overlaps), `scheduling.ParseCron(expr)` (standard 5-field
cron). The options: `scheduling.WithTimeout`, `scheduling.WithConcurrencyPolicy`
(no effect on FixedDelay, serial by construction).

The lock is a cloud concept too: the `WithLock(l, key, ttl)` option takes the
`cloud/lock.Locker` directly — the type the starter-lock-{redis,etcd,consul}
beans already are — with no adapter in between.

## Multi-replica de-duplication

To ensure a job runs on only one replica at a time, attach a lock with the `WithLock` option: inject a `lock.Locker` bean (from
`starter-lock-{redis,etcd,consul}`) into the job's constructor and pass it
straight through. Each fire acquires the lock; the loser skips.

```go
func NewNightlyJob(svc *Service, lk lock.Locker) (*scheduling.Job, error) {
    tr, err := scheduling.ParseCron("0 2 * * *")
    if err != nil {
        return nil, err
    }
    return scheduling.NewJob("nightly", tr, svc.Prune,
        scheduling.WithLock(lk, "nightly", 5*time.Minute))
}
```

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"   // contributes the "jobs" locker
    _ "go-spring.org/starter-scheduler"
)
```

## Graceful shutdown

On `SIGTERM` the scheduler stops firing and waits for in-flight runs to
finish, bounded by the framework's shutdown context — in production the
orchestrator (K8s terminationGracePeriod, systemd TimeoutStopSec) is the real
deadline and force-kills the process past it, so the starter adds no second
knob.

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

## Example

See [`example/`](example) for a runnable demo exercising `fixed-rate`,
`fixed-delay`, `cron` and a lock-guarded job (backed by an in-process
`MemoryLocker`, so no docker is required):

```bash
cd example && ./check.sh
```
