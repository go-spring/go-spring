# starter-websocket-coder Example

[English](README.md) | [中文](README_CN.md)

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

终端 2，使用 websocat 测试。所有路由都被 `requireApp` 中间件保护，
`X-App: go-spring` 头是必填的：
```bash
# /echo 文本回声（协商 echo.v1 子协议）
websocat ws://127.0.0.1:9797/echo -H "X-App: go-spring" --protocol echo.v1
# 输入任意文本，收到回声

# /json JSON 回声（输入 {"name":"world"} -> {"message":"Hi, world"}）
websocat ws://127.0.0.1:9797/json -H "X-App: go-spring"

# 不带头则握手被拒，返回 HTTP 403
websocat ws://127.0.0.1:9797/echo
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。