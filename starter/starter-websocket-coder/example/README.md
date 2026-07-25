# starter-websocket-coder Example

演示 starter-websocket-coder 的 WebSocket 文本/JSON 回声与中间件鉴权（使用 coder/websocket 库）。

## 功能验证

- **文本回声**：`/echo` 端点接收文本消息并原样返回，含子协议协商
- **JSON 回声**：`/json` 端点接收 JSON 消息并返回问候语
- **中间件鉴权**：`X-App: go-spring` 头校验，缺少时返回 403 Forbidden

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-websocket-coder/example
go run . -manual
```

终端 2，使用 websocat 测试：
```bash
websocat ws://127.0.0.1:9797/echo
# 输入任意文本，收到回声
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。