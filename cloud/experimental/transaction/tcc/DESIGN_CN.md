# tcc Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`tcc` 是零依赖 stdlib 层的 TCC(Try / Confirm / Cancel)。它单独放在子包,
正是因为失败语义与 Saga 不同:Try 阶段是**预留**而非提交,全 Try 成功后才
做全局决策(commit / rollback)。Saga 在父包 [`transaction`](../DESIGN.md);
AT 在 [`transaction/at`](../at/DESIGN.md)。

## 1. 职责与边界

- 编排两阶段业务事务:全 Try 一遍;若都成功,记录 commit 决策,顺序
  Confirm;若有失败,记录 rollback 决策,逆序 Cancel 已 try 的 participant。
- 通过 `Store` 持久化事务日志,让阶段间崩溃能干净续跑:`StatusConfirming`
  前向恢复,`StatusTrying` / `StatusCancelling` 后向恢复。
- 暴露 `Observer` 缝隙 —— starter 接 otel,stdlib 不 import otel。
- 拒绝写业务。三个阶段是调用方的函数;coordinator 只做序列化 + 重试。
- 拒绝跨进程解决 TCC 三大问题——幂等、空回滚、防悬挂是 participant 义务,
  在包 doc 里明确要求。

## 2. 关键抽象与缝隙

- **`Participant` 三阶段函数全部必填。** 不像 Saga step 的 `Compensate` 允
  许 nil——TCC 的契约就是"三个阶段都在";任一为 nil 就是编程错误,发生
  副作用前就被拒。
- **`Coordinator` 接口。** 内建实现同步执行阶段;将来外部 store 实现遵同一
  接口——切到跨进程编排是构造变化不是业务变化。
- **`Store` 接口。** 唯一可插拔持久化缝隙。`MemoryStore` 内建;持久化 starter
  贡献 bean。
- **`ParticipantRegistry` + `GlobalTCC`。** 显式、无反射的
  `@GlobalTransactional(type = TCC)`:业务在 wiring 期注册;interceptor 按
  joinpoint 方法名查。未注册方法透明放行。
- **`Observer` 缝隙**——每阶段一 span(nil 关闭)。
- **`RetryPolicy = resilience.Policy` 别名**——TCC 阶段重试与出站韧性共用
  一套配置。Confirm / Cancel 因契约"最终必成功",建议非零策略。
- **决策日志驱动恢复。** Recovery 读 `Status`:`StatusConfirming`(决策 =
  commit)= 前向 Confirm;其他 in-flight = 后向 Cancel。任意崩溃点都能幂等
  地续跑。

## 3. 约束

- **Confirm / Cancel 必须幂等。** 崩溃驱动的重试会重放。这些阶段的
  `RetryPolicy` 是安全网。
- **Cancel 必须容忍 nil result**(空回滚)。Try 可能在记录结果前崩溃;
  participant 用 transaction id 给预留做 key,自己识别"没东西可释放"就
  no-op。
- **防悬挂是 participant 义务。** 迟到的 Try 出现在 Cancel 之后不能再预留;
  框架把 transaction id 传进回调,participant 用它拒绝迟到 Try。
- **已 commit 的事务日志会被删。** 存储中永远不会看到 `StatusCommitted`;
  recovery 只扫非终态。
- **Confirm 顺序,Cancel 逆序。** Confirm 顺序对应"准备 / 确认"心智模型;
  Cancel 逆序与 Saga、AT 保持一致,三姊妹包统一。
- **Confirm / Cancel 失败不掩盖 Try 失败。** cancel 路径跑起来时,
  `Result.Errors` 先列失败的 Try——`Cancelled` 结果依然能解释为什么回滚。

## 4. 取舍与被否决方案

- **与 Saga 分包。** Saga 补偿的是真副作用;TCC 确认的是暂态预留。合成一
  个 API 会把隔离保证(TCC 有,Saga 没有)模糊掉。
- **显式 `ParticipantRegistry` > 反射注解。** Java 走 classload 读注解;
  Go wiring 期一行 register。aspect 就是普通 map lookup,不反射业务代码。
- **先做进程内 coordinator。** 单服务带若干 participant 是 Go 主流拓扑。
  接口开着,以后 starter 可以接远程 coordinator。
- **Try 的 Retry 可选,Confirm / Cancel 的 Retry 建议开。** Try 天然可能失
  败(业务校验),失败正是触发 Cancel 的信号;而 Confirm / Cancel 失败是
  协调期 bug,应由 retry 熨平。
- **`GlobalTCC` 兜底用方法名做 id。** 与 Saga `GlobalTransactional` 同款权
  衡:`WithTransactionID` 才是对的,方法名只在"同时只有一个 instance"下
  勉强正确。
