# starter-gorm-mysql 使用说明 — 参考手册

深度使用参考。概览见 [README_CN.md](README_CN.md)。锚定 [example/](example/)、
[example-health/](example-health/)、[example-load/](example-load/)、[example-otel/](example-otel/) 与
[example-cloudnative/](example-cloudnative/)。**GORM 语义属于 [gorm 官方文档](https://gorm.io/docs/)；
MySQL DSN 参数语义属于 [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#dsn-data-source-name)**
——本文只写绑定面与 go-spring 的增量（装配、服务发现、TLS、可观测、健康检查）。

**激活条件**：每个 `spring.gorm.mysql.<name>` 条目对应一个 client bean（对 `spring.gorm.mysql`
做 OnProperty 前缀检查）。没有 `enabled` 开关——存在条目即激活；无条目则完全不注册。共享部分
（wrapper 生命周期、连接池、observe 插件链、健康检查、`UseDBCustomizer`、10 个 `Common` key）
见 [starter-gorm 的 USAGE](../starter-gorm/USAGE_CN.md)，此处不再重复。

---

## 1. 完整工程示例

一个贴近真实的服务：固定 addr 的 `primary` 实例 + 经 etcd 服务发现解析的 `cluster` 实例 +
actuator 健康检查 + OTel 可观测。目录结构：

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
    go-spring.org/spring               v1.3.4
    go-spring.org/starter-gorm-mysql   latest
    go-spring.org/starter-registry-etcd latest   // etcd discovery 后端
    go-spring.org/starter-actuator     latest   // /health + /readyz
    go-spring.org/starter-otel         latest   // trace/metric 导出
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
    _ "go-spring.org/starter-registry-etcd" // 注册 etcd discovery 后端
    _ "go-spring.org/starter-gorm-mysql"
)

func main() {
    gs.Provide(dao.NewService).Export(gs.As[gs.Rooter]())
    gs.Run()
}
```

**dao/dao.go** —— 两个实例按实例名注入：

```go
package dao

import (
    StarterMySql "go-spring.org/starter-gorm-mysql"
)

type Service struct {
    Primary *StarterMySql.DB `autowire:"primary"` // 固定 addr，带连接池调优
    Cluster *StarterMySql.DB `autowire:"cluster"` // 走服务发现（etcd）
}

func NewService() *Service { return &Service{} }

func (s *Service) Init() error { // gs InitMethod：建表 + 两条链路冒烟
    if err := s.Primary.AutoMigrate(&KV{}); err != nil {
        return err
    }
    return s.Cluster.Exec("SELECT 1").Error
}
```

**conf/app.properties** —— 完整带注释的配置面：

```properties
# --- primary：固定 addr ---------------------------------------------------------
spring.gorm.mysql.primary.user=root
spring.gorm.mysql.primary.password=123456
spring.gorm.mysql.primary.addr=127.0.0.1:3306
spring.gorm.mysql.primary.db=test
spring.gorm.mysql.primary.parseTime=true          # DATETIME 扫描为 time.Time
spring.gorm.mysql.primary.max-open-conns=10
spring.gorm.mysql.primary.max-idle-conns=5
spring.gorm.mysql.primary.conn-max-lifetime=30m
spring.gorm.mysql.primary.slow-threshold=200ms

# --- cluster：服务发现解析（etcd）----------------------------------------------
spring.gorm.mysql.cluster.user=root
spring.gorm.mysql.cluster.password=123456
spring.gorm.mysql.cluster.db=test
spring.gorm.mysql.cluster.service-name=mysql-cluster
spring.gorm.mysql.cluster.discovery=local        # -> spring.discovery.etcd.local.*
spring.gorm.mysql.cluster.conn-max-lifetime=30m  # 约束故障切换滞后（见 §4.3）

# --- 名为 "local" 的 etcd discovery 后端 ----------------------------------------
spring.discovery.etcd.local.endpoints=127.0.0.1:2379
spring.discovery.etcd.local.key-prefix=/services/

# --- actuator（聚合 gorm:mysql:<name> 指示器）-----------------------------------
spring.actuator.addr=:9370

# --- 可观测 ---------------------------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
```

**拉起外部依赖**：

```bash
docker run -d --name mysql -p 127.0.0.1:3306:3306 \
  -e MYSQL_ROOT_PASSWORD=123456 -e MYSQL_DATABASE=test mysql:8
docker run -d --name etcd -p 127.0.0.1:2379:2379 \
  quay.io/coreos/etcd:v3.5.16 etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:2379
# Jaeger all-in-one（可选）：
docker run -d --name jaeger -p 127.0.0.1:16686:16686 -p 127.0.0.1:4317:4317 jaegertracing/all-in-one
# 向 etcd 注册两个 MySQL 实例（registrar 的 key/value 布局）：
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/a \
  '{"service_name":"mysql-cluster","addr":"127.0.0.1:3306"}'
go run .
```

**验证**：

```bash
curl -i 127.0.0.1:9370/readyz                 # UP：gorm:mysql:primary + gorm:mysql:cluster
curl -i 127.0.0.1:9370/health                 # 聚合指示器
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration
curl -s '127.0.0.1:16686/api/traces?service=demo&limit=1' | grep '"data":\['
```

---

## 2. 装配与时序

```
import starter-gorm-mysql
  └─ init: gormcore.Register(Dialect{Prefix: "spring.gorm.mysql", Engine: "mysql", ...})
gs.Run()
  ├─ OnProperty("spring.gorm.mysql") 命中；conf.BindEach 为每个 <name> 条目绑定一份 Config
  ├─ 每实例：build(ctx, c)
  │    ├─ addr/service-name 存在性检查（必须设其一）
  │    ├─ TLS：c.TLS.Build() -> *tls.Config；mysql.RegisterTLSConfig("gstls_<n>", cfg)
  │    ├─ DSN(): user:pass@tcp(addr)/db?params
  │    └─ 服务发现（设了 service-name 且 mesh 关闭）：
  │         mysql.RegisterDialContext("gsdisco_<svc>_<n>", 经 Resolver.Pick 拨号)
  │         并把 DSN 改写为 net(gsdisco_...)(<service-name>)
  ├─ gormcore.Open：gorm.Open -> ApplyPool（含启动 ping、ping-timeout 上限）
  │    -> ApplyDBCustomizers -> *DB bean（Name=<name>，Init，Destroy）
  ├─ 健康指示器 "gorm:mysql:<name>" 以 health.Indicator 导出
  ├─ DB.Init（在注入 Observability 字段之后）：observe 插件（db.system=mysql）
  │    + 资源 "gorm:mysql:<addr|service-name>" 的 resilience/fault executor 回调
  └─ DB.Destroy（SIGTERM）：关 executor -> 停发现 watch + 注销 TLS -> 关连接池
```

**一次查询逐层走读**（引入 starter-otel 且 `observe.enabled=true`）：
`db.WithContext(ctx).Exec("SELECT 1")` → resilience 回调包裹 processor（配置了治理中心则生效，
否则 no-op）→ gorm observe 插件的 before/after 回调开启 span（`db.system=mysql`）、记录
`db.client.operation.duration` 指标、输出访问日志（级别来自 wrapper 的 `observability` 字段，
默认 `brief`）→ database/sql 取池内连接（新连接经注册的发现 dialer 拨号）→ 驱动执行。

**为什么改写 DSN**：go-sql-driver 只有在 DSN 的 *network* 段等于注册名时才走
`RegisterDialContext` 的自定义拨号器——host 段随即被拨号器忽略（它自己挑活实例）。starter 同时
改写两者（`Network=gsdisco_...`、`Addr=<service-name>`），因此发现模式下你配置的 `addr` 完全
不会被拨号；配置里只有 `user`/`password`/`db` 和参数项生效。若自己手拼同样 DSN 却忘了改写，
驱动会对写下的 host 走普通 TCP 拨号。

---

## 3. 逐 key 行为参考

`spring.gorm.mysql.<name>.*` 下的 MySQL 专有 key（10 个共享 `Common` key——连接池、ping、
slow-threshold、service-name/scheme/discovery、observe.enabled——见
[starter-gorm 的 USAGE](../starter-gorm/USAGE_CN.md)）：

| Key | 类型 | 默认 | 行为 / 联动 | 配错后果 |
|-----|------|------|------------|----------|
| `user` | string | — | DSN user 段。**必填**（无 expr 校验；空值拼出 `":pass@..."`，驱动报认证错误）。 | 启动 ping 认证失败。 |
| `password` | string | — | DSN password 段。**必填**，不做 URL 转义（go-sql-driver 要求对 `:` `/` 等做百分号转义——需自行处理）。 | 认证失败；含特殊字符的密码未转义会破坏解析。 |
| `db` | string | — | 库名。**必填。** | 首次查询报 `No database selected`（ping 不碰库，能通过）。 |
| `addr` | string | — | `tcp(...)` 里的 host:port。未设 `service-name` 时**必填**。⚠ 发现模式下 `addr` 是死 key——拨号前被替换为服务名。 | 两者都不设 → build 报 `one of addr or service-name must be set`。 |
| `net` | string | "" | DSN network 词；空值渲染为 `tcp`。⚠ 发现模式下被拨号器名替换——死 key。除 tcp 外只有 `unix` 有意义（addr=socket 路径）。 | 设 `net=udp` 等 → 驱动拨号错误。 |
| `timeout` | duration | 0 | DSN `timeout=`（拨号+认证上限），`time.Duration.String()` 格式。 | 0 = 驱动默认（依赖 OS）；太小 → 间歇拨号失败。 |
| `readTimeout` / `writeTimeout` | duration | 0 | DSN `readTimeout=` / `writeTimeout=`。 | readTimeout 小于最慢合法查询 → 读中途 `invalid connection`。 |
| `charset` | string | "" | DSN `charset=`。 | 空 = 服务端默认（老服务端常 latin1）→ 乱码。 |
| `parseTime` | bool | **false** | 仅在设置时输出 `parseTime=true`。⚠ 经典坑：默认 false 时把 `DATETIME`/`TIMESTAMP` 列扫进 `time.Time` 会失败（`unsupported Scan`）；带时间字段的 gorm 模型必须 `parseTime=true`。 | 所有时间列运行期扫描报错。 |
| `loc` | string | "" | DSN `loc=`，做 QueryEscape（`Asia/Shanghai` → `Asia%2FShanghai`）。仅在 `parseTime=true` 时有意义。 | 不设 = UTC；时区列出现意外偏移。 |
| `tls.*` | block | off | 6 个共享 `tlsconf` key（`tls.enabled`、`tls.ca-file`、`tls.cert-file`、`tls.key-file`、`tls.server-name`、`tls.insecure-skip-verify`）。启用时向驱动注册名为 `gstls_<n>` 的 `*tls.Config`，DSN 写 `tls=gstls_<n>`。⚠ DSN 内建短名 `tls=true`/`skip-verify`/`preferred` **无法**通过配置触达——starter 一律用自注册配置。`ca-file` 留空回退系统根证书。 | `ca-file` 不可读会在 build 期失败（fail-fast）。`server-name` 未设且 addr 是 IP → 校验模式下证书主机名不匹配。 |

方言侧 ⚠ 耦合：`parseTime`+`loc` 成对出现；`service-name` 下 `addr`+`net` 均为死 key；
`tls.enabled=true` 要求引用的证书文件在绑定时存在。

配置核对（本 starter 的 value tag）：`user`、`password`、`net`、`addr`、`db`、`timeout`、
`readTimeout`、`writeTimeout`、`charset`、`parseTime`、`loc`、`tls`——与上表一一对应，无多余项。

---

## 4. 验证与故障演练

### 4.1 基线

```bash
curl -i 127.0.0.1:9370/readyz                       # 两个 gorm:mysql:* 指示器均 UP
curl -s 127.0.0.1:9090/metrics | grep db_client_operation_duration   # 每操作直方图
```

### 4.2 TLS

把 `tls.*` 指向真实证书并重启；`build` 里的启动日志
`creating gorm mysql client, addr=... ` 会先出现——CA 文件损坏在任何拨号**之前**失败。
服务端验证协商：TLS 会话的 `SHOW STATUS LIKE 'Ssl_cipher'` 非空。

### 4.3 服务发现故障切换演练（摘除一个端点）

1. 注册两个实例（第二个指向另一台 MySQL，或同机不同 key）：

```bash
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/a '{"service_name":"mysql-cluster","addr":"127.0.0.1:3306"}'
ETCDCTL_API=3 etcdctl put /services/mysql-cluster/b '{"service_name":"mysql-cluster","addr":"127.0.0.1:3307"}'
```

2. 启动应用；两个 key 都进入 resolver 端点集；每条**新**连接经 `Resolver.Pick()` 二选一拨号。
3. 摘除一个端点：

```bash
ETCDCTL_API=3 etcdctl del /services/mysql-cluster/b
```

4. etcd watch 推送新快照；新拨号只到 `a`。已建立的连接继续用到关闭——用
   `conn-max-lifetime`（上面 30m）约束故障切换的最长滞后；演练想立刻看到效果可设
   `conn-max-lifetime=30s`。
5. 观察火焰：`docker stop` 被删 key 背后的 MySQL——新拨号命中幸存者（查询继续 200）、
   `db.client.operation.duration` 继续记录、watch 稳定后访问日志不再出现新的拨号错误。

### 4.4 parseTime 演练

去掉 `parseTime=true`，跑一个含 `time.Time` 字段的模型 → 该列每次扫描报
`unsupported Scan, storing driver.Value type []uint8 into type *time.Time`。恢复该 key。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| build 报 `one of addr or service-name must be set` | 两个 key 都没配 | 设 `addr` 或 `service-name`。 |
| 启动失败 `gorm ping:` | addr/凭据错误，或 `ping-timeout` 内 DB 不可达 | 修配置或调大 `ping-timeout`；这是 fail-fast ping，不是运行期故障。 |
| `unsupported Scan ... []uint8 ... time.Time` | `parseTime` 默认 false | 设 `parseTime=true`（需要时加 `loc`）。 |
| 发现型 client 拨的是配置 `addr` 而非注册中心端点 | 未设 `service-name`——走了直连路径 | 设 `service-name`（后端不为 "default" 时加 `discovery`）。 |
| 发现拨号报 `no endpoints` / 端点集为空 | 后端名不匹配或无存活 key | 检查 `spring.discovery.etcd.<name>` 与 `discovery` key 一致；检查前缀下 etcd key。 |
| 时间值差了几个小时 | `parseTime=true` 但 `loc` 未设（默认 UTC） | 设 `loc=Asia/Shanghai`（或你的时区）。 |
| 慢查询中途连接断开 | `readTimeout` 小于查询耗时 | 调大 `readTimeout` 或优化查询。 |
| 每查询无 span/指标 | `observe.enabled=false`，或未引入 starter-otel | 重新开启 / 引入 starter-otel（无 OTel 全局时 observe 是静默 no-op）。 |
| 本地 TLS 正常、生产报主机名不匹配 | `tls.server-name` 未设且 addr 是 IP | 把 `tls.server-name` 设为证书 CN。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 12 个方言专有（+6 tls、+10 共享 Common、+1 wrapper `observability`） |
| 其中必填 | 3（`user`/`password`/`db`）+ `addr`/`service-name` 二选一 |
| quickstart 前置外部依赖 | 1（MySQL；发现另需 etcd，trace 另需 Jaeger） |
| "注意/坑"条数 | 6（parseTime、password 转义、tls 短名不可达、发现模式死 addr/net、ping 不校验 db、loc 耦合） |

设计嫌疑：`user`/`password`/`db` 缺 expr `Require` 校验（空值以驱动认证/`No database selected`
错误而非绑定期配置错误暴露）；驱动的内建 `tls=true`/`skip-verify` 模式不经完整 tlsconf 块无法
使用；`parseTime=false` 默认值与"带时间字段的 gorm 模型"这一常见设定相悖。
