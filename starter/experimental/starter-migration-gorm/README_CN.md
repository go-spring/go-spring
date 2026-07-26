# starter-migration-gorm

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-migration-gorm` 在应用启动时运行
[`go-spring.org/spring/migration`](../../spring/migration) 定义的数据库结构迁移能力，
底层复用应用已注册的 [gorm](https://gorm.io) `*gorm.DB` bean。它是把 **Flyway** /
**Liquibase** 放到 classpath 的 Go-Spring 等价物——带版本、带校验和、只向前的迁移，
在第一个请求到达之前完成——但用 Go 惯用法达到等价效果，而非复刻它们的 XML/DSL 机制。

它是**客户端形态变体的集成 starter**（见 [DESIGN.md](../DESIGN.md) §2.2）：它消费一个
命名的 `*gorm.DB` bean，而不是自己开连接；并且是多实例的——在 `spring.migration.<name>`
下可绑定多个数据库。它导出的是 `gs.Runner` 而非 server：Runner 在所有 bean 装配完成之后、
任何 server 开始服务之前运行，这正是结构迁移需要的时序——表在第一个请求到达 repository 或
DAO 之前就已存在。迁移失败会中止启动（fail-fast），所以处于未知 schema 状态的数据库绝不
对外服务。

## 与 Flyway 的对应

| Flyway | starter-migration-gorm |
|---|---|
| classpath 上的 `V<version>__<name>.sql` | `//go:embed` 目录或 `source-dir` 中的 `V<version>__<name>.sql` |
| `flyway_schema_history` 表 | `schema_migrations` 表（可通过 `table` 配置） |
| 重跑时的校验和校验 | SHA-256 校验和——改动已应用脚本即 fail-fast 报错 |
| `baselineVersion` | `baseline`——把 `<=` 它的版本记为已应用而不运行 |
| out-of-order 开关 | `allow-out-of-order`（默认关闭） |
| 只向前（社区版） | 只向前；`Down` 字段保留但从不自动执行 |

## 安装

```bash
go get go-spring.org/starter-migration-gorm
```

## 快速开始

分工是**应用拥有迁移脚本，starter 拥有运行器**——与 `starter-batch`、
`starter-scheduler` 相同的分工方式。

### 1. 引入 starter

```go
import _ "go-spring.org/starter-migration-gorm"
```

### 2. 提供迁移脚本

提供一个**以配置项命名**的 `migration.Source` bean（推荐的 `go:embed` 方式，迁移脚本随
单一二进制一起分发）：

```go
//go:embed migrations
var migrationsFS embed.FS

gs.Provide(func() migration.Source {
    return migration.NewFSSource(migrationsFS, "migrations")
}).Name("app")
```

……或者不提供 bean，改用 `source-dir` 指向磁盘上的目录。

### 3. 每个数据库配置一个条目

```properties
spring.migration.app.db-ref=app        # 要迁移的 *gorm.DB bean 名称
spring.migration.app.source-dir=./sql  # 可选；当没有名为 "app" 的 Source bean 时使用
```

启动时 Runner 会按版本升序应用所有待执行迁移，每条记入 `schema_migrations`，并打印应用了
多少条。见 [example/example.go](example/example.go) 的可运行示例，它验证了启动应用、
二次幂等空跑、以及校验和漂移 fail-fast。

### 4. 以编程方式运行迁移（可选）

对于一次性的管理命令或测试，可直接把 `*gorm.DB` 包装成 `migration.Store`：

```go
store, _ := migrationgorm.NewStore(db, "schema_migrations")
applied, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
```

## 工作原理

- **版本表**——`EnsureVersionTable` 创建 `schema_migrations(version, name,
  checksum, applied_at)`，列类型在 MySQL、PostgreSQL、SQLite 之间可移植。表名会被校验为
  纯 SQL 标识符，因为它被拼接进 DDL（无法作为绑定参数）。
- **顺序**——启用的条目按名称排序运行；条目内部按版本升序运行。版本 `0` 和重复版本会被拒绝。
- **幂等**——已记录的版本会被跳过；若其记录的校验和与源不同，则运行失败，而不是静默重跑或忽略改动。
- **事务**——每条迁移的 `Up` 与其版本行在同一个 gorm 事务内应用。PostgreSQL 和 SQLite 的
  DDL 是事务性的，`Up` 失败会干净回滚；MySQL 每条 DDL 自动提交（Flyway 同样受此限制），但
  版本行只在 `Up` 成功后写入，因此迁移途中崩溃会让该行缺失，下次运行重试。

## 数据库与源的选择

| 情形 | 行为 |
|---|---|
| 设置了 `db-ref` | 迁移该名称的 `*gorm.DB` bean；未知名称 fail-fast |
| `db-ref` 为空，仅一个 `*gorm.DB` bean | 使用该唯一 bean |
| `db-ref` 为空，存在多个 bean | fail-fast——需设置 `db-ref` 消歧 |
| 存在以条目命名的 `migration.Source` bean | 优先（`go:embed` 方式） |
| 无此 bean，但设置了 `source-dir` | 从磁盘读取 `V<version>__<name>.sql` 文件 |
| 两者都没有 | fail-fast |

## 注意事项

- **只向前、失败即停**，与 Flyway 社区版一致：没有自动降级迁移。
  `migration.Migration` 上的 `Down` 字段为工具保留，Runner 从不执行它。
- **历史不可变**。版本一旦应用，其脚本即被冻结——改动它会改变校验和并使下次启动失败。
  应改为新增一条更高版本的迁移。

## 配置

绑定于 `${spring.migration.<name>}`——每个数据库一个条目。

| 键 | 默认值 | 说明 |
|---|---|---|
| `spring.migration.<name>.enabled` | `true` | 开关此条目。 |
| `spring.migration.<name>.db-ref` | `""` | 要迁移的 `*gorm.DB` bean 名称。 |
| `spring.migration.<name>.source-dir` | `""` | 磁盘上 `.sql` 文件目录；当没有以条目命名的 `Source` bean 时作为回退。 |
| `spring.migration.<name>.baseline` | `0` | 把 `<=` 此值的版本记为已应用而不运行。 |
| `spring.migration.<name>.allow-out-of-order` | `false` | 允许应用低于已应用最高版本的版本（补缺）。 |
| `spring.migration.<name>.table` | `schema_migrations` | 版本表名；必须是纯 SQL 标识符。 |

## 许可证

Apache 2.0。见 [LICENSE](../../LICENSE)。
