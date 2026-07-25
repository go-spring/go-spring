# starter-kafka-sarama Example

演示 starter-kafka-sarama 的 Kafka Sarama 客户端（消息生产与消费）。

## 功能验证

- **消息发布**：通过 Sarama 客户端向 Kafka topic 发送消息
- **消息消费**：从 Kafka topic 消费消息并验证内容

> 需要 Kafka 服务运行。`check.sh` 通过 docker compose 启动 Kafka。

## 手动验证

```bash
cd starter-kafka-sarama/example
go run .
```

预期输出：
```
published: value
consumed: value
```

需要先启动 Kafka：
```bash
# 启动 Kafka
docker compose up -d

# 等待 Kafka 就绪
sleep 10

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Kafka，运行示例并验证消息，退出码 0 表示通过。