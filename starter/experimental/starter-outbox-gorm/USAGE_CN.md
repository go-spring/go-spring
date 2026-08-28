# starter-outbox-gorm 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`publish.go`、`store.go`、`model.go`、`health/health.go`、
`schema.json`）、relay 核心（`go-spring.org/cloud/experimental/outbox`）以及自断言的
[example/](example/)（`example/check.sh`）核对。transactional outbox *模式*本身是标准
文献；本文只讲 go-spring 的装配、接线与运维增量。

**激活条件**：仅当存在 `spring.outbox` 配置项时模块才注册 bean（`gs.OnProperty("spring.outbox")`，
starter.go:59；该判断是前缀匹配，任意 `spring.outbox.*` key 都会触发）。没有 `enabled`
key。实例多命名：`spring.outbox.<name>.*` 下每个条目对应一个 relay 实例。

---

## 1. 完整工程示例

一个真实的订单服务：写订单与发布领域事件原子提交，带健康探针与死信演练。文件树：

```
demo/
├── go.mod
├── main.go
├── order.go
└── conf/
    ├── app.properties
    └── govern.yaml          # outbox 本身不需要；为对齐生态件展示
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-outbox-gorm latest
    go-spring.org/starter-gorm-mysql  latest   // 任意 gorm driver starter；提供 *gorm.DB bean
    go-spring.org/starter-kafka       latest   // 任意 broker starter；注册 "kafka" binder
    go-spring.org/starter-actuator    latest   // 可选：outbox:<name> 健康指示器
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-kafka"
    _ "go-spring.org/starter-outbox-gorm"

    _ "demo/order"
)

func main() { gs.Run() }
```

**order.go** — 你需要写的唯一 outbox 相关代码：

```go
package order

import (
    "go-spring.org/spring/gs"
    StarterOutboxGorm "go-spring.org/starter-outbox-gorm"
    "gorm.io/gorm"
)

func init() {
    gs.Provide(newService)
}

type Service struct{ db *gorm.DB }

func newService(db *gorm.DB) *Service { return &Service{db: db} }

// CreateOrder 在同一个数据库事务里写订单 AND 发事件。共享同一事务正是该模式的
// 全部意义：两行要么一起提交，要么都不提交（example.go 的 rollback 断言验证）。
func (s *Service) CreateOrder(order *Order, payload []byte) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(order).Error; err != nil {
            return err // 订单 AND outbox 行一起回滚
        }
        return StarterOutboxGorm.Publish(tx, "orders.events", order.ID, payload, nil)
    })
}
```

**conf/app.properties** — 完整带注释的 outbox 配置面（此处为生产量级数值；
example/ 使用缩小的值以加速冒烟）：

```properties
# --- database（driver starter 自身的 key）--------------------------------------
spring.gorm.db.dataSourceName=user:pass@tcp(127.0.0.1:3306)/demo

# --- outbox：spring.outbox 下每个条目一个 relay 实例 ---------------------------
# "main" 是实例名 → bean 名、健康指示器 "outbox:main"。
spring.outbox.main.binder=kafka        # 必填：broker starter 注册的 binder 名
spring.outbox.main.db=                 # 空 → 自动注入唯一的 *gorm.DB bean
spring.outbox.main.auto-migrate=true   # 启动时用 gorm 建 outbox_message 表
spring.outbox.main.poll-interval=1s    # 下限钳制 100ms
spring.outbox.main.batch-size=100      # 每次拉取行数
spring.outbox.main.max-attempts=8      # 死信前的总尝试次数
spring.outbox.main.backoff-base=1s     # 每次失败翻倍
spring.outbox.main.backoff-max=1m      # 单次重试等待上限
spring.outbox.main.dlq-suffix=.dlq     # "orders.events" → "orders.events.dlq"；"" 关闭 DLQ 拷贝

# --- actuator（健康指示器）----------------------------------------------------
spring.actuator.addr=:9370
```

**验证**（开箱即用的 example —— 零外部依赖，内存 sqlite + `mem` binder）：

```bash
cd starter/experimental/starter-outbox-gorm/example && ./check.sh
# → "outbox example OK: atomicity, retry, dead-letter all passed"
#   "outbox example smoke test passed"
```

example 自断言：已提交事务恰好投递其消息；回滚的事务什么都不投递；flaky 目的地在
最后一次允许的尝试上成功；poison 目的地落入 `poison.dlq` 且带 `x-dlq-retries: 3`；
终态表内为 2 sent / 1 dead / 0 pending（example.go:159-236）。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-outbox-gorm
  └─ gs.Module(gs.OnProperty("spring.outbox"))                 [starter.go:59]
        │   ${spring.outbox} 下每个条目 <name> 一轮（conf.BindEach）
        ├─ Provide(newRelay).Name(<name>).Init(Destroy 已接线)
        │     参数：IndexArg(1, 绑定的 Config)、IndexArg(2, TagArg(db) *gorm.DB)、
        │           IndexArg(3, ValueArg(name))
        │     以 gs.Rooter 导出                                      [starter.go:61-66]
        └─ Provide(health.Indicator "outbox:<name>")              [starter.go:69-71]
              .Name("outbox:" + name) —— 必须命名：多实例健康 bean 需要
              不同的 (Name,Type) 键，否则容器报 duplicate beans。

