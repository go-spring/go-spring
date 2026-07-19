# starter-transaction-tcc 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-transaction-tcc` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),贡献
`stdlib/transaction/tcc` 中的 TCC(Try / Confirm / Cancel)分布式事务能力。
面向"资源需在 try 与 commit 之间被持有"的短链强一致场景。

## 1. 职责与边界

- **在范围内:**`tcc.Coordinator`、`*tcc.ParticipantRegistry`、默认内存
  `tcc.Store`,以及启动时把中断事务推进到决策结果的 `gs.Runner`。
- **不在范围内:**参与者的业务逻辑(每个 `Try` / `Confirm` / `Cancel` 属
  应用代码);持久化 `Store` 实现(独立模块)。

## 2. 为什么与 Saga 分包

失败语义差异足够大,合并会稀释表达力:

| | Saga | TCC |
|---|---|---|
| 前向步骤 | 真实效果 | 预留资源 |
| 失败时 | 补偿函数 | 取消预留 |
| 隔离性 | 无 | Confirm 前对业务不可见 |
| `Compensate == nil` | 不可逆步骤(允许) | 编程错误 |

`stdlib/transaction/tcc/` 子包位于 `stdlib` 模块内(不新增 `go.mod`),但
**有意不并入 Saga 的 `transaction` 包**。

## 3. 关键决策

- **`validate` 早于任何副作用。**三阶段函数不可为 nil,参与者名不能重复
  ——TCC 参与者缺阶段永远是编程错误,不同于 Saga 允许的不可逆步骤。Execute
  入口即 fail-fast。
- **Try-all → Confirm-all(正序) / Cancel-tried(逆序)。**Confirm 按
  原序(幂等重放);Cancel 按逆序(补偿式路径)。
- **Confirm 失败返回 `(res, nil)`。**事务已决策提交;Confirm 错误经
  `StatusConfirmFailed` + `Result.Errors` 传出,不占用顶层错误返回
  (那是 Try 失败的通道)。
- **崩溃恢复 = 决策日志驱动。**状态分为在途
  `Trying` / `Confirming` / `Cancelling` 与终态
  `Committed` / `Cancelled` / `ConfirmFailed` / `CancelFailed`。每个 Try
  持久化 `Trying + InProgress`;全部 Try 成功时**在跑 Confirm 前**持久化
  **`Confirming`(提交决策)**。
  恢复按状态:
  - `Confirming` → 前向 Confirm(决策已定,幂等重放)。
  - `Trying` / `Cancelling` → 后向 Cancel(未达提交决策;InProgress 步以
    nil `tried` 领头 = 空回滚)。
  - 终态 / 不存在 → 幂等 no-op。
- **`Store.Pending` 返回所有非终态**,不像 Saga 只扫 Running——TCC 有更多
  在途状态需要续跑。
- **同样的 `OnMissingBean` 让位机制。**默认 `MemoryStore` 用
  `gs.OnMissingBean` 注册;持久化 Store starter 接过 Coordinator 与恢复
  扫描。
- **Observer 缝隙对齐 Saga。**`otelObserver` 在 `starter-otel` globals 上
  发 `tcc.{try|confirm|cancel} <participant>` span——同款零依赖模式。

## 4. 参与者三义务

stdlib 无法跨进程强制,starter 文档强调 + 例子(库存 + 余额账本)演示:

- **幂等**——Confirm / Cancel 可能重放;第二次调用需 no-op。
- **空回滚**——Cancel 可能对一个 Try 未记录结果的参与者触发(`tried ==
  nil`),此时不做任何事。
- **防悬挂**——迟到于 Cancel 的 Try 不能再次预留;按事务 id 键控可检测。

## 5. 取舍 / 弃选方案

- **把 TCC 并入 Saga 的 `transaction` 包——弃选。**失败语义不同;分开的
  抽象各自更具表达力。
- **Execute 把 Confirm 错误当顶层 error 返回——弃选。**Confirm 失败逻辑上
  在提交之后,归属 Result,而非顶层 error 通道。
