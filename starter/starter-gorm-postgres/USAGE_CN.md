# starter-gorm-postgres 使用说明 — 参考手册

深度使用参考。概览见 [README_CN.md](README_CN.md)。锚定 [example/](example/)、
[example-health/](example-health/)、[example-load/](example-load/)、[example-otel/](example-otel/) 与
[example-cloudnative/](example-cloudnative/)。**GORM 语义属于 [gorm 官方文档](https://gorm.io/docs/)；
PostgreSQL 连接串语义属于 [pgx](https://github.com/jackc/pgx/blob/master/connstring.go)
（libpq 兼容，含 `sslmode`）**——本文只写绑定面与 go-spring 的增量（装配、服务发现、TLS、可观测、健康检查）。

**激活条件**：每个 `spring.gorm.postgres.<name>` 条目对应一个 client bean（对
`spring.gorm.postgres` 做 OnProperty 前缀检查）。没有 `enabled` 开关。共享部分（wrapper 生命周期、
连接池、observe 插件链、健康检查、`UseDBCustomizer`、10 个 `Common` key）见
[starter-gorm 的 USAGE](../starter-gorm/USAGE_CN.md)，此处不再重复。

---

## 1. 完整工程示例

固定 host 的 `primary` 实例 + actuator 健康检查 + OTel 可观测。目录结构：

```
demo/
├── go.mod
├── main.go
├── dao/
│   └── dao.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
module demo

go 1.26

require (
    go-spring.org/spring                 v1.3.4
    go-spring.org/starter-gorm-postgres  latest
    go-spring.org/starter-actuator       latest   // /health + /readyz
    go-spring.org/starter-otel           latest   // trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "demo/dao"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-gorm-postgres"
)

func main() {
    gs.Provide(dao.NewService).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**dao/dao.go** —— 按实例名注入：

```go
package dao

import (
    StarterPostgres "go-spring.org/starter-gorm-postgres"
)

type Service struct {
    DB *StarterPostgres.DB `autowire:"primary"`
}

func NewService() *Service { return &Service{} }

func (s *Service) Init() error { // gs InitMethod：建表 + 冒烟查询
    if err := s.DB.AutoMigrate(&KV{}); err != nil {
        return err
    }
    return s.DB.Exec("SELECT 1").Error
}
```

**conf/app.properties** —— 完整带注释的配置面：

```properties
# --- primary：固定 host ---------------------------------------------------------
spring.gorm.postgres.primary.host=127.0.0.1
spring.gorm.postgres.primary.port=5432
spring.gorm.postgres.primary.user=postgres
spring.gorm.postgres.primary.password=123456
spring.gorm.postgres.primary.db=test
spring.gorm.postgres.primary.sslmode=disable      # 冒烟库为明文
spring.gorm.postgres.primary.max-open-conns=10
spring.gorm.postgres.primary.max-idle-conns=5
spring.gorm.postgres.primary.conn-max-lifetime=30m
spring.gorm.postgres.primary.slow-threshold=200ms
# TLS 走 sslmode + 证书文件（注释态；见 §3）：
# spring.gorm.postgres.primary.sslmode=verify-full
# spring.gorm.postgres.primary.sslrootcert=/path/ca.pem
# spring.gorm.postgres.primary.sslcert=/path/client-cert.pem
# spring.gorm.postgres.primary.sslkey=/path/client-key.pem

# --- actuator（聚合 gorm:postgres:<name> 指示器）--------------------------------
spring.actuator.addr=:9370

# --- 可观测 ---------------------------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
```

**拉起并验证**：

```bash
docker run -d --name postgres -p 127.0.0.1:5432:5432 \
  -e POSTGRES_PASSWORD=123456 -e POSTGRES_DB=test postgres:16
# Jaeger all-in-one（可选）：
docker run -d --name jaeger -p 127.0.0.1:16686:16686 -p 127.0.0.1:4317:4317 jaegertracing/all-in-one
go run .

curl -i 127.0.0.1:9370/readyz                 # UP：gorm:postgres:primary
curl -i 127.0.0.1:9370/health
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
curl -s '127.0.0.1:16686/api/traces?service=demo&limit=1' | grep '"data":\['
```

---

## 2. 装配与时序

```
import starter-gorm-postgres
  └─ init: gormcore.Register(Dialect{Prefix: "spring.gorm.postgres", Engine: "postgresql", ...})
gs.Run()
  ├─ OnProperty("spring.gorm.postgres") 命中；conf.BindEach 为每个 <name> 条目绑定一份 Config
  ├─ 每实例：build(ctx, c)
  │    ├─ host/service-name 存在性检查（必须设其一）
  │    ├─ DSN(): "host=.. port=.. user=.. password=.. dbname=.. sslmode=.."（+ 可选段）
  │    └─ 服务发现（设了 service-name 且 mesh 关闭）：先 pgx.ParseConfig(DSN)，再把 pgx
  │         DialFunc 替换为基于 round-robin pick pool 的拨号闭包；直连模式直接用 DSN
  ├─ gormcore.Open：gorm.Open -> ApplyPool（含启动 ping、ping-timeout 上限）
  │    -> ApplyDBCustomizers -> *DB bean（Name=<name>，Init，Destroy）
  ├─ 健康指示器 "gorm:postgres:<name>" 以 health.Indicator 导出
  ├─ DB.Init（在注入 Observability 字段之后）：observe 插件（db.system=postgresql）
  │    + 资源 "gorm:postgresql:<host|service-name>" 的 resilience/fault executor 回调
  └─ DB.Destroy（SIGTERM）：关 executor -> 停发现 watch -> 关连接池
```

**一次查询逐层走读**（引入 starter-otel 且 `observe.enabled=true`）：
`db.WithContext(ctx).Exec("SELECT 1")` → resilience 回调包裹 processor → gorm observe 插件的
before/after 回调开启 span（`db.system=postgresql`）、记录 `db.client.operation.duration`、输出
访问日志（级别来自 wrapper 的 `observability` 字段，默认 `brief`）→ database/sql 取池内连接
（发现模式下新物理连接经替换后的 pgx `DialFunc` 拨号）→ pgx 执行。

**为什么用 pgx `DialFunc` 而非改写 DSN**：与 mysql 驱动（自定义拨号 network 名）不同，pgx
直接在解析后的配置上暴露拨号钩子。`build` 调一次 `pgx.ParseConfig`，然后把 `DialFunc` 换成
忽略 network/addr 参数、经 round-robin pool 选端点走 TCP 拨号的闭包。推论：发现模式下
`host`/`port` 不会到达网络，但**必须能解析为合法 pgx DSN**——`ParseConfig` 在替换之前运行，
因此 `port` 非数字或越界会让 build 失败，尽管该值永远不会被拨号。（example 用
`host=0.0.0.0 port=5432` 作占位。）

---

## 3. 逐 key 行为参考

`spring.gorm.postgres.<name>.*` 下的 PostgreSQL 专有 key（10 个共享 `Common` key 见
[starter-gorm 的 USAGE](../starter-gorm/USAGE_CN.md)）：

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `host` | string | — | DSN `host=`。未设 `service-name` 时**必填**。⚠ 发现模式下是死 key（不会被拨号）但仍参与解析。 | 两者都不设 → build 报 `one of host or service-name must be set`。 |
| `port` | string | `5432` | DSN `port=`。⚠ 发现模式：不会被拨号，但必须是合法 TCP 端口（`pgx.ParseConfig` 先跑）。 | 非数字端口即使在发现模式下也让 build 失败。 |
| `user` | string | — | DSN `user=`。**必填**（无 expr 校验）。 | 启动 ping 认证失败。 |
| `password` | string | — | DSN `password=`。**必填。** 含空格的值会破坏空格分隔的 DSN——不做任何引号/转义处理。 | 认证失败，或带空格密码导致 DSN 解析错误。 |
| `db` | string | — | DSN `dbname=`。**必填。** | ping 能过；首次查询失败（`does not exist` / 进错库）。 |
| `sslmode` | string | `disable` | DSN `sslmode=`——主要的 TLS 开关（libpq 取值：`disable`/`allow`/`prefer`/`require`/`verify-ca`/`verify-full`；语义见 [pgx/libpq 文档](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNECT-SSLMODE)）。2026-08 起新增与 mysql 对称的 `tls.*` 块：启用时向 pgx 连接注入 `*tls.Config`；`sslmode` 仍决定是否协商——`tls.enabled=true` 且 `sslmode=disable` 启动即报错。 | 对仅 TLS 的服务端留着 `disable` → 连接被拒；`require` 不配根证书 → 不校验服务端证书。 |
| `sslrootcert` | string | "" | DSN `sslrootcert=`（PEM CA 路径）。对 `verify-ca`/`verify-full` 有意义。 | 路径不可读 → 首次使用时 pgx 连接错误。 |
| `sslcert` / `sslkey` | string | "" | DSN `sslcert=` / `sslkey=`（客户端证书对），用于 mTLS 认证。⚠ `sslmode=disable` 时设置了也无效果——模式不升级就是死 key。 | 只设证书对不升 sslmode → 静默明文。 |
| `timezone` | string | "" | DSN `TimeZone=`（字面大写 T 参数，gorm postgres 驱动的约定）。设置会话时区。 | 不设 = 服务端默认。 |
| `connectTimeout` | duration | 0 | DSN `connect_timeout=`，取整**秒**（**向上取整**：500ms → 1、2500ms → 3，2026-08 修复；原先截断会让亚秒值变 0 = 无上限）。 | 0 = 不设上限（OS 默认）。 |

方言侧 ⚠ 耦合：`sslmode` 门控 `sslrootcert`/`sslcert`/`sslkey`；`tls.enabled` + `sslmode=disable` → 启动报错；发现模式废掉 `host`/`port` 的
拨号但保留其解析；这些 key 无法表达含空格的 DSN 值。

配置核对（本 starter 的 value tag）：`host`、`port`、`user`、`password`、`db`、`sslmode`、
`timezone`、`connectTimeout`、`sslrootcert`、`sslcert`、`sslkey`、`tls.*`——与上表一一对应，无多余项。

---

## 4. 验证与故障演练

### 4.1 基线

```bash
curl -i 127.0.0.1:9370/readyz                       # gorm:postgres:primary 指示器 UP
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
```

### 4.2 TLS 演练

1. 把 `sslmode=verify-full` + `sslrootcert` 指向你的 CA 并重启；协商失败会在启动 ping
   （`ping-timeout` 上限）处 fail-fast。
2. 服务端验证：`SELECT ssl, version FROM pg_stat_ssl WHERE pid = pg_backend_pid();`
   返回 `ssl | TLSv1.3`。
3. 反向检查：对仅 TLS 的服务端把 `sslmode=disable` → 启动即报连接错误（而非运行期意外）。

### 4.3 配错库演练

去掉 `db`（或写错）：应用照常启动——启动 ping 不碰库——随后 `Init` 里的 `AutoMigrate` 报
`database "x" does not exist`。请把 `db` 视为必填项。

### 4.4 服务发现故障切换（要点）

复刻 mysql starter 的演练（其 USAGE §4.3）：加一个 `service-name=postgres-cluster` 的实例 +
etcd discovery 后端；注册 `/services/postgres-cluster/<id>` key，值为
`{"service_name":"postgres-cluster","addr":"host:5432"}`；`etcdctl del` 删掉一个 key → watch
推送新快照 → 新物理连接（受 `conn-max-lifetime` 约束）只拨幸存者。演练中用
`SELECT inet_server_addr()` 区分落到哪台服务端。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| build 报 `one of host or service-name must be set` | 两个 key 都没配 | 设 `host` 或 `service-name`。 |
| 发现模式下 build 失败，报端口/DSN 解析错误 | 占位 `port` 不是合法端口（`pgx.ParseConfig` 先于拨号替换运行） | 保留一个合理的 `port=5432`。 |
| 启动失败 `gorm ping:` | host/凭据错误、服务端未起或 TLS 姿态不匹配 | 修配置；对照服务端检查 `sslmode`；`ping-timeout` 只决定等多久。 |
| 服务端要求 TLS，连接被拒 | 默认 `sslmode=disable` | 设 `sslmode=require`（或校验模式 + `sslrootcert`）。 |
| 设了 `sslcert`/`sslkey` 但连接仍是明文 | `sslmode` 还是 `disable`——模式不要求时这些 key 是死的 | 提升 `sslmode`。 |
| 启动正常，首次查询报 `database ... does not exist` | `db` 缺失/写错——ping 不碰库 | 正确设置 `db`。 |
| 带空格的密码导致启动失败 | 空格分隔 DSN，未做引号处理 | 改密码，或记入设计问题。 |
| 发现型 client 拨的是配置 `host` | 未设 `service-name`——走了直连路径 | 设 `service-name`（后端不为 "default" 时加 `discovery`）。 |
| 每查询无 span/指标 | `observe.enabled=false` 或缺 starter-otel | 重新开启 / 引入 starter-otel。 |
| `connect_timeout` 对亚秒值看似无效 | 截断到整秒（1500ms → 1） | 用整秒 duration。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 11 个方言专有（+10 共享 Common、+1 wrapper `observability`） |
| 其中必填 | 3（`user`/`password`/`db`）+ `host`/`service-name` 二选一 |
| quickstart 前置外部依赖 | 1（PostgreSQL；trace 另需 Jaeger） |
| "注意/坑"条数 | 6（tls/sslmode 冲突守卫、sslmode 门控、占位端口解析、ping 不校验 db、带空格密码、connect_timeout 取整） |

设计嫌疑：`user`/`password`/`db` 缺 expr `Require` 校验（空值以驱动错误暴露在 ping/首次查询）；
DSN 值不做空格转义。（与 mysql 的 TLS 词汇不一致、`connectTimeout` 亚秒截断均已于
2026-08 修复：新增 `tls.*` 块，亚秒值向上取整。）
