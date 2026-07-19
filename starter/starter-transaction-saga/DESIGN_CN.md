# starter-transaction-saga 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-transaction-saga` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),把 `stdlib/transaction` 中的 Saga
分布式事务能力接入 Go-Spring。它以进程内 Coordinator + aspect 链达
`@GlobalTransactional(SAGA)` 等价效果,不复刻 Seata 的 TC/TM/RM,也不依赖
字节码魔法。

## 1. 职责与边界

- **在范围内:**`transaction.Coordinator` bean、`*transaction.StepRegistry`
  bean、默认内存 `transaction.Store`,以及启动时补偿在途 Saga 的
  `gs.Runner`。
- **不在范围内:**Step 定义(由业务持有);TCC / AT(各自 starter);持久化
  Store 实现(如 `starter-transaction-saga-gorm` 独立模块);网络传输——
  Saga 是进程内的。

## 2. 关键决策

- **在 Saga / AT / TCC 中选 Saga。**AT 需 SQL 解析 + undo_log + gorm 插件,
  仅覆盖 SQL 资源;TCC 强制 Try/Confirm/Cancel 三段侵入;Saga 是唯一 ROI
  合理且能覆盖非 SQL 下游(MQ / HTTP / 缓存)的模式。
- **不合并为通用事务抽象。**AT / TCC 独立包 / starter——三者失败语义足够
  不同,合并会稀释表达力。
- **默认内存 Store,持久化 Store 通过 `OnMissingBean` 让位。**starter 用
  `Condition(enabled, gs.OnMissingBean[transaction.Store]())` 注册
  `MemoryStore`;持久化 starter(如 `starter-transaction-saga-gorm`)贡献
  自己的 `transaction.Store` 时,默认自动让位——无需改动业务代码即可开启
  崩溃恢复。
- **`Observer` 缝隙做 tracing。**`stdlib/transaction` 是零依赖,不能 import
  otel。starter 提供 `otelObserver`,每个阶段开一个子 span
  (`saga.action|compensate <step>`),走 `starter-otel` 装的 globals——
  零依赖模式下的标准做法(call-site span helper 落在 starter 层)。
- **重试策略复用 `stdlib/resilience`。**`RetryPolicy = resilience.Policy`
  (`stdlib/transaction` 中的类型别名),重试经 resilience `default` driver
  执行,不重造循环。

## 3. 恢复

- **仅后向恢复。**崩溃后,所有在途 Step 逆序补偿。补偿本来就必须幂等,
  故这是最安全的最小语义;前向恢复还需 Action 幂等,留待后续。
- **Step 必须在 wiring 阶段注册**,不能从自定义 `gs.Runner` 内注册——否则
  与恢复 Runner 产生竞态,可能被判 "no steps registered" 而跳过。
- **持久化时机 = 可重放日志。**每次 Action 前 Save(Running + InProgress +
  已完成集);成功后 Save(并入 Completed 并清 InProgress)。committed 删
  日志;compensated / failed 保留终态日志供事后诊断。
- **日志写失败被吞。**Saga 已推进,因日志写失败让整体失败更糟。

## 4. 约束与风险

- **无隔离性。**Saga 无读 / 写屏障——业务边界如需隔离请配合
  `stdlib/lock`。
- **补偿必须幂等。**任意偏移点上的崩溃都可能重试;Coordinator 收集而非
  首错终止,故补偿链多失败会把每个错误一并放进 `Result.Errors`。
- **`Compensate` 为 nil = `CompensationFailed`,而非静默跳过。**不可逆
  Step 是应用需自担的设计决策。
- **生产必须配持久化 Store。**内存版仅适合测试 / 单进程 demo,重启不可
  恢复。

## 5. 取舍 / 弃选方案

- **与 TCC / AT 合并为同一抽象——弃选。**失败语义不同;分包保各自表达力。
- **`stdlib/transaction` 依赖 otel——弃选。**`Observer` 缝隙保留零依赖
  不变量。
- **在恢复时凭空造 Step——弃选。**Action / Compensate 是函数不可持久化;
  通过 Registry 按方法名重建是唯一正确路径。
