# starter-transaction-saga-gorm

[English](README.md) | [中文](README_CN.md)

`starter-transaction-saga-gorm` 为 Saga 分布式事务提供**基于 gorm 的持久化**
[`transaction.Store`](../../spring/transaction),让 Go-Spring 应用能在崩溃后
恢复在途 Saga。它是
[`starter-transaction-saga`](../starter-transaction-saga) 的持久化伴随件:
Coordinator 把 saga 日志写到这里,启动时的恢复 `Runner` 从这里读回在途快照。

它属于 **Contributor** 形态(见 [DESIGN.md](../DESIGN.md) §2.3):不开端口。
Saga starter 的默认内存 Store 用 `gs.OnMissingBean` 注册,故贡献本 Store 会
让默认自动让位——**无需改动业务代码**就开启崩溃恢复。

## 安装

```bash
go get go-spring.org/starter-transaction-saga-gorm
```

## 快速开始

### 1. 引入 `*gorm.DB` 和本 Store

本 Store 通过 autowire 拿到已有的 `*gorm.DB`,故需搭配任一 gorm-driver
starter 使用(mysql、postgres、sqlserver、clickhouse)。

```go
import (
    _ "go-spring.org/starter-gorm-mysql"        // 提供 *gorm.DB
    _ "go-spring.org/starter-transaction-saga"  // Saga 能力
    _ "go-spring.org/starter-transaction-saga-gorm"
)
```

### 2. 选中本 Store

```properties
spring.transaction.saga.store=gorm
```

构造时会调用 `db.AutoMigrate(&sagaSnapshot{})`,建表失败即快速失败。表名
固定为 `saga_snapshots`,不受 gorm 复数化规则影响。

## Schema

| 列名           | 类型   | 说明                                          |
| -------------- | ------ | --------------------------------------------- |
| `id`           | 主键   | saga id                                       |
| `method`       | string | 通过 `StepRegistry.Lookup` 重建 Step          |
| `status`       | int    | 建索引;`Pending` 扫描 `StatusRunning`        |
| `in_progress`  | string | 当前正在执行的 Step                           |
| `completed`    | text   | JSON 编码的 `[]string`                        |
| `step_results` | text   | JSON 编码的 `map[string]any`                  |
| `updated_at`   | time   | 最后写入时间                                  |

slice/map 字段以 **JSON 编码**存进 text 列,让 schema 与后端无关(不依赖各
方言的 array/JSON 类型)。

## JSON 往返注意

`Step.Action` 的返回值以 JSON 存储,恢复时会以 JSON 形态回来——数字变
`float64`、结构体变 `map[string]any`,而非原始 Go 类型。需要抗崩溃的 Saga
应让 Action 返回值保持"JSON 友好"(id、token 等标量),并避免在
`Compensate` 中依赖复杂 Go 类型。正在执行的 Step 恢复时结果固定为 nil,不
受此限。

## 配置

绑定到 `${spring.transaction.saga.gorm}`。

| 键 | 默认 | 说明 |
|---|---|---|
| `spring.transaction.saga.store` | (未设) | 必须为 `gorm`,本 Store 才会注册。 |
| `spring.transaction.saga.gorm.db` | `` | 应用注册多个 `*gorm.DB` 时选中命名实例。当前版本始终 autowire 默认实例,该字段仅作信息。 |

## 许可

Apache 2.0。见 [LICENSE](../../LICENSE)。
