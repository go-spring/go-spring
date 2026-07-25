# starter-http-client Example

演示 starter-http-client 的 HTTP 客户端，含服务发现与链路追踪。

## 功能验证

- **直连模式**：通过固定地址调用后端
- **服务发现**：通过 discovery 查找服务实例
- **链路追踪**：集成 OpenTelemetry，自动传播 traceparent
- **请求/响应**：验证 Echo 往返

## 手动验证

```bash
cd starter-http-client/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。