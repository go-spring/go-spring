# goutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `goutil` is a thin, safe way to
launch goroutines with panic recovery and an optional return value.

## 1. Responsibilities & Boundaries

- Launch a goroutine that will not crash the process on panic — the recovered
  panic is routed to a global `OnPanic` hook.
- Give the caller a `Wait()` handle so goroutines can be joined without
  hand-rolling channels or `sync.WaitGroup` bookkeeping.
- Cover both `func(ctx)` (`Go`) and `func(ctx) (T, error)` (`GoValue`) shapes.
- Not a worker pool, semaphore, or cancellation library. `errgroup`,
  `semaphore`, and friends stay in `golang.org/x/sync`.

## 2. Key Seams

- **Global `OnPanic` callback**: single package-level `var` that applications
  overwrite during initialization to plug into their logging / metrics stack.
  Kept a variable (not a getter/setter) because there is exactly one point of
  configuration and set-once is enough.
- **`CancelMode`**: `InheritCancel` passes the caller's context through;
  `DetachCancel` wraps it with `context.WithoutCancel` so the goroutine
  outlives its launcher. The choice is explicit at every call site — no
  "default" that quietly changes behaviour.
- **`Status` / `ValueStatus[T]`**: return handles built around a single
  `close(chan)` for completion. `ValueStatus[T].Wait` also surfaces the
  recovered panic as an error, so `GoValue` callers see one error channel
  regardless of whether the failure came from `return err` or `panic`.

## 3. Constraints

- Cancellation is cooperative. The launched function must observe
  `ctx.Done()`; `goutil` never kills a goroutine.
- `OnPanic` runs inside the recovering goroutine — a slow or panicking hook
  will stall / kill the very shutdown path it is supposed to observe.
  Applications should keep it cheap and never let it panic.
- The default `OnPanic` writes to stdout via `fmt.Printf`. This is
  deliberately zero-config for tests and small programs; anything serious
  should override it.
