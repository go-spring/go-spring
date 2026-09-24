# timeutil

[English](README.md) | [中文](README_CN.md)

`timeutil` provides small time-related helpers the Go standard library does
not — currently `Sleep`, the context-aware counterpart of `time.Sleep`.

## Sleep

`Sleep(ctx, d)` waits for `d` or until `ctx` is done and reports whether the
full duration elapsed. A goroutine pacing a retry or re-poll loop sleeps in
cancellable units, so shutdown does not wait out a long sleep that no longer
matters:

```go
if !timeutil.Sleep(ctx, backoff) {
    return ctx.Err()
}
```
