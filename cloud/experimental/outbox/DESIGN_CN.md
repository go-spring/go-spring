# outbox — 设计

## 问题

"事务提交时发消息"这件事出奇地难。两个朴素做法都会翻车：

1. **提交后发布**——commit 与 publish 之间崩溃，事件悄悄丢失。
2. **提交前发布**——回滚留下幻影事件，消费者看到一个从未发生的写。

事务消息（transactional outbox）把消息变成与业务写同库同事务的一行：提交即（最终）发布，回滚即丢弃；后台 relay 把行搬到真实 broker。这是标准的非 XA 答案，也是唯一能与普通 SQL 数据库组合的答案。

## 在代码树中的位置

`cloud/experimental/outbox`，与 `messaging` 平级——刻意不进
`cloud/experimental/transaction`。transaction 家族（saga / tcc / at）共享
"全局事务 + 补偿"的问题域，有 Coordinator/Store 概念；outbox 没有这些概念。
它唯一的血缘是 messaging：复用 `Message`、`Binder` 与 DLQ header 契约。本包
只 import `cloud/experimental/messaging` 与标准库——无 gorm、无 spring、无 otel。

## 形状

```
             （业务事务）                        （后台）
  Publish(tx, dest, msg) ──► outbox 表 ──► Relay ──Binder──► broker
  （在 starter-outbox-gorm）    （Store 实现）     │
                                                  ├─ 成功 ──► MarkSent
                                                  ├─ 失败 ──► MarkFailed（退避）
                                                  └─ 超限 ──► DLQ 副本 ──► MarkDead
```

- **Store 是唯一的存储接缝。** Relay 是其上的纯逻辑：用 fake 可单测，跨存储
  引擎可复用。`Fetch` 的契约要求并发 relay 不会拿到同一条 pending 记录——
  怎么做到是后端细节（gorm：`FOR UPDATE SKIP LOCKED`；SQLite：单写者）。
- **写侧是 starter 里的普通函数**，不是 callback。gorm callback 自动捕获无
  从得知 destination，且魔法拦截违背容器"只装配不裁决"总纲。在应用自己的
  事务里显式 `Publish(tx, ...)` 对行为诚实，且只多一行。
- **relay 跑在 bean Init/Destroy 循环上**，不是 `gs.Server`：它是 worker
  不是端点，阻塞就绪信号是错的。

## 投递语义（以及为什么）

- **at-least-once**，不是 exactly-once：publish→MarkSent 的窗口在任何不把
  broker 塞进事务的设计里都无法消除；exactly-once 是消费侧课题（幂等/去重）。
  把这一点说破，其余选择就都能保持简单。
- **不承诺全局顺序。** relay 内做 per-key 串行化会引入队头阻塞和第二个状态
  机；生产者设置 `Record.Key` 时 broker 的 keyed 分区已解决按实体有序。批内
  按 ID 升序投递——零成本，且单写者的常见场景白得 FIFO。
- **先 DLQ 后 dead。** 超限记录若 DLQ 副本发布失败则保持 pending（下轮重试）：
  丢死信比重复投递更糟。与消费侧 `messaging.DeadLetter` 哲学对称；两条路径
  打同一套 header 契约（`x-dlq-error` / `x-dlq-retries` / `x-dlq-key`）。
- **毒丸不堵批。** 每条记录独立推进，一个坏目的地不能饿死其他消息。
- **Mark 失败可容忍。** 发布成功后 `MarkSent` 失败则记录保持 pending，下轮
  重发——已声明的 at-least-once 语义自然吸收。

## 旋钮

`Config` 刻意小且零值可用：轮询间隔（1s，下限 100ms）、批量（100）、最大
尝试（8）、退避基数/封顶（1s / 1m）、DLQ 后缀（".dlq"，空禁用）。不做按
目的地覆盖、不做 cron 调度、不做 LISTEN/NOTIFY——每一个都在某处坑过人，
且对正确性都非必需。

## 可观测

`Observer` 是接缝不是依赖：cloud 不 import 追踪库。gorm starter 带一个 log
适配器（retry → WARN，dead → ERROR）；需要时 otel 适配器照 transaction 家族
的手法补。

## 非目标

- Inbox / 消费侧去重表（对称的模式，若需要另起包）。
- sent 行的保留/归档——运维课题；`sent` 行留作审计，由使用方清理。
- 超出 `SKIP LOCKED` 免费提供的 relay 分片协调。
