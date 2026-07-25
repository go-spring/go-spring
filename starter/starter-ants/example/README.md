# starter-ants Example

演示 starter-ants 的 goroutine 池管理。

## 功能验证

- **并发任务提交**：向 CPU 池提交 100 个任务并验证全部执行
- **实例隔离**：io 池和 cpu 池独立配置，容量不同（2 vs 8）
- **非阻塞过载保护**：io 池容量 2，非阻塞模式下满池提交返回 `ErrPoolOverload`
- **池指标**：Running/Free/Cap/Waiting 指标可读
- **Panic 处理**：通过 `SetPanicHandler` 捕获任务 panic，不崩溃 worker
- **MetricsObserver**：聚合所有池的指标快照

## 手动验证

```bash
cd starter-ants/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。