# starter-transaction-tcc-gorm 使用说明 — 参考

详细使用参考。概览见 [README_CN.md](README_CN.md)。下文每条行为声明都对着 starter 源码
(`starter.go`、`config.go`、`store.go`)以及它的 bean 消费方——starter-transaction-tcc
(`starter.go`、`recovery.go`、`config.go`)与能力核心
`cloud/experimental/transaction/tcc/coordinator.go` 核过。**TCC 模式语义(Try / Confirm /
Cancel)属[分布式事务文献](https://seata.apache.org/docs/user-mode/tcc-mode)——本页只讲
go-spring 的增量。**

本 starter 只贡献一样东西:给 TCC 协调器一个持久化的、基于 gorm 的 `tcc.Store`,这正是打开
崩溃恢复的开关。范围说明:store 持久化的是 TCC **日志**(快照),不是业务数据;业务效果的
confirm/cancel 仍由你的 `Participant` 函数完成。本模块**没有 example/**——下面的完整工程是
对着 `store_test.go`(含端到端 execute-then-recover 测试)核过的,不是对着运行中的数据库冒烟
验过的。

---

## 1. 完整工程示例

一个下单 TCC,业务库是 MySQL:预留库存、扣款,再一个故意失败的 Try 逼出对已 try 参与者的取消。
TCC 日志——含终态 `Cancelled` 记录——落在 `tcc_snapshots` 并扛过重启。文件树:

```
demo/
├── go.mod
├── main.go
├── order.go
└── conf/
    └── app.properties
```

**go.mod**(相关依赖):

```
require (
    go-spring.org/spring                      v1.3.x
    go-spring.org/starter-gorm-mysql           latest   // 提供 *gorm.DB
    go-spring.org/starter-transaction-tcc      latest   // 协调器 + 注册表 + 恢复 Runner
    go-spring.org/starter-transaction-tcc-gorm latest
    go-spring.org/starter-otel                 latest   // 可选:真实 trace 导出
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-tcc"
    _ "go-spring.org/starter-transaction-tcc-gorm"
)

func main() { gs.Run() }
```

**order.go** — 应用的全部面:

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction/tcc"
    "go-spring.org/spring/gs"
)

// 参与者注册必须发生在接线期(bean 构造):启动恢复 Runner 会按持久化的方法名从
// ParticipantRegistry 重建每个崩溃的事务,晚注册(如在自定义 Runner 里注册)可能在恢复
// 运行时尚未就位(starter-transaction-tcc/starter.go 包注释、recovery.go)。
func init() {
    gs.Provide(func(coord tcc.Coordinator, reg *tcc.ParticipantRegistry) *OrderTcc {
        reg.Register("OrderService.Place", []tcc.Participant{
            // 参与者 1:预留库存(Try);confirm 花掉它、cancel 释放它。
            {Name: "reserve-stock",
                Try:     func(ctx context.Context) (any, error) { return reserve(ctx, 2) },
                Confirm: func(ctx context.Context, _ any) error { return spend(ctx, 2) },
                Cancel:  func(ctx context.Context, _ any) error { return release(ctx, 2) }},
            // 参与者 2:扣款。
            {Name: "charge-payment",
                Try:     func(ctx context.Context) (any, error) { return charge(ctx, 30) },
                Confirm: func(ctx context.Context, _ any) error { return commitCharge(ctx, 30) },
                Cancel:  func(ctx context.Context, _ any) error { return refund(ctx, 30) }},
            // 参与者 3:发布——Try 里故意失败,逼出对 charge-payment 再 reserve-stock
            // 的取消,且协调器取的每个快照都落进 tcc_snapshots。
            {Name: "publish-order",
                Try:     func(ctx context.Context) (any, error) { return nil, errors.New("broker unavailable") },
                Confirm: func(ctx context.Context, _ any) error { return nil },
                Cancel:  func(ctx context.Context, _ any) error { return nil }},
        })
        return &OrderTcc{coord: coord}
    }).Export(gs.As[gs.Rooter]())
}

type OrderTcc struct {
    coord tcc.Coordinator
}

func (o *OrderTcc) Run(ctx context.Context) error {
    // 相当于 @GlobalTransactional:Try 出错时协调器已逆序取消所有已 try 的参与者,
    // 并写下终态日志。
    return tcc.GlobalTCC(o.coord, "OrderService.Place", func(ctx context.Context) error {
        return nil // 参与者经上面的注册表条目运行
    })
}
```

(`reserve`/`charge` 等是你普通的业务调用——经自动注入的 `*gorm.DB` 走 SQL,或走 RPC 客户端;
TCC 语义见上述文献。)

**conf/app.properties** — 完整、带注释的面:

```properties
# --- 数据源(starter-gorm-mysql;提供自动注入的 *gorm.DB) --------------------
spring.gorm.mysql.instances.dsn=app:pass@tcp(127.0.0.1:3306)/demo?parseTime=true

# --- TCC 持久化 store(本 starter 的激活键) ----------------------------------
# 必须恰好是 "gorm",本 Store 才注册
# (OnProperty ... HavingValue("gorm"),无 MatchIfMissing——starter.go)。
# 设了它,tcc starter 的内存默认 Store 让位(OnMissingBean),
# 协调器与恢复 Runner 转而消费这一个。
spring.transaction.tcc.store=gorm

# --- TCC 能力(父 starter;tracing/恢复开关) ---------------------------------
spring.transaction.tcc.enabled=true
# 默认 true,列出以便发现。每个阶段一个 otel 子 span:
# tcc.try <participant> / tcc.confirm <participant> / tcc.cancel <participant>。
spring.transaction.tcc.tracing=true
# 启动恢复 Runner:扫 Pending() 并取消崩溃的事务。
spring.transaction.tcc.recover-on-start=true

# --- 可观测(starter-otel,可选) ----------------------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**验证**(对着配置的 MySQL):

```bash
# 失败的事务跑完后:
mysql> SELECT id, method, status, in_progress, tried FROM tcc_snapshots;
# 一行:事务 id、"OrderService.Place"、status=4(Cancelled);列见 §4.1
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-gorm-mysql + starter-transaction-tcc + starter-transaction-tcc-gorm
  ├─ tcc starter:ParticipantRegistry、内存 Store [OnMissingBean]、
  │   协调器、恢复 Runner [条件:enabled + recover-on-start]
  └─ 本 starter:gs.Provide(newGormStore as tcc.Store)
        [条件:OnProperty("spring.transaction.tcc.store").HavingValue("gorm")]
        │
gs.Run()
  ├─ 配置绑定:${spring.transaction.tcc.gorm} → gormConfig(0 个 value tag)
  ├─ *gorm.DB 自动注入 newGormStore 的第二个 ctor 参数
  ├─ store 构造:db.AutoMigrate(&tccSnapshot{})——建 tcc_snapshots,
  │  失败则快速失败(bean error)
  ├─ 顺序:持久化 Store 赢得 tcc starter 的 OnMissingBean 槽,于是
  │  newCoordinator 消费它(内存默认永不构造)
  ├─ 恢复 Runner.Run():Store.Pending() → 对每个快照,按方法名从注册表
  │  重建参与者并 coord.Recover()
  └─ 你的 Rooter/Runner bean 在装配后运行
```

构造日志行:`create gorm tcc store success`(tag `AppDef`)。

### 2.2 一次失败事务 —— 全走读,含持久化点

引自 `cloud/experimental/transaction/tcc/coordinator.go` 与 `store.go`:

1. **开始** —— `GlobalTCC(coord, method, fn)` 查出该方法的参与者并调 `coord.Execute`;
   每次 Try 之前协调器先写一个 `StatusTrying` 快照、点名即将运行的参与者,于是中途崩溃
   已知需要 cancel(`persist`)。
2. **reserve-stock try 成功** —— 协调器用 `tried=["reserve-stock"]`、
   `try_results={"reserve-stock": ...}` upsert 快照(`OnConflict UpdateAll`,store.go)。
   每个事务 id 一行、不断被覆盖——`tcc_snapshots` 是**当前状态**的日志,不是追加式历史。
3. **charge-payment try 成功** —— 同样节奏:快照变成 `tried=[reserve-stock,
   charge-payment]`。每次 `persist` 写入都是崩溃可续的存档点。
4. **publish-order 失败** —— `errors.New("broker unavailable")`。协调器逆序取消:
   `refund`(charge-payment)、再 `release`(reserve-stock)。每次 cancel 在 trace 里可见,
   并更新同一行快照。
5. **终态记录** —— `finish`:已**提交**的事务其行被删(活干完了,无需恢复);**取消或失败**
   的事务其行**保留**终态状态供运维查看(`StatusCancelled` = 4)。
6. **崩溃恢复(重启)** —— 恢复 Runner 读 `Pending()`(每个非终态行——store.go),按持久化的
   方法名从注册表重建参与者列表,`coord.Recover` 从日志位置取消:先进在途参与者(以 nil 结果
   ——绕开 JSON 往返问题),再逆序取消各已 try 参与者。未触及的参与者不取消。没有注册参与者的
   方法会记日志并跳过;单个恢复错误会记日志且不打断其余事务。

注意失败可持久性的不对称:`persist` 的存储错误是**有意吞掉**的——事务已推进,因一次日志写入
失败而让操作失败,比日志里留个缺口更糟。

---

## 3. 逐 key 行为参考

对本模块 `grep -rhoE 'value:"[^"]+"' --include='*.go'` 得到 **零**个 tag;下面的激活键是 bean
**条件**属性(grep 不可见),列入是因为它是打开本 Store 的唯一途径。本模块**完全不拥有 value
tag 键**——`*gorm.DB` 永远是容器的默认实例。

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `spring.transaction.tcc.store` | string | (未设置) | **激活键**(条件,非 value tag)。必须恰好 `gorm`——`OnProperty ... HavingValue("gorm")`,无 `MatchIfMissing`。 | 其它值 / 未设置 → 本 Store 永不注册、tcc starter 的内存默认留存:无 `tcc_snapshots` 表、**无崩溃恢复**,静默。 |

父 starter 中驱动本 Store 所喂机器的键(文档在
[starter-transaction-tcc](../starter-transaction-tcc),列此因它们改变本 Store 的可观测行为):
`spring.transaction.tcc.enabled`(默认 true)、`spring.transaction.tcc.tracing`(默认 true
——每阶段子 span)、`spring.transaction.tcc.recover-on-start`(默认 true——让本 Store 有意义的
Runner)。

**表结构**(由构造函数的 `AutoMigrate` 建,后端无关——只有 text 与 int 列,无穷方言特有类型):
`tcc_snapshots(id PK, method, status int 有索引, tried text, in_progress, try_results text,
updated_at)`。`tried` / `try_results` 为 JSON 编码(`[]string` / `map[string]any`)。

---

## 4. 验证与故障演练

### 4.1 取消记录已持久化

跑完整工程后:

```sql
SELECT id, method, status, in_progress, tried, try_results FROM tcc_snapshots;
-- 终态行:status = 4 (Cancelled),tried 带着被取消的参与者,
-- try_results 握着每个 Try 的(JSON 类型化)结果。
```

提交路径演练:修好失败参与者重跑——已提交事务的行被**删除**,所以成功后表为空是对的,
`Pending()` 也不返回任何东西。

### 4.2 store 跨重启持久(kill -9 打断事务)

1. 让扣款参与者的 Try 睡足够久,好在其间动手。
2. 起应用、触发事务,在它运行时 `kill -9` 进程。
3. 检查:`tcc_snapshots` 有一行 `status = 0 (Trying)`,`tried=["reserve-stock"]`、
   `in_progress="charge-payment"`。
4. 重启应用。恢复 Runner 记 `tcc recovery: transaction recovered`,先取消在途参与者(nil 结果)、
   再取消已 try 的,该行状态翻成 Cancelled。此序列由
   `TestGormStore_EndToEndExecuteThenRecover`(store_test.go)覆盖。
5. 失败子演练:若该方法未在接线期注册,恢复会记
   `tcc recovery: skip method with no registered participants`,该行停在 Trying——重新声明事务、
   再重启一次。

### 4.3 在 trace 里观察恢复

`tracing=true` + starter-otel 下,重启期取消按阶段发子 span,带事务 id / 参与者 / 阶段标签——
§4.2 重启后去 collector 查。

### 4.4 AutoMigrate 快速失败演练

把 `spring.gorm.mysql.instances.dsn` 指向用户无 DDL 权限的库:store bean 在构造期失败
(`auto-migrate tcc_snapshots failed`,tag `AppDef`)并中止启动——配错在启动暴露,而不是第一次
事务时。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 无 `tcc_snapshots` 表、什么都不持久 | `spring.transaction.tcc.store` 缺失或非 `gorm`——内存默认 Store 生效中 | 设为恰好 `gorm`;启动时核 `create gorm tcc store success` 日志行。 |
| 启动失败:`auto-migrate tcc_snapshots failed` | 自动注入的 `*gorm.DB` 无 DDL 权限或服务不可达 | 授予 DDL / 修驱动 starter 的 DSN(starter.go)。 |
| 崩溃的业务永不恢复,日志说 `no registered participants` | 参与者未在接线期注册(在 Runner 里注册,或方法名改了) | 在 bean 构造里、用 `GlobalTCC` 记录的**同一个**方法名注册(recovery.go)。 |
| cancel 拿到 `float64` 而非 `int`,或 `map[string]any` 而非结构体 | 恢复时的 JSON 往返:结果以 JSON 形态回来(store.go) | 让 Try 结果保持 JSON 友好(id、token、标量);在途参与者总以 nil 结果恢复。 |
| 恢复的事务取消了从未 try 的参与者 | ——不可能:恢复受日志的 `tried` + `in_progress` 约束 | 若真遇到,是真 bug——请上报。 |
| 多个 `*gorm.DB` 实例,TCC 日志落进错的库 | 不存在实例选择键;永远自动注入容器的默认实例 | 重构 bean 让 TCC 库成为默认,或把它包进自己的 starter。 |
| 表被终态行撑大 | 设计如此:取消/失败的行保留供检查;只有已提交事务被删 | 自行按计划清理已审计的终态行;把它们当审计轨迹。 |
| 恢复吞掉 DB 错误 | `Pending()` 失败只记日志(recovery.go);`persist` 错误按设计吞掉 | 监控 DB 与 `AppDef` 日志;别把静默当成功。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 0 个 value tag + 1 个激活条件键 |
| 必填 | 1 个(`spring.transaction.tcc.store=gorm`) |
| quickstart 前置外部依赖 | 1 个(gorm 驱动 starter 背后的数据库) |
| 「注意/坑」条数 | 8 |

设计嫌疑清单:

- 激活需额外一个属性,而 tcc 能力本身 import 即激活——这个不对称换来显式 Store 选择;可辩护,
  但值得写明。
- **没有 example/、也没有把 tcc + gorm store 全路径端到端接起来的集成冒烟**(只有 store 单测)
  → 补 example/;本文档的完整工程仅代码核过。
- `persist` 按设计吞掉存储错误(推进 > 日志完整)——store 宕机时事务中途留的缺口,恢复分不清
  「参与者从未 try」还是「日志丢了」;没有指标暴露 Save 失败。
- 恢复结果的 JSON 往返类型只在代码注释里写明,是静默的坑(store.go)。
- 终态行永久保留、无留存策略;已提交行被删,故审计覆盖随结果不对称。
