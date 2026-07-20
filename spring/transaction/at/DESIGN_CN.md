# at Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`at` 是 AT(Automatic Transaction)在零依赖 stdlib 层的实现。它让 Go 拿到
Seata AT 的效果——由资源侧捕获的 before-image 自动派生 undo / rollback——
且不把任何 SQL / ORM 知识拉进 stdlib。姊妹模式 Saga、TCC 分别在
[`spring/transaction`](../DESIGN.md) 与 [`spring/transaction/tcc`](../tcc/DESIGN.md)。

## 1. 职责与边界

- 编排全局 AT 事务:`Begin` 起事务,branches 边写边 `Register`,然后 `Commit`
  (删 undo log)或 `Rollback`(按 before-image 还原)。
- 管理全局锁生命周期:获取锁是 branch 的事(与写在同一个本地事务里发生);
  释放锁在 coordinator 里,一个全局事务释放一次。
- 拒绝碰 SQL。抓 image / 持久化 undo log / 还原行都是 ORM / driver 相关,
  藏在 `Branch` 缝隙后面。coordinator 从不 touch `*sql.DB`。
- 拒绝提供读隔离。AT 通过 `GlobalLock` 只做写-写隔离;读隔离超出范围(本地
  事务的 read committed 依然生效)。

## 2. 关键抽象与缝隙

- **`Branch` 接口 = 资源缝隙。** 一个全局事务里一个数据库一个 `Branch`;
  starter(gorm / 其他 ORM)实现。coordinator 按 `Branch.ID()` 去重——一
  个资源在同 XID 下写多次也只 commit / rollback 一次。
- **`GlobalLock` 接口。** 内建 `MemoryGlobalLock` 覆盖单进程(进程内并发全
  局事务的真写-写隔离);分布式部署换共享后端(redis / db)。传 nil = 关
  隔离,只有单写场景 / 测试可以这么做。
- **XID 挂 context。** `Begin` 生成 XID,通过 `WithXID` 塞进 ctx。资源
  interceptor 用 `XIDFromContext` 读:"没有" = 不在全局事务里,starter 保
  持透明。
- **`GlobalAT` aspect。** 不像 Saga `GlobalTransactional` / TCC `GlobalTCC`
  要注册步骤——AT 没手写步骤:branch 从资源 interceptor 自动注册。aspect
  只负责 begin / resolve + 注入 XID。
- **`Observer` 缝隙**,starter 接 otel 不进 stdlib。
- **`RetryPolicy = resilience.Policy` 别名**——二阶段重试与出站韧性共用
  同一套配置。

## 3. 约束

- **`Branch.Commit` / `Branch.Rollback` 必须幂等。** 崩溃 / 重试都可能重放,
  rollback 更是可能对已还原的行再来一次。`RetryPolicy` 就是这个契约的兜底。
- **Rollback 按登记顺序倒着来**——与 Saga / TCC 反向约定一致——后 register
  的 branch 先 undo。
- **`take` 让 resolve 单发。** Commit / Rollback 都会原子摘掉 XID 的 branch
  列表;二次调用扑空返 `ErrUnknownTransaction`。结构上就防重解决。
- **全局锁释放是 best-effort。** `Release` 出错吞掉——事务已经解决完,让一
  个已完成的操作因释放锁失败而失败反而更糟;锁后端自己靠 TTL / 监控回收滞
  留 key。
- **Commit 失败 = 清理失败,不是一致性失败。** 业务数据一阶段已本地提交,
  只是 undo log 没清干净,让运维单独清就好。Rollback 失败不同,可能行没还
  原成功,需要告警(`StatusRollbackFailed`)。
- **嵌套 `GlobalAT` 复用外层 XID。** ctx 上已有 XID 时 aspect 透明放行,内层
  branch 加入外层全局事务。AT 不做嵌套全局事务。

## 4. 取舍与被否决方案

- **自动补偿 > 手写补偿。** 这是 AT 的定义——业务只写正向 SQL。Saga / TCC 
  独立成子包,正因补偿语义完全不同。
- **每资源一个 branch bean,不做 driver 注册表。** branch 需要活连接与 ORM
  DML 拦截,不是声明式策略;缝隙 = 接口类型,与 `spring/lock` /
  `spring/batch` 同款。
- **先做进程内 coordinator。** 单服务带若干数据库是 Go 主流拓扑;外部
  Seata TC 是加一跳,大多服务不需要。接口开着,starter 后面可以接远程。
- **不做读隔离。** 真 AT 读隔离要经 coordinator 看其他在途事务状态,给每次
  读都加网络依赖。全局锁做写-写隔离已覆盖常见冲突,读保持纯 SQL 就好。
