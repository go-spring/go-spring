# starter-transaction-saga 使用说明 — 参考手册

概览见 [README_CN.md](README_CN.md)。所有行为声明均经源码核对（starter 的
`starter.go` / `config.go` / `recovery.go` / `starter_test.go`，以及被封装的协调器
[`go-spring.org/cloud/experimental/transaction`](../../../cloud/experimental/transaction) 的
`coordinator.go` / `global.go` / `store.go` / `transaction.go`，tracing 部分为
[`cloud/experimental/transaction/observe.go`](../../../cloud/experimental/transaction/observe.go)）。
**Saga 模式自身的语义（前向步骤 + 逆向补偿、Saga/TCC/AT 选型）属于分布式事务通识** —— 见
[Seata Saga 模式文档](https://seata.apache.org/docs/user/saga) 与 README 对比表；本页只写
go-spring 的增量。

**激活方式**：blank import 即生效 —— 三个 key 全部默认开启（`MatchIfMissing`）。没有激活
key；`spring.transaction.saga.enabled=false` 是显式退出。本 starter 是 Contributor 形态：
不开端口、不起 server，只注册 bean。

> ⚠ **本模块没有可运行 example** —— 不存在 `example/` 目录（不像 starter-transaction-tcc）。
> 下面的完整工程基于源码验证的行为 + 兄弟 tcc example 的结构构建；已按协调器与单测逻辑
> 核对，但未作为随仓 example 冒烟运行。见 §6。

---

## 1. 完整工程示例

一个单进程"订单网关"，把三个下游效果（库存、支付、通知）编排为一个 Saga。第三个步骤
故意失败，前两个步骤被逆向补偿 —— 预期输出附在代码后。

```
demo/
├── go.mod
├── main.go
├── order/
│   └── order.go        # 步骤定义 + OrderService + 自测
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring                 v1.3.x
    go-spring.org/cloud                  latest  // transaction 协调器在此
    go-spring.org/starter-transaction-saga latest
    go-spring.org/starter-otel           latest  // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    _ "demo/order"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"           // tracing 全局（可选）
    _ "go-spring.org/starter-transaction-saga"
)

func main() { gs.Run() }
```

**order/order.go** —— 完整 Saga 面：

```go
package order

import (
    "context"
    "errors"
    "sync"

    "go-spring.org/cloud/experimental/transaction"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

// 两个玩具下游。真实部署里它们是对其他服务的 HTTP/MQ 调用；构成 Saga 的关键是
// 每个 Action 立即产生真实效果，失败时由业务级 Compensate 撤销。
type inventory struct {
    mu       sync.Mutex
    deducted map[string]int // 按 saga id 键控 -> 补偿幂等
}

func (i *inventory) deduct(id string, n int) error {
    i.mu.Lock(); defer i.mu.Unlock()
    if _, done := i.deducted[id]; done { return nil } // 重放安全
    i.deducted[id] = n
    log.Info(context.Background(), log.TagAppDef, "inventory: deducted %d for %s", n, id)
    return nil
}

func (i *inventory) restock(id string) error {
    i.mu.Lock(); defer i.mu.Unlock()
    if n, ok := i.deducted[id]; ok {          // 幂等：缺失 = 已撤销
        delete(i.deducted, id)
        log.Info(context.Background(), log.TagAppDef, "inventory: restocked %d for %s", n, id)
    }
    return nil
}

type payment struct {
    mu    sync.Mutex
    spent map[string]int
}

func (p *payment) charge(id string, amount int) (string, error) {
    p.mu.Lock(); defer p.mu.Unlock()
    if _, done := p.spent[id]; done { return "", nil }
    p.spent[id] = amount
    log.Info(context.Background(), log.TagAppDef, "payment: charged %d for %s", amount, id)
    return "pay:" + id, nil // token 经 StepResults 传给 Compensate
}

func (p *payment) refund(_ context.Context, result any) error {
    // result 是 Action 的返回值（支付 token）；真实代码里退款 API 以它定位。
    log.Info(context.Background(), log.TagAppDef, "payment: refunded %v", result)
    return nil
}

type OrderService struct {
    Coord transaction.Coordinator   `autowire:""` // starter 提供的 bean
    Reg   *transaction.StepRegistry `autowire:""`

    inv *inventory
    pay *payment
}

func init() {
    gs.Provide(newOrderService).Export(gs.As[gs.Rooter]())
}

func newOrderService() *OrderService {
    s := &OrderService{inv: &inventory{deducted: map[string]int{}},
        pay: &payment{spent: map[string]int{}}}

    // 在装配期（bean 构造）注册步骤，挂在逻辑方法名下 —— 恢复 Runner 正是用这个
    // key 回查步骤（recovery.go）。步骤 3 故意失败，用于 §4 的演练。
    s.Reg.Register("OrderService.Place",
        transaction.Step{
            Name: "DeductInventory",
            // Action/Compensate 从协调器递来的 ctx 里读 saga id —— 即 place()
            // 用 WithSagaID 构建的那个 ctx。
            Action: func(ctx context.Context) (any, error) {
                id, _ := transaction.SagaIDFromContext(ctx)
                return nil, s.inv.deduct(id, 2)
            },
            Compensate: func(ctx context.Context, r any) error {
                id, _ := transaction.SagaIDFromContext(ctx)
                return s.inv.restock(id)
            },
        },
        transaction.Step{
            Name: "ChargePayment",
            Action: func(ctx context.Context) (any, error) {
                id, _ := transaction.SagaIDFromContext(ctx)
                return s.pay.charge(id, 50)
            },
            Compensate: func(ctx context.Context, r any) error { return s.pay.refund(ctx, r) },
        },
        transaction.Step{
            Name:   "NotifyUser", // 失败 -> 触发 2、1 的逆向补偿
            Action: func(context.Context) (any, error) { return nil, errors.New("notify: SMTP unavailable") },
            // 无需 Compensate：该步骤永不成功。（已成功步骤若 Compensate 为 nil
            // 属于补偿失败 —— 见 §5。）
        },
    )
    return s
}

func (s *OrderService) place(ctx context.Context, id string) (transaction.Result, error) {
    ctx = transaction.WithSagaID(ctx, id) // 边缘处注入幂等 key
    steps, _ := s.Reg.Lookup("OrderService.Place")
    // Execute 是运行期入口；Recover（§2.2/§4.3）由 starter 的恢复 Runner 驱动，
    // 不由应用代码调用。
    return s.Coord.Execute(ctx, transaction.Saga{ID: id, Method: "OrderService.Place", Steps: steps})
}
```

装饰器等价物 —— 无反射版的 `@GlobalTransactional(SAGA)`：用
`transaction.GlobalTransactional(coord, reg)` 包住 `place`，按方法名驱动注册表查找；未注册
的方法直接透传 `proceed`。两条路径共用同一个协调器 bean。

自测 main 的写法照抄
[starter-transaction-tcc/example/example.go](../starter-transaction-tcc/example/example.go)：
启动 500ms 后在 goroutine 里跑一次 `place`，断言 `err != nil` 且
`res.Status == transaction.StatusCompensated`，然后 `syscall.Kill(os.Getpid(), syscall.SIGTERM)`。

**conf/app.properties** —— 完整注释配置面：

```properties
# --- saga --------------------------------------------------------------------
# 三个 key 全部默认开；blank import 即可。此处列出便于发现。
spring.transaction.saga.enabled=true
spring.transaction.saga.tracing=true

# 启动恢复扫描。⚠ 默认内存 Store 下是 no-op（重启即丢日志）；只有接了持久化
# Store 才真正生效 —— 见下。
spring.transaction.saga.recover-on-start=true

# --- 可选持久化 store（starter-transaction-saga-gorm）------------------------
# 需与该 starter 的 blank import + 某个 gorm driver starter（mysql/postgres/...）
# 同时启用：gorm Store 接管协调器日志与恢复扫描（OnMissingBean），并
# AutoMigrate saga_snapshots 表。
# spring.transaction.saga.store=gorm

# --- 可观测（starter-otel）----------------------------------------------------
spring.observability.enable=true
spring.observability.service-name=saga-demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

**预期输出**（故障演练 —— `place(ctx, "order-1")`，NotifyUser 失败）：

```
... saga coordinator created tracing=true
... inventory: deducted 2 for order-1
... payment: charged 50 for order-1
... payment: refunded pay:order-1        # 补偿按步骤逆序执行
... inventory: restocked 2 for order-1
```

且 `place` 返回 `err = "notify: SMTP unavailable"`、
`res.Status = StatusCompensated`、`res.Errors[0] = {Step: NotifyUser, Phase: Action}`。

**验证**（`go run .` 后）：上述五行日志出现；退出码 0。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-transaction-saga
  ├─ gs.Provide(transaction.NewStepRegistry)            [条件: enabled]
  ├─ gs.Provide(MemoryStore as transaction.Store)       [enabled + OnMissingBean[Store]]
  ├─ gs.Provide(newCoordinator)                         [enabled; 导出为 Coordinator]
  └─ gs.Provide(newRecoveryRunner)                      [enabled + recover-on-start; 导出为 gs.Runner]
        │
gs.Run()
  ├─ 配置绑定: ${spring.transaction.saga} → Config（3 个 value tag，全部 := 默认）
  ├─ bean 装配: Store 注入协调器；StepRegistry + Store + Coordinator 注入恢复 Runner
  ├─ Rooter/bean 构造: 你的代码在此调用 reg.Register(...) —— 这就是注册必须
  │   在装配期而非 Runner 里做的原因
  ├─ 日志: "saga coordinator created tracing=<v>"
  ├─ Runner 执行（除非持久化 Store 里留有 StatusRunning 快照，恢复 Runner 首跑
  │   是 no-op）—— 绝不令启动失败
  └─ SIGTERM: 标准容器停机；无 saga 专属 drain
```

持久化 Store starter（saga-gorm）在 `spring.transaction.saga.store=gorm` 下注册自己的
`transaction.Store`；因内存默认项是 `OnMissingBean`，持久化 store 胜出，协调器与恢复扫描
同时切换过去 —— 业务代码零改动（starter.go 注释："It steps aside (OnMissingBean) the
moment a durable-Store starter contributes its own transaction.Store"）。

### 2.2 一次事务逐层走读 —— 含失败路径

`place(ctx, "order-1")`，步骤 [DeductInventory, ChargePayment, NotifyUser(失败)]：

1. `GlobalTransactional`（或直接调用）按方法名查步骤；未注册的方法透明落到 `proceed`
   （global.go：未声明即不拦截）。
2. saga id 取自 ctx（`WithSagaID`）；缺失时退回方法名 —— 仅对单实例在飞正确，因此务必
   显式设置 id。
3. **步骤 1**：协调器先持久化意图 —— 快照 `{Status: Running, InProgress:
   "DeductInventory"}` —— 再跑 Action（coordinator.go："Record the intent before running:
   the Action may cause a side effect a crash would strand"）。成功后重写快照，把该步骤折入
   `Completed` 并清空 `InProgress`。
4. **步骤 2** 同样：意图 → Action（"charged 50"）→ 确认完成，支付 token 记入
   `StepResults`。
5. **步骤 3 Action 失败**：失败错误先追加进 `Result.Errors`（被补偿的 saga 仍能解释为何
   回滚）；随后 `compensate` 按**逆序**跑已成功步骤 —— ChargePayment.Compensate(token) →
   DeductInventory.Compensate —— 各自在与 action 相同的 retry 策略与 observer 下执行；
   被触及的步骤若 Compensate 为 nil，置 `StatusCompensationFailed` 并记为不可逆（上抛，
   绝不静默跳过）。
6. **finish**：已提交 saga 的日志被**删除**（无需恢复）；被补偿/失败 saga 的日志**保留**
   终态，供运维查看。
7. `Execute` 返回 `(Result{Status: Compensated, Errors:[…]}, 原始 Action 错误)` —— 补偿
   结果绝不掩盖 action 错误。

值得知道的设计取舍：`persistRunning` 期间的持久化错误被有意吞掉 —— "the saga has
already made progress and failing the whole operation on a log write would be worse than a gap
in the log"（coordinator.go）。重试复用 `resilience.Policy`（"default" executor），使 saga
步骤重试与出站 resilience 共用一套旋钮。

---

## 3. 逐 key 行为参考

恰好 3 个 key（已核对：`grep -rhoE 'value:"[^"]+"' … | sort -u` → 3 个 tag）。均为
`spring.transaction.saga` 下的**顶层绝对 key** —— starter 只绑定一个 `Config` 实例，没有
多实例 group。

| key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.transaction.saga.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()` —— 缺省即开启。`false` 导入模块但不贡献任何 bean（测试二进制要类型不要装配时有用）。 | 误设 `false` → 无 Coordinator bean → 装配期 autowire `transaction.Coordinator` 失败（可见，不静默）。 |
| `spring.transaction.saga.tracing` | bool | `true` | 协调器构造时挂 `transaction.SagaObserver{}` → 在 starter-otel 全局上为每个步骤相位开一个 otel 子 span。无 starter-otel 时是 no-op（无 span 也无告警）。 | 期望 trace 却没引 starter-otel → 什么都没有；且任何地方都不报错。 |
| `spring.transaction.saga.recover-on-start` | bool | `true` | 注册恢复 `gs.Runner`：扫描 `Store.Pending()` 找 `StatusRunning` 快照并逐个补偿（backward recovery）。⚠ **与 store seam 耦合**：默认内存 `MemoryStore` 重启后扫描恒空 —— 除非持久化 Store starter 生效（如 `spring.transaction.saga.store=gorm` + starter-transaction-saga-gorm），此 key 实际是死 key。 | 持久化 store 下设 `false` → 崩溃遗留 saga 永不补偿（静默不一致）；无持久化 store 设 `true` → no-op、无告警。 |

由**其他模块**持有、在此联动的 key（不属于本 starter 配置面）：
`spring.transaction.saga.store=gorm` 与 `spring.transaction.saga.gorm.*` 属
starter-transaction-saga-gorm；`spring.observability.*` 属 starter-otel。

---

## 4. 验证与故障演练

### 4.1 补偿演练（进程内失败）

跑完整工程：NotifyUser 故意失败。自测断言：

```go
res, err := svc.place(ctx, "order-1")
// err != nil, res.Status == transaction.StatusCompensated
// res.Errors[0].Step == "NotifyUser", Phase == PhaseAction
```

两次补偿必须都执行（逆序 —— 对照 §1 日志行）。若把某个 Compensate 改成故意失败，
`res.Status` 变 `StatusCompensationFailed`，`res.Errors` 同时含 action 错误与补偿失败
—— 这就是人工介入的告警信号。

### 4.2 不可逆步骤演练

给 ChargePayment 去掉 `Compensate`，让 NotifyUser 失败：协调器在 `Result.Errors` 记录
`step "ChargePayment" is irreversible (no Compensate)`，置 `StatusCompensationFailed`，并
**继续**补偿其余步骤（上抛而非跳过）。

### 4.3 重启恢复演练（需持久化 store）

1. 引入 starter-transaction-saga-gorm + 某个 gorm driver starter；设
   `spring.transaction.saga.store=gorm`；driver 指向数据库。启动时 store 会 AutoMigrate
   `saga_snapshots`（失败即 fail-fast）。
2. 起一个 saga，让步骤 2 的 Action 阻塞（sleep），在步骤中途 kill -9 进程。
3. 重启。恢复 Runner 扫描 Pending，按持久化的 `Method` 从 StepRegistry 重建步骤并补偿：
   在飞步骤**最先**、以 nil result 补偿（其 Action 返回值从未被记录 —— 补偿器必须容忍），
   随后按逆序补偿已完成步骤。测试 `TestRecoveryRunner_CompensatesPendingSaga` 钉死了
   `!b, !a` 顺序。
4. 观察：`saga recovery: saga "order-1" recovered with status Compensated`。方法不再注册的
   saga 记录 `no steps registered for method ... skipping` 并跳过。
   注意：恢复期间 Runner 的 ctx **不携带** saga id —— 补偿应基于递入的 Action result
   （`r any`），或让预留可容忍 nil-result 的在飞补偿（空补偿），与 TCC 的 empty-rollback
   义务同理。

### 4.4 观测

- Trace（需 starter-otel）：每相位一个 span，`saga.action <step>` / `saga.compensate
  <step>`，属性 `saga.id`、`saga.step`、`saga.phase`；失败以 error 状态记录。按服务名查
  Jaeger；被补偿的 saga 呈现为 action span 失败 + 其下的 compensate span。
- 日志：tag `AppDef`；`saga coordinator created tracing=…` 与 `saga recovery: …` 系列。
- 无 metrics、无健康指标（见 §6）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| `transaction.Coordinator` autowire 报错 | `enabled=false` 或条件未命中 | 去掉禁用项；blank import 即默认开启。 |
| 崩溃后补偿没有执行 | 内存默认 store —— 重启丢日志 | 加 saga-gorm（+`store=gorm`）或自备持久化 `transaction.Store` bean；仅设 `recover-on-start` 不够。 |
| `saga recovery: no steps registered for method "X"` | 步骤在 Runner 里/启动后注册，或注册与执行的方法名漂移 | 在 bean 构造期、以 saga 持久化的精确 `Method` 字符串注册。 |
| 两个并发 saga 互相覆盖 | 未用 `WithSagaID` → id 退回方法名，每方法只剩一个日志槽 | 边缘处设 id：`transaction.WithSagaID(ctx, id)`。 |
| 结果里出现 `StatusCompensationFailed` | 补偿报错，或已成功步骤的 Compensate 为 nil | 查 `Result.Errors`；让 Compensate 幂等并配 `Step.Retry`；遗留的失败日志按运维积压处理。 |
| `Recover requires a Store` 错误 | 无 store 的协调器被手动调用 `Recover` | 只影响手工 Recover；starter 恒挂内存 store，出现即说明协调器是手搓的。 |
| `tracing=true` 却无 span | 未引 starter-otel / observability 关闭 | 引入 starter-otel 并配 `spring.observability.*`；否则 observer 是静默 no-op。 |
| `execute` 返回错误但账本残留部分效果 | Compensate 不幂等，或忽略递入的 Action result | 基于递入 `result`（token/id）补偿，副作用按 saga id 键控；恢复后可能重放。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 3（全 bool、全默认开） |
| 其中必填 | 0 |
| quickstart 前置外部依赖 | 0（持久化演练才需 1 个 DB；trace 才需 collector） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（审计台账；保留原有，新增在后）：

- **本模块无 `example/` 目录**（tcc、at-gorm 都有）—— §1 的完整工程系源码核对，未作为
  随仓 example 冒烟 → 建议仿照 tcc example 的自测模式补一个。
- `recover-on-start` 在默认内存 Store 下是 no-op，真实含义要靠第二个模块才显现 → 可接受，
  已文档化（保留）。
- 包文档写 "the container holds two beans"（starter.go:23-28），但 `init()` 注册了**四个**
  bean（registry/store/coordinator/runner）→ 文档漂移，修注释。
- 无 metrics、无健康指标：卡在 `CompensationFailed` 的 saga 除手工翻 store 残留行外不可见
  → 考虑 Pending/失败 saga 的 gauge 或健康检查。
- saga 进行中的持久化错误被设计性吞掉（coordinator.go persistRunning）→ 持久化实现必须
  自监控；至少考虑一条 warn 日志。
- README 快速开始里 `Step.Action`/`Compensate` 的签名带一个源码中不存在的
  `*transaction.StepResults` 参数（真实为 `Action func(ctx) (any, error)`）→ 修 README。
