# event
[English](README.md) | [中文](README_CN.md)

`event` is an in-process publish/subscribe bus — the Go-idiomatic equivalent
of Spring's `ApplicationEventPublisher` / `@EventListener`. Modules communicate
by publishing typed events instead of calling each other directly, keeping
producers and consumers decoupled.

## Features

- Zero third-party dependencies; part of the stdlib foundation layer.
- Any concrete struct is an event — no marker interface or annotation scanning.
- Type-safe generic subscription (`Subscribe[T]` / `SubscribeAsync[T]`); the
  only reflection is a single type key used to route a published value to the
  handlers registered for that exact dynamic type.
- Synchronous handlers run inline in deterministic order (`WithOrder`); their
  errors are combined with `errors.Join` so one faulty subscriber cannot
  silently suppress the rest.
- Asynchronous handlers run on dedicated buffered workers (`WithBuffer`,
  `WithErrorHandler`); a slow handler never stalls the publisher.
- Graceful `Close`: async workers drain events already buffered before exiting.
- nil / empty transparent: publishing without subscribers is a no-op; a nil bus
  yields a no-op cancel.
- Optional container integration through the `Listener` interface — a bean
  exported as `event.Listener` is collected and its `Register(bus)` is invoked
  once, mirroring `health.Indicator`'s Export-based collection.

## Quick Start

Import path: `go-spring.org/spring/event`.

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/cloud/event"
)

type ConfigChanged struct{ Key, Value string }

func main() {
    bus := event.New()
    defer bus.Close()

    cancel := event.Subscribe(bus, func(ctx context.Context, e ConfigChanged) error {
        fmt.Printf("reload %s=%s\n", e.Key, e.Value)
        return nil
    })
    defer cancel()

    _ = bus.Publish(context.Background(), ConfigChanged{Key: "log.level", Value: "debug"})
}
```

Container-managed listeners implement `event.Listener` and export the
interface; a registrar collects them and calls `Register(bus)` after wiring.
Every exported listener bean must be given a distinct name to avoid the
`__default__` conflict when several beans share one Export.
