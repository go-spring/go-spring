# goutil

[English](README.md) | [中文](README_CN.md)

`goutil` 以内置 panic 恢复的方式启动 goroutine，goroutine 内的 panic 不再拖垮进程；
被 recover 的 panic 通过全局 `OnPanic` 回调上报，用于记录日志、上报监控或触发告警。
包内覆盖 `func(ctx)`（`Go`）与 `func(ctx) (T, error)`（`GoValue`）两种签名，每次启动都
返回带 `Wait()` 的句柄，无需手写 channel 或维护 `sync.WaitGroup`。另有两个兄弟入口补全
这条链路：`SafeRun` 同步执行函数并把 panic 转成正常返回路径上的 error；`ReportPanic`
把你自己的 recover 站点拿到的 panic 值经同一个 `OnPanic` 回调上报。

## 使用方式

### 基本用法

```go
package main

import (
    "context"
    "fmt"
    "time"

    "go-spring.org/stdlib/goutil"
)

func main() {
    // 启动一个带 panic 恢复的 goroutine
    status := goutil.Go(context.Background(), func(ctx context.Context) {
        fmt.Println("goroutine 正在运行...")
        time.Sleep(100 * time.Millisecond)
        fmt.Println("goroutine 完成")
    }, goutil.InheritCancel)

    // 等待 goroutine 完成
    status.Wait()
}
```

### Panic 恢复与自定义处理

```go
// 自定义 panic 处理逻辑（如记录日志、上报监控）
goutil.OnPanic = func(ctx context.Context, info goutil.PanicInfo) {
    // ctx 可用于传递请求 ID、追踪信息等上下文
    log.Printf("[PANIC] 捕获到 panic: %v\n堆栈信息:\n%s", info.Panic, info.Stack)
}

// 即使发生 panic，程序也不会崩溃
goutil.Go(context.Background(), func(ctx context.Context) {
    panic("出错了!")
}, goutil.InheritCancel).Wait()
```

### Context 取消模式

**InheritCancel**（默认）：子 goroutine 继承父 context 的取消信号

```go
ctx, cancel := context.WithCancel(context.Background())

goutil.Go(ctx, func(ctx context.Context) {
    select {
    case <-time.After(time.Second):
        fmt.Println("任务完成")
    case <-ctx.Done():
        fmt.Println("任务被取消")
    }
}, goutil.InheritCancel)

// 50ms 后取消 context
time.Sleep(50 * time.Millisecond)
cancel() // 子 goroutine 会收到取消信号
```

**DetachCancel**：子 goroutine 不受父 context 取消影响

```go
ctx, cancel := context.WithCancel(context.Background())

goutil.Go(ctx, func(ctx context.Context) {
    // 即使父 context 被取消，这个 goroutine 仍会继续执行
    time.Sleep(time.Second)
    fmt.Println("任务完成，不受父 context 取消影响")
}, goutil.DetachCancel)

cancel() // 不会影响子 goroutine
```

### 获取返回值（GoValue）

```go
result := goutil.GoValue(context.Background(), func(ctx context.Context) (int, error) {
    // 模拟耗时操作
    time.Sleep(100 * time.Millisecond)
    return 42, nil
}, goutil.InheritCancel)

// 等待并获取结果
value, err := result.Wait()
if err != nil {
    log.Printf("错误：%v", err)
    return
}
fmt.Printf("结果：%d\n", value)
```

### Panic 转换为错误

```go
value, err := goutil.GoValue(context.Background(), func(ctx context.Context) (string, error) {
    panic("运行时错误")
}, goutil.InheritCancel).Wait()

// value 是空字符串（类型 T 的零值）
// err 包含 panic 值；完整堆栈经 OnPanic 上报
fmt.Printf("value: %q, error: %v\n", value, err)
```

### 重要提示

**1. Context 取消是协作式的** —— goroutine 不会自动响应取消，需要在函数内部主动检查。

错误示范（不会响应取消）：

```go
goutil.Go(ctx, func(ctx context.Context) {
    time.Sleep(time.Hour) // 即使 ctx 被取消，也会继续执行
}, goutil.InheritCancel)
```

正确示范（主动检查取消）：

```go
goutil.Go(ctx, func(ctx context.Context) {
    select {
    case <-time.After(time.Hour):
        // 完成任务
    case <-ctx.Done():
        // 清理并退出
        return
    }
}, goutil.InheritCancel)
```

**2. Defer 始终执行** —— 即使发生 panic：

```go
goutil.Go(context.Background(), func(ctx context.Context) {
    file, _ := os.Open("data.txt")
    defer file.Close() // panic 时也会执行

    processData(file) // 可能 panic
}, goutil.InheritCancel)
```

### 典型应用场景

Web 服务器后台任务：

```go
http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
    // 异步处理上传的文件
    goutil.Go(r.Context(), func(ctx context.Context) {
        // 处理文件...
        // 即使 panic 也不会导致服务器崩溃
    }, goutil.DetachCancel)

    w.WriteHeader(http.StatusAccepted)
})
```

定时任务：

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

批量并发处理：

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
                log.Printf("处理失败：%v", err)
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

### API 列表

- `Go(ctx context.Context, f func(ctx context.Context), mode CancelMode) *Status` —— 启动带 panic 恢复的 goroutine，`Status.Wait()` 等待结束
- `GoValue[T any](ctx context.Context, f GoValueFunc[T], mode CancelMode) *ValueStatus[T]` —— 同上，但捕获 `(T, error)`；panic 被恢复后转成 `Wait()` 返回的 error
- `SafeRun(ctx context.Context, f func(ctx context.Context) error) error` —— 同步执行 `f`，把 panic 转成返回的 error
- `ReportPanic(ctx context.Context, recovered any)` —— 把已 recover 的 panic 值上报给 `OnPanic`（用于自己的 recover 站点）
- `OnPanic func(ctx context.Context, info PanicInfo)` —— 全局 panic 回调，默认打印到 stdout
- `CancelMode`：`InheritCancel`（透传 context）/ `DetachCancel`（`context.WithoutCancel`）

## 关键设计

`goutil` 是 goroutine 启动的薄封装，不是并发框架。

- **全局 `OnPanic` 缝隙**：一个包级 `var`，应用在初始化时覆盖它以接入日志 / 监控栈。
  刻意选择"变量"而不是 getter/setter：整个进程只有一个配置点，set-once 已经够用。
- **显式 `CancelMode`**：`InheritCancel` 透传原 context；`DetachCancel` 用
  `context.WithoutCancel` 包装，让 goroutine 生命周期超过发起者。每个调用点必须显式
  指定，不设"默认"，避免行为默默改变。
- **`Status` / `ValueStatus[T]`**：两个句柄都基于单次 `close(chan)` 完成同步。
  `ValueStatus[T].Wait` 还会把恢复到的 panic 转成 error 返回，所以 `GoValue` 调用方
  只需要看一个错误通道，无论失败来自 `return err` 还是 `panic`。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
