# starter-scheduler Example

Demonstrates scheduled task scheduling.

## Features

- **Cron Scheduled Tasks**: `tick` task executes periodically according to a cron expression
- **Delayed Tasks**: `delay` task executes after a delay
- **Distributed Lock**: Ensures single-instance task execution via locker

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