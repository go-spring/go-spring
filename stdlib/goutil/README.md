# goutil

[English](README.md) | [中文](README_CN.md)

`goutil` launches goroutines with built-in panic recovery, so a panic inside a
goroutine no longer crashes the process; the recovered panic is routed to a
global `OnPanic` callback for logging, metrics, or alerting. It covers both
`func(ctx)` (`Go`) and `func(ctx) (T, error)` (`GoValue`) shapes, and every
launch returns a handle with `Wait()` for joining without hand-rolled channels
or `sync.WaitGroup` bookkeeping. Two sibling entry points complete the story:
`SafeRun` runs a function synchronously and converts a panic into an error on
the normal return path, and `ReportPanic` reports an already-recovered panic
value (from your own recover sites) through the same `OnPanic` callback.

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "go-spring.org/stdlib/goutil"
)

func main() {
    // Launch a goroutine with panic recovery
    status := goutil.Go(context.Background(), func(ctx context.Context) {
        fmt.Println("goroutine is running...")
        time.Sleep(100 * time.Millisecond)
        fmt.Println("goroutine completed")
    }, goutil.InheritCancel)

    // Wait for goroutine to complete
    status.Wait()
}
```

### Panic Recovery and Custom Handling

```go
// Customize panic handling logic (e.g., logging, monitoring)
goutil.OnPanic = func(ctx context.Context, info goutil.PanicInfo) {
    // ctx can carry request ID, trace info, etc.
    log.Printf("[PANIC] recovered panic: %v\nStack trace:\n%s", info.Panic, info.Stack)
}

// Program won't crash even if panic occurs
goutil.Go(context.Background(), func(ctx context.Context) {
    panic("something went wrong!")
}, goutil.InheritCancel).Wait()
```

### Context Cancellation Modes

**InheritCancel** (default): Child goroutine inherits parent context's cancellation

```go
ctx, cancel := context.WithCancel(context.Background())

goutil.Go(ctx, func(ctx context.Context) {
    select {
    case <-time.After(time.Second):
        fmt.Println("task completed")
    case <-ctx.Done():
        fmt.Println("task cancelled")
    }
}, goutil.InheritCancel)

// Cancel context after 50ms
time.Sleep(50 * time.Millisecond)
cancel() // child goroutine receives cancellation signal
```

**DetachCancel**: the child goroutine is not affected by parent context cancellation.

```go
ctx, cancel := context.WithCancel(context.Background())

goutil.Go(ctx, func(ctx context.Context) {
    // This goroutine continues even if parent context is cancelled
    time.Sleep(time.Second)
    fmt.Println("task completed, unaffected by parent context cancellation")
}, goutil.DetachCancel)

cancel() // does not affect child goroutine
```

### Getting Return Values (GoValue)

```go
result := goutil.GoValue(context.Background(), func(ctx context.Context) (int, error) {
    // Simulate time-consuming operation
    time.Sleep(100 * time.Millisecond)
    return 42, nil
}, goutil.InheritCancel)

// Wait and get result
value, err := result.Wait()
if err != nil {
    log.Printf("error: %v", err)
    return
}
fmt.Printf("result: %d\n", value)
```

### Panic Converted to Error

```go
value, err := goutil.GoValue(context.Background(), func(ctx context.Context) (string, error) {
    panic("runtime error")
}, goutil.InheritCancel).Wait()

// value is empty string (zero value of type T)
// err contains the panic value; the full stack goes to OnPanic
fmt.Printf("value: %q, error: %v\n", value, err)
```

### Important Notes

**1. Context cancellation is cooperative** — goroutines don't automatically
respond to cancellation; you must check explicitly in the function.

Wrong (does not respond to cancellation):

```go
goutil.Go(ctx, func(ctx context.Context) {
    time.Sleep(time.Hour) // continues even if ctx is cancelled
}, goutil.InheritCancel)
```

Right (actively checks cancellation):

```go
goutil.Go(ctx, func(ctx context.Context) {
    select {
    case <-time.After(time.Hour):
        // complete task
    case <-ctx.Done():
        // cleanup and exit
        return
    }
}, goutil.InheritCancel)
```

**2. Defer always executes** — even when a panic occurs:

```go
goutil.Go(context.Background(), func(ctx context.Context) {
    file, _ := os.Open("data.txt")
    defer file.Close() // executes even on panic

    processData(file) // may panic
}, goutil.InheritCancel)
```

### Typical Use Cases

Web server background tasks:

```go
http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
    // Asynchronously process uploaded file
    goutil.Go(r.Context(), func(ctx context.Context) {
        // process file...
        // won't crash server even if panic occurs
    }, goutil.DetachCancel)

    w.WriteHeader(http.StatusAccepted)
})
```

Scheduled tasks:

```go
go func() {
    ticker := time.NewTicker(time.Minute)
    for range ticker.C {
        goutil.Go(context.Background(), func(ctx context.Context) {
            runScheduledTask(ctx)
        }, goutil.InheritCancel)
    }
}()
```

Batch concurrent processing:

```go
func ProcessBatch(items []Item) error {
    var wg sync.WaitGroup
    results := make(chan Result, len(items))

    for _, item := range items {
        wg.Add(1)
        go func(it Item) {
            defer wg.Done()
            res, err := goutil.GoValue(context.Background(), func(ctx context.Context) (Result, error) {
                return processItem(it), nil
            }, goutil.InheritCancel).Wait()

            if err != nil {
                log.Printf("processing failed: %v", err)
                return
            }
            results <- res
        }(item)
    }

    wg.Wait()
    close(results)
    return nil
}
```

### API

- `Go(ctx context.Context, f func(ctx context.Context), mode CancelMode) *Status` — launch a goroutine with panic recovery; `Status.Wait()` joins it
- `GoValue[T any](ctx context.Context, f GoValueFunc[T], mode CancelMode) *ValueStatus[T]` — same, capturing `(T, error)`; a recovered panic comes back as an error from `Wait()`
- `SafeRun(ctx context.Context, f func(ctx context.Context) error) error` — run `f` synchronously, converting a panic into the returned error
- `ReportPanic(ctx context.Context, recovered any)` — report an already-recovered panic value to `OnPanic` (use at your own recover sites)
- `OnPanic func(ctx context.Context, info PanicInfo)` — global panic callback; default prints to stdout
- `CancelMode`: `InheritCancel` (pass the context through) / `DetachCancel` (`context.WithoutCancel`)

## Design

`goutil` is a thin wrapper, not
a concurrency framework.

- **Global `OnPanic` seam**: a package-level `var` that applications overwrite
  during initialization to plug into their logging / metrics stack. A
  variable rather than a getter/setter pair, because there is exactly one
  configuration point and set-once is enough.
- **Explicit `CancelMode`**: `InheritCancel` passes the context through;
  `DetachCancel` wraps it with `context.WithoutCancel` so the goroutine
  outlives its launcher. Chosen at every call site — no "default" that
  quietly changes behavior.
- **`Status` / `ValueStatus[T]`**: both handles synchronize on a single
  `close(chan)`. `ValueStatus[T].Wait` also surfaces the recovered panic as
  an error, so `GoValue` callers see one error channel whether the failure
  came from `return err` or a `panic`.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
