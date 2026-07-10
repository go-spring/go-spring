# add-mq 子流程(占位,待补充)

在已有 Go-Spring 项目里新增 MQ 消费或生产:消费入口在 `mqsvr`,producer 在 `infra/mq`。

## 何时使用

- 关键词:加 MQ 消费 / 加 consumer / 发消息 / 加 producer / 接 Kafka·RabbitMQ 消息

## 触及的 layout 位置

- `api/server/mqsvr/`(消费入口、handler、中间件)
- `infra/mq/`(消息 producer)
- `application/<bc>/`(消费/发布的用例编排)

## 工作流程

> **待补充**:此工具尚未编写。填充时区分消费/生产两侧,消费入口属 `mqsvr`、producer 属 `infra/mq`;遵守 `docs/agent-rules/domain-rules.md`(领域事件在事务边界内收集、持久化后发布)与 `../../SKILL.md` 共用约定。
