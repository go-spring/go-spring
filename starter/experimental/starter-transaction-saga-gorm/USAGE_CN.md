# starter-transaction-saga-gorm 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。本文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`store.go`）及其 bean 的消费方：starter-transaction-saga
（`starter.go`、`recovery.go`、`config.go`）与能力核心
`cloud/experimental/transaction/coordinator.go` 核验。**Saga 模式语义（前向步骤 + 逆向
补偿、backward recovery）属于[分布式事务文献](https://seata.apache.org/docs/user/saga)**
——本文只写 go-spring 的增量。

本 starter 只贡献一件事：给 Saga coordinator 一个 gorm 背书的 durable
`transaction.Store`，正是它打开崩溃恢复。边界说明：store 持久化的是 saga *日志*
（snapshot），不是业务数据；业务效果的补偿仍是你的 `Compensate` 函数。本模块**没有
example/**——下面的工程示例对照 `store_test.go`（含端到端 execute-then-recover 测试）
做过代码级核验，未经运行期冒烟。

---

## 1. 完整工程示例

一个跑在 MySQL 业务库上的下单 saga：预留库存、扣款、以及一个故意失败的发布步骤驱动
逆向补偿。saga 日志——包括终态 `Compensated` 记录——落在 `saga_snapshots` 并跨重启存续。
文件树：

```
demo/
├── go.mod
├── main.go
├── saga.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring                       v1.3.x
    go-spring.org/starter-gorm-mysql            latest   // 提供 *gorm.DB
    go-spring.org/starter-transaction-saga      latest   // coordinator + registry + 恢复 Runner
    go-spring.org/starter-transaction-saga-gorm latest
    go-spring.org/starter-otel                  latest   // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-saga"
    _ "go-spring.org/starter-transaction-saga-gorm"
)

func main() { gs.Run() }
```

**saga.go** —— 应用全部业务面：

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction"
    "go-spring.org/spring/gs"
)

// 步骤注册必须发生在装配期（bean 构造）：启动恢复 Runner 按持久化的 method 名
// 从 StepRegistry 重建崩溃的 saga，注册晚了——比如在自定义 Runner 里——恢复运行时
// 步骤可能还不存在（starter-transaction-saga/starter.go 包注释，recovery.go:56-63）。
func init() {
    gs.Provide(func(coord transaction.Coordinator, reg *transaction.StepRegistry) *OrderSaga {
        reg.Register("OrderService.Place",
            // 步骤 1：预留库存（前向）；补偿时释放。
            transaction.Step{Name: "reserve-stock",
                Action:     func(ctx context.Context) (any, error) { return reserve(ctx, 2) },
                Compensate: func(ctx context.Context, _ any) error { return release(ctx, 2) }},
            // 步骤 2：扣款。
            transaction.Step{Name: "charge-payment",
                Action:     func(ctx context.Context) (any, error) { return charge(ctx, 30) },
                Compensate: func(ctx context.Context, _ any) error { return refund(ctx, 30) }},
            // 步骤 3：发布——故意失败，驱动 charge-payment、reserve-stock 依次逆向补偿；
            // coordinator 产生的每个 snapshot 都持久化到 saga_snapshots。
            transaction.Step{Name: "publish-order",
                Action:     func(ctx context.Context) (any, error) { return nil, errors.New("broker unavailable") },
                Compensate: func(ctx context.Context, _ any) error { return nil }},
        )
        return &OrderSaga{coord: coord, reg: reg}
    }).Export(gs.As[gs.Rooter]())
}

type OrderSaga struct {
    coord transaction.Coordinator
    reg   *transaction.StepRegistry
}

func (o *OrderSaga) Run(ctx context.Context) error {
    place := transaction.GlobalTransactional(o.coord, o.reg)

    // @GlobalTransactional 的等价物：返回错误时，coordinator 已经逆向补偿完所有
    // 已完成步骤并写下了终态日志。
    return place(ctx, "OrderService.Place", func(ctx context.Context) error {
        return nil // 步骤经由上面的 registry 条目执行
    })
}
```

（`reserve`/`charge` 等是你普通的业务调用——用注入的 `*gorm.DB` 跑 SQL 或走 RPC；
Saga 语义见链接文献。）

**conf/app.properties** —— 完整带注释的配置面：

```properties
# --- 数据源（starter-gorm-mysql；提供被注入的 *gorm.DB）----------------------
spring.gorm.mysql.dsn=app:pass@tcp(127.0.0.1:3306)/demo?parseTime=true

# --- Saga durable store（本 starter 的激活 key）-------------------------------
# 必须恰为 "gorm" 这个 Store 才会注册
# （OnProperty ... HavingValue("gorm")，无 MatchIfMissing —— starter.go:49-51）。
# 设置后，saga starter 的内存默认 Store 让位（OnMissingBean），coordinator 与
# 恢复 Runner 转而消费它。
spring.transaction.saga.store=gorm

# --- Saga 能力（父 starter；tracing/恢复开关）---------------------------------
# 默认 true，写出便于发现。每个步骤阶段一个 otel 子 span：
# saga.action <step> / saga.compensate <step>。
spring.transaction.saga.tracing=true
# 启动恢复 Runner：扫 Pending() 并补偿崩溃的 saga。
spring.transaction.saga.recover-on-start=true

# --- 可观测（starter-otel，可选）----------------------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**验证**（对着配置好的 MySQL）：

```bash
# 失败 saga 跑完后：
mysql> SELECT id, method, status, in_progress, completed FROM saga_snapshots;
# 一行：saga id、"OrderService.Place"、status=2（Compensated）；列含义见 §4.1
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-gorm-mysql + starter-transaction-saga + starter-transaction-saga-gorm
  ├─ saga starter: StepRegistry、内存 Store [OnMissingBean]、
  │   Coordinator、恢复 Runner [条件: enabled + recover-on-start]
  └─ 本 starter: gs.Provide(newGormStore as transaction.Store)
        [条件: OnProperty("spring.transaction.saga.store").HavingValue("gorm")]
        │
gs.Run()
  ├─ 配置绑定: ${spring.transaction.saga.gorm} → gormConfig（1 个 value tag）
  ├─ *gorm.DB 注入 newGormStore 构造函数第二个参数
  ├─ store 构造: db.AutoMigrate(&sagaSnapshot{}) —— 建 saga_snapshots 表，
  │  失败即 fail-fast（bean 报错）                          (starter.go:57-60)
  ├─ 顺序: durable Store 抢下 saga starter 的 OnMissingBean 槽位，
  │  newCoordinator 消费它（内存默认根本不会构造）
  ├─ 恢复 Runner.Run(): Store.Pending() → 逐条按 method 名从 registry 重建步骤
  │  并 coord.Recover()                                      (recovery.go:49-73)
  └─ 你的 Rooter/Runner bean 在装配之后运行
```

构造期日志：`gorm saga store created`（tag `AppDef`）。

### 2.2 一次失败的 saga —— 逐层走读（含持久化时点）

引 `cloud/experimental/transaction/coordinator.go` 与 `store.go`：

1. **Begin** —— `GlobalTransactional(coord, reg)` 查出方法的步骤并调
   `coord.Execute`；coordinator 经 `persistRunning` 写下初始 `StatusRunning`
   snapshot——saga 从第一步起就是持久的（coordinator.go:225-230）。
2. **reserve-stock 执行** —— 成功后 coordinator 以 `completed=["reserve-stock"]`、
   `step_results={"reserve-stock": ...}` upsert 运行态 snapshot（`Store.Save` 是
   `OnConflict UpdateAll` 的 upsert，store.go:65-73）。每个 saga id 一行、不断覆盖
   ——`saga_snapshots` 是*当前状态*日志，不是 append-only 历史。
3. **charge-payment 执行** —— 同节奏：snapshot 变为
   `completed=[reserve-stock, charge-payment]`。每次 `persistRunning` 写入都是一个
   崩溃后可续跑的存档点。
4. **publish-order 失败** —— `errors.New("broker unavailable")`。coordinator 逆向
   补偿：`refund`（charge-payment）、再 `release`（reserve-stock）。每次补偿在
   trace 里可见（`saga.compensate <step>` span），并更新同一行 snapshot。
5. **终态记录** —— `finish`：*已提交* saga 的行被删除（工作已完成，无需恢复）；
   *已补偿或失败* saga 的行**保留**终态 status，供运维检查
   （coordinator.go:235-244）。跑完后表里留有一行 `Compensated`——补偿落库的证据。
6. **崩溃恢复（重启）** —— 恢复 Runner 读 `Pending()`（全部 `StatusRunning` 行，
   store.go:94-111），按持久化 method 名从 registry 重建步骤列表，
   `coord.Recover` 从日志位置起补偿：先补 in-progress 步骤（结果恒为 nil——绕开
   JSON 往返的类型坑），再逆序补每个已完成步骤；未到达的步骤不补偿（单测
   `TestGormStore_EndToEndExecuteThenRecover` 核验：顺序 `!b`、`!a`，绝不会
   `!c`）。method 未注册步骤的 saga 记日志跳过；单条恢复报错不影响其余 saga，
   且 Runner 永不让启动失败（recovery.go:45-73）。

注意持久化的失败不对称：`persistRunning` 的写入错误被*有意吞掉*——saga 已有进展，
为一条日志写失败而掀翻整个操作比日志留个洞更糟（coordinator.go:218-224）。

---

## 3. 逐 key 行为参考

对本模块执行 `grep -rhoE 'value:"[^"]+"' --include='*.go'` 得**零个** tag；下表的
激活 key 是 bean *条件*属性（grep 不可见），因其是打开本 Store 的唯一方式而收录。
本模块现已**不持有任何 value-tag key**——绑定但从未读取的死键
`spring.transaction.saga.gorm.db` 已删除；`*gorm.DB` 恒取容器默认实例。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.transaction.saga.store` | string | （未设） | **激活 key**（条件属性，非 value tag）。必须恰为 `gorm`——`OnProperty ... HavingValue("gorm")`，无 `MatchIfMissing`（starter.go:49-51）。 | 其他值/未设 → 本 Store 不注册，saga starter 的内存默认继续生效：无 `saga_snapshots` 表、**无崩溃恢复**，且无任何告警。 |

决定本 Store 所喂机制的父 starter key（文档在
[starter-transaction-saga](../starter-transaction-saga)，此处列出因其改变本 Store 的可
观察行为）：`spring.transaction.saga.enabled`（默认 true）、
`spring.transaction.saga.tracing`（默认 true——`saga.action/compensate <step>` span，
属性 `saga.id`/`saga.step`/`saga.phase`）、`spring.transaction.saga.recover-on-start`
（默认 true——让本 Store 有意义的那个 Runner）。

**表结构**（构造函数里 `AutoMigrate` 创建，后端无关——只有 text 与 int 列，无方言特定
类型）：`saga_snapshots(id PK, method, status int 带索引, in_progress, completed text,
step_results text, updated_at)`（store.go:35-46）。`completed` / `step_results` 为 JSON
编码（`[]string` / `map[string]any`）。

---

## 4. 验证与故障演练

### 4.1 补偿记录落库

跑工程示例，然后：

```sql
SELECT id, method, status, in_progress, completed, step_results FROM saga_snapshots;
-- 终态行：status = 2（Compensated），completed 载有被补偿的步骤，
-- step_results 存各 Action 的（JSON 类型化的）结果。
```

提交路径演练：修好步骤 3 再跑——已提交 saga 的行被*删除*（coordinator.go:239-241），
所以成功之后表为空是正确的，`Pending()` 也返回空。

### 4.2 store 跨重启持久化（saga 中途 kill -9）

1. 让步骤 2（`charge-payment`）sleep 足够久以便在中途动手。
2. 启动应用、触发 saga，在步骤 2 运行期间 `kill -9` 进程。
3. 检查：`saga_snapshots` 留有一行 `status = 0 (Running)`，
   `completed=["reserve-stock"]`、`in_progress="charge-payment"`。
4. 重启应用。恢复 Runner 打日志
   `saga recovery: saga "<id>" recovered with status Compensated`，先补 in-progress
   步骤（结果 nil）再补已完成步骤，行的 status 翻为 Compensated。该序列恰由
   `TestGormStore_EndToEndExecuteThenRecover` 覆盖（store_test.go:93-149）。
5. 失败子演练：若装配期没注册该 method，恢复打
   `no steps registered for method ...; skipping`，行保持 Running——重新声明 saga
   定义后再重启。

### 4.3 在 trace 里观察恢复

`tracing=true` + starter-otel 时，重启期的补偿发出带 `saga.id` / `saga.step` /
`saga.phase` 标签的 `saga.compensate <step>` span，挂在 Runner 传入 context 之下——
§4.2 重启后去 collector 里查。

### 4.4 AutoMigrate fail-fast 演练

把 `spring.gorm.mysql.dsn` 指向无 DDL 权限的库：store bean 构造失败
（`auto-migrate saga_snapshots failed`，tag `AppDef`），启动中止——配错在开机时暴露，
而不是第一条 saga 上。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 没有 `saga_snapshots` 表、什么都不持久化 | `spring.transaction.saga.store` 未设或不为 `gorm`——内存默认 Store 生效 | 恰设为 `gorm`；开机确认 `gorm saga store created` 日志。 |
| 启动报 `auto-migrate saga_snapshots failed` | 注入的 `*gorm.DB` 无 DDL 权限或连不上 | 授权 DDL / 修 driver starter 的 DSN（starter.go:57-60）。 |
| 崩溃 saga 不恢复，日志 `no steps registered for method` | 步骤未在装配期注册（在 Runner 里注册，或 method 改名） | 在 bean 构造里以 `GlobalTransactional` 记录时的*同一* method 名注册（recovery.go:56-63）。 |
| Compensate 拿到 `float64` 而非 `int`，或 `map[string]any` 而非 struct | 恢复时的 JSON 往返：结果以其 JSON 形态回来（store.go:52-57） | Action 结果保持 JSON 友好（id、token、标量）；in-progress 步骤恢复时恒为 nil 结果。 |
| 恢复的 saga 补偿了从未跑过的步骤 | ——不会发生：恢复以日志的 `completed` + `in_progress` 为界（单测核验） | 若观察到，是真 bug——上报。 |
| 多个 `*gorm.DB` 实例，saga 日志落错库 | 不存在实例选择 key（原有死键 `db` 已删除）；恒注入容器默认实例 | 重排 bean 让 saga 库成为默认，或为它单写一个 starter 包装。 |
| 表里终态行越积越多 | 设计如此：compensated/failed 行保留供检查；只有 committed saga 被删 | 按自己的节奏清理已审计的终态行；把它们当审计痕迹。 |
| 恢复吞掉 DB 错误 | `Pending()` 失败只记日志并返回 nil（recovery.go:51-54）；`persistRunning` 错误按设计吞掉 | 监控 DB 与 `AppDef` 日志；别把沉默当成功。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|------|
| 配置 key 总数 | 0 个 value tag + 1 个激活条件 key（原死键 `db` tag 已删） |
| 其中必填 | 1（`spring.transaction.saga.store=gorm`） |
| quickstart 前置外部依赖数 | 1（gorm driver starter 背后的数据库） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（保留既有 + 新增）：

- 激活需要一个额外属性，而本家族其他 starter 均空导入即激活——不对称换来显式
  Store 选择；可辩护但应明说。
- ~~`spring.transaction.saga.gorm.db` 绑定但从未使用~~——已解决：死键删除（多实例
  选择仍未实现；前缀保留给未来的 Store 选项）。
- **没有 example/、也没有把 saga + gorm store 全链路接起来的集成冒烟**（只有 store
  单测）→ 补 example/；本文的工程示例仅做过代码级核验。
- `persistRunning` 按设计吞掉 store 错误（进展 > 日志完整性）——saga 中途的
  durable-store 故障留下恢复时无法与"步骤未跑过"区分的空洞；Save 失败没有指标暴露。
- 恢复结果的 JSON 往返类型坑只写在代码注释里（store.go:52-57），是静默陷阱。
- 终态行永久保留、无退役机制；committed 行被删——审计覆盖按结局不对称。
