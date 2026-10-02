# starter-rabbitmq Example

演示 starter-rabbitmq 的 RabbitMQ 消息客户端。

## 功能验证

- **默认交换机**：通过默认 exchange 发布/消费消息
- **队列声明**：自动声明队列
- **消息确认**：消费后自动 ack

> 需要 RabbitMQ 服务运行。`check.sh` 通过 docker compose 启动 RabbitMQ。

## 手动验证

```bash
cd starter-rabbitmq/example
go run . -manual
```

预期输出：
```
published to queue "hello"
consumed from queue "hello": value
```

需要先启动 RabbitMQ：
```bash
# 启动 RabbitMQ
docker compose up -d

# 等待 RabbitMQ 就绪
sleep 10

# 运行示例
go run . -manual
```

浏览器打开 `http://127.0.0.1:15672`（guest/guest）查看管理界面。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 RabbitMQ，运行示例并验证消息，退出码 0 表示通过。
