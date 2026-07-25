# starter-websocket Example

演示 starter-websocket 的 WebSocket 文本/JSON 回声与中间件鉴权。

## 功能验证

- **文本回声**：`/echo` 端点接收文本消息并原样返回
- **JSON 回声**：`/json` 端点接收 JSON 消息并返回问候语
- **中间件鉴权**：`X-App: go-spring` 头校验，缺少时返回 403 Forbidden

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-websocket/example
go run . -manual
```

终端 2，使用 websocat 测试：
```bash
websocat ws://127.0.0.1:9696/echo
# 输入任意文本，收到回声
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。