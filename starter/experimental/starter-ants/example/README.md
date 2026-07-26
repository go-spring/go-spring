# starter-ants Example

Goroutine pool management for starter-ants.

## Features

- **Concurrent Task Submission**: Submit 100 tasks to the CPU pool and verify all complete
- **Instance Isolation**: IO pool and CPU pool have independent capacities (2 vs 8)
- **Non-blocking Overload Protection**: IO pool capacity is 2; submitting when full in non-blocking mode returns `ErrPoolOverload`
- **Pool Metrics**: Running/Free/Cap/Waiting metrics are exposed
- **Panic Handling**: Capture task panics via `SetPanicHandler`; the worker does not crash
- **MetricsObserver**: Aggregates metric snapshots across all pools

## Manual Testing

```bash
cd starter-ants/example
go run . -manual
```

The program keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` runs automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.