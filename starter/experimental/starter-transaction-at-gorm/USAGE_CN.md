# starter-transaction-at-gorm 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。本文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`plugin.go`、`branch.go`、`undolog.go`）与能力核心
`cloud/experimental/transaction/at`（`at.go`、`coordinator.go`、`lock.go`、`global.go`）
核验。**AT 模式语义（undo log / before-image、两阶段提交/回滚、全局锁隔离——即
"Seata AT 做什么"）属于[分布式事务文献](https://seata.apache.org/docs/user/at-mode)**，
本文只写 go-spring 的增量。

**边界**：global lock 与 undo log 都是进程内的——这是单进程 AT 等价物，不是分布式
Seata TC/TM/RM 部署（starter.go 包注释）。**激活**：空导入即可——两个配置 key 都默认
开启。**没有 recovery Runner**：崩溃后没有任何代码重放 undo log（见 §4.3、§6）。

---

## 1. 完整工程示例

两个玩具数据库（内存 sqlite）——账户余额与库存数量——由一个全局事务扣减。第二个库中
一次故意制造的 SQL 失败驱动两库自动回滚，均从捕获的 before-image 恢复。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring                    v1.3.x
    go-spring.org/starter-transaction-at-gorm latest
    go-spring.org/starter-otel               latest   // 可选：真实 trace 导出
    gorm.io/gorm                             latest
    gorm.io/driver/sqlite                    latest   // 或 mysql/postgres driver starter
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-at-gorm"
)

func main() { gs.Run() }
```

**service.go** —— 应用全部业务面：

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction/at"
    "go-spring.org/spring/gs"
    atgorm "go-spring.org/starter-transaction-at-gorm"
    "gorm.io/gorm"
)

type account struct {
    ID      int64 `gorm:"primaryKey;column:id"`
    Balance int   `gorm:"column:balance"`
}
func (account) TableName() string { return "account" }

type stock struct {
    ID    int64 `gorm:"primaryKey;column:id"`
    Count int   `gorm:"column:count"`
}
func (stock) TableName() string { return "stock" }

// starter 提供的两个 bean 直接注入构造函数：Coordinator（begin/commit/rollback）
// 与 GlobalLock（写-写隔离）。
type BankService struct {
    coord     at.Coordinator
    accountDB *gorm.DB
    stockDB   *gorm.DB
}

func newBankService(coord at.Coordinator, lock at.GlobalLock) (*BankService, error) {
    accountDB, err := openDB("account-db")
    if err != nil { return nil, err }
    stockDB, err := openDB("stock-db")
    if err != nil { return nil, err }

    if err := accountDB.AutoMigrate(&account{}); err != nil { return nil, err }
    if err := stockDB.AutoMigrate(&stock{}); err != nil { return nil, err }

    // 接入第 1 步：建 at_undo_log 表（fail-fast）。
    if err := atgorm.Migrate(accountDB); err != nil { return nil, err }
    if err := atgorm.Migrate(stockDB); err != nil { return nil, err }

    // 接入第 2 步：安装 AT plugin，每个数据库用互不相同的 resource id ——
    // 它是 undo log 与 lock key 里的 branch id，Coordinator 也按它去重 branch。
    if err := accountDB.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { return nil, err }
    if err := stockDB.Use(atgorm.NewPlugin("stock-db", coord, lock)); err != nil { return nil, err }

    if err := accountDB.Create(&account{ID: 1, Balance: 100}).Error; err != nil { return nil, err }
    if err := stockDB.Create(&stock{ID: 1, Count: 10}).Error; err != nil { return nil, err }
    return &BankService{coord: coord, accountDB: accountDB, stockDB: stockDB}, nil
}

// purchase 跑一个全局事务。failStock=true 故意让第二个库失败：全局事务随之回滚，
// 两个库都从 before-image 恢复——全程没有写过任何补偿代码。
func (s *BankService) purchase(ctx context.Context, cost, qty int, failStock bool) error {
    ctx, xid := s.coord.Begin(ctx)

    err := s.accountDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return tx.Model(&account{}).Where("id = ?", 1).
            Update("balance", gorm.Expr("balance - ?", cost)).Error
    })
    if err == nil {
        err = s.stockDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
            if failStock {
                return errors.New("stock service unavailable") // 故意失败
            }
            return tx.Model(&stock{}).Where("id = ?", 1).
                Update("count", gorm.Expr("count - ?", qty)).Error
        })
    }

    if err != nil {
        if rbErr := s.coord.Rollback(context.Background(), xid); rbErr != nil {
            return err // 回滚错误与业务错误一并上报
        }
        return err
    }
    return s.coord.Commit(context.Background(), xid)
}
```