gs.Run()
  ├─ 配置绑定：${spring.outbox.<name>} → Config（value tag；expr: binder != ''）
  ├─ bean 装配：*gorm.DB 由 TagArg(c.DB) 解析 —— db key 为空自动注入
  │             唯一的 *gorm.DB bean；命名 key 则选指定 bean
  ├─ Relay.Init()  [starter.go:97]
  │     1. messaging.GetBinder(binder) —— 运行期而非构造期解析，
  │        因此晚注册 binder 的 broker starter 也能工作
  │     2. auto-migrate=true 时执行 Migrate(db)
  │     3. outbox.NewRelay(gormStore, binder, cfg, logObserver) 并在
  │        后台 goroutine 启动循环（Init 立即返回）
  ├─ 就绪信号：不受影响 —— relay 是 worker 不是 gs.Server；阻塞它会
  │        破坏就绪信号（starter.go:55-58 注释）
  └─ SIGTERM：Relay.Destroy() —— drain 契约见 §4.4。
```

Init 失败是致命的：未知 binder 名或 auto-migrate 失败都会带实例名打一条 ERROR 并
中止启动（starter.go:98-107）。

### 2.2 一条消息的端到端走读

写入侧（`publish.go:42-61`）：

1. 你的代码开 `db.Transaction(...)`；业务写入 + `Publish(tx, dest, key, payload, headers)`
   插入同一个事务。`Publish` 把 headers 序列化为 JSON，插入 `outbox_row`：
   `status="pending"`、`next_retry_at=now`、`created_at=now`。
2. 提交 —— 行只有此刻才对 relay 可见。回滚则静默消失。

Relay 循环（`cloud/experimental/outbox/relay.go`，由 starter.go:109-116 驱动）：

3. **Fetch** —— 每 `poll-interval`，`gormStore.Fetch` 选取至多 `batch-size` 行：
   `status='pending' AND next_retry_at <= now`，按 `id` 升序；mysql/postgres 上额外加
   `FOR UPDATE SKIP LOCKED`，并发 relay 实例永不共享同一行（store.go:58-74）。
4. **Deliver** —— 批内按 ID 序逐条，relay 按 destination 惰性打开
   `messaging.Publisher`（进程内缓存），发布 Key/Payload/Headers。单条失败不会中止
   批次 —— 一条毒消息不能饿死其余记录（relay.go:111-127）。
5. **Mark** —— 成功：`MarkSent` 把行翻成 `status='sent'` 并写 `sent_at`（WHERE 里
   `status='pending'` 守卫，竞争的 mark 不会二次生效）。失败：见 §2.3。

若 fetch 本身失败，relay 每轮 poll 打一条 ERROR 并继续 —— 死库不能沉默，也不能
拖垮进程（relay.go:113-119）。

### 2.3 重试与死信 —— 精确语义

失败尝试时（relay.go:164-190）：

- `attempts`（已失败次数）+ 1 < `max-attempts` → `MarkFailed`：`attempts+1`、
  `last_error`、`next_retry_at = now + backoff(attempts)`，其中 backoff =
  `backoff-base` 每次失败翻倍、以 `backoff-max` 封顶（outbox.go:157-166）。
  触发 Observer `OnRetry`。
- 尝试耗尽（`>= max-attempts`）：
  - `dlq-suffix` 非空 → relay 先向 `destination + dlq-suffix` 发布**副本**，携带原始
    headers 外加 `x-dlq-error`、`x-dlq-retries`（尝试次数）与 `x-dlq-key`；只有副本
    发布成功才 `MarkDead`（`status='dead'`、`last_error`）。
  - `dlq-suffix=""` → 直接 `MarkDead`，无副本（无人盯的数据通路 —— 见 §6）。
  - 若 DLQ 发布本身失败，记录**不**丢：`MarkFailed` 保持 pending，下一轮重走整条
    死信路径（relay.go:181-186 —— 丢死信比重投递更糟）。
- 边界：broker 发布成功但 `MarkSent` 失败（store 挂了）时记录保持 pending，下一轮
  会再次投递 —— 这就是 at-least-once 的来源；消费者必须幂等或按 Key 去重
  （relay.go:133-141，outbox.go:22-24）。

行的生命周期：`pending → sent | dead`。`sent` 行 starter 永不清理 —— 表增长
（与归档）是运维事务。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.outbox.<name>.` 下 —— 每条目一个实例。默认值来自
config.go:29-63；归一化（钳制）来自 outbox.go:132-153。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-----------|----------|
| `binder` | string | — | **必填**（`expr:"binder != ''"`，config.go:36）。broker starter 注册的 binder 名（`kafka`、`nats`…）或你自己的 `messaging.RegisterBinder`。Init 时解析，晚注册的 binder 也可用。 | 缺失/未知 → 启动失败，报 `outbox %q: …` ERROR。 |
| `db` | string | "" | 指定承载 outbox 表的 `*gorm.DB` bean 名。空则自动注入唯一的 `*gorm.DB` bean（无论哪个 driver starter 提供的）。 | 名字无对应 bean → 启动装配失败。 |
| `auto-migrate` | bool | false | `true` 时 Init 里跑 `db.AutoMigrate(&outboxRow{})` —— 建 `outbox_message` 及 `(status, next_retry_at)` 派发索引（model.go:40-42,55-57）。默认关：schema 由迁移工具管理时按 README DDL 建表。⚠ auto-migrate 不会升级旧 DDL 建的既有表。 | false 且无表 → 每轮 fetch ERROR（每轮 poll 打日志、relay 继续跑但什么都不投递）。 |
| `poll-interval` | duration | 1s | 表排空后两次 poll 的间隔。`<=0` → 1s；`<100ms` 钳到 100ms（outbox.go:133-137）。 | 过低 → 对死库忙轮询刷 fetch ERROR；过高 → 空闲后延迟大。 |
| `batch-size` | int | 100 | 单次 fetch 行数上限。`<=0` → 100。 | 大批次持锁（SKIP LOCKED）更久、拉长一个 drain 周期。 |
| `max-attempts` | int | 8 | 死信前总尝试次数。`0` → 8；负数 → 1（首次失败立即死信）。 | 1 + 抖动 broker → 秒进死信。 |
| `backoff-base` | duration | 1s | 首次失败后的等待；其后每次失败翻倍。`<=0` → 1s。 | 与 `backoff-max` 共同决定总重试窗口：1s..1m 下 8 次约 3.5 分钟后进 DLQ。 |
| `backoff-max` | duration | 1m | 单次 backoff 上限。`<=0` → 1m。 | — |
| `dlq-suffix` | string | ".dlq" | 拼在 destination 后得到死信目的地。⚠ 空串是**关闭** DLQ 拷贝 —— 耗尽记录直接 `dead`、任何地方都没有副本（config.go:59-62）。 | 本想"无后缀"，实际静默关掉了死信投递。 |

