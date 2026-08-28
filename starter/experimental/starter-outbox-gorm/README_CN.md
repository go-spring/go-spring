# starter-outbox-gorm

Go-Spring 的 gorm 事务消息：业务写与消息发布在同一个数据库事务中原子提交，后台 relay 把 `outbox_message` 表搬运到任意已注册的 `messaging.Binder`（kafka、nats……）。中立内核在 `cloud/experimental/outbox`。

## 接线

空白导入本 starter，在 `spring.outbox` 下每个 relay 一条配置：

```properties
spring.outbox.main.binder=kafka
spring.outbox.main.auto-migrate=true
```

每条配置自动注入一个 `*gorm.DB`（来自你已在用的 gorm 方言 starter；`db` 可指定具名 bean），启动时解析 binder，贡献健康指示器（`outbox:<name>`），并在 bean 的 Init/Destroy 上跑 relay 循环——优雅关停会排空在途记录。

## 发布

写侧就是你自己事务里的普通函数——没有 callback、没有魔法：

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil { return err }
    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
})
```

`PublishMessage(tx, dest, msg)` 接收 `*messaging.Message` 信封。

## 表结构

`auto-migrate`（默认关）通过 `Migrate` 建表；schema 由外部管理时用：

```sql
CREATE TABLE outbox_message (
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    destination   VARCHAR(255) NOT NULL,
    msg_key       VARCHAR(255),
    payload       BLOB         NOT NULL,
    headers       TEXT,
    status        VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempts      INT          NOT NULL DEFAULT 0,
    next_retry_at DATETIME     NOT NULL,
    last_error    TEXT,
    created_at    DATETIME     NOT NULL,
    sent_at       DATETIME
);
CREATE INDEX idx_outbox_dispatch ON outbox_message (status, next_retry_at);
```

mysql（8+）与 postgres 上 relay 用 `FOR UPDATE SKIP LOCKED` 取数，多实例可并发跑 relay。SQLite 自身单写者串行。MySQL 5.7 不支持 `SKIP LOCKED`——请只跑单个 relay 实例。

## 配置

| key | 默认 | 含义 |
|---|---|---|
| `db` | （按类型注入） | 支撑 outbox 表的 `*gorm.DB` bean 名 |
| `binder` | — 必填 | messaging binder 名（kafka、nats……） |
| `auto-migrate` | `false` | 启动时建表 |
| `poll-interval` | `1s` | 空转轮询间隔（下限 100ms） |
| `batch-size` | `100` | 每次轮询取的记录数 |
| `max-attempts` | `8` | 死信前的总投递尝试次数 |
| `backoff-base` / `backoff-max` | `1s` / `1m` | 指数退避基数/封顶 |
| `dlq-suffix` | `.dlq` | 死信目的地后缀；留空禁用 DLQ 副本 |

投递语义为 **at-least-once**；消费侧必须幂等。顺序只有批内 ID 序——需要按实体有序请设置消息 key。完整语义见 `cloud/experimental/outbox/DESIGN_CN.md`。

## 示例

`example/` 在内存 sqlite 与进程内 binder 上跑通整个模式，自断言原子性（提交即投递、回滚不投递）、退避重试与死信。`example/check.sh` 是其冒烟测试。
### 日志 tag

本模块的运行期日志使用 tag `_app_outbox`（事务 outbox）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.outbox.type=Logger
logger.outbox.level=WARN
logger.outbox.tag=_app_outbox
```