（`openDB` 与 `gs.Provide(newBankService)` 的接线在
[example/example.go](example/example.go)——直接照抄；每个 sqlite 句柄设
`MaxOpenConns(1)`，避免共享内存库被每连接复制。）

**conf/app.properties** —— 完整带注释的配置面（两个 key 均为默认值，写出来便于发现）：

```properties
# --- AT 分布式事务 -----------------------------------------------------------
# 空导入即可；设 false 则只引入模块、不装配 coordinator/lock bean
# （OnProperty ... MatchIfMissing）。
spring.transaction.at.enabled=true

# 每个 branch 二阶段操作发一个 otel 子 span（at.commit <branch> /
# at.rollback <branch>），挂在 starter-otel 安装的全局上。没有 starter-otel 时
# 几乎零开销（全局 tracer 是 no-op）。
spring.transaction.at.tracing=true

# 启动崩溃恢复：扫描每个已接入库的 at_undo_log，回放（回滚）上次运行崩溃遗留的
# undo log。仅当数据库被多进程共享、恢复由外部处理时才设 false。
spring.transaction.at.recover-on-start=true

# --- 可观测（starter-otel，可选；取自 example-otel）--------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

**验证**（与冒烟验证过的 example 同构）：

```bash
bash example/check.sh
# 期望输出：
#   commit path OK: balance=70 stock=8 undo=0
#   rollback path OK: balance=70 stock=8 undo=0 - stock service unavailable
#   isolation path OK: second global transaction rejected with ErrLockConflict
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-transaction-at-gorm
  ├─ gs.Provide(MemoryGlobalLock as at.GlobalLock)   [条件: enabled + OnMissingBean]
  ├─ gs.Provide(newCoordinator as at.Coordinator)    [条件: enabled]
  └─ gs.Provide(newRecoveryRunner as gs.Runner)      [条件: enabled + recover-on-start]
        │
gs.Run()
  ├─ 配置绑定: ${spring.transaction.at} → Config      （3 个 value tag）
  ├─ lock bean: 进程内 MemoryGlobalLock；容器里出现 durable-lock bean 时
  │  它让位（OnMissingBean）                              (starter.go:77-79)
  ├─ coordinator bean: at.NewCoordinator(WithGlobalLock(lock)
  │  [tracing=true 时再加 WithObserver(AtObserver{})])   (starter.go:91-98)
  ├─ 你的 bean 构造函数执行: 每库 Migrate(db) + db.Use(NewPlugin(resource, coord, lock))
  │  —— 接入是代码接线，配置里看不见；plugin 同时把句柄登记给恢复扫描
  │  （plugin.go Initialize）
  └─ 恢复 Runner：扫描每个已接入库的 at_undo_log，回滚上次运行崩溃遗留的
     孤儿（§4.3）；无 server、无端口
