# starter-gorm-sqlite 使用说明（参考手册）

详细使用文档。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照源码
（`starter.go`、`config.go`）与可运行的 [example/](example/)（冒烟脚本
`example/check.sh`，无需 docker）核实。**SQLite 与 GORM 语义以官方文档为准** —
驱动：[glebarez/sqlite](https://github.com/glebarez/sqlite)（纯 Go，基于
[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)，无 cgo）；ORM：[gorm.io](https://gorm.io/docs/)。
共享的 wrapper 生命周期、连接池/observe/health 接线与 `UseDBCustomizer` 见
[gormcore](../starter-gorm/USAGE_CN.md)，此处不再重复。

**激活条件**：`spring.gorm.sqlite` 下每个条目注册一个 `*starter.DB` bean（外加配对的
`health.Indicator`）；没有任何条目时 starter 不注册任何东西，应用正常启动
（`starter_test.go:TestSqliteDefaultsNotTriggered`）。SQLite 是进程内数据库：**本方言
没有 TLS 配置块，也没有服务发现通道**（共享 Common 的 discovery keys 已刻意不再绑定 —— 见 §3.2）。

---

## 1. 完整工程示例

一个把小型 KV 存进文件型 SQLite 数据库的服务，外加一个内存实例用于测试。文件树
（与 [example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── check.sh        # 复制 example/check.sh
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-gorm-sqlite latest
    gorm.io/gorm                  v1.31.x
)
```

**main.go**：

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    starter "go-spring.org/starter-gorm-sqlite"
)

type greeting struct {
    ID      uint   `gorm:"primaryKey"`
    Message string `gorm:"size:255"`
}

type Service struct {
    DB *starter.DB `autowire:"primary"` // conf 里的 "primary" 实例
}

var manual = flag.Bool("manual", false, "保持进程运行")

func main() {
    flag.Parse()
    bean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(bean.Interface().(*Service))
        }()
    }
    gs.Run()
}

func runTest(s *Service) {
    ctx := context.Background()
    var version string
    if err := s.DB.WithContext(ctx).Raw("SELECT sqlite_version()").Scan(&version).Error; err != nil {
        fmt.Println("VERSION failed:", err)
        os.Exit(1)
    }
    if err := s.DB.WithContext(ctx).AutoMigrate(&greeting{}); err != nil { /* ... */ }
    if err := s.DB.WithContext(ctx).Create(&greeting{Message: "hello"}).Error; err != nil { /* ... */ }
    fmt.Println("SQLite round trip OK:", version)
    syscall.Kill(os.Getpid(), syscall.SIGTERM)
}
```

**conf/app.properties** —— 实际使用的完整配置面（复制自 `example/conf/app.properties`）：

```properties
# 内存数据库；每个连接都会打开自己的 :memory: 存储，因此必须把连接池钉在
# 单连接上，内存往返才稳定（见 §4.1）。
spring.gorm.sqlite.primary.file=:memory:
spring.gorm.sqlite.primary.journal-mode=wal
spring.gorm.sqlite.primary.busy-timeout=5000
spring.gorm.sqlite.primary.foreign-keys=true
spring.gorm.sqlite.primary.max-open-conns=1

# 文件型数据库展示更典型的多连接配置。
# spring.gorm.sqlite.file.file=/tmp/go-spring-example.db
# spring.gorm.sqlite.file.max-open-conns=8
```

**验证**（与 `example/check.sh` 同构）：

```bash
go run . > smoke.out 2>&1 &
# ... 等待后：
grep "SQLite round trip OK:" smoke.out    # 必须打印；仅看退出码不算证明
```

外部依赖：**零** —— SQLite 进程内、驱动纯 Go（无 cgo，可任意交叉编译）。这是唯一
冒烟测试不需要 docker 的 gorm 方言 starter。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-gorm-sqlite
  └─ init(): gormcore.Register(Dialect{Prefix: "spring.gorm.sqlite", ...})
        │
gs.Run()
  ├─ OnProperty("spring.gorm.sqlite") 仅在存在 ≥1 条目时触发
  ├─ conf.BindEach: 每个条目 → Config（value tag；file 过 expr "$ != ''" 校验）
  ├─ build(): Config → Spec{Dialector: glebarez/sqlite.Open(DSN), Pool,
  │          Resource: "gorm:sqlite"+file, ObserveEnabled}      — starter.go:49-56
  ├─ gormcore.Open: gorm.Open → ApplyPool（启动 ping，PingTimeout 兜底）
  │                 → DBCustomizers → *DB wrapper
  ├─ gs 注入 Observability → DB.Init(): observe 插件（db.system=sqlite）
  │                 + resilience/fault executor 回调（resource "gorm:sqlite"）
  ├─ 运行；就绪检查折叠进 gorm:sqlite:<name> PING 指示器
  └─ SIGTERM → DB.Destroy(): 关 executor → closers（sqlite 无）→ 关连接池
```

没有 TLS 注册（无传输层）、没有 discovery 拨号器（"服务器"就是文件路径）——
`starter.go:46-48` 的注释明确写了这一点。

### 2.2 一次查询的逐层走读

`db.WithContext(ctx).First(&g, 1)`：

1. gorm 的 `gorm:query` processor 执行 —— 但 gormcore 的 `ApplyCallbacks` 已把它
   *替换*为在实例 resilience executor（`${govern}` 的 timeout/retry/breaker，放火时
   还有 fault 注入器）之下运行的包装。`gorm.ErrRecordNotFound` 视为成功，"无行"
   不会触发熔断（`starter-gorm/resilience/callbacks.go:runGuard`）。
2. observe 插件的 `before_query` 开 span + in-flight 指标
   （`starter-gorm/observe/plugin.go`；此时还拿不到 SQL）。
3. 原 processor 执行：连接池取连接（SQLite：` :memory:` 的唯一连接，或文件库的 N
   个连接之一 —— 每个连接都会按 DSN 重新应用 `_pragma` 设置）、语句构建、行扫描。
4. `after_query` 把 SQL 写入 span、结束 span + 时长指标、写访问日志（级别来自
   wrapper 级 `observability` key）。
5. executor 包装把拒绝信号（`ErrCircuitOpen` 等）传播到 `tx.Error`。

---

## 3. 逐 key 行为参考

key 位于 `spring.gorm.sqlite.<name>.*`。连接池 keys（6 个，`gormcore.PoolSettings`）、
`observe.enabled` 与 wrapper 级 `observability` 为共享项 —— 见
[gormcore](../starter-gorm/USAGE_CN.md#2-配置参考)。

### 3.1 SQLite 特有 key（config.go:29-49）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `file` | string | — **必填**（expr `$ != ''`） | SQLite 路径或 `:memory:`；DSN() 的唯一来源。 | 空 → 绑定期 expr 失败，实例（及应用）启动失败。 |
| `journal-mode` | string | `wal` | 追加为 `_pragma=journal_mode(wal)`。取值 wal/delete/truncate/persist/memory/off（SQLite 语义，见 [pragma 文档](https://sqlite.org/pragma.html#pragma_journal_mode)）。空字符串则完全省略该 pragma（config.go:56）。 | `memory`/`off` 削弱持久性；非法模式 → SQLite 带警告回退，数据照常可用。 |
| `busy-timeout` | int（毫秒） | 5000 | `_pragma=busy_timeout(5000)`；并发写者等待而不是报 `SQLITE_BUSY`。`0` 省略该 pragma。 | 太小 + 并发写者 → 查询期 `database is locked`，而非启动期。 |
| `foreign-keys` | bool | true | 为 true 时追加 `_pragma=foreign_keys(1)`；**false 是省略 pragma，而不是发 `foreign_keys(0)`**（config.go:62-64）。⚠ 逐连接生效：经 DSN 对池中每个连接生效。 | 期望 `foreign-keys=false` 时仍有关联约束 → 约束静默不生效（SQLite 默认本就关闭）。 |
| `max-open-conns`（共享池 key） | int | 0 | ⚠ 与 `file=:memory:` 组合必须为 `1` —— 见 §4.1。 | 缺失 → 间歇性 "table not found" / 写丢失。 |

### 3.2 已移除的 key

`service-name`、`scheme`、`discovery` 曾随嵌入的 `gormcore.Common` 绑定，但 sqlite 的
`build` **从不读取** —— 没有可发现的服务器。已于 2026-08 **移除**：sqlite Config 只嵌
入 `gormcore.PoolSettings`（连接池 + ping + slow-threshold）外加 `observe.enabled`。
现在再设置会以未知 key 报错，而不是被静默忽略。

### 3.3 本方言不存在的 key

`tls.*` —— sqlite 的 Config 根本没有 TLS 块（不同于 sqlserver/clickhouse）；
"传输加密"在这里是文件系统权限问题。

---

## 4. 验证与故障演练

### 4.1 演练：`:memory:` 并发陷阱

成因：`file=:memory:` 时池中**每个连接都打开自己私有的内存库**（SQLite 语义，不是
驱动选择）。`max-open-conns` 未设（0 = 不限）时，AutoMigrate 可能跑在连接 A，插入
跑在连接 B → `no such table`。

演示（example 正是这么钉的：`example/conf/app.properties:3-8` 设
`max-open-conns=1`）：

```properties
# spring.gorm.sqlite.primary.max-open-conns=   ← 注释掉即可复现
spring.gorm.sqlite.primary.file=:memory:
```

```bash
go run .   # 间歇性失败 "no such table: greetings"
# 恢复 max-open-conns=1：
go run .   # 打印 "SQLite round trip OK:"
```

内存态又需要并发的替代方案：**文件库**（`file=/tmp/x.db`，可放 tmpfs），或经
`UseDBCustomizer` 手写 `file=file::memory:?cache=shared`（DSN 构造器不会替你加
`cache=shared`）。

### 4.2 验证接线

```bash
grep "SQLite round trip OK:" smoke.out     # CRUD + 事务往返
curl -s :9370/readyz                        # 引入 starter-actuator 后：gorm:sqlite:<name> 折叠在内
```

每查询可观测项（`observe.enabled=true`，默认开）：每个 Create/Query/Update/Delete 一个
`db.system=sqlite` 的 span、时长/in-flight 指标、带 SQL 语句的访问日志。按实例关闭用
`observe.enabled=false`（插件完全不安装）。

### 4.3 演练：写者争用下的 busy-timeout

两个实例指向同一个文件库，`busy-timeout=1`，一侧开一个故意的长写事务 —— 另一侧等
约 1ms 即报 `SQLITE_BUSY`。调回默认 5000 再试。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| `:memory:` 上 "no such table" | 池 > 1 连接，各连各的内存库 | `max-open-conns=1`（`:memory:` 必配）。 |
| 并发写下 `database is locked` | `busy-timeout` 太小 / journal-mode 非 wal | 保持默认（`busy-timeout=5000`、`journal-mode=wal`）。 |
| 实例启动失败：expr `$ != ''` | `file` 为空/缺失 | 设置 `file` —— 唯一必填 key。 |
| 外键约束不生效 | `foreign-keys=false`（pragma 被省略，SQLite 默认关闭） | 置 `true`。 |
| 第二个实例看不到第一个的数据 | 文件路径不一致 / 相对路径依赖 cwd | 用绝对路径；example 的 `init()` 会 chdir 到源码目录。 |
| 查询无 span/指标 | `observe.enabled=false` 或未引入 starter-otel | 重新开启 / 引入 starter-otel（无它时钩子静默空转）。 |
| 慢查询日志条目格式怪 | `slow-threshold>0` 自 2026-08 起将 GORM 的 warn 输出改经 `go-spring.org/log`（TagAppDef）转发 | 内容是 GORM 的单行文本；嫌吵可按消息过滤。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 方言特有 key | 4（+6 个共享池 key、+1 `observe.enabled`、+1 个 wrapper `observability`） |
| 必填 | 1（`file`） |
| quickstart 外部依赖 | 0（唯一没有外部依赖的 gorm 方言） |
| 死绑定 key | 0（discovery keys 已于 2026-08 移除） |
| "注意/坑" 条数 | 4 |

设计嫌疑：`foreign-keys=false` 是"省略"
而非"取反"（旋钮不对称）；DSN 构造器表达不了无值 pragma 与
`file::memory:?cache=shared`（config.go:50-53 已注明）；`:memory:` 与连接池的陷阱在
配置面上不可见 —— 启动期没有守卫拒绝 `:memory:` + `max-open-conns != 1` 的组合。
