# starter-otel Example

演示 starter-otel 的 OpenTelemetry 链路追踪与日志关联。

## 功能验证

- **Trace ↔ Log 关联**：创建 Span 后，日志自动携带 trace_id/span_id
- **Span 导出**：Span 正确记录并导出到配置的 exporter

## 手动验证

```bash
cd starter-otel/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
trace_id and span_id present in log
span exported
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。