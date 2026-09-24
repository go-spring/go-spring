# starter-scheduler Example

Demonstrates scheduled task scheduling.

## Features

- **Cron Scheduled Tasks**: `tick` task executes periodically according to a cron expression
- **Delayed Tasks**: `delay` task executes after a delay
- **Distributed Lock**: Ensures single-instance task execution via locker
- **Struct Job with Dependency Injection**: `report` task is a struct implementing
  `scheduler.Job` — its constructor receives config bound from `spring.example.*`
  (via `gs.TagArg`), and the bean's `Condition` reads the `report-enabled` switch.
  This is the form to reach for when a job needs beans or bound configuration:
  a closure registered with `scheduler.Provide` runs before the container, so it
  can never capture either.

## Manual Testing

```bash
cd starter-scheduler/example
go run . -manual
```

The service keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.