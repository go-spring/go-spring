# timeutil

[English](README.md) | [中文](README_CN.md)

`timeutil` 提供 Go 标准库没有的小型时间相关工具——目前是 `Sleep`，即
`time.Sleep` 的 context 感知版本。

## Sleep

`Sleep(ctx, d)` 等待 `d` 或直到 `ctx` 结束，返回是否等满了时长。给重试/轮询
循环定步的 goroutine 以"可取消的睡眠"为单位等待，停机时不必陪一段已无意义的
长睡眠干等：

```go
if !timeutil.Sleep(ctx, backoff) {
    return ctx.Err()
}
```
