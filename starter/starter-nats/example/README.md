# starter-nats Example

演示 starter-nats 的 NATS 消息客户端。

## 功能验证

- **连接健康**：验证 NATS 连接健康状态
- **多连接隔离**：main 和 work 两个独立连接
- **消息发布/订阅**：通过 NATS 发布和订阅消息

> 需要 NATS 服务运行。`check.sh` 通过 docker compose 启动 NATS。

## 手动验证

```bash
cd starter-nats/example
go run .
```

预期输出：
```
main connection: healthy
work connection: healthy
message published
message received
```

需要先启动 NATS：
```bash
# 启动 NATS
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 NATS，运行示例并验证消息，退出码 0 表示通过。