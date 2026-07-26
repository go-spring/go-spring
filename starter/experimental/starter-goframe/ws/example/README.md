# starter-goframe/ws Example

演示 starter-goframe/ws 的 WebSocket 文本回声服务。

## 功能验证

- **WebSocket 回声**：发送 `ping`，接收 `ping`

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-goframe/ws/example
go run . -manual
```

终端 2，使用 websocat 测试：
```bash
websocat ws://127.0.0.1:8002/echo
# 输入 ping，收到 ping
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。