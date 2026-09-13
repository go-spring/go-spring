# starter-scheduler Example

演示 starter-scheduler 的定时任务调度。

## 功能验证

- **Cron 定时任务**：`tick` 任务按 cron 表达式定期执行
- **延迟任务**：`delay` 任务延迟执行
- **分布式锁**：通过 locker 确保任务单实例执行

## 手动验证

```bash
cd starter-scheduler/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。