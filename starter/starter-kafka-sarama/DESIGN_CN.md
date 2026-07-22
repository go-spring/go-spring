# starter-kafka-sarama 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-kafka-sarama` 属于 Client 形态（`starter/DESIGN.md` §2.2），
提供 Sarama `sarama.Client` 实例。与 `starter-kafka`（franz-go 版）共享
`spring.kafka` 配置前缀——切换实现是空导入换包，配置不用改
（`project_starter_kafka_sarama`）。

## 1. 职责与边界

- 用 `gs.Group` 把 `spring.kafka.<name>` 每条绑到 `sarama.Client`
  bean。不做默认单实例。
- 暴露的 bean 是 `sarama.Client`——producer、consumer group、admin 客户端
  由调用方在共享 client 上构建，一个连接池服务所有下游角色。
- 有意不套 `*Conn` 包装：不同于 NATS 的双 API，Sarama 天然以
  `sarama.Client` 接口做 bean。
- 不接 OTel producer/consumer hook。Sarama 无 interceptor 缝隙；
  `otelsarama` 已废弃且锁在 Shopify fork
  （`project_starter_kafka_sarama`）。tracing 放在调用点。

## 2. 关键抽象与缝隙

- **共享前缀，不共享 bean。** `spring.kafka` 是缝隙
  （`feedback_websocket_shared_config_prefix`）：sarama 与 franz-go 读同
  一棵树，但每进程只 import 一个。切换=`_ "…/starter-kafka"` ↔
  `_ "…/starter-kafka-sarama"`。
- **单 client 服务所有角色。** 基于共享 `sarama.Client` 构建的 producer /
  consumer group / admin 共用其 metadata 缓存与 broker 连接。starter 不
  预建这些，调用方在拥有生命周期的地方建。
- **TLS / SASL 由配置驱动。** enabled 开关围栏；`sasl.enabled=true` 时
  `mechanism` 选 `PLAIN` / `SCRAM-SHA-256` / `SCRAM-SHA-512`。
- **Producer 参数是 Sarama 原生。** `producer.required-acks`、
  `producer.idempotent`、`producer.compression` 直接映射到 `sarama.Config`
  字段——不在上头再叠抽象。

## 3. 约束

- **`Brokers` 必填。** 无 localhost 兜底；空 broker 列表启动失败。
- **Sarama version 必须钉。** `version`（如 `2.6.0`）不设会退到最基线协议，
  很多特性（SASL 机制、header、idempotent producer）要求最低协议版本。
- **不做 OTel producer/consumer 包装。** 需要 tracing 就在
  publish / consume 边界加 span helper；starter 不会静默改
  `sarama.Config` 去插入并不存在的 interceptor。
- **`destroy = Close`。** `sarama.Client.Close` 释放 broker 连接。基于其
  构造的 consumer group / producer 由调用方先关。

## 4. 权衡 / 已否决方案

- **顺带打包 producer / consumer bean——否决。** 各调用方的 producer /
  consumer group 各有 topic、partitioning、错误处理与 rebalance 策略；
  一刀切 bean 藏起来的东西比省下的多。starter 暴露 `sarama.Client`，
  调用方拥有专用 bean。
- **`otelsarama` hook——否决。** 上游已废弃、锁死在厂商 fork（Shopify）；
  接进来等于把 starter 绑到停更路径。
