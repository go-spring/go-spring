# goutil

[English](README.md) | [中文](README_CN.md)

安全地运行 goroutine，内置 panic 恢复机制。

## 功能特性

- **Panic 恢复**：自动捕获 goroutine 中的 panic，防止程序崩溃
- **全局回调**：提供 `OnPanic` 回调函数，用于记录日志、上报监控或触发告警
- **Context 控制**：支持灵活的 context 取消模式（继承/分离）
- **返回值支持**：支持获取 goroutine 的返回值和错误
- **同步等待**：提供 `Wait()` 方法等待 goroutine 完成

## 使用示例

### 基本用法

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/go-spring/stdlib/goutil"
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
// err 包含 panic 信息和堆栈跟踪
fmt.Printf("value: %q, error: %v\n", value, err)
```

## 重要提示

### 1. Context 取消是协作式的

goroutine 不会自动响应 context 取消，需要在函数内部主动检查。

**错误示范**：不会响应取消

```go
goutil.Go(ctx, func(ctx context.Context) {
    time.Sleep(time.Hour) // 即使 ctx 被取消，也会继续执行
}, goutil.InheritCancel)
```

**正确示范**：主动检查取消

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

### 2. Defer 始终执行

即使发生 panic，defer 语句仍会正常执行：

```go
goutil.Go(context.Background(), func(ctx context.Context) {
    file, _ := os.Open("data.txt")
    defer file.Close() // panic 时也会执行
    
    processData(file) // 可能 panic
}, goutil.InheritCancel)
```

## 典型应用场景

### 1. Web 服务器后台任务

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

### 2. 定时任务

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

### 3. 批量并发处理

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

## 与其他方案的对比

| 方案 | Panic 恢复 | Context 控制 | 返回值 | 同步等待 |
|------|-----------|-------------|--------|---------|
| go func() | ❌ | ✅ | ✅ | ❌ |
| errgroup.Group | ❌ | ✅ | ✅ | ✅ |
| **goutil** | ✅ | ✅ | ✅ | ✅ |

## 许可证

Apache License 2.0
