# starter-pulsar Example

演示 starter-pulsar 的 Apache Pulsar 消息客户端。

## 功能验证

- **消息发布**：向 Pulsar topic 发送消息
- **消息消费**：订阅 topic 并接收消息
- **消息确认**：消费后确认（ack）

> 需要 Pulsar 服务运行。`check.sh` 通过 docker compose 启动 Pulsar。

## 手动验证

```bash
cd starter-pulsar/example
go run .
```

预期输出：
```
subscribed
published
message received
```

需要先启动 Pulsar：
```bash
# 启动 Pulsar
docker compose up -d

# 等待 Pulsar 就绪
sleep 15

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Pulsar，运行示例并验证消息，退出码 0 表示通过。