# starter-batch Example

演示 starter-batch 的批处理任务执行与断点续传。

## 功能验证

- **两阶段执行**：Phase 1 崩溃模拟断点，Phase 2 从断点恢复
- **断点续传**：崩溃后重启从上次 checkpoint 继续执行
- **Job 注册**：通过 `JobDefinition` 接口注册批处理任务

## 手动验证

```bash
cd starter-batch/example
go run . -manual
```

预期输出（两阶段自动执行，Phase 2 从断点恢复并完成）：
```
phase 1: ...
phase 2: ...
```

服务保持运行，可以用对应 CLI 工具验证。`Ctrl+C` 退出服务。

需要 Docker 环境运行完整冒烟测试（依赖 MySQL）。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 MySQL，运行示例并验证断点续传，退出码 0 表示通过。