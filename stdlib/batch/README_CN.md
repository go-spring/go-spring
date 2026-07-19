# batch
[English](README.md) | [中文](README_CN.md)

`batch` 是零依赖的批处理 / 短任务抽象——Go 惯用法版的 Spring Batch + Spring
Cloud Task。[`Job`](job.go) 是有序的 [`Step`](job.go) 列表;
[`ChunkStep`](chunk.go) 按分块读→处理→写→提交;[`JobRepository`](repository.go)
持久化进度,崩溃后从最近一次提交的 chunk 恢复。

## 特性

- 泛型 `Reader[T]` / `Processor[I,O]` / `Writer[T]`——纯接口,无 XML、无注解。
- `ChunkStep[I, O]`——按 chunk 读、处理、写、提交;已提交的 chunk 重启后不
  重读。
- `Func(name, fn)`——把一次性函数包成单步 Cloud Task job。
- `JobRepository` 缝隙 + 内建 `NewMemoryRepository()`(测试用);持久化后端由
  starter 贡献 bean(见 `starter-batch-redis`)。
- Reader 可实现 `Checkpointer` 持久化位置;Reader/Writer 可实现 `Closer` 释放
  资源。
- 韧性:设置 `ChunkStep.Retry` 或注入 `resilience.Executor`,为每个 chunk 加
  重试 / 熔断。

## 用法

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/stdlib/batch"
)

func main() {
    items := []int{1, 2, 3, 4, 5}
    i := 0

    step := &batch.ChunkStep[int, int]{
        Name:      "double",
        ChunkSize: 2,
        Reader: batch.ReaderFunc[int](func(ctx context.Context) (int, bool, error) {
            if i >= len(items) {
                return 0, false, nil
            }
            v := items[i]
            i++
            return v, true, nil
        }),
        Processor: batch.ProcessorFunc[int, int](func(_ context.Context, v int) (int, bool, error) {
            return v * 2, true, nil
        }),
        Writer: batch.WriterFunc[int](func(_ context.Context, out []int) error {
            fmt.Println("wrote:", out)
            return nil
        }),
    }

    job := &batch.Job{Name: "doubler", Steps: []batch.Step{step}}
    _, _ = job.Run(context.Background(), batch.NewMemoryRepository(), batch.Params{"date": "2026-07-19"})
}
```

## 重启语义

- 提交发生在 chunk 写入成功**之后**。写完还没提交就崩溃,重启会重放这个
  chunk——writer 必须幂等才能做到 exactly-once。
- 已完成的 step 重启时跳过;被中断的 step 从持久化的 `Checkpoint` 处恢复
  (若 reader 实现了 `Checkpointer`)。
- 同名 + 同 params 的两次运行是同一个 instance;第二次调用会恢复第一次的
  执行。
