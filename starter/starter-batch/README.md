# starter-batch

[English](README.md) | [中文](README_CN.md)

`starter-batch` runs [`spring/batch`](../../spring/batch) jobs — chunk-oriented
batches and one-shot Cloud Tasks — as part of the Go-Spring application
lifecycle. Blank-import it, register a `JobDefinition` per job, and either flip
`run-on-startup=true` (Cloud Task) or trigger the job through the exported
`*Launcher` bean from another server (e.g. `starter-scheduler`).

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it opens no network port. Instead it
exports a `gs.Server` so the runner joins the server lifecycle — startup
launches begin once the application is ready and, on `SIGTERM`, in-flight
launches are drained before the process exits.

The engine, `Reader`/`Processor`/`Writer` interfaces, `JobRepository` seam and
the in-process memory repository all come from the zero-dependency
[`spring/batch`](../../spring/batch) package; this starter is the thin
integration layer that binds configuration and the IoC container to it. Durable
backends (Redis, SQL, ...) are separate starters that contribute their own
`batch.JobRepository` bean.

## Installation

```bash
go get go-spring.org/starter-batch
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-batch"
```

### 2. Register a `JobDefinition` per job

`batch.Provide` names the bean after the job and exports it as `JobDefinition`,
so the runner collects it and matches it to its config entry by name.

```go
import (
    starter "go-spring.org/starter-batch"
    "go-spring.org/spring/cloud/batch"
)

var reportStep = batch.Func("generate", func(ctx context.Context) error {
    return svc.GenerateReport(ctx)
})

func main() {
    // Cloud Task: a one-shot function wrapped as a single-step job.
    starter.Provide("report", reportStep)

    // Chunk job: multiple typed steps composed into one job.
    starter.Provide("reconcile", &batch.ChunkStep[Row, Row]{
        Name:      "load",
        Reader:    &csvReader{path: "/data/txn.csv"},
        Writer:    &sqlWriter{db: db},
        ChunkSize: 500,
    })

    gs.Run()
}
```

### 3. Declare which jobs run on startup

```properties
# Fire "report" once when the application is ready (Cloud Task).
spring.batch.jobs.report.run-on-startup=true
spring.batch.jobs.report.params.date=2026-07-19

# "reconcile" has no entry: it is launched on demand by another bean
# (a scheduler.Job, an HTTP handler) via the *Launcher bean.
```

A configured job with no matching `JobDefinition` bean is a **fail-fast
startup error**, so a typo surfaces at boot rather than as a job that silently
never fires.

## Two shapes: Cloud Task vs. scheduled batch

The starter supports two ways to trigger a job, matching Spring Batch's two
common patterns:

| Shape        | How it fires                                                            | Config                                                     |
|--------------|------------------------------------------------------------------------|------------------------------------------------------------|
| Cloud Task   | Once when the application becomes ready.                                | `spring.batch.jobs.<name>.run-on-startup=true`             |
| Scheduled    | On every fire of a scheduler trigger (cron / fixed-rate / fixed-delay). | `spring.scheduler.jobs.<name>.cron=…` + inject `*Launcher` |

For the scheduled shape, register a `scheduler.Job` that injects `*Launcher`
and calls `Launch`:

```go
import (
    scheduler "go-spring.org/starter-scheduler"
    starter   "go-spring.org/starter-batch"
    "go-spring.org/spring/cloud/batch"
)

type NightlyReconcile struct {
    Launcher *starter.Launcher `autowire:""`
}

func (n *NightlyReconcile) JobName() string { return "nightly-reconcile" }

func (n *NightlyReconcile) Run(ctx context.Context) error {
    _, err := n.Launcher.Launch(ctx, "reconcile", batch.Params{
        "date": time.Now().Format("2006-01-02"),
    })
    return err
}

func main() {
    gs.Provide(&NightlyReconcile{}).
        Name("nightly-reconcile").
        Export(gs.As[scheduler.Job]())
    gs.Run()
}
```

```properties
spring.scheduler.jobs.nightly-reconcile.cron=0 2 * * *
```

Startup and manual launches share the same repository, so restart semantics are
identical regardless of how a run was triggered.

## Progress store (JobRepository)

The engine records progress through a `batch.JobRepository` bean. The runner
picks one in three steps:

1. If `spring.batch.repository` is set, it must name an existing repo bean.
   Missing is a **fail-fast startup error**.
2. Otherwise, if exactly one repo bean exists, it is used (the common case for
   an app that imports one durable backend, e.g. `starter-batch-redis`).
3. Otherwise, the runner falls back to `batch.NewMemoryRepository()`. This is
   **in-process only** — a crash loses all progress. Fine for tests and demos.

Multiple repo beans without an explicit `spring.batch.repository` is a
**fail-fast startup error** — silently picking one would surprise the operator.

## Graceful shutdown

On `SIGTERM` the runner cancels every in-flight startup launch and waits for
them to finish, bounded by `spring.batch.drain-timeout` (default `30s`) — a
safety net on top of the framework-level `app.shutdown.timeout`. A step that
honours its context returns promptly and leaves the step in `stopped` state,
which is *restartable* on the next boot from the last committed checkpoint.

## Configuration reference

| Key                                            | Default | Description                                                                                              |
|------------------------------------------------|---------|----------------------------------------------------------------------------------------------------------|
| `spring.batch.enabled`                         | `true`  | Enable the runner (active once ≥1 `JobDefinition` bean).                                                 |
| `spring.batch.repository`                      | —       | Name of a `batch.JobRepository` bean to use as the progress store.                                       |
| `spring.batch.drain-timeout`                   | `30s`   | Max time `Stop` waits for in-flight startup launches.                                                    |
| `spring.batch.jobs.<name>.run-on-startup`      | `false` | When `true`, launch the job once after the application is ready.                                         |
| `spring.batch.jobs.<name>.params.<k>`          | —       | Startup launch parameters. Together with the job name they identify the job instance in the repository. |

## Example

See [`example/`](example) for a runnable demo.