```

构造期日志：`at coordinator created tracing=true`（tag `AppDef`）。

### 2.2 一次带失败的全局事务 —— 逐层走读

`purchase(ctx, 50, 5, failStock=true)`，理由均引源码：

1. **Begin** —— `coord.Begin(ctx)` 生成随机 16 字节 hex XID，在 coordinator 的
   `active` 表登记 `xid → nil`，返回 `WithXID(ctx, xid)`（coordinator.go:73-79）。
   上下文里的 XID 是区分"全局事务写"与"普通写"的唯一信号。
2. **branch 接入（构造期已完成）** —— `db.Use(NewPlugin(...))` 注册了 gorm 回调：
   `gorm:update` 前/后、`gorm:delete` 前/后、`gorm:create` 后（plugin.go:70-89）。
   `Migrate` 事先建好 `at_undo_log`——plugin 自己写这张表时被表名守卫跳过
   （plugin.go:99-100），捕获不会递归。
3. **SQL 执行并捕获 undo log** —— `accountDB.Update("balance", ...)`：
   - `before_update` 触发：`active()` 检查 suppress 标志 / undo-log 表 / XID
     （plugin.go:94-107），用语句**自带的 WHERE 子句**（`whereExpr`，保证镜像恰是
     将被写入的行）SELECT before-image，并对这些行 all-or-nothing 地拿全局锁——
     锁冲突直接让**语句**失败，本地事务回滚，而不是暂存一个冲突变更
     （plugin.go:113-131, lock.go:47-64）。
   - UPDATE 执行；`after_update` 触发：按主键回读 after-image，把
     `{before, after}` JSON 编码进一条 undo 行，并**在业务数据同一连接/事务上**
     INSERT，两者原子提交（plugin.go:186-190），随后注册 branch
     （`coord.Register`；按 resource id 去重——每个库每个 XID 恰好一个 branch，
     coordinator.go:81-95）。
   - 本地 gorm 事务提交：业务变更 + undo log **一起**落库。这是 AT 一阶段。
4. **失败** —— stock 步骤在任何 SQL 之前返回
   `errors.New("stock service unavailable")`：`stock-db` 从未注册 branch。错误传回
   `purchase`，后者调用 `coord.Rollback(ctx, xid)`。
5. **经 undo log 回滚** —— coordinator 取走 branch 列表（单次生效：对同一 XID 第二次
   Commit/Rollback 报 `ErrUnknownTransaction`，coordinator.go:134-143），按**注册逆序**
   展开（coordinator.go:120-128）。branch 的 `Rollback` 以 `ORDER BY id DESC` 读 undo
   行——最新语句先被撤销（branch.go:53-68）——`restore` 按 SQL 类型逆操作：INSERT → 按
   after-image 主键删行，DELETE → 从 before-image 重新插入，UPDATE → 写回 before-image
   值（branch.go:73-107）。所有恢复操作跑在 **suppressed context** 上，不会被再次捕获
   成新 undo log（plugin.go:34-39）。最后删除这些 undo log。
6. **释放锁** —— coordinator 释放该 XID 持有的全部锁 key；释放错误被有意吞掉，因为
   事务已定局（coordinator.go:149-153）。

成功路径上，第 5 步变为：每个 branch **删除自己的 undo log**（二阶段提交很廉价——业务
数据一阶段已本地提交，branch.go:42-46），随后释放锁。

---

## 3. 逐 key 行为参考

前缀 `${spring.transaction.at}`——共享的 `spring.transaction` 能力命名空间，Saga
（`spring.transaction.saga.*`）与 TCC 可并存。wrapper 的 `value` tag 是该前缀下的
顶层绝对 key。已用 `grep -rhoE 'value:"[^"]+"' --include='*.go'` 核对——恰好这两个：

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.transaction.at.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()`（starter.go:70）。`false` 不装配任何 bean——只引入模块。 | `false` → 容器装配时注入 `at.Coordinator`/`at.GlobalLock` 失败（无此 bean）；运行期写入不被捕获、静默退化为非 AT。 |
| `spring.transaction.at.tracing` | bool | `true` | coordinator 构造时追加 `at.WithObserver(transactionobserve.AtObserver{})`（starter.go:93-94）。每个 branch 阶段一个 span：`at.commit <branch>` / `at.rollback <branch>`，属性 `at.xid` / `at.branch` / `at.phase`（cloud/observe/transaction/observer.go:129-142）。 | `false`（或 `true` 但没引 starter-otel）：静默——无 span、无告警。 |
| `spring.transaction.at.recover-on-start` | bool | `true` | 注册崩溃恢复 gs.Runner（starter.go:95-99、recovery.go）。启动时扫描每个已接入库的 `at_undo_log`，逐个回滚孤儿 XID（见 §4.3）。 | `false` → 崩溃遗留的孤儿 undo log 不被回放：`at_undo_log` 持续增长，一阶段的业务变更保持已应用，需人工对账。 |

