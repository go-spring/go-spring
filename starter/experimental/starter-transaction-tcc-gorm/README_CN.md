# starter-transaction-tcc-gorm

[English](README.md) | [中文](README_CN.md)

`starter-transaction-tcc-gorm` 为 Go-Spring 应用贡献一个**持久化的、基于 gorm 的**
[`tcc.Store`](../../../cloud/experimental/transaction/tcc),使崩溃遗留的在途 TCC 事务
可被恢复。它是 [`starter-transaction-tcc`](../starter-transaction-tcc) 的持久化伴生
件:协调器把 TCC 日志写到这里,启动期的恢复 `Runner` 把在途快照读回来。

它是 **Contributor** 形态的 starter(见 [DESIGN.md](../../DESIGN.md) §2.3):不占端口。
tcc starter 用 `gs.OnMissingBean` 注册它的内存默认 Store,因此贡献这个 Store 会让默认
实现让位——**无需改代码**即打开崩溃恢复。

## 安装

```bash
go get go-spring.org/starter-transaction-tcc-gorm
```

## 快速开始

### 1. 引入 `*gorm.DB` 与本 Store

Store 自动注入一个已有的 `*gorm.DB`,所以与任何你已在用的 gorm 驱动 starter(mysql、
postgres、sqlserver、clickhouse)搭配即可。

```go
import (
    _ "go-spring.org/starter-gorm-mysql"       // 提供 *gorm.DB
    _ "go-spring.org/starter-transaction-tcc"  // TCC 能力
    _ "go-spring.org/starter-transaction-tcc-gorm"
)
```

### 2. 选中本 Store

```properties
spring.transaction.tcc.store=gorm
```

构造时 Store 会调用 `db.AutoMigrate(&tccSnapshot{})`,建表失败则快速失败。表名固定为
`tcc_snapshots`,不受 gorm 复数化规则影响。

## 表结构

| 列              | 类型    | 说明                                          |
| --------------- | ------- | --------------------------------------------- |
| `id`            | pk      | 事务 id                                        |
| `method`        | string  | 经 `ParticipantRegistry.Lookup` 重建参与者     |
| `status`        | int     | 有索引;`Pending` 扫描非终态状态                 |
| `tried`         | text    | JSON 编码的 `[]string`                         |
| `in_progress`   | string  | 正在 try 的参与者                              |
| `try_results`   | text    | JSON 编码的 `map[string]any`                   |
| `updated_at`    | time    | 最后写入时间                                   |

切片与 map 字段以 **JSON 编码**存入 text 列,使表结构保持后端无关(不依赖任何方言特有的
array/JSON 类型)。

## JSON 往返注意

`Participant.Try` 的返回值以 JSON 存储,所以恢复时它会以 JSON 形态回来——数字变成
`float64`、结构体变成 `map[string]any` 等,而非原本的 Go 类型。需要扛过崩溃的事务应让
Try 结果保持 JSON 友好(id、token 等标量),不要在 `Confirm`/`Cancel` 里依赖富 Go 类型。
在途参与者总是以 nil 结果恢复,因此完全绕开了这一点。

## 配置

绑定在 `${spring.transaction.tcc.gorm}` 之下。

| Key | 默认值 | 说明 |
|---|---|---|
| `spring.transaction.tcc.store` | (未设置) | 必须为 `gorm`,本 Store 才注册。 |

`${spring.transaction.tcc.gorm}` 前缀目前不带自己的键:`*gorm.DB` 由容器自动注入(注册了
多个实例时取默认实例)。

## 许可证

Apache License 2.0,详见 [LICENSE](../../../LICENSE)。
