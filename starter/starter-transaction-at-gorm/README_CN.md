# starter-transaction-at-gorm

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-transaction-at-gorm` 将
[`go-spring.org/stdlib/transaction/at`](../../stdlib/transaction/at) 中定义的
**AT（自动事务）** 分布式事务能力接入 Go-Spring 应用，底层基于
[gorm](https://gorm.io)。它以 Go 惯用法达到与 **Seata AT** 等价的效果，无需复刻
Seata 的 TC/TM/RM 角色。

AT 最大的特点是**无侵入**：你不用编写任何补偿代码。一个 gorm 插件拦截每条 DML
语句，捕获*前镜像*（对 INSERT 还捕获*后镜像*），写入一条与业务数据在同一本地事务中
原子提交的 `at_undo_log` 记录，并获取全局行锁以实现写-写隔离。全局回滚时，协调器
依据 undo log 自动还原每一行被改动的数据。

它属于 **Contributor** 形态的 starter（见 [DESIGN.md](../DESIGN.md) §2.3）：不开端口、
不启动服务，只注册 bean。

## Saga vs. TCC vs. AT —— 如何选型？

Go-Spring 提供三种分布式事务模式，按"谁来写回滚逻辑"和"需要多强的隔离"来选：

| | **Saga** | **TCC** | **AT** |
|---|---|---|---|
| 回滚逻辑 | 手写（每步一个补偿函数） | 手写（每个参与者一个 `Cancel`） | **依据前镜像自动派生** |
| 正向步骤 | 立即真实生效 | **预留**资源（暂定） | 立即真实生效（本地提交） |
| 隔离性 | 无 | Confirm 前预留不可见 | **全局行锁**拒绝冲突写入 |
| 资源类型 | 任意（MQ、HTTP、缓存） | 任意 | **gorm 接入的 SQL 数据库** |
| 适用场景 | 长的跨服务流程 | 短的强一致流程 | SQL 数据，且希望以最少代码获得自动回滚 |

数据在 SQL 库、想白拿回滚，选 **AT**；资源需在 try 与 commit 之间被*持有*，选
**TCC**；每步真实生效、靠显式补偿撤销，选 **Saga**。

## 安装

```bash
go get go-spring.org/starter-transaction-at-gorm
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-transaction-at-gorm"
```

容器随即持有两个 bean：

- 一个 `at.Coordinator` —— 进程内协调器，对每个登记的分支执行提交（删除 undo log）
  或回滚（依据 undo log 还原）；
- 一个 `at.GlobalLock` —— 进程内全局行锁，在并发全局事务之间提供写-写隔离。

### 2. 为每个数据库启用 AT

对每个需要参与的 `*gorm.DB`，先迁移一次 undo log 表，再用一个独立的**资源 id**
（记录进 undo log 与锁键）安装插件。`coord` 与 `lock` 即上述两个 bean。

```go
if err := atgorm.Migrate(db); err != nil { ... }
if err := db.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { ... }
```

### 3. 执行全局事务

开启全局事务，把每个数据库写操作放进**各自的本地 gorm 事务**中（使 undo log 与业务
改动原子提交），随后提交或回滚：

```go
ctx, xid := coord.Begin(ctx)
err := accountDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    return tx.Model(&Account{}).Where("id = ?", 1).
        Update("balance", gorm.Expr("balance - ?", cost)).Error
})
if err != nil {
    _ = coord.Rollback(context.Background(), xid) // 依据前镜像还原
    return err
}
return coord.Commit(context.Background(), xid)    // 删除 undo log
```

数据库在 `ctx` 下首次写入时自动登记为一个分支；一个数据库即使写多次，也只提交/回滚
一次。可运行的双库示例（覆盖提交路径、回滚路径与写-写冲突）见
[example/example.go](example/example.go)。

### 4. 或声明为 `@GlobalTransactional`

把 `at.GlobalAT` 接入拦截器链 —— 即 `@GlobalTransactional(AT)` 的零反射等价物。它开启
全局事务、注入 xid，成功则提交、出错则回滚：

```go
chain := aspect.NewChain(at.GlobalAT(coord))
```

与 Saga、TCC 不同，AT **不需要方法注册表**：分支是从携带全局事务 id 的 context 下执行
的 SQL 中被发现的。

## 工作原理

- **UPDATE / DELETE** —— *前置*回调 SELECT 受影响行得到前镜像，并在写入前对其主键
  获取全局锁；*后置*回调记录 undo log 并登记分支。
- **INSERT** —— *后置*回调读取生成的主键得到后镜像，获取锁并记录 undo log，以便回滚
  时删除该行。
- **回滚** —— 该事务的 undo log 按最新优先重放：INSERT 被删除、DELETE 依前镜像重新
  插入、UPDATE 还原为前镜像值；随后删除 undo log。
- **防递归** —— 插件自身的二阶段写操作运行在被抑制的 context 上，且跳过
  `at_undo_log` 表，因此还原动作不会被再次捕获为新的 undo。

## 注意事项

- **写操作要放进 gorm 事务内。** undo log 与业务语句在同一连接上写入，只有外层本地
  事务才能让两者原子提交。
- **进程内。** 全局锁与 undo 还原都在进程内 —— 这是单进程的 AT 等价实现，而非分布式
  Seata TC 部署。
- **镜像精度。** 前/后镜像以 JSON 编码，数值列会经 `float64` 往返（与
  `starter-transaction-saga-gorm` 相同的取舍）。

## 配置

绑定于 `${spring.transaction.at}`（与 Saga、TCC 共享 `spring.transaction` 能力命名
空间，多种模式可并存）。

| 键 | 默认 | 说明 |
|---|---|---|
| `spring.transaction.at.enabled` | `true` | 开关本 starter 的 bean。 |
| `spring.transaction.at.tracing` | `true` | 在 `starter-otel` 安装的全局对象上，为每个分支阶段（提交 / 回滚）开一个 otel 子 span；无 otel 时为空操作。 |

## 许可证

Apache 2.0，见 [LICENSE](../../LICENSE)。