无枚举 key；无联动 key。注意"接入"旋钮（resource id、哪些库加入）是**代码不是配置**——
见 §6。

---

## 4. 验证与故障演练

### 4.1 SQL 错误回滚 —— 前后对照 at_undo_log

跑 example（`bash example/check.sh`，或 example/ 下 `go run . -manual`）。`runTest`
里的断言就是这一演练：

- 失败的 `purchase(50, 5, failStock=true)` 之后：`balance=70 stock=8 undo=0`——账户
  扣的 50 **从 before-image 恢复回 70**，尽管其本地事务早已提交；undo log 也已清空。
- 对真实数据库，等价手工检查是 `SELECT COUNT(*) FROM at_undo_log`——每个已定局的
  全局事务之后应为 0；持续非 0 表示某个 branch 的二阶段失败（见 §5 的
  StatusCommitFailed/RollbackFailed）。

### 4.2 写-写隔离演练

example 路径 3：让一个全局事务持有 account 行 1 不放，再开第二个碰同一行——第二条
语句以 `at: global row lock conflict`（`at.ErrLockConflict`）失败，第二个事务什么都没
暂存。在 trace 里读作 errored 的 `at.*` span，或像 example 那样断言
`errors.Is(err, at.ErrLockConflict)`。

### 4.3 崩溃恢复演练 —— kill -9 后的孤儿 undo log —— **已修复：启动 Runner**

恢复 Runner（recovery.go；`spring.transaction.at.recover-on-start`，默认开）解决了
本节此前记录的嫌疑：

- **一、二阶段之间 kill -9**：业务变更与 undo log 已在本地提交。coordinator 的
  `active` map 随进程消亡，全局事务不可能再提交——下次启动时 Runner 扫描每个已接入
  库的 `at_undo_log`（启动时任何行都是孤儿：Runner 执行前本进程不可能有在途事务），
  以 ERROR 级日志报出条数与最旧条目，然后按 XID 逐个重放 undo 回滚
  （`gormBranch.Rollback`，最新日志先行）。`TestATRecovery_ReplaysOrphanedUndoLogs`
  钉死。
- **二阶段回滚进行中 kill -9**：行只恢复了一半。重放是安全的：
  `Branch.Rollback` 幂等（branch.go:52-53）——undo log 只有在完整重放后才删除，
  下次启动会接着做完。
- 回放失败记 ERROR 且保留该 XID 的 undo log（不删任何东西），退化为旧行为——可见、
  且留有手工恢复所需数据。
- Runner 绝不令启动失败，各库独立扫描。

两个边界须知：

- **装配期接入**：Runner 靠 plugin 的 `Initialize` 登记发现数据库（plugin.go），因此
  plugin 必须在 bean 构造期 `db.Use`，不能放到更晚的 Runner 里——否则扫描漏掉该库。
- **单进程契约**：多进程共享数据库会破坏"启动时的行皆为孤儿"这一假设（会回滚别的
  实例的在途事务）。这类部署须设 `recover-on-start=false` 并自行对账；跨库 branch
  顺序也不重建（各库按自身 id 逆序重放）。

基于落盘数据库的手工演练：让 example 接 sqlite 文件库，在两个 branch 事务之间加
sleep，中途 kill -9，重启——期望先看到 `at recovery: found N orphaned undo-log
entries ...` ERROR，随后每个 XID 一条 `at recovery: global transaction "<xid>"
rolled back` INFO，且 `SELECT COUNT(*) FROM at_undo_log` → 0。

