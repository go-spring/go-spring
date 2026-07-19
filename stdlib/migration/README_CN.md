# migration
[English](README.md) | [中文](README_CN.md)

`migration` 是一个零依赖的数据库结构迁移抽象——Flyway / Liquibase 的 Go 惯用法等价物。
一个 [`Migration`](migration.go) 是带编号、带名称、带校验和的结构变更单元；一个
[`Source`](migration.go) 产出它们的有序集合（来自嵌入目录或代码）；一个
[`Runner`](runner.go) 在版本表中恰好一次、只向前地应用尚未应用的迁移。

抽象拥有*算法*（排序、校验和守卫、乱序策略、baseline）；*存储*是后端在自己驱动之上实现的
[`Store`](migration.go) 缝隙（gorm 的实现见 `starter-migration-gorm`）。

## 特性

- `Migration{Version, Name, Checksum, Up}`——单条迁移；`Up` 通过
  [`Execer`](migration.go) 运行其语句，因此与驱动无关。
- `NewFSSource(fsys, dir)`——从 `//go:embed` 目录（或任意 `fs.FS`）读取 Flyway 风格的
  `V<version>__<name>.sql` 文件，并对每个做 SHA-256。
- `NewSource(migs...)`——注册用 Go 代码编写的迁移。
- `Runner.Migrate`——确保版本表存在，然后按版本升序应用每条未应用的迁移并记录。
- 校验和守卫——改动已应用脚本是 fail-fast 报错，而非静默重跑。
- `Options{AllowOutOfOrder, Baseline}`——补缺策略与「接管已有 schema」的 baseline。
- 只向前——`Migration` 上的 `Down` 字段保留但从不执行。

## 用法

```go
package main

import (
    "context"
    "embed"

    "go-spring.org/stdlib/migration"
)

//go:embed migrations
var migrationsFS embed.FS

func run(ctx context.Context, store migration.Store) error {
    src := migration.NewFSSource(migrationsFS, "migrations")
    applied, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
    if err != nil {
        return err
    }
    _ = applied // 本次应用的迁移（重复空跑时为空）
    return nil
}
```

`store` 由后端提供——对 gorm，`starter-migration-gorm` 把 `*gorm.DB` 包装成
`migration.Store`。若想用代码而非 `.sql` 文件注册迁移：

```go
src := migration.NewSource(migration.Migration{
    Version: 1,
    Name:    "create users",
    Up: func(ctx context.Context, exec migration.Execer) error {
        return exec.ExecContext(ctx, "CREATE TABLE users (id BIGINT PRIMARY KEY)")
    },
})
```

## 语义

- 迁移按版本升序运行；版本 `0` 和重复版本会被提前拒绝。
- 已记录的版本会被跳过；若其记录的校验和与源不同，`Migrate` 失败——迁移是不可变的历史。
- `AllowOutOfOrder=false`（默认）时，低于已应用最高版本的版本会被拒绝；设为 true 可允许补缺。
- `Baseline=N` 把所有 `<= N` 的版本记为已应用而不运行，用于接管已带 schema 的数据库。
- 只向前、失败即停：迁移失败会中止本次运行，不再尝试后续迁移。
