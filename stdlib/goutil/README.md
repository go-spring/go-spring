# goutil

[English](README.md) | [中文](README_CN.md)

Run goroutines safely with built-in panic recovery.

## Features

- **Panic Recovery**: Automatically captures panics in goroutines to prevent crashes
- **Global Callback**: Provides `OnPanic` callback for logging, metrics, or alerting
- **Context Control**: Supports flexible context cancellation modes (inherit/detach)
- **Return Value Support**: Captures return values and errors from goroutines
- **Synchronization**: Provides `Wait()` method to wait for goroutine completion

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/go-spring/stdlib/goutil"
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

**DetachCancel**: Child goroutine不受 parent context cancellation

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
// err contains panic info and stack trace
fmt.Printf("value: %q, error: %v\n", value, err)
```

## Important Notes

### 1. Context Cancellation is Cooperative

Goroutines don't automatically respond to context cancellation; you must check explicitly in the function.

**Wrong**: Does not respond to cancellation

```go
goutil.Go(ctx, func(ctx context.Context) {
    time.Sleep(time.Hour) // continues even if ctx is cancelled
}, goutil.InheritCancel)
```

**Right**: Actively checks cancellation

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

### 2. Defer Always Executes

Even when a panic occurs, defer statements execute normally:

```go
goutil.Go(context.Background(), func(ctx context.Context) {
    file, _ := os.Open("data.txt")
    defer file.Close() // executes even on panic
    
    processData(file) // may panic
}, goutil.InheritCancel)
```

## Typical Use Cases

### 1. Web Server Background Tasks

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

### 2. Scheduled Tasks

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

### 3. Batch Concurrent Processing

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

## Comparison with Other Approaches

| Approach | Panic Recovery | Context Control | Return Values | Synchronization |
|----------|---------------|-----------------|---------------|-----------------|
| go func() | ❌ | ✅ | ✅ | ❌ |
| errgroup.Group | ❌ | ✅ | ✅ | ✅ |
| **goutil** | ✅ | ✅ | ✅ | ✅ |

## License

Apache License 2.0