### 4.4 tracing 演练

`example-otel/`（docker-compose：Jaeger 在 :4317/:16686）跑同样的三条路径，随后验证
服务 `transaction-at-gorm-otel-example` 的 trace 已出现。期望：每个已提交事务一个
`at.commit account-db` + 一个 `at.commit stock-db` span，失败路径上一个
`at.rollback account-db` span，均带 XID 标签。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| Register 报 `at: unknown global transaction` | 该 XID 已被 Commit/Rollback 定局（单次生效），或没走 Begin | 一个 XID 一次定局；重试请重新 Begin（coordinator.go:134-143）。 |
| 写入报 `at: global row lock conflict` | 另一个在途全局事务持有该行 | 等对方定局后重试；这是隔离保障在起作用，不是 bug。 |
| 写入正常但没有 undo log、不回滚 | 库没接入：漏了 `Migrate`/`db.Use(NewPlugin(...))`，或写入跑在没有 XID 的 context 上 | 每库接入；写入必须跑在 `coord.Begin` 返回的 context 里（或用 `at.GlobalAT(coord)`）。 |
| `at_undo_log` 表缺失 / INSERT 失败 | 该库首次被捕获写入前没调 `Migrate(db)` | 启动时紧挨 `AutoMigrate` 调 `Migrate`，fail-fast。 |
| 两个库互相干扰 / branch 被重复释放 | 两个 `NewPlugin` 传了同一个 `resource` id | 每库一个独立 id——它是 branch id，也参与 lock key。 |
| 回滚报 `branch %q rollback: ...` | 恢复 SQL 失败（schema 漂移、行已不存在、连接问题） | 查死 XID 的 `at_undo_log.context` JSON；手工恢复；语义见 StatusRollbackFailed（at.go:180-182）。 |
| undo-log 行只增不清 | `recover-on-start=false`，或某次回放失败（ERROR 会点名 XID 且保留日志），或 StatusCommitFailed 的 branch | 监控 `SELECT COUNT(*) FROM at_undo_log`；查死 XID 的 `at_undo_log.context` JSON 手工恢复（安全：它们的事务不可能再定局）。 |
| gorm 报插件重名 | 同一 handle 调了两次 `db.Use`（名字 `at:<resource>`） | 每个 handle 接入一次。 |
| 重启后崩溃孤儿没被回滚 | plugin 在装配之后才装（Runner 里 / `gs.Run` 内），恢复扫描没看到该库 | 像 example 那样在 bean 构造期 `db.Use(NewPlugin(...))`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|------|
| 配置 key 总数 | 3 |
| 其中必填 | 0 |
| quickstart 前置外部依赖数 | 0（example-otel 追踪另需 1 个 Jaeger） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（保留既有 + 新增）：

- ~~**AT 无崩溃恢复 Runner**~~——已解决：启动恢复 Runner 会回放孤儿 undo log（§4.3），
  补齐与 Saga/TCC 的对称。遗留边界：仅限单进程契约（数据库被共享时设
  `recover-on-start=false`）、跨库 branch 顺序不重建。
- `Migrate`/`db.Use` 是每库手工步骤，配置里不可见——可接受（gorm plugin 本就是代码接线），
  但漏接一个库不会 fail-fast，它就静默地以非 AT 数据库参与。
- 接入顺序是约定而非强制：`Migrate` 须先于首次捕获写入；先装 plugin 后 Migrate 只在
  首条被拦截的 DML 上才失败。
- 二阶段失败语义不对称：commit 失败=清理失败（数据一致），rollback 失败=可能不一致、
  需要告警（at.go:176-182）——但 starter 除 span error 状态外没有暴露区分两者的
  指标/告警钩子。
- 核心 coordinator 有 `RetryPolicy`（`at.WithRetry`）但 starter 未接任何配置——二阶段
  重试经属性不可达。
- undo-log 增长监控完全留给用户（无 undo-log 数量指标）。