联动说明：

- `db` 必须指向业务事务写入的**同一个数据库** —— 原子性保证是"同一事务"，只有
  同库才存在。
- 跨批次/跨重启不保证顺序；若 broker 支持按 key 分区（kafka partition），在
  `Publish` 传 `key` 让同 key 消息有序（relay.go:34-37）。
- 这些 key 都是实例作用域（`spring.outbox.<name>.*`）；本 starter 没有顶层的
  `${observability:=}` 式绝对 key。

---

## 4. 验证与故障演练

### 4.1 原子性

example 运行时（或你自己的服务）：一个事务里下订单后提交、另一个在 `Publish` 之后
回滚。观察 broker：只有已提交的消息到达；`SELECT status, count(*) FROM
outbox_message GROUP BY status` 无幻影行。example 断言的正是这一点（example.go:159-172）。

### 4.2 带退避的重试

把某目的地指向短暂不可用的 broker（或用 example 的 `orders.flaky`：失败两次后
成功）。观察 WARN 日志：

```
WARN ... _app_outbox ... outbox: record 3 to "orders.flaky" failed (...), retry at 2026-08-28T10:00:02Z
```

（starter.go:147-150）。`retry at` 时间戳按 `backoff-base` 翻倍推进。broker 恢复后
投递成功、行翻 `sent`。

### 4.3 死信演练

向永久失败的目的地发消息（example：`poison`）。`max-attempts` 次尝试后，副本落入
`poison.dlq`，带 `x-dlq-error`、`x-dlq-retries`、`x-dlq-key`；行变为 `dead`；打一条
ERROR（starter.go:153-156）。验证：

```sql
SELECT id, destination, attempts, last_error FROM outbox_message WHERE status = 'dead';
```

切换演练：启动前把 `max-attempts=1` 即可看到立即死信。

### 4.4 停机 / drain 契约

SIGTERM 时容器调用 `Relay.Destroy()`（starter.go:125-136）：取消循环 context 并等待，
**以硬编码 5s 为上限**（`DrainTimeout`，starter.go:122）。循环停止拉取新批次，并把
正在投递的那条做完 —— 或标 failed 留给下一轮 —— 然后返回（relay.go:91-94）。超时则
放弃等待（行保持 pending、重启后继续；投递可能重复 —— at-least-once）。

