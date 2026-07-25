# starter-goframe/ws Example

演示 starter-goframe/ws 的 WebSocket 文本回声服务。

## 功能验证

- **WebSocket 回声**：发送 `ping`，接收 `ping`

## 手动验证

```bash
cd starter-goframe/ws/example
go run .
```

预期输出：
```
Response from server: ping
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。