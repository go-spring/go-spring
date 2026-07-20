# transaction Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`transaction` 在零依赖 stdlib 层提供 Saga——最终一致性形态的分布式事务。它以
"效果"替换 Seata Saga + Spring `@GlobalTransactional`:有序的可补偿步骤,失
败按逆序回滚。TCC 与 AT 分别在 `tcc` / `at` 子包——两种模式的失败语义不同,
拆开更诚实。

## 1. 职责与边界

- 编排可补偿业务步骤:`Action` 正向,后一步失败则按逆序跑 `Compensate`。
- 通过 `Store` 持久化 saga 日志,让崩溃进程能续跑补偿;只做**后向恢复**。
- 暴露 `Observer` 缝隙——starter 接 otel,stdlib 不 import otel。
- 拒绝隔离。saga 中间态对其他读者可见,业务代码自己防脏读(状态位 /
  `spring/lock`)。
- 拒绝 SQL 解析 / 生成 undo log:那是 AT,放在 `transaction/at`,是刻意分开
  的缝隙。

## 2. 关键抽象与缝隙

- **`Coordinator` 接口。** 内建实现同步跑步骤;未来外部 store 实现遵同一接
  口——切到跨进程编排是构造变化,不是业务变化。
- **`Store` 接口。** 唯一可插拔持久化缝隙。`MemoryStore` 内建满足"单网关进
  程带若干下游"的常见场景;starter 贡献 gorm / redis / etcd 后端 bean。
- **`StepRegistry` + `GlobalTransactional`。** 显式、无反射版的
  `@GlobalTransactional`。业务在 wiring 期注册方法的步骤;interceptor 按
  joinpoint 方法名查。未注册方法透明放行,不反射业务函数体。
- **`Observer` 缝隙。** starter 每 phase 起一个 span(action / compensate)。
  nil 完全关掉观测。
- **`RetryPolicy = resilience.Policy` 别名。** 有意复用同一套字段
  (`MaxRetries`、`Timeout` ...),saga 步骤重试与出站韧性共用配置,不重复
  实现。

## 3. 约束

- **Compensate 必须幂等。** 按 `RetryPolicy` 重试;崩溃恢复后可能对已回滚
  的资源再放一次。`Compensate` 为 nil = 不可逆步骤:回滚触及时记
  `StatusCompensationFailed` 报警,决不静默跳过。
- **只做后向恢复。** 崩溃后 `Recover` 补偿所有可能副作用过的步骤——先补偿
  in-flight 那步(result 未持久化,传 nil),再按逆序补偿完成的。不做前向续
  跑:崩溃点未知,前向可能重复副作用。
- **In-flight 日志写失败不使整个 saga 失败。** `persistRunning` 中的
  `Store.Save` 错误刻意吞掉:action 已经发生,因日志缺口整个操作失败反而更
  糟。真正需要一致性的是终态写;持久化 store 自己监控。
- **已 Commit 的 saga 日志会被删。** 存储中永远不会看到 `StatusCommitted`;
  `Pending` 只返 `StatusRunning`,让恢复接住。
- **Saga id 由调用方给。** `WithSagaID` 挂 request context,让 id 对齐调用方
  的幂等 key。interceptor 兜底用方法名——只对"同一时刻只有一个 instance"
  正确,其他都误导。

## 4. 取舍与被否决方案

- **Saga 优先于通用 TX 2PC。** Seata TC/TM/RM 三角色带来自己的协调器与故障
  模式;Saga 的正向+补偿正好对 Go 微服务实际要覆盖的 MQ 发送、HTTP 调用、
  缓存写。强隔离(AT)作为子包按需选。
- **先做进程内 coordinator。** "一个网关进程编排若干下游"是 Go 微服务的
  主流形态,外部 coordinator 是多加一跳。接口开着,以后 starter 可以接。
- **不自动生成 compensation。** SQL undo log(AT)会把包拉进数据库驱动,而
  非 SQL 下游还是要手写补偿——干脆把 `Compensate` 摆到前台。
- **显式 `StepRegistry` > 反射扫注解。** Java 走字节码,Go 一行 wiring 就
  注册完。aspect 就是普通 map lookup,不反射业务代码。
