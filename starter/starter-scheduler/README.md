# starter-scheduler

[English](README.md) | [中文](README_CN.md)

`starter-scheduler` runs periodic and cron-scheduled background jobs as part of
the Go-Spring application lifecycle. Blank-import it, register a `Job` per unit
of work, and declare each job's trigger in configuration — the starter drives
them, participates in graceful shutdown, and can de-duplicate jobs across
replicas via a distributed lock.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it opens no network port. Instead it
exports a `gs.Server` so the scheduler joins the server lifecycle — jobs start
firing once the application is ready and, on `SIGTERM`, in-flight runs are
drained before the process exits.

The trigger and concurrency primitives come from the zero-dependency
[`stdlib/scheduling`](../../stdlib/scheduling) package; this starter is the thin
integration layer that binds configuration and the IoC container to it.

## Installation

```bash
go get go-spring.org/starter-scheduler
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. Register a Job per unit of work

`scheduler.Provide` names the bean after the job and exports it as `Job`, so the
scheduler collects it and matches it to its config entry by name.

```go
import scheduler "go-spring.org/starter-scheduler"

func main() {
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return svc.Cleanup(ctx)
    })
    gs.Run()
}
```

### 3. Declare each job's schedule

```properties
# every 5 minutes (standard 5-field cron)
spring.scheduler.jobs.cleanup.cron=*/5 * * * *

# every 30 seconds, measured from each scheduled fire time
spring.scheduler.jobs.heartbeat.fixed-rate=30s

# 10 seconds after the previous run finishes (never overlaps)
spring.scheduler.jobs.reindex.fixed-delay=10s
```

A configured job with no matching `Job` bean — or a `Job` bean whose trigger is
missing or ambiguous — is a **fail-fast startup error**, so a typo surfaces at
boot rather than as a job that silently never fires.

## Triggers

Each job must set **exactly one** of:

| Key           | Meaning                                                            |
|---------------|-------------------------------------------------------------------|
| `cron`        | Standard 5-field cron expression (`min hour dom month dow`).       |
| `fixed-rate`  | Fire every interval, measured from each scheduled fire time.       |
| `fixed-delay` | Fire this long *after the previous run finishes*; never overlaps.  |

## Per-job options

| Key           | Default | Meaning                                                                 |
|---------------|---------|-------------------------------------------------------------------------|
| `timeout`     | `0`     | When positive, the run's context is cancelled after this elapses.       |
| `concurrency` | `skip`  | Overlap policy for `fixed-rate`/`cron`: `skip`, `queue` or `replace`.   |
| `lock`        | —       | Name of a `lock.Locker` bean; only the holder runs each fire.           |
| `lock-key`    | job name| Key acquired on the locker.                                             |
| `lock-ttl`    | `30s`   | Lease duration; auto-renewed while the job holds it.                    |

`concurrency` has no effect on `fixed-delay` jobs, which are serial by
construction.

## Multi-replica de-duplication

To ensure a job runs on only one replica at a time, point its `lock` at a
`lock.Locker` bean contributed by `starter-lock-{redis,etcd,consul}`. Each fire
acquires the lock; the loser skips.

```properties
spring.scheduler.jobs.nightly.cron=0 2 * * *
spring.scheduler.jobs.nightly.lock=jobs      # a lock.Locker bean named "jobs"
spring.scheduler.jobs.nightly.lock-ttl=5m
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
bounded by `spring.scheduler.drain-timeout` (default `30s`) — a safety net on
top of the framework-level `app.shutdown.timeout`.

## Configuration reference

| Key                              | Default | Description                                  |
|----------------------------------|---------|----------------------------------------------|
| `spring.scheduler.enabled`       | `true`  | Enable the scheduler (active once ≥1 Job).   |
| `spring.scheduler.drain-timeout` | `30s`   | Max time `Stop` waits for in-flight runs.    |
| `spring.scheduler.jobs.<name>.*` | —       | Per-job trigger and options (see above).     |

## Example

See [`example/`](example) for a runnable demo exercising `fixed-rate`,
`fixed-delay`, `cron` and a lock-guarded job (backed by an in-process
`MemoryLocker`, so no docker is required):

```bash
cd example && ./check.sh
```
