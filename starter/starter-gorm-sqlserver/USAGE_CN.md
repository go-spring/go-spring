# starter-gorm-sqlserver 使用说明（参考手册）

详细使用文档。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照源码
（`starter.go`、`config.go`、`starter_test.go`）与可运行示例
[example/](example/)、[example-load/](example-load/)、[example-otel/](example-otel/)
（docker 门控 `check.sh`）核实。**SQL Server 连接语义以驱动为准** ——
[microsoft/go-mssqldb](https://github.com/microsoft/go-mssqldb)（经
[gorm.io/driver/sqlserver](https://gorm.io/docs/connecting_to_the_database#SQL-Server)）；
GORM 语义见 [gorm 文档](https://gorm.io/docs/)。共享的 wrapper 生命周期、连接池/
observe/health 接线与 `UseDBCustomizer` 见 [gormcore](../starter-gorm/USAGE_CN.md)，
此处不再重复。

**激活条件**：`spring.gorm.sqlserver` 下每个条目注册一个 `*starter.DB` bean（外加配对
的 `health.Indicator`）；没有任何条目时 starter 不注册任何东西
（`starter_test.go:TestSqlserverNotTriggered`）。

---

## 1. 完整工程示例

一个带直连实例的服务，外加一个地址来自注册中心（discovery backend）的实例。文件树
（与 [example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── discovery.go
├── conf/
│   └── app.properties
├── docker-compose.yml   # 取自 example/docker-compose.yml
└── check.sh             # 取自 example/check.sh
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-gorm-sqlserver latest
    gorm.io/gorm                     v1.31.x
)
```

**main.go**（节选自 `example/example.go`）：

```go
package main

import (
    "flag"
    "net/http"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    starter "go-spring.org/starter-gorm-sqlserver"
    "gorm.io/gorm"
)

// `key`/`value` 是 SQL Server 保留字 —— 用 gorm tag 重映射列名。
type KV struct {
    ID    uint   `gorm:"primaryKey"`
    Key   string `gorm:"column:kkey;size:64;uniqueIndex"`
    Value string `gorm:"column:vvalue;size:255"`
}

type Service struct {
    DB          *starter.DB `autowire:"primary"`
    DiscoveryDB *starter.DB `autowire:"discovery"`
}

var manual = flag.Bool("manual", false, "保持服务运行")

func main() {
    flag.Parse()
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    http.HandleFunc("/sqlserver_version", func(w http.ResponseWriter, r *http.Request) {
        s := svrBean.Interface().(*Service)
        var version string
        if err := s.DB.Raw("SELECT @@VERSION").Scan(&version).Error; err != nil {
            _, _ = w.Write([]byte(err.Error()))
            return
        }
        _, _ = w.Write([]byte(version))
    })
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(svrBean.Interface().(*Service))  // 迁移/CRUD/事务 + discovery 往返
        }()
    }
    gs.Run()
}
```

**discovery.go** —— 注册 `discovery` 实例所解析的 discovery backend（真实部署里这是
公司级适配器）：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:1433", Healthy: true}))
}
```

**conf/app.properties**（复制自 `example/conf/app.properties`）：

```properties
spring.gorm.sqlserver.primary.user=sa
spring.gorm.sqlserver.primary.password=Str0ng!Passw0rd
spring.gorm.sqlserver.primary.host=127.0.0.1
spring.gorm.sqlserver.primary.port=1433
spring.gorm.sqlserver.primary.db=master
# 连接池 / ping / 慢日志
spring.gorm.sqlserver.primary.dialTimeout=5s
spring.gorm.sqlserver.primary.connectTimeout=10s
spring.gorm.sqlserver.primary.max-open-conns=10
spring.gorm.sqlserver.primary.max-idle-conns=5
spring.gorm.sqlserver.primary.conn-max-lifetime=30m
spring.gorm.sqlserver.primary.conn-max-idle-time=5m
spring.gorm.sqlserver.primary.ping-timeout=5s
spring.gorm.sqlserver.primary.slow-threshold=200ms
# TLS（此处关闭）。开启加密：
# spring.gorm.sqlserver.primary.tls.enabled=true
# spring.gorm.sqlserver.primary.tls.insecure-skip-verify=true
# spring.gorm.sqlserver.primary.tls.ca-file=/path/server-cert.pem

# discovery 实例：host/port 故意设为哑值 —— 因 service-name 生效而被忽略，
# 地址来自 discovery backend。
spring.gorm.sqlserver.discovery.user=sa
spring.gorm.sqlserver.discovery.password=Str0ng!Passw0rd
spring.gorm.sqlserver.discovery.host=0.0.0.0
spring.gorm.sqlserver.discovery.port=0
spring.gorm.sqlserver.discovery.db=master
spring.gorm.sqlserver.discovery.service-name=sqlserver-cluster
```

**docker-compose.yml** —— 带健康检查的 SQL Server 2022（启动慢；端口先于服务可用
打开）：

```yaml
services:
  sqlserver:
    image: mcr.microsoft.com/mssql/server:2022-latest
    ports: ["127.0.0.1:1433:1433"]
    environment: { ACCEPT_EULA: "Y", MSSQL_SA_PASSWORD: "Str0ng!Passw0rd" }
    healthcheck:
      test: ["CMD-SHELL", >-
        /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "Str0ng!Passw0rd" -No -Q "SELECT 1"
        || /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P "Str0ng!Passw0rd" -Q "SELECT 1"]
      interval: 5s
      retries: 40
      start_period: 30s
```

**验证**：

```bash
docker compose up -d && wait-healthy     # 或直接 ./check.sh（全套）
go run .                                  # 打印 "Response from server:" + "Response from discovered server:"
go run . -manual &                        # 然后：
curl http://127.0.0.1:9090/sqlserver_version
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-gorm-sqlserver
  └─ init(): gormcore.Register(Dialect{Prefix: "spring.gorm.sqlserver",
        Engine: "microsoft.sql_server", HealthPrefix: "gorm:sqlserver:"})
gs.Run()
  ├─ conf.BindEach → 每条目一个 Config
  ├─ build()                                        — starter.go:63-107
  │    ├─ 守卫：host 或 service-name 必须有一个（快速失败）
  │    ├─ c.NewResolver(ctx)                        — 无 service-name 或 mesh 开启时为 nil
  │    ├─ discovery 路径：msdsn.Parse(DSN) → mssql.NewConnectorConfig
  │    │    → connector.Dialer = resolverDialer     — 每次拨号重新挑一个存活实例
  │    │    → sqlserver.New(Config{Conn: sql.OpenDB(connector)})
  │    └─ 直连路径：sqlserver.Open(DSN)
  ├─ gormcore.Open: gorm.Open → ApplyPool（启动 ping）→ DBCustomizers
  ├─ DB.Init(): observe 插件（db.system=microsoft.sql_server）+ resilience 回调
  │            （resource "gorm:sqlserver" + service-name/host）
  ├─ 运行；resolver 的后台 watch 持续刷新端点集
  └─ SIGTERM → DB.Destroy(): executor → closers（停 discovery watch）→ 关连接池
```

### 2.2 Discovery 拨号路径 —— 与哑地址要求

适配点**不是** `database/sql` 的 `RegisterDialContext`；go-mssqldb 提供了专门钩子：
`mssql.NewConnectorConfig(msdsn.Parse(dsn))` 得到的 Connector 有一个 `Dialer` 字段，
接受任意 `mssql.Dialer`。starter 设置 `resolverDialer{r: resolver, nd: &net.Dialer{}}`；
其 `DialContext` **忽略 network 与 addr 参数**，改拨 `resolver.Pick()` 返回的
`ep.Addr`（starter.go:124-130）—— 因此池中每个新连接都落在当前存活的实例上，地址
变化无需重建客户端即生效。

推论：设置 `service-name` 后，`host`/`port` 永远不会被拨号 —— 但它们**也不是可选项**：
DSN 仍会嵌入它们（`sqlserver://...@host:port?...`），且必须能通过 `msdsn.Parse` 解析。
example 特意设哑值 `0.0.0.0`/`0`，以证明地址确实来自 discovery
（example/conf/app.properties 的 `discovery` 块）。mesh 模式下（`NewResolver` 返回
nil）直接使用配置的 Host —— sidecar 负责 discovery+LB。

### 2.3 一次查询的逐层走读

`db.WithContext(ctx).Raw("SELECT @@VERSION").Scan(&v)`：

1. `gorm:raw` processor —— 已被 gormcore 的 executor 包装替换（`${govern}` 的
   timeout/retry/breaker，放火时含 fault 注入器；`gorm.ErrRecordNotFound` 视为成功）。
2. observe 插件锚在 `before_raw` 的 span（db.system=microsoft.sql_server）+ 指标。
3. 原 processor：池取连接 → resolverDialer.DialContext（discovery 实例）或驱动直拨
   → TDS 登录（encrypt 按 DSN，见 §3.2）→ 查询。
4. `after_*` 写入 SQL、结束 span/指标、写访问日志。

---

## 3. 逐 key 行为参考

key 位于 `spring.gorm.sqlserver.<name>.*`。Common keys（10 个）与 wrapper 级
`observability` 见 [gormcore](../starter-gorm/USAGE_CN.md#2-配置参考)。

### 3.1 连接 key（config.go:32-45）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `user` / `password` | string | — **必填**（tag 无默认值） | 查询转义后进 DSN userinfo。 | 缺失 → 启动期绑定错误。 |
| `host` | string | "" | 无 `service-name` 时拨号它。⚠ `host`/`service-name` 二选一强制（build 守卫，starter.go:64-66）。 | 两者皆空 → 实例快速失败 "one of host or service-name must be set"。 |
| `port` | string | 1433 | 字符串，拼在 `:` 后。 | 端口错 → 启动 ping 在 `ping-timeout` 内失败。 |
| `db` | string | — **必填** | DSN 的 `database=` 参数。 | 缺失 → 绑定错误。 |
| `dialTimeout` | duration | 0 | → DSN `dial+timeout=<秒>`（整数秒；亚秒值**向上取整**到 1s —— 截断成 0 等于"无超时"）。0 = 驱动默认。 | 太小 → 高负载网络间歇性拨号超时。 |
| `connectTimeout` | duration | 0 | → DSN `connection+timeout=<秒>`（同样的取整规则）。0 = 驱动默认。SQL Server 无 DSN 级读写超时 —— 逐操作超时走 `WithContext` + governance。 | 太小 → 负载下登录超时。 |

### 3.2 TLS 块（`tls.*`，sqlserver 本地子集，见 config.go）

映射目标是 **DSN 参数**，不是 `*tls.Config`。只绑定 DSN 能表达的三个 key；共享 tlsconf
块的 `cert-file`/`key-file`/`server-name` 已于 2026-08 **移除** —— 它们在 DSN 里没有位置，
此前被静默忽略：

| Key | 默认值 | 映射到 | 说明 |
|-----|--------|--------|------|
| `tls.enabled` | false | `encrypt=true` | 关闭时不发 `encrypt` 参数；应用 go-mssqldb 默认（见[其文档](https://github.com/microsoft/go-mssqldb#connection-parameters)）。 |
| `tls.insecure-skip-verify` | false | `TrustServerCertificate=true` | 仅在 `encrypt=true` 时发出（嵌套于 Enabled，config.go:81-83）。 |
| `tls.ca-file` | "" | `certificate=<URL 转义路径>` | PEM 服务器证书路径；`starter_test.go` 验证（`certificate=%2Fca.pem`）。 |
已移除的 key：`tls.cert-file`、`tls.key-file`、`tls.server-name` —— sqlserver Config 现在
绑定自有的三字段 TLS 块（enabled / insecure-skip-verify / ca-file）。再设置会以未知
key 报错。mTLS 仍无法经 DSN 表达 —— 需要时用自定义 connector。

### 3.3 Discovery keys（来自 Common）

`service-name`（激活 resolver 拨号器）、`scheme`、`discovery`（backend 名，默认
"default"）—— 见 gormcore。⚠ 设了 `service-name` 时，`host`/`port` 保持可解析的哑值；
它们被忽略但仍嵌在 `msdsn.Parse` 要校验的 DSN 里。

---

## 4. 验证与故障演练

### 4.1 演练：TLS 握手失败（对明文/不受信服务器 encrypt）

1. 启动 compose 里的 SQL Server（其自签证书不被本机信任）。
2. 开启加密但不加信任：

```properties
spring.gorm.sqlserver.primary.tls.enabled=true
```

3. `go run .` → 启动 ping 在 `ping-timeout` 内报 TLS 信任错误
   （certificate signed by unknown authority）。
4. 补上信任（任一即可修复）：

```properties
spring.gorm.sqlserver.primary.tls.insecure-skip-verify=true   # 仅开发环境
# 或：tls.ca-file=/path/to/server-cert.pem
```

5. 再翻回 `tls.enabled=false` —— 明文 TDS 恢复。双向演练 encrypt 映射。

### 4.2 验证 discovery 寻址

example 的 `discovery` 实例用哑值 `0.0.0.0:0`；`Response from discovered server:` 打印
即证明实际拨号地址来自 backend。接入真实 backend 后扩容服务，可观察新的池连接落在
新实例上（每次拨号 `Pick()`）。

### 4.3 Resilience / 放火演练（example-load）

```bash
cd example-load && docker compose up -d
go run . -duration=10s                       # SELECT 1 基线吞吐
# 放火 —— 编辑 conf/app.properties（starter-governance 热加载）：
#   govern.fault.enabled=true  govern.fault.rate=0.5  govern.fault.error=generic
go run . -duration=10s                       # 错误分布显示 ~50% 注入
```

熔断（`govern.default.error-threshold=20`）触发同样体现在错误分布里；`WithContext`
透传 harness 截止时间，500ms 超时可打断在途查询。

### 4.4 可观测（example-otel）

运行 `example-otel`（Jaeger 经其 compose 启动）：每查询一个
`db.system=microsoft.sql_server` 的 span（含 SQL 语句）、:9090/metrics 的时长/in-flight
指标、按 `observability` 级别的访问日志。`slow-threshold=200ms` 另外换入 gorm 慢查询
logger（经 go-spring.org/log 转发，TagAppDef，消息体为纯文本）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动失败："one of host or service-name must be set" | 两者皆空 | 设置其一（build 守卫，starter.go:64-66）。 |
| 主机健康但启动 ping 超时 | 服务器仍在初始化（mssql 镜像慢） | 以 compose 健康检查为准，别看端口开没开。 |
| 启动期 TLS 信任错误 | `tls.enabled=true` 且无 `insecure-skip-verify`/`ca-file` | 补信任或关 encrypt（§4.1）。 |
| 期望 mTLS 但客户端从不出示证书 | `tls.cert-file`/`key-file` 已移除 | 配置表达不了；需自定义 connector。 |
| discovery 哑地址下证书主机名不匹配 | 无 `tls.server-name` key；DSN 里是哑值 `0.0.0.0` | 任选：证书用真实主机名、`insecure-skip-verify`（开发）、自定义拨号器。 |
| discovery 实例连不上 | backend 名不匹配（`discovery` key）或服务未注册 | 核对 `RegisterDiscovery` 名与配置；resolver 错误启动期有日志。 |
| 负载下登录超时 | `connectTimeout` 太小 | 调大，或保持 0 用驱动默认。 |
| `key`/`value` 列 SQL 报错 | SQL Server 保留字 | 用 gorm tag 重映射（`column:kkey`），照 example 做。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 方言特有 key | 连接 7 + tls 6 = 13（+10 Common，+1 wrapper） |
| 必填 | 3（`user`、`password`、`db`）+ host 或 service-name |
| quickstart 外部依赖 | 1（SQL Server，docker） |
| 死绑定 key | 0（多余的 tlsconf key 已于 2026-08 移除） |
| "注意/坑" 条数 | 5 |

设计嫌疑：discovery 接管寻址时仍要求可解析的哑 host/port 供 DSN 解析；没有通往基于
`*tls.Config` 的 connector 的路径（mTLS）。（共享块不匹配与亚秒截断已于 2026-08 修复。）