注意：这里的 drain 是 relay 自身的取消语义。负载均衡的实例 "Weight=0" 摘流语义
**不**适用于此 —— outbox relay 不注册进任何 registry/lb 池；其停机路径只有上面的
`Destroy`。

### 4.5 可观测面

- 日志 tag `_app_outbox`（starter.go:52）。可独立调级：

```properties
logger.outbox.type=Logger
logger.outbox.level=WARN
logger.outbox.tag=_app_outbox
```

- 重试 → WARN、死信 → ERROR、成功发布静默 —— broker 侧访问日志已记录
  （starter.go:138-156）。
- `outbox.Observer` 缝隙（outbox.go:168-181）同步收到 OnPublished/OnRetry/OnDead。
  starter 只接了日志 observer；尚未内置 metrics/otel 适配器 —— 需要时自行直接
  包 `outbox.Relay`。

### 4.6 健康检查

starter 注册名为 `outbox:<name>` 的指示器 bean（starter.go:69-71）—— `.Name()` 必须
设置，否则两个实例会以重复 `(Name,Type)` 键报 duplicate beans。探针是对 relay 所用
同一个 `*gorm.DB` 的裸 `SELECT 1`（health/health.go:29-33）：能发现库不可达，但**不**
反映 relay 延迟、死信深度或 pending 积压。

```bash
curl -s :9370/health | jq '.components["outbox:main"]'
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败：`outbox "main": binder "kafka" not found` | broker starter 未 import，或名字写错 | import broker starter，或在 Init 前自行 `messaging.RegisterBinder("kafka", …)`。 |
| 启动时 auto-migrate 报错后失败 | 库不可达 / 无 DDL 权限 | 修连通性，或按 README DDL 预建表并设 `auto-migrate=false`。 |
| 持续 `fetch failed (keep polling)` ERROR | 表不存在（auto-migrate 关且 DDL 未执行）或库挂 | 执行 DDL；relay 撑得住但 fetch 失败期间不投递。 |
| 消息投递了两次 | 发布与 MarkSent 之间崩溃/重投 —— at-least-once 设计使然 | 消费者幂等或按 `Key` 去重。 |
| 行大量积压 `pending` | broker 挂：按 `backoff-max` 封顶退避 | 恢复 broker；盯 `retry at` 日志。监控 `SELECT count(*) … WHERE status='pending'`。 |
| 行变 `dead` 但没有 DLQ 副本 | `dlq-suffix=""`（DLQ 关闭） | 设后缀；必要时手工重发 dead 行。 |
| 消息间乱序 | 仅批内按 ID 有序；跨批次无保证 | `Publish` 传分区 `Key`，依赖 broker 的 key 有序性。 |
| MySQL 5.7 并发 relay 重复投递 | 5.7 无 `FOR UPDATE SKIP LOCKED`（store.go:36-38） | 每表只跑一个 relay 实例，或升级 mysql 8+。 |
| 停机日志 `drain timed out` | 在途批次超过硬编码 5s | 无害 —— 行保持 pending、重启后继续。 |
| 指示器 UP 但不投递 | 指示器只探库连通性 | 改看 relay 日志（`_app_outbox`）与 pending 行数。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 9 |
| 其中必填 | 1（`binder`） |
| quickstart 前置外部依赖 | 2（DB + broker；随附 example 用 sqlite + mem binder 为 0） |
| "注意/坑"条数 | 7（同事务才有原子性；at-least-once；仅批内有序；MySQL 5.7 单 relay；DLQ 关闭无人盯；5s drain 上限；sent 行增长） |

设计嫌疑清单（供设计裁决）：

1. ~~索引不匹配~~ — 已修 2026-08-27：gorm model 现建 `(status, next_retry_at)`，
   与 README DDL 及 `Fetch` 谓词一致。
2. ~~`Store.Fetch` 错误静默~~ — 已修 2026-08-27：fetch 失败每轮 poll 打 ERROR
   （relay 继续轮询、不崩）。
3. `toRecord` 里损坏的 `headers` JSON 被静默丢弃（store.go:117-121）。
4. `DrainTimeout=5s` 硬编码，不可配置。
5. DLQ 关闭（`dlq-suffix=""`）时耗尽记录静默直奔 `dead`、无副本 —— 无人盯的数据通路。
6. ~~死字段 `Relay.pubOnce`~~ — 已修 2026-08-27（删除）。
7. 健康指示器名字暗示 outbox 健康，实际只探库连通性。
8. 仅日志的 Observer 使 `outbox.Observer` 缝隙没有内置 metrics/otel 适配器
   （observe-messaging 桥接是自然归宿）。
