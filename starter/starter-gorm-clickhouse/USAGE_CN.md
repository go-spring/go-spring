# starter-gorm-clickhouse 使用说明（参考手册）

详细使用文档。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照源码
（`starter.go`、`config.go`、`starter_test.go`）与可运行示例
[example/](example/)、[example-load/](example-load/)、[example-otel/](example-otel/)
（docker 门控 `check.sh`）核实。**ClickHouse 连接语义以驱动为准** ——
[ClickHouse/clickhouse-go/v2](https://github.com/ClickHouse/clickhouse-go/v2)，经
[gorm.io/driver/clickhouse](https://github.com/go-gorm/clickhouse)；GORM 语义见
[gorm 文档](https://gorm.io/docs/)。共享的 wrapper 生命周期、连接池/observe/health
接线与 `UseDBCustomizer` 见 [gormcore](../starter-gorm/USAGE_CN.md)，此处不再重复。

**激活条件**：`spring.gorm.clickhouse` 下每个条目注册一个 `*starter.DB` bean（外加配对
的 `health.Indicator`）；没有任何条目时 starter 不注册任何东西
（`starter_test.go:TestClickhouseNotTriggered`）。

---

## 1. 完整工程示例

一个带直连实例和 discovery 实例的服务，对接单节点 ClickHouse。文件树（与
[example/](example/) 同构）：

```
demo/
├── go.mod
├── main.go
├── discovery.go
├── conf/
│   └── app.properties
├── docker-compose.yml   # 取自 example/docker-compose.yml（clickhouse-server:24）
└── check.sh             # 取自 example/check.sh
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-gorm-clickhouse latest
    gorm.io/gorm                       v1.31.x
)
```

**main.go**（节选自 `example/example.go`）—— 注意 ClickHouse 特有的建模：

```go
package main

import (
    "flag"
    "net/http"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    starter "go-spring.org/starter-gorm-clickhouse"
)

// ClickHouse 不像 OLTP 引擎那样强制唯一索引 —— 不用 `uniqueIndex`。
// AutoMigrate 必须显式给引擎。
type KV struct {
    ID    uint64 `gorm:"primaryKey"`
    Key   string `gorm:"column:kkey"`
    Value string `gorm:"column:vvalue"`
}
func (KV) TableName() string { return "kv" }

type Service struct {
    DB          *starter.DB `autowire:"primary"`
    DiscoveryDB *starter.DB `autowire:"discovery"`
}

var manual = flag.Bool("manual", false, "保持服务运行")

func main() {
    flag.Parse()
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    http.HandleFunc("/clickhouse_version", func(w http.ResponseWriter, r *http.Request) {
        s := svrBean.Interface().(*Service)
        var version string
        if err := s.DB.Raw("SELECT version()").Scan(&version).Error; err != nil {
            _, _ = w.Write([]byte(err.Error()))
            return
        }
        _, _ = w.Write([]byte(version))
    })
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(svrBean.Interface().(*Service))
        }()
    }
    gs.Run()
}

func runTest(s *Service) {
    // AutoMigrate 需要引擎：经 table_options 给 MergeTree + ORDER BY。
    _ = s.DB.Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)").AutoMigrate(&KV{})
    _ = s.DB.Create(&KV{ID: 1, Key: "key", Value: "value"}).Error
    // ClickHouse 无多语句事务 —— 用批量插入 + Count 代替 s.DB.Transaction(...)，
    // 与 example 的演示一致。
    batch := []KV{{ID: 2, Key: "k2", Value: "v2"}, {ID: 3, Key: "k3", Value: "v3"}}
    _ = s.DB.Create(&batch).Error
    var discVersion string
    _ = s.DiscoveryDB.Raw("SELECT version()").Scan(&discVersion).Error // 验证 discovery
    syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}
```

**discovery.go** —— 注册 `discovery` 实例所解析的 backend：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:9000", Healthy: true}))
}
```

**conf/app.properties**（复制自 `example/conf/app.properties`）：

```properties
spring.gorm.clickhouse.primary.user=default
spring.gorm.clickhouse.primary.password=
spring.gorm.clickhouse.primary.addr=127.0.0.1:9000        # native 协议端口
spring.gorm.clickhouse.primary.db=default
spring.gorm.clickhouse.primary.max-open-conns=10
spring.gorm.clickhouse.primary.max-idle-conns=5
spring.gorm.clickhouse.primary.conn-max-lifetime=30m
spring.gorm.clickhouse.primary.conn-max-idle-time=5m
spring.gorm.clickhouse.primary.ping-timeout=5s
spring.gorm.clickhouse.primary.slow-threshold=200ms
# TLS（此处关闭；冒烟服务器为明文）。开启安全 native 连接：
# spring.gorm.clickhouse.primary.tls.enabled=true
# spring.gorm.clickhouse.primary.tls.insecure-skip-verify=false
# spring.gorm.clickhouse.primary.tls.ca-file=/path/ca.pem
# spring.gorm.clickhouse.primary.tls.cert-file=/path/client-cert.pem   # 支持 mTLS
# spring.gorm.clickhouse.primary.tls.key-file=/path/client-key.pem

# discovery 实例：addr 故意设哑值 —— 因 service-name 生效而被忽略，
# 地址来自 discovery backend。
spring.gorm.clickhouse.discovery.user=default
spring.gorm.clickhouse.discovery.password=
spring.gorm.clickhouse.discovery.addr=0.0.0.0:0
spring.gorm.clickhouse.discovery.db=default
spring.gorm.clickhouse.discovery.service-name=clickhouse-cluster
```

**docker-compose.yml** —— 双端口 + 健康检查的 ClickHouse 24：

```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:24
    ports: ["127.0.0.1:9000:9000", "127.0.0.1:8123:8123"]
    ulimits: { nofile: { soft: 262144, hard: 262144 } }
    healthcheck:
      test: ["CMD-SHELL", "wget --spider -q localhost:8123/ping || exit 1"]
      interval: 3s
      retries: 60
```

**验证**：

```bash
docker compose up -d && wait-healthy     # 或直接 ./check.sh
go run .                                  # "Response from server:" + "Response from discovered server:"
go run . -manual & curl http://127.0.0.1:9090/clickhouse_version
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-gorm-clickhouse
  └─ init(): gormcore.Register(Dialect{Prefix: "spring.gorm.clickhouse",
        Engine: "clickhouse", HealthPrefix: "gorm:clickhouse:"})
gs.Run()
  ├─ conf.BindEach → 每条目一个 Config
  ├─ build()                                        — starter.go:61-131
  │    ├─ 守卫：addr 或 service-name 必须有一个（快速失败）
  │    ├─ useDiscovery = service-name != "" && !mesh.Enabled()
  │    ├─ useNative = useDiscovery || TLS.Enabled
  │    │    native: ch.Options{Addr, Auth, Dial/ReadTimeout}
  │    │      + opts.TLS = tlsconf.Build()          （TLS 开启时）
  │    │      + opts.DialContext = resolver 选点     （discovery 时）
  │    │      → clickhouse.New(Config{Conn: ch.OpenDB(opts)})
  │    └─ 直连路径：clickhouse.Open(DSN)              （无 TLS、无 discovery）
  ├─ gormcore.Open: gorm.Open → ApplyPool（启动 ping）→ DBCustomizers
  ├─ DB.Init(): observe 插件（db.system=clickhouse）+ resilience 回调
  │            （resource "gorm:clickhouse" + service-name/addr）
  └─ SIGTERM → DB.Destroy(): executor → closers（停 discovery watch）→ 关连接池
```

为什么有两条路径（源码注释，starter.go:73-76）：URL 风格 DSN 既表达不了自定义
`*tls.Config`，也表达不了 discovery 拨号器 —— 任一出现都强制走 native `ch.OpenDB`
构造；否则保持原来的 DSN 路径。

### 2.2 Discovery 拨号路径 —— 与哑地址要求

native 驱动的 `ch.Options.DialContext` 是两参 `func(ctx, addr string)` —— starter 的
实现**忽略 addr 参数**，改拨 `resolver.Pick()` 返回的 `ep.Addr`
（starter.go:107-113），因此池中每个新连接都落在当前存活的实例上；地址变化无需
重建客户端即生效。设置 `service-name` 后，`addr` 永远不会被拨号 —— 但 `build` 仍要求
`addr` **或** `service-name` 非空，且 native 路径上会先填 `opts.Addr =
[]string{c.Addr}` 再被 `DialContext` 覆盖：保持一个可解析的哑值（example 用
`0.0.0.0:0`）以证明寻址归 discovery 管。mesh 模式下直接使用配置的 Addr（sidecar
负责 discovery+LB），但 TLS 仍可能强制走 native 路径。

### 2.3 一次查询的逐层走读

`db.WithContext(ctx).Raw("SELECT version()").Scan(&v)`：

1. `gorm:raw` processor —— 已被 gormcore 的 executor 包装替换（`${govern}` 的
   timeout/retry/breaker，放火时含 fault 注入器；`gorm.ErrRecordNotFound` 视为成功）。
2. observe 插件 span（db.system=clickhouse）+ in-flight 指标。
3. 原 processor：池取连接 → native DialContext（discovery 重新选点）→ native 协议
   握手（设了 `opts.TLS` 则加密）→ 查询。
4. `after_*` 写入 SQL、结束 span/指标、写访问日志。

---

## 3. 逐 key 行为参考

key 位于 `spring.gorm.clickhouse.<name>.*`。Common keys（10 个）与 wrapper 级
`observability` 见 [gormcore](../starter-gorm/USAGE_CN.md#2-配置参考)。

### 3.1 连接 key（config.go:31-45）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `addr` | string | "" | **native** 协议的 host:port（通常 9000 —— 不是 HTTP 的 8123）。⚠ `addr`/`service-name` 二选一强制（build 守卫）。 | 填了 HTTP 端口 → 握手乱码 / 启动 ping 失败。两者皆空 → "one of addr or service-name must be set"。 |
| `user` | string | "default" | DSN userinfo / `ch.Auth.Username`。 | 用户错 → 启动 ping 期认证错误。 |
| `password` | string | "" | DSN userinfo / `ch.Auth.Password`。 | — |
| `db` | string | "default" | DSN path / `ch.Auth.Database`。 | 库不存在 → 启动失败。 |
| `dialTimeout` | duration | 0 | DSN `dial_timeout=2s`（Go duration 字符串，不截断）/ `ch.Options.DialTimeout`。 | 太小 → 冷集群拨号超时。 |
| `readTimeout` | duration | 0 | DSN `read_timeout=30s` / `ch.Options.ReadTimeout`。⚠ 长分析查询需要足够大或不设。 | 太小 → 大 SELECT 中途中断。 |

### 3.2 TLS 块（`tls.*`，共享 tlsconf.TLSConfig 绑定于 config.go:45）

与 sqlserver（DSN 参数）不同，ClickHouse 的 TLS 是真正的 `*tls.Config`：开启后
**把 starter 切到 native 驱动路径**（`useNative`，starter.go:78），设置
`opts.TLS = c.TLS.Build()` —— **从不发出 `secure_connection` / `https` DSN 参数**。
此处六个 key 全部有效：

| Key | 默认值 | 行为 |
|-----|--------|------|
| `tls.enabled` | false | 开 → native 路径 + `opts.TLS`。不可读的 `ca-file` 在任何拨号*之前*就让 build 失败（starter_test.go 钉死）。 |
| `tls.ca-file` | "" | 验证服务端的根 CA 包（`RootCAs`）。 |
| `tls.cert-file` + `tls.key-file` | "" | 客户端密钥对 —— 本方言**可表达 mTLS**（tlsconf `Build()` 两者都加载）。 |
| `tls.server-name` | "" | 覆盖对服务器证书校验的名字 —— 按 IP 拨号时有用。 |
| `tls.insecure-skip-verify` | false | 仅限开发的逃生门。 |

### 3.3 Discovery keys（来自 Common）

`service-name`（激活 resolver 拨号器）、`scheme`、`discovery`（backend 名，默认
"default"）。⚠ 设了 `service-name` 后 `addr` 变哑值 —— 仍需可解析以填充 `opts.Addr`。

### 3.4 方言陷阱（ClickHouse 是 OLAP）

- 无多语句事务 —— `db.Transaction(...)` 不是正确的工具；批量写入 + 引擎自身的异步
  语义才是（见 [文档](https://clickhouse.com/docs)）。
- 不强制唯一索引；`uniqueIndex` tag 只是装饰（example 特意省略）。
- `AutoMigrate` 需要引擎：传 `gorm:table_options`（如
  `ENGINE=MergeTree ORDER BY (id)`），照 example 做。

---

## 4. 验证与故障演练

### 4.1 演练：TLS 开/关

1. 基线：按 §1 原样运行（明文 9000）—— 往返 OK。
2. 未开 TLS 的服务器上开 TLS：

```properties
spring.gorm.clickhouse.primary.tls.enabled=true
```

3. `go run .` → native 路径握手挂起/失败，**启动 ping 在 `ping-timeout` 内失败** ——
   证明 TLS 确实生效（走了 native 路径），而不是静默空转。
4. 对接开了 TLS 的服务器（`clickhouse/clickhouse-server:24` 配 TLS，端口 9440）：
   设 `addr=<host>:9440`、`tls.enabled=true`，加 `tls.ca-file`/`tls.server-name`；
   往返恢复。再加 `cert-file`+`key-file` 即要求 mTLS（本方言六个 key 全活）。
5. 翻回 `tls.enabled=false` + 原地址 —— 明文 DSN 路径重连（顺带演练了 §2.1 的双路径
   切换）。

### 4.2 验证 discovery 寻址

example 的 `discovery` 实例用哑值 `0.0.0.0:0`；`Response from discovered server:`
打印即证明地址来自 backend。扩容 backend 端点后，可观察新的池连接落在它上面
（每次拨号 `Pick()`）。

### 4.3 Resilience / 放火演练（example-load）

```bash
cd example-load && docker compose up -d
go run . -duration=10s                        # SELECT 1 基线
# 放火（starter-governance 热加载）：
#   govern.fault.enabled=true  govern.fault.rate=0.5  govern.fault.error=generic
go run . -duration=10s                        # 错误分布显示 ~50% 注入
```

操作故意保持纯读往返 —— ClickHouse 的列存 schema 模型让逐次迭代的插入/迁移很脆弱
（example-load 注释）。

### 4.4 可观测（example-otel）

运行 `example-otel`（Jaeger 经 compose 启动）：每查询一个 `db.system=clickhouse` 的
span（含 SQL 语句）、:9090/metrics 的 prometheus 指标、按 `observability` 级别的访问
日志。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动失败："one of addr or service-name must be set" | 两者皆空 | 设置其一（build 守卫，starter.go:62-64）。 |
| 握手乱码 / ping 立即失败 | `addr` 指向 HTTP 端口 8123 而非 native 9000 | 用 native 端口。 |
| 开 TLS 后启动 ping 失败 | 该端口的服务器未开 TLS | §4.1：换 9440 + tls keys，或关闭。 |
| 大分析查询中途中断 | `readTimeout` 太小 | 调大或保持 0。 |
| `AutoMigrate` 失败（缺 ENGINE） | ClickHouse 要求表引擎 | `Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)")`。 |
| 事务代码报错 / 无效果 | ClickHouse 无多语句事务 | 批量写入；见 §3.4。 |
| 有 uniqueIndex tag 仍出现重复行 | ClickHouse 不强制唯一索引 | 在写入/模型层去重。 |
| discovery 实例连不上 | backend 名不匹配 / 服务未注册 | 核对 `RegisterDiscovery` 名与 `discovery` key。 |
| TLS 通但证书主机名不匹配 | 按 IP 拨号 | 设 `tls.server-name`（本方言有效）。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 方言特有 key | 连接 6 + tls 6 = 12（+10 Common，+1 wrapper） |
| 必填 | 0 硬性（addr 或 service-name 由 build 守卫） |
| quickstart 外部依赖 | 1（ClickHouse，docker） |
| 死绑定 key | 0 |
| "注意/坑" 条数 | 5 |

设计嫌疑：DSN 与 native 双构造路径使 TLS/discovery 行为与纯 DSN 方言分叉 —— 在配置
面上不可见；discovery 路径上哑 `addr` 仍填充 `opts.Addr`；同一 `readTimeout` key 在
两条路径上语义不同（DSN 字符串 vs `ch.Options`）。
