# starter-migration-goose

[English](README.md) | [中文](README_CN.md)

`starter-migration-goose` 在启动期对任意 gorm `*gorm.DB` 执行
[goose](https://github.com/pressly/goose) schema 迁移。算法归 goose(版本记
录、校验和、事务、方言细节);本 starter 只管接线:配置键、IoC 生命周期、
gorm→`*sql.DB` 桥接。

## 安装

```bash
go get go-spring.org/starter-migration-goose
```

## 快速开始

```go
import _ "go-spring.org/starter-migration-goose"
```

```properties
spring.migration.app.db-ref=app       # *gorm.DB bean 名;仅一个 bean 时可省
spring.migration.app.dir=./sql        # goose SQL 迁移目录
```

starter 注册一个 `gs.Runner`:在所有 bean 装配后、任何 server 服务前运
行——把未应用的迁移(`00001_init.sql`、`00002_seed.sql`、...)按序只进不退
地执行(每条独立事务),记入 `goose_db_version`。失败即中止启动——坏
schema 不接流量。多数据库就是 `spring.migration` 下多个条目。

迁移文件用 goose 的 SQL 格式(`-- +goose Up` / `-- +goose Down` 指令);
Down 半边永不执行(只进不退)。

## 配置

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `spring.migration.<name>.enabled` | `true` | 置 `false` 保留配置但跳过执行。 |
| `spring.migration.<name>.db-ref` | (唯一 bean) | 要迁移的 `*gorm.DB` bean。存在多个时必填。 |
| `spring.migration.<name>.dir` | — | goose SQL 迁移目录(必填)。 |
| `spring.migration.<name>.table` | `goose_db_version` | 版本表名。 |
| `spring.migration.<name>.allow-missing` | `false` | 允许补低于已应用最高版本的缺口迁移。 |

## 说明

- 方言:mysql、postgres、sqlite、sqlserver、clickhouse(从 gorm
  dialector 映射;其他直接 fail-fast)。
- 单一自包含二进制可在启动时把 embed 的迁移拷到临时目录,`dir` 指过去。
- 端到端可运行演示见 [example/](example/)(内存 sqlite、启动应用 + 幂等
  自断言、`check.sh` 冒烟)。
