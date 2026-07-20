# scheduling
[English](README.md) | [中文](README_CN.md)

`scheduling` is a framework-agnostic, zero-dependency abstraction for periodic
and scheduled background jobs — the Go-idiomatic equivalent of Spring's
`@Scheduled` / `TaskScheduler`. A job is a plain function bound to a
`Trigger`, driven by a `Scheduler` that participates in the application
lifecycle.

## Features

- Zero third-party dependencies; the cron parser lives in this package on
  purpose so stdlib stays self-contained.
- Three built-in triggers: `FixedRate`, `FixedDelay`, `Cron` (5-field
  expression parsed by `ParseCron`).
- `ConcurrencyPolicy` (`Skip`, `Queue`, `Replace`) governs overlapping fires
  for fixed-rate / cron; fixed-delay is intrinsically serial.
- Per-run `WithTimeout` cancels the job's context after the deadline.
- `WithLock(locker, key)` — multi-replica de-duplication via a minimal local
  `Locker` interface. A `spring/lock.Locker` is adapted by the integration
  layer (`starter-scheduler`) so this package stays dependency-free.
- Panic-guarded runs, deterministic drain on `Stop`, optional `Observer` hook
  for metrics / logging.

## Quick Start

Import path: `go-spring.org/spring/scheduling`.

```go
package main

import (
    "context"
    "log"
    "time"

    "go-spring.org/spring/scheduling"
)

func main() {
    sch := scheduling.NewScheduler(scheduling.WithObserver(func(e scheduling.Event) {
        if e.Err != nil {
            log.Printf("job %s failed: %v", e.Name, e.Err)
        }
    }))

    _, err := sch.Schedule("heartbeat", scheduling.FixedRate(10*time.Second),
        func(ctx context.Context) error {
            log.Println("tick")
            return nil
        },
        scheduling.WithConcurrencyPolicy(scheduling.Skip),
        scheduling.WithTimeout(3*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    _ = sch.Start(ctx)
    time.Sleep(35 * time.Second)
    cancel()
    _ = sch.Stop(context.Background())
}
```

For a cron schedule use `scheduling.Cron("*/5 * * * *")` (panics on bad
expression) or `scheduling.ParseCron` when you want to handle the error. For
multi-replica de-duplication use `starter-scheduler`, which adapts
`spring/lock.Locker` and bakes TTL / renew options into the local `Locker`
this package expects.
