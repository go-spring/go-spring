# goutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`goutil` 是一个薄封装，用于以带 panic 恢复的方式
启动 goroutine，并可选地捕获返回值。

## 1. 职责与边界

- 启动一个不会因 panic 而拖垮进程的 goroutine —— 被 recover 的 panic 会通过
  全局 `OnPanic` 钩子上报。
- 通过 `Wait()` 提供 join 语义，让调用方不必手动写 channel 或
  `sync.WaitGroup`。
- 同时覆盖 `func(ctx)`（`Go`）与 `func(ctx) (T, error)`（`GoValue`）两种签名。
- 不是 worker 池、信号量或取消框架。`errgroup`、`semaphore` 之类留在
  `golang.org/x/sync`。

## 2. 关键缝隙

- **全局 `OnPanic` 回调**：一个包级 `var`，应用在初始化时覆盖它以接入日志 /
  监控栈。选择"变量"而不是 setter/getter，是因为整个进程只有一个配置点，
  set-once 已经够用。
- **`CancelMode`**：`InheritCancel` 透传原 context；`DetachCancel` 用
  `context.WithoutCancel` 包装，让 goroutine 生命周期超过发起者。每个调用点
  必须显式指定，不设"默认"避免行为默默改变。
- **`Status` / `ValueStatus[T]`**：返回句柄基于单次 `close(chan)` 完成同步。
  `ValueStatus[T].Wait` 还会把恢复到的 panic 转成 error 返回，所以
  `GoValue` 调用方只需要看一个错误通道，无论失败来自 `return err` 还是
  `panic`。

## 3. 约束

- 取消是协作式的。被启动的函数必须自己观察 `ctx.Done()`；`goutil` 不会
  强杀 goroutine。
- `OnPanic` 在 recover 后的同一个 goroutine 内执行 —— 慢钩子或钩子本身
  panic 都会挡住 / 拖垮它本应观察的关停路径。应用需要保持它简短，且不能
  panic。
- 默认 `OnPanic` 直接 `fmt.Printf` 到 stdout。这是给测试和小程序用的零
  配置行为；正式服务必须覆盖。
