# outbox

Go-Spring 的事务消息（transactional outbox）：把消息写进 outbox 表并**与业务写同处一个数据库事务**，由后台 relay 把表中的记录投递到真实 broker，从而让"事务提交即发消息、回滚即不发"成为原子行为。

它是"事务提交时要发事件、回滚时不能发"这一问题的 Go 语义答案——不需要分布式事务协调器，也没有双写不一致。

## 包内有什么

本包是 broker/存储中立的内核：

- `Record` — outbox 一行的中立投影（`pending → sent | dead`）
- `Store` — 持久化接缝（`Fetch` / `MarkSent` / `MarkFailed` / `MarkDead`）
- `Relay` — 投递循环：轮询 → 发布 → 指数退避重试 → 死信
- `Observer` — 观测接缝（publish / retry / dead 事件）
- `MemoryStore` — 仅供测试/演示的内存实现

**写侧**（在你的事务里插入 outbox 表）与 **gorm Store 实现**在存储后端 starter——`go-spring.org/starter-outbox-gorm`——因为它们需要事务句柄。投递走任意已注册的 `messaging.Driver`（kafka、nats……），换 broker 只是改接线。

## 语义

- **at-least-once。** 发布成功后、`MarkSent` 前崩溃会重发。消费侧必须幂等（或按记录 ID/业务键去重）。
- **不保证全局顺序。** 批内按 ID 升序投递；跨批/跨实例/重启不承诺。需要按实体有序就设置 `Record.Key`，交给 broker 的 keyed 分区。
- **有界重试后进 DLQ。** 失败按指数退避（默认 1s 翻倍、封顶 1m）；超过 `MaxAttempts`（默认 8）后按 `messaging` 的 DLQ header 契约把副本发到 `destination + DLQSuffix`（".dlq"；留空禁用副本）。DLQ 发布失败则记录保持 pending——丢死信比重复投递更糟。
- **一条毒丸不堵整批。** 每条记录独立推进。

## 用法示意

```go
// 业务事务内（starter-outbox-gorm）：
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil { return err }
    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
})
// relay（由 starter 按 spring.outbox.instances.* 配置接线）负责投递
```

配置、表 DDL 与自断言示例见 `starter/experimental/starter-outbox-gorm`。
