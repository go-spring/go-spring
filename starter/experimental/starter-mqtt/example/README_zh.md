# starter-mqtt Example

演示 starter-mqtt 的 MQTT 客户端。

## 功能验证

- **连接状态**：验证 MQTT broker 连接健康
- **消息发布**：向 MQTT topic 发布消息
- **消息订阅**：订阅 topic 并接收消息

> 需要 MQTT Broker 运行。`check.sh` 通过 docker compose 启动 Mosquitto。

## 手动验证

```bash
cd starter-mqtt/example
go run . -manual
```

预期输出：
```
connected
published
message received
```

需要先启动 MQTT Broker：
```bash
# 启动 Mosquitto
docker compose up -d

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，可以用对应 CLI 工具验证。`Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Mosquitto，运行示例并验证消息，退出码 0 表示通过。
