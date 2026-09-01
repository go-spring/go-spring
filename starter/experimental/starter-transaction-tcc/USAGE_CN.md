# starter-transaction-tcc 使用说明 — 参考手册

概览见 [README_CN.md](README_CN.md)。所有行为声明均经源码核对（starter 的
`starter.go` / `config.go` / `recovery.go` / `starter_test.go`）、可运行的
[example/](example/)（`bash example/check.sh`）与 [example-otel/](example-otel/)
（docker-compose 起 Jaeger），以及被封装的协调器
[`go-spring.org/cloud/experimental/transaction/tcc`](../../../cloud/experimental/transaction/tcc)
（`tcc.go` / `coordinator.go` / `global.go` / `store.go`）；tracing 部分为
[`cloud/experimental/transaction/observe.go`](../../../cloud/experimental/transaction/observe.go)。
**TCC 模式自身的语义（Try/Confirm/Cancel 及参与者三义务 —— 幂等、空回滚、防悬挂）属于
分布式事务通识** —— 见 [Seata TCC 模式文档](https://seata.apache.org/docs/user/anchor?version=1.7.0)
与 README 的"Participant obligations"；本页只写 go-spring 的增量。

**激活方式**：blank import 即生效 —— 三个 key 全部默认开启（`MatchIfMissing`）。没有激活
key；`spring.transaction.tcc.enabled=false` 是显式退出。Contributor 形态：不开端口、不起
server，只注册 bean。`spring.transaction` 命名空间与 starter-transaction-saga 共享，Saga
与 TCC 可并行启用、互不冲突。

---

## 1. 完整工程示例

一个单进程"订单服务"，把预留库存与冻结余额编排为一个 TCC 事务。第二个参与者的 Try
故意失败（金额过大），第一个参与者的预留随即被 Cancel —— 正是
[example/example.go](example/example.go) 断言的场景。文件树：

```
demo/
├── go.mod
├── main.go
├── order/
│   └── order.go        # 账本 + OrderService + 自测
└── conf/
    └── app.properties
```

**go.mod**（关键依赖，与 [example/go.mod](example/go.mod) 同构）：

```
require (
    go-spring.org/spring                v1.3.x
    go-spring.org/cloud                 latest  // tcc 协调器在此
    go-spring.org/starter-transaction-tcc latest
    go-spring.org/starter-otel          latest  // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    _ "demo/order"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"          // tracing 全局（可选）
    _ "go-spring.org/starter-transaction-tcc"
)

func main() { gs.Run() }
```

**order/order.go** —— 完整 TCC 面（节选自随仓 example；账本实现以它为准）：

```go
package order

import (
    "context"
    "errors"
    "sync"

    "go-spring.org/cloud/experimental/transaction/tcc"
    "go-spring.org/spring/gs"
)

// 一个 TCC 资源：`available` 池 + 按 txID 的 `frozen` 预留。Try 把数值移入
// frozen（已预留、但不对外表现为已消费）；Confirm 落下 frozen（消费定局）；
// Cancel 把它退回 available。
type ledger struct {
    mu        sync.Mutex
    available int
    frozen    map[string]int // 按 tx id 键控 -> Cancel 幂等、Try 防悬挂
}

func (l *ledger) try(txID string, n int) error {
    l.mu.Lock(); defer l.mu.Unlock()
    if _, done := l.frozen[txID]; done { return nil } // 重放 = 不重复预留
    if l.available < n { return errors.New("insufficient") }
    l.available -= n
    l.frozen[txID] = n
    return nil
}

func (l *ledger) confirm(txID string) error { // 幂等：缺失 = 已确认
    l.mu.Lock(); defer l.mu.Unlock()
    delete(l.frozen, txID)
    return nil
}

func (l *ledger) cancel(txID string) error { // 幂等 + 空回滚安全
    l.mu.Lock(); defer l.mu.Unlock()
    if n, ok := l.frozen[txID]; ok {
        l.available += n
        delete(l.frozen, txID)
    }
    return nil
}

type OrderService struct {
    Coord tcc.Coordinator          `autowire:""` // starter 的 bean
    Reg   *tcc.ParticipantRegistry `autowire:""` // starter 的注册表 bean

    stock, balance *ledger
}

func init() {
    gs.Provide(func() *OrderService {
        s := &OrderService{stock: newLedger(10), balance: newLedger(100)}

        // 在装配期（bean 构造）注册参与者，挂在逻辑方法名下 —— 恢复 Runner 正是
        // 用这个 key 回查参与者（recovery.go）。三个相位全部必填：缺一个会被
        // 协调器的 validate 在任何副作用之前拒绝。
        s.Reg.Register("OrderService.Place",
            tcc.Participant{
                Name: "ReserveStock",
                // 各相位从协调器递来的 ctx 里读 tx id —— 即 place() 用
                // WithTransactionID 构建的那个 ctx。
                Try: func(ctx context.Context) (any, error) {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return nil, s.stock.try(id, 2)
                },
                Confirm: func(ctx context.Context, _ any) error {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return s.stock.confirm(id)
                },
                Cancel: func(ctx context.Context, _ any) error {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return s.stock.cancel(id)
                },
            },
            tcc.Participant{
                Name: "FreezeBalance",
                Try: func(ctx context.Context) (any, error) {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return nil, s.balance.try(id, 999) // 故意失败
                },
                Confirm: func(ctx context.Context, _ any) error {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return s.balance.confirm(id)
                },
                Cancel: func(ctx context.Context, _ any) error {
                    id, _ := tcc.TransactionIDFromContext(ctx)
                    return s.balance.cancel(id)
                },
            },
        )
        return s
    }).Export(gs.As[gs.Rooter]())
}
```

（随仓 example 里 `place` 改为每次调用内联构造参与者、闭包捕获 `txID` 参数 —— 两种写法
都有效。此处展示的 注册表 + `tcc.GlobalTCC(coord, reg)` 形态才是能按方法名恢复的关键；
注意**恢复期间** Runner 的 ctx 不携带 tx id，生产环境的 Confirm/Cancel 应基于 Try result
或自持预留表定位，而非依赖 ctx。）

运行路径（自测，与 example 的 `runTest` 同构；此处金额为参数，而上面的注册块固定了
演练用的 2/999 —— 二选一保持两条路径一致即可）：

```go
func (s *OrderService) place(ctx context.Context, txID string, qty, cost int) (tcc.Result, error) {
    ctx = tcc.WithTransactionID(ctx, txID) // 边缘处注入幂等 key
    return s.Coord.Execute(ctx, tcc.Transaction{
        ID:     txID,
        Method: "OrderService.Place", // 恢复 key —— 必须与注册名一致
        Participants: []tcc.Participant{
            {
                Name:    "ReserveStock",
                Try:     func(context.Context) (any, error) { return nil, s.stock.try(txID, qty) },
                Confirm: func(context.Context, any) error { return s.stock.confirm(txID) },
                Cancel:  func(context.Context, any) error { return s.stock.cancel(txID) },
            },
            {
                Name:    "FreezeBalance",
                Try:     func(context.Context) (any, error) { return nil, s.balance.try(txID, cost) },
                Confirm: func(context.Context, any) error { return s.balance.confirm(txID) },
                Cancel:  func(context.Context, any) error { return s.balance.cancel(txID) },
            },
        },
    })
}

// 路径 1 —— 提交：place(ctx, "order-commit", 3, 60) -> StatusCommitted，
//   stock 10->7、balance 100->40、无冻结。
// 路径 2 —— 回滚：place(ctx, "order-rollback", 2, 999) -> FreezeBalance 的
//   Try 报 "insufficient"；协调器按尝试逆序 Cancel 已 Try 的参与者 —— 
//   ReserveStock 的预留被释放；两本账回到 7/40、0 冻结。
```

**conf/app.properties** —— 完整注释配置面（复制自
[example/conf/app.properties](example/conf/app.properties) 并加 tracing）：

```properties
# --- tcc ---------------------------------------------------------------------
# 三个 key 全部默认开；blank import 即可。此处列出便于发现。
spring.transaction.tcc.enabled=true
spring.transaction.tcc.tracing=true

# 启动恢复扫描。⚠ 默认内存 Store 下是 no-op（重启即丢日志）；只有存在持久化
# tcc.Store bean 才真正生效。注意：与 saga（starter-transaction-saga-gorm）不同，
# 目前没有为 TCC 发布持久化 Store starter —— 需自备 tcc.Store bean。
spring.transaction.tcc.recover-on-start=true

# --- 可观测（starter-otel）—— 完整可用副本见 example-otel/conf ---------------
spring.observability.enable=true
spring.observability.service-name=tcc-demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**预期输出**（故障演练 —— `place(ctx, "order-rollback", 2, 999)`）：

```
... tcc coordinator created tracing=true
commit path OK: Committed                    （自测里路径 1 先跑）
rollback path OK: Cancelled - tcc: participant FreezeBalance Try: insufficient
```

`place` 返回 `err = "insufficient"`、`res.Status = tcc.StatusCancelled`、
`res.Errors = [{Participant: FreezeBalance, Phase: Try, Err: insufficient}]`，两本账与路径 2
之前完全一致（stock 7/0、balance 40/0）—— ReserveStock 的预留已被 Cancel 释放。

**验证**：

```bash
bash example/check.sh          # 随仓冒烟：跑两条路径、自断言、退出码 0
# trace 变体：
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :4317/:16686
cd example-otel && go run .    # 断言 span 到达 Jaeger 后自退出
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-transaction-tcc
  ├─ gs.Provide(tcc.NewParticipantRegistry)             [条件: enabled]
  ├─ gs.Provide(MemoryStore as tcc.Store)               [enabled + OnMissingBean[Store]]
  ├─ gs.Provide(newCoordinator)                         [enabled; 导出为 Coordinator]
  └─ gs.Provide(newRecoveryRunner)                      [enabled + recover-on-start; 导出为 gs.Runner]
        │
gs.Run()
  ├─ 配置绑定: ${spring.transaction.tcc} → Config（3 个 value tag，全部 := 默认）
  ├─ bean 装配: Store 注入协调器；registry + Store + Coordinator 注入恢复 Runner
  ├─ Rooter/bean 构造: 你的代码在此调用 reg.Register(...) —— 装配期注册是硬性
  │   要求（恢复按方法名重建参与者）
  ├─ 日志: "tcc coordinator created tracing=<v>"
  ├─ Runner 执行（除非持久化 Store 留有非终态快照，恢复 Runner 是 no-op）
  │   —— 绝不令启动失败
  └─ SIGTERM: 标准容器停机；无 tcc 专属 drain
```

自备的持久化 `tcc.Store` bean（作为容器中唯一的 `tcc.Store` 注册）会同时接管协调器日志
与恢复扫描 —— 与 saga 的 gorm store 同一个 seam，只是目前需要自己写。

### 2.2 一次事务逐层走读 —— 含失败路径

`place(ctx, "order-rollback", 2, 999)`，参与者 [ReserveStock, FreezeBalance(失败)]：

1. **先 validate**：协调器在任何副作用之前拒绝缺 Name、重名、或 Try/Confirm/Cancel 为
   nil 的参与者（coordinator.go `validate` —— "a missing one is a programming error caught
   before any side effect"）。结果状态 `CancelFailed`，错误直接返回。
2. **Try ReserveStock**：协调器先持久化意图 —— 快照 `{Status: Trying, InProgress:
   "ReserveStock"}`（Try 崩溃前可能已部分预留，恢复必须知道要 Cancel 这个参与者）——
   再执行 Try。成功后重写快照，把参与者折入 `Tried` 并清空 `InProgress`。
3. **Try FreezeBalance 失败**（"insufficient"）：失败错误先追加进 `Result.Errors`；随后
   `cancel` 按尝试**逆序**对已 Try 参与者执行 —— ReserveStock.Cancel(txID) 释放预留。
   Cancel 收到的是 Try 记录的返回值（此处为 nil）；参与者必须容忍 nil（空回滚）。
4. **finish**：已提交事务的日志被**删除**；已取消/失败的事务日志**保留**终态供检查。
5. `Execute` 返回 `(Result{Status: Cancelled, Errors:[…]}, Try 的错误)`。

成功路径的分歧在决策点：全部 Try 成功后，协调器先**持久化提交决策**（`StatusConfirming`），
然后才按正序 Confirm 全部已 Try 参与者。Confirm 失败**不会**让 `Execute` 返回错误
（"a confirm failure is not a try error"）；它以 `StatusConfirmFailed` + `Result.Errors`
浮出 —— 这是人工介入的信号，因为 TCC 契约要求 Confirm/Cancel 最终成功（给它们配非零
的 `Participant.Retry`）。

值得知道的设计取舍：在飞日志的持久化错误被有意吞掉（"failing the whole operation on a
log write would be worse than a gap in the log"，coordinator.go）；重试复用
`resilience.Policy`（"default" executor），使 TCC 相位重试与出站 resilience 共用一套旋钮。

---

## 3. 逐 key 行为参考

恰好 3 个 key（已核对：`grep -rhoE 'value:"[^"]+"' … | sort -u` → 3 个 tag）。均为
`spring.transaction.tcc` 下的**顶层绝对 key** —— 只绑定一个 `Config` 实例，没有多实例
group。

| key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.transaction.tcc.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()` —— 缺省即开启。`false` 导入模块但不贡献任何 bean。 | 误设 `false` → 无 Coordinator bean → 装配期 autowire `tcc.Coordinator` 失败（可见）。 |
| `spring.transaction.tcc.tracing` | bool | `true` | 构造时挂 `transaction.TccObserver{}` → 在 starter-otel 全局上为每个参与者相位开一个 otel 子 span（`tcc.try/confirm/cancel <name>`）。无 starter-otel 时是 no-op。 | 期望 trace 却没引 starter-otel → 什么都没有，且无任何告警。 |
| `spring.transaction.tcc.recover-on-start` | bool | `true` | 注册恢复 `gs.Runner`：扫描 `Store.Pending()` 找**非终态**快照（`Trying`/`Confirming`/`Cancelling`）并推向已决结果 —— `Confirming` 正向 Confirm，否则逆向 Cancel。⚠ **与 store seam 耦合且当前为死 key**：没有随仓的持久化 TCC Store starter；内存 store 重启后扫描恒空。自备 `tcc.Store` bean 才能激活。 | 持久化 store 下设 `false` → 崩溃遗留事务永不收敛（静默不一致）；无持久化 store 设 `true` → no-op、无告警。 |

由**其他模块**持有的联动 key：`spring.observability.*`（starter-otel）。注意代码中**不存在**
`spring.transaction.tcc.store` key —— README 曾声称有；选型靠贡献 `tcc.Store` bean，而非
配置项（见 §6）。

---

## 4. 验证与故障演练

### 4.1 提交与取消演练（随仓）

```bash
bash example/check.sh
```

example 自断言两条路径（提交：账面下降、无冻结；回滚：Try 失败 + 逆序 Cancel、账面不
变），任何偏差都以非零退出。日志行：

```bash
go run ./example | grep -E 'commit path OK|rollback path OK'
```

### 4.2 Confirm 失败时的行为演练

让第二个参与者的 `Confirm` 返回错误，跑一个完全可 Try 的事务（如 cost 60）：全部 Try
成功、提交决策落盘（`StatusConfirming`）、失败的 Confirm 把结果降级为
`StatusConfirmFailed`；第一个参与者保持已 Confirm（Confirm 正序执行、**没有** "confirm
后再 cancel" 的回退），且 `Execute` 返回 **nil 错误**、失败只在 `Result.Errors` 里。这个
不对称就是 TCC 契约：Confirm 必须最终成功（重试 + 告警），因为 Cancel 无法撤销已确认的
参与者。

### 4.3 持久化边界 —— 依赖恢复前必读

**本 starter 的 TCC 是内存态、非持久化的。** 默认 `tcc.Store` 是进程内 map：崩溃
时 TCC 日志随协调器状态一起消失，恢复 Runner
（`spring.transaction.tcc.recover-on-start`，默认开）重启后是空转——无日志可扫。崩溃
事务的 Confirm/Cancel 决策就此丢失，Try 到一半的参与者留给业务侧自行对账。

今天就需要可崩溃恢复的分布式事务，请用 **Saga** +
[starter-transaction-saga-gorm](../starter-transaction-saga-gorm)
（`spring.transaction.saga.store=gorm`），其持久化 saga 日志天然驱动同类启动恢复。
要让 TCC 自身持久化，需贡献你自己的 `tcc.Store` bean（见下一节）——本仓库不附带
持久化 TCC store。

### 4.4 重启恢复演练（需自备持久化 store）

1. 贡献一个持久化 `tcc.Store`（仿
   [starter-transaction-saga-gorm](../starter-transaction-saga-gorm)：导出为 `tcc.Store`
   的 bean，AutoMigrate 一张 `tcc_snapshots` 式的表）。内存默认项经 `OnMissingBean` 让位。
2. 事务中途 kill -9。三个崩溃点恢复方向不同：
   - 崩在 **Try**（`Trying`、`InProgress` 有值）：逆向 Cancel；在飞参与者**最先**、以 nil
     Try 结果被 Cancel（空回滚），随后按逆序 Cancel 已 Try 者；
   - 崩在**提交决策之后**（`Confirming`）：正向 Confirm —— 已落盘的决策不被二次猜测；
     Confirm 的幂等义务保证重放安全（`TestRecoveryRunner_ConfirmsPendingCommit` 钉死）；
   - 崩在 **Cancel** 期间（`Cancelling`）：继续逆向 Cancel。
3. 观察：`tcc recovery: transaction "tx-N" recovered with status Committed|Cancelled`。
   已提交事务**删除**日志；方法不再注册的记录 `no participants registered for method ...
   skipping`。
4. 恢复 Runner 绝不令启动失败；单事务错误记日志并跳过。

### 4.5 观测

- Trace（需 starter-otel）：每相位一个 span，`tcc.try <participant>` / `tcc.confirm <…>` /
  `tcc.cancel <…>`，属性 `tcc.id`、`tcc.participant`、`tcc.phase`；失败以 error 状态记录。
  [example-otel](example-otel/) 端到端验证了这条链（`docker compose up -d` 后 `go run .`，
  程序查询 Jaeger API 断言服务 `transaction-tcc-otel-example` 的 trace 存在）。
- 日志：tag `AppDef`；`tcc coordinator created tracing=…` 与 `tcc recovery: …` 系列。
- 无 metrics、无健康指标（见 §6）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| `tcc.Coordinator` autowire 报错 | `enabled=false` 或条件未命中 | 去掉禁用项；blank import 即默认开启。 |
| `tcc: participant "X" must define Try, Confirm and Cancel`（任何动作前） | 存在 nil 相位 —— 协调器 `validate` 在副作用前拒绝 | 三个相位都补齐；与 Saga 步骤不同，TCC 没有可选相位。 |
| 崩溃后 Cancel 没有执行 | 内存默认 store —— 重启丢日志 | 自备持久化 `tcc.Store` bean；仅设 `recover-on-start` 无用。 |
| `tcc recovery: no participants registered for method "X"` | 参与者在 Runner 里/启动后注册，或方法名漂移 | 在 bean 构造期、以持久化的精确 `Method` 字符串注册。 |
| 并发事务重复预留 | 预留未按 tx id 键控 → Try 重放不是 no-op（违反防悬挂） | 一切预留以事务 id 为键（`WithTransactionID`），照 example 的账本做。 |
| `StatusConfirmFailed` 且 `Execute` 返回 nil 错误 | Confirm 最终失败 —— 设计上不算 try 错误 | 对 Confirm 重试/告警（`Participant.Retry` 非零）；查 `Result.Errors`；不存在 Confirm 后的 Cancel 回退。 |
| Cancel 报 "already released" 类错误 | Cancel 不幂等 / 不耐空回滚 | 让 Cancel 容忍缺失的预留（nil tried 值）—— 见 example 的 `cancel`。 |
| `tracing=true` 却无 span | 未引 starter-otel / observability 关闭 | 引入 starter-otel 并配 `spring.observability.*`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 3（全 bool、全默认开） |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0（仅 example-otel 需 Jaeger） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（审计台账；保留原有，新增在后）：

- README 曾声称存在持久化 Store 选型 key `spring.transaction.tcc.store`；代码不读它、也无
  TCC 持久化 Store starter → 已改为"自备 `tcc.Store` bean"（保留）。
- README 曾说"两个 bean"；容器实际持有四个（registry、内存 store、协调器、恢复
  Runner） → README 已修（保留，数量已更正）。
- starter.go 包文档同样写 "two beans"（24-31 行），而 `init()` 注册了四个 → 与 README 同源
  漂移，修注释。
- `recover-on-start` 默认即死 key，且没有随仓持久化 store（与 saga 的 `-gorm` starter 不
  对称）→ 要么补一个 tcc-gorm starter，要么把 seam 写得更醒目。
- 无 metrics、无健康指标：卡在 `ConfirmFailed`/`CancelFailed` 的事务除手工翻保留的 store
  行外不可见 → 考虑对非终态/失败快照加 gauge 或健康检查。
- 在飞日志的持久化错误被设计性吞掉（coordinator.go `persist`）→ 持久化实现必须自监控；
  至少考虑一条 warn 日志。
