# starter-otel Example

演示 starter-otel 的 OpenTelemetry 链路追踪与日志关联。

## 功能验证

- **Trace ↔ Log 关联**：创建 Span 后，日志自动携带 trace_id/span_id
- **Span 导出**：Span 正确记录并导出到配置的 exporter

## 手动验证

```bash
cd starter-otel/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。