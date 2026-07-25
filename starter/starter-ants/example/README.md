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
go run .
```

预期输出（断言通过后程序自动退出）：
```
Nonblocking pool correctly rejected submit
CPU pool: running: 0 free: 8 cap: 8 waiting: 0
Panic handler fired: 1 times
=== Pool Metrics ===
  io: cap=2 running=0 waiting=0 free=2
  cpu: cap=8 running=0 waiting=0 free=8
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。