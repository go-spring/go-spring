# starter-migration-gorm 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照本模块源码
（`starter.go`、`config.go`、`runner.go`、`store.go`）、它驱动的引擎
（`cloud/data/migration` 的 `migration.go`、`runner.go`、`source_fs.go`）、gs 启动序列
（`spring/gs/internal/gs_app/app.go:272-330`）以及可运行、自带断言的
[example/](example/)（内嵌 migrations + `check.sh` 冒烟）核对。**Flyway 自身语义见
[Flyway 官方文档](https://documentation.red-gate.com/fd/quickstart-migrations-184127599.html)**——
本 starter 是该模式的 Go-Spring 等价物：forward-only、fail-stop，对齐 Flyway 社区版
（无自动 down-migration，`migration.go:31-33`）。

**激活方式**：仅当配置了 `spring.migration.*` 时才注册 Runner bean（`starter.go:57-60`，
`gs.OnProperty("spring.migration")` 是前缀检查，任意 `spring.migration.<name>...` key 都会
激活）。blank-import 本 starter；若什么都没配置，就别为副作用 import 它。

---

## 1. 完整工程示例

一个在首个请求可能到来之前就从内嵌 SQL 完成迁移的服务：

```
demo/
├── go.mod
├── main.go
├── db.go               # 打开并命名 *gorm.DB bean
├── migrations/
│   ├── V1__create_widgets.sql
│   └── V2__seed_widgets.sql
└── conf/app.properties
```

**go.mod**（关键依赖）：

```
require (
    gorm.io/driver/sqlite             latest   // 或 starter-gorm-mysql 等
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-gorm-sqlite latest   // 或你的方言 starter
    go-spring.org/starter-migration-gorm latest
)
```

**migrations/V1__create_widgets.sql** / **V2__seed_widgets.sql**（取自 example 原文）：

```sql
CREATE TABLE widgets (
    id   INTEGER PRIMARY KEY,
    name VARCHAR(64) NOT NULL
);
```
```sql
INSERT INTO widgets (id, name) VALUES (1, 'sprocket');
INSERT INTO widgets (id, name) VALUES (2, 'gizmo');
```

**db.go** —— 应用**按名字**提供数据库：

```go
package main

import (
    "embed"

    "go-spring.org/cloud/data/migration"
    "go-spring.org/spring/gs"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

//go:embed migrations
var migrationsFS embed.FS

func openDB() (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open("file:demo?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    sqlDB, _ := db.DB()
    sqlDB.SetMaxOpenConns(1) // 内存 sqlite 需要钉住单连接
    return db, nil
}

func init() {
    // 两个 bean 都命名 "app"，与 spring.migration.app 配置项对齐：
    // starter 以这个共享名匹配配置项、DB bean 与 Source bean。
    gs.Provide(openDB).Name("app")
    gs.Provide(func() migration.Source {
        return migration.NewFSSource(migrationsFS, "migrations")
    }).Name("app")
}
```

**main.go**：

```go
package main

import (
    _ "go-spring.org/starter-migration-gorm"

    "go-spring.org/spring/gs"
)

func main() { gs.Run() }
```

**conf/app.properties** —— 完整、带注释的配置面：

```properties
# 名为 "app" 的迁移项。匹配 *gorm.DB bean "app" 与 migration.Source bean "app"。
# 只有一个 *gorm.DB bean 时 db-ref 可省，此处写出便于发现。
spring.migration.app.db-ref=app
spring.migration.app.enabled=true
```

**验证**（example 自带的三保证冒烟，`example/example.go:105-156`）：

```bash
cd demo && CGO_ENABLED=1 go run .      # 或跑自带 example 的 ./check.sh
# 日志: migration: entry "app" applied 2 migration(s)
# 文件型 DB 上还可直接查结果：
#   sqlite3 demo.db 'SELECT version, name FROM schema_migrations;'   -- 2 行
#   sqlite3 demo.db 'SELECT COUNT(*) FROM widgets;'                  -- 2
```

第二次启动不应用任何东西：`applied 0 migration(s)`（版本行已存在）。

---

## 2. 装配与时序 —— 迁移何时运行

`gs.Run()` → `App.Start()` 依次执行（`gs_app/app.go:274-283`）：

1. properties 刷新、日志初始化
2. **IoC 容器刷新** —— 所有 bean 装配完成（Rooter 的 `Init` 在此运行）；迁移 Runner 的
   字段被填充：`Entries` 绑 `${spring.migration}`，`DBs` 与 `Sources` 按 bean 名收集全部
   `*gorm.DB` / `migration.Source` bean（`runner.go:34-48`）
3. **Runner 同步、顺序执行**（`app.go:311-316`）——迁移发生在这里；任何错误中止
   `Start`，应用在开始服务前退出（`starter.go:39-41`："a broken schema never serves traffic"）
4. **server 启动**（各自 goroutine）；就绪等待所有 server 的 `ReadySignal`
   （`app.go:318-330`）

因此迁移发生在**所有 bean 装配之后、任何 server 接流量之前**——repository 或 DAO 永远
查不到未建出的表。⚠ 多个 Runner **之间**的顺序是 `App.Runners` 的 bean 集合顺序；迁移
Runner 与你的其他 Runner 之间没有显式依赖边——若别的 Runner 读 schema，需自行确认它排在
迁移之后（见 §6 设计嫌疑）。

`Run` 内部按 **name 排序**处理各 entry（`runner.go:54-59`）；disabled 的 entry 打 Info 日志
跳过。每个 entry 独立地：`pickDB`（db-ref → 按名取 bean；db-ref 为空时仅当恰好存在一个
`*gorm.DB` bean 才可用，`runner.go:90-110`）、`pickSource`（与 entry 同名的
`migration.Source` bean 优先；否则用磁盘上的 `source-dir`，`runner.go:115-124`）、然后
`Migrate`。

## 3. 逐 key 行为参考

key 位于 `spring.migration.<name>.*`（经 `${spring.migration}` 做 map 绑定，
`runner.go:37`）。同一集合在 `schema.json` 声明。

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|-------------|----------|
| `<name>`（entry 本身） | map key | — | **激活**：任意 `spring.migration.<name>.*` key 即注册 Runner。entry 名必须与 `migration.Source` bean 名一致（若使用 Source bean）。 | 名字不匹配 → 落到 `source-dir`；它也为空则 fail-fast "neither a migration.Source bean ... nor a source-dir"。 |
| `enabled` | bool | true | entry 级开关；保留配置但跳过执行，打 Info 日志（`runner.go:62-65`）。 | — |
| `db-ref` | string | "" | 指名 `*gorm.DB` bean。为空 + 恰好一个 DB bean → 用它；为空 + 0 或 ≥2 个 bean → fail-fast（`runner.go:90-110`）。⚠ 多库场景漏配要么报错（0/≥2）要么有迁错库的风险。 | "no *gorm.DB bean of that name is registered" / "set db-ref to disambiguate"。 |
| `source-dir` | string | "" | 磁盘上 `V<n>__<name>.sql` 文件目录，经 `os.DirFS` 读取（`runner.go:120`）。存在同名 Source bean 时被忽略（embed 优先）。相对路径按进程 CWD 解析。 | 路径错误 → 启动时报 "read dir"；两种来源都没有 → fail-fast。 |
| `baseline` | uint64 | 0 | `Version <= baseline` 的迁移**只记录不执行**（`runner.go:104-109`；`MarkApplied` 只写版本行，`store.go:111-113`）。0 关闭。 | 偏高 → 较新迁移被静默按 baseline 跳过；偏低 → 在已有同名对象的库上执行 → SQL "already exists"。 |
| `allow-out-of-order` | bool | false | 允许应用版本**低于**已应用最高版本的迁移（补洞）。默认 false = 历史必须严格递增（`migration/runner.go:110-114`）。 | 已应用 V5 后补低版本脚本且未开此开关 → 启动失败 "out-of-order migration; set allow-out-of-order"。 |
| `table` | string | schema_migrations | 版本表名。必须是纯 SQL 标识符 `^[A-Za-z_][A-Za-z0-9_]*$`——启动时校验（`store.go:32,54-57`）；它会被内插进 DDL，无法参数绑定。 | 非法名 → 任何 DDL 之前 fail-fast；事后改名会遗弃旧历史（旧表的行永远不再被读）。 |

迁移文件命名（引擎语义，`source_fs.go:37-44`）：`V<version>__<name>.sql`——前导 V 可省、
大小写不敏感，双下划线分隔，version 是**按数值**比较的非负整数（V2 < V10）。文件内容
SHA-256 进 checksum。非 `.sql` 文件忽略；不递归子目录。文件名畸形是硬错误而非静默跳过。
文件按分号切分语句（感知引号与 `--` 注释）——含内部分号的存储过程 / PL-pgSQL 块必须整文件
单语句发出（`source_fs.go:130-135`）。

## 4. 验证与故障演练

所有演练与 example 的保证同构（`example/example.go:105-156`）；请用文件型 sqlite 让状态
跨重启存活。

1. **启动应用 + 幂等**：启动一次 → `schema_migrations` 2 行、`widgets` 2 行；再启动 →
   日志 `applied 0 migration(s)`。
2. **checksum 漂移（篡改历史）**：应用过 `V1__create_widgets.sql` 后编辑它 → 重启失败，
   报 `checksum mismatch for version 1 ... a migration already applied was edited; migrations
   are immutable history`（`migration/runner.go:95-99`）。回滚编辑——或有意接受新现实时手工
   更新已存 checksum。
3. **dirty 状态恢复（迁移中途崩溃）**：Up SQL 与版本行在同一个 gorm 事务里
   （`store.go:99-108`）。PostgreSQL/SQLite 的 DDL 可事务化 → Up 失败干净回滚，下次启动
   重试。⚠ MySQL 每条 DDL 自动提交（与 Flyway 同样的 MySQL 限制，`store.go:96-98`）：
   崩溃可能留下已执行的 DDL 但**没有**版本行——下次启动重试该文件并死于 "table already
   exists"；需手工修复（补齐/删除对象，或补写版本行）后重启。
4. **在既有 schema 上 baseline**：把 entry 指向已带 V1 schema 的数据库；设
   `spring.migration.app.baseline=1` → V1 只记录不执行，V2 正常应用。演练：检查
   `schema_migrations` 已含 V1（`applied_at` 已写）且 V1 的对象原封未动。
5. **乱序补洞**：放入 V1、V2、V3、V5 四个文件（故意缺 V4），启动——四个全部应用。之后
   新增 `V4__gap.sql` 再启动 → 启动失败 "out-of-order migration; set allow-out-of-order"
   （V4 低于已应用最高版 V5）。设 `spring.migration.app.allow-out-of-order=true` 重启 →
   V4 在 V5 之下应用。洞补完后移除该开关。
6. **程序化运行（不经 starter）**：`store, _ := migrationgorm.NewStore(db, "schema_migrations")`
   然后 `migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)`——管理命令与
   冒烟测试背后的导出 seam（`store.go:45-47`）。

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| starter 不运行、无迁移日志 | 未配置任何 `spring.migration.*` key | 至少加一个 entry——`OnProperty("spring.migration")` 是激活开关 |
| `references db-ref %q but no *gorm.DB bean of that name` | db-ref 拼写错误，或 bean 注册了别的名字 | `gs.Provide(openDB).Name("app")` 的名字与 db-ref 完全一致 |
| `no db-ref but N *gorm.DB beans exist` | 多数据库且未显式选择 | 每个 entry 设 `db-ref` |
| `neither a migration.Source bean ... nor a source-dir` | entry 名 ≠ Source bean 名且无 source-dir | 对齐名字，或设 `source-dir` |
| `checksum mismatch for version N` | 已应用的 .sql 文件被编辑 | 回滚编辑（历史不可变）；或有意更新已存 checksum |
| `out-of-order migration; set allow-out-of-order` | 新文件版本低于已应用最高版 | 以新的最高版本补洞，或设 `allow-out-of-order=true` |
| 重试时报 `table already exists`（MySQL） | DDL 中途自动提交、版本行缺失（§4.3） | 手工对账 schema 后重启；重试即成功 |
| `invalid version-table name` | `table` key 不过标识符正则 | 只用纯标识符——不带引号、不带点 |
| 迁移成功但应用仍报缺表 | 其他 Runner 在迁移之前读了 schema（Runner 顺序即集合顺序） | 审计 Runner 顺序；见 §6 设计嫌疑 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 每 entry 6 个（另加 `${spring.migration}` map 绑定） |
| 其中必填 | 语法上 0；实践上 `db-ref`（多库）与 Source bean / `source-dir` 二选一 |
| quickstart 前置外部依赖数 | 1（数据库；内存 sqlite 时为 0） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单：与其他 Runner 的顺序是隐式的——依赖 runner-before-server 而无显式依赖边；
`schema.json` 把 `baseline` 声明成 `object`（应为 `integer`——元数据 bug，配置绑定本身是
uint64 且工作正常）；迁移中途失败未作为独立错误类别暴露（dirty 状态只能靠重试时的
"table already exists" 反推）；`source-dir` 相对路径依赖 CWD（考虑文档化或绝对化）。
