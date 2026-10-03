# starter-influxdb 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`observe.go`、`driver.go`、`health.go`）
与可运行的 [example/](example/) 核对——文中方括号为 file:line 抽查点。**InfluxDB 语义与
influxdb-client-go API 属于[客户端官方文档](https://docs.influxdata.com/influxdb/v2/api-guide/client-libraries/go/)**——
以下全部是 go-spring 的增量。

**激活条件**：出现任意 `spring.influxdb.instances.*` key 即激活（模块是
`OnProperty("spring.influxdb")` 前缀检查 [starter.go:41]）。每个 `spring.influxdb.instances.<name>`
条目创建一个名为 `<name>` 的 `*StarterInfluxdb.Client` bean，外加名为
`influxdb:<name>` 的健康指示器（`health=false` 可跳过）；启动探活为 opt-in（`ping=true`）。

---

## 1. 完整工程示例

单 server 双实例（example 自身的拓扑），真实写入 + Flux 查询，组合 actuator/otel。
文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/influxdata/influxdb-client-go/v2 v2.14.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-influxdb   latest
    go-spring.org/starter-actuator   latest   // 可选：readiness + /metrics
    go-spring.org/starter-otel       latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-influxdb"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入 wrapper，走受保护路径写入、Flux 查回读，并演示托管异步 writer：

```go
package service

import (
    "context"

    influxdb2 "github.com/influxdata/influxdb-client-go/v2"
    "go-spring.org/spring/gs"
    StarterInfluxdb "go-spring.org/starter-influxdb"
)

type Service struct {
    // 始终注入 wrapper 类型 *StarterInfluxdb.Client。它内嵌
    // influxdb2.Client 接口，QueryAPI/WriteAPI/DeleteAPI/... 被原样提升。
    Main *StarterInfluxdb.Client `autowire:"a"` // 配了 org+bucket
    Raw  *StarterInfluxdb.Client `autowire:"b"` // 只配 url+token（纯查询用）
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 1. 阻塞写——resilience 保护，写向配置的 org/bucket。
            p := influxdb2.NewPointWithMeasurement("cpu").
                AddTag("host", "server-01").
                AddField("usage_idle", 42.5)
            if err := s.Main.WritePoints(ctx, p); err != nil {
                panic(err)
            }

            // 2. 经内嵌的 API 做 Flux 查询（传输层仍被声明并被 executor 门控——见 §2.3）。
            raw, err := s.Main.QueryAPI(s.Main.Org()).QueryRaw(ctx,
                `from(bucket:"example") |> range(start: -1m) |> filter(fn: (r) => r._measurement == "cpu")`,
                influxdb2.DefaultDialect())
            _ = raw // CSV；包含 usage_idle,42.5

            // 3. 异步批量写——错误排干进 go-spring 日志。
            w := s.Main.ManagedWriteAPI()
            w.WritePoint(ctx, influxdb2.NewPointWithMeasurement("mem").
                AddField("used_percent", 61.0))
            _ = err
        }
    })
}
```

**conf/app.properties** —— 上述用到的完整配置面：

```properties
# --- 实例 "a"：写+查客户端 --------------------------------------------------
spring.influxdb.instances.a.server-url=http://127.0.0.1:8086
spring.influxdb.instances.a.auth-token=go-spring-example-token
spring.influxdb.instances.a.org=go-spring
spring.influxdb.instances.a.bucket=example

# --- 实例 "b"：同 server 第二个客户端（本文作纯查询用）----------------------
spring.influxdb.instances.b.server-url=http://127.0.0.1:8086
spring.influxdb.instances.b.auth-token=go-spring-example-token

# --- actuator + otel --------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**验证**（先起 InfluxDB——直接用 example 的自初始化 compose 文件）：

```bash
cd starter-influxdb/example && docker compose -p demo up -d
# 等一次性 setup（admin/org/bucket/token）完成并报告 pass：
curl -fsS http://127.0.0.1:8086/health | grep '"pass"'

go run .                          # server 不可达时启动快速失败（fail fast；需 ping=true，默认关闭）
curl -s :9370/readyz | jq .      # components 含 influxdb:a 与 influxdb:b
curl -s :9370/metrics | grep -E 'db.client'   # 调用级+尝试级 duration 直方图 + active gauge
grep _app_influxdb_access app.log | tail -3   # 每次调用一条访问记录
docker exec influxdb-example influx query \
  'from(bucket:"example") |> range(start:-1m) |> filter(fn:(r) => r._measurement=="cpu")' \
  --org go-spring --token go-spring-example-token   # usage_idle=42.5
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-influxdb
  └─ gs.Module(OnProperty("spring.influxdb"))：出现任意 spring.influxdb.instances.* key 即触发
        └─ conf.BindEach("${spring.influxdb}") → 每个 <name> 条目一个 Config
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │         IndexArg(2, ?Driver))    // 可选 Driver bean；多个并存时
              │                                    // 由 ${..driver} 按名指定
              │     .Name(<name>).Destroy((*Client).Destroy)
              └─ Provide health.Indicator，名为 "influxdb:<name>"
                    （经 TagArg 按名注入刚注册的 client，导出为 health.Indicator）

gs.Run()
  ├─ 构造 newClient [starter.go:79]：
  │    1. 可选 Driver bean——没有则回退内置 DefaultDriver
  │       （d == nil [starter.go:83-85]）；容器中存在多个 Driver bean 时，
  │       实例可按名指定：`spring.influxdb.instances.<name>.driver = <bean 名>`
  │       （留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的 bean 不存在则启动失败）
  │    2. 构造器把治理 bean 打包成 cloud.ClientParams{Resilience: mgr, Fault: inj}，
  │       交给 driver.CreateClient [driver.go:67]
  │    3. driver.CreateClient → NewClient [client.go:102]：由治理包构建 executor
  │       exec = params.ExecutorFor("influxdb", "influxdb:<server-url>")，再在裸
  │       client 所乘的 dynamicTransport 上换入链——dyn.Swap(declareTransport{base:
  │       resilience.NewRoundTripper(http.DefaultTransport, exec)})——因此 driver
  │       一返回，声明+治理链即已生效
  │    4. 仅 ping=true：fail-fast 探测 HealthCheck(ctx, w) → /health 必须报告 "pass"，
  │       否则刚装配好的 client 被拆解（executor + 连接）并启动失败 [starter.go:99-102]
  ├─ 就绪：指示器翻 UP（每次探测 = 一趟 /health 往返）
  └─ SIGTERM → Destroy [client.go:119]：Client.Close()——flush 异步 writer
       的残留批次——随后 exec.Close()
```

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:52`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterInfluxdb.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.influxdb` 下每个实例都经它构建。
driver 随配置一并收到 `cloud.ClientParams` 包，返回模块的 `*Client`——它构建裸 client 并
连同 `params` 一起交给 `NewClient`，由后者在构建期施加治理；不安装 `dynamicTransport`
的自定义 driver 得到的 client 没有 transport 层声明/治理。
没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`（`driver.go:67`，`starter.go:83-85`）。
没有 per-config 的 `driver` key。

server 不可达或未初始化会导致启动失败——进程不会带着一个死的
InfluxDB 进入"服务中"。注意 OSS server 在一次性 setup 完成前报告状态不同，因此
bootstrap 顺序竞争会在启动期暴露，而不是表现为间歇性写失败（见 README 的设计说明）。

### 2.2 请求链——精确顺序与理由

SDK 发出的每个 HTTP 请求都经过换入的 transport：

```
influxdb-client-go → declareTransport（把 Operation 声明到 ctx）→
resilience round-tripper（exec.Execute：开调用 span、记调用级/尝试级时长指标 + 访问日志）
→ http.DefaultTransport → 网络
```

设计理由（源码注释 [client.go:102-113]、[observe.go]）：

- **声明层最外层、resilience 在其内**：`declareTransport` 把请求的 Operation 放上
  ctx 并转发给 resilience round-tripper，后者在 `Execute` 入口读取它。声明**必须**位于
  executor **之外**——若像 round-tripper 的 `base` 那样逐次尝试才声明，就无人读取：发射点
  只在调用入口读一次 Operation。
- **resilience 发射、starter 只声明**：executor 许可（限流/熔断/注入故障，按服务键
  `influxdb:<server-url>` 圈定——每实例一个 scope，而非每操作）最先检查；随后由 resilience
  层统管 span + 时长指标 + 访问日志。influxdb-client-go 自身不带 OTel 插桩，但这不再是本
  starter 的事：它不发射任何信号。
- span 名是 `"<METHOD> <path>"`，如 `POST /api/v2/write`；而指标标签是有界的：
  `db.operation=<method>`（`post`/`get`/`delete`……）；URL path——可能带 org/bucket/
  measurement——作 `db.statement` 只进 span 与日志，永不进标签 [observe.go]。
- HTTP 5xx 在 round-tripper 内被映射为可重试失败，重试时逐次回卷请求体
  （`GetBody`）；无可回卷 body 的请求只跑一次（resilience/roundtripper.go:83-95）。

### 2.3 一次写与一次查的逐层走读

**`WritePoints`（阻塞）** [client.go:132]：

1. org/bucket 守卫——配置为空时返回带指引的错误
   `influxdb: write helpers need org and bucket`，不碰网络。
2. 创建 `WriteAPIBlocking(org, bucket)`，整个写入在**第二层** executor 运行内
   （`o.exec.Execute`）——过载敏感路径上的逐调用保护。
3. SDK 发出 `POST /api/v2/write`；该请求再穿过 declareTransport 与传输层 executor，
   因此一次 WritePoints 两次跨越 executor（两层共用同一服务键，故限流/熔断状态
   共享——内层的拒绝也计入外层的视野）。

**`QueryAPI(org).QueryRaw`（内嵌的 SDK 方法）**：不加逐调用守卫（查询路径的 resilience
刻意留白——见 README 的设计说明；日后加 GuardedQuery 是增量不破坏）。但请求在传输层
仍被声明并被 executor 门控，因为所有请求都是。

**`ManagedWriteAPI`（异步）** [client.go:152]：后台批量，**不**经过任何 executor——
由 SDK 自己的批量重试掌控；逐点加守卫会重复计数（见 README 的设计说明）。失败批次落到
`Errors()` channel，由 wrapper 排干进 go-spring 日志（`influxdb: async write
failed: ...`）——不排干会在首次失败时阻塞 writer。Destroy 的 `Client.Close()` 会
flush 残留批次。

### 2.4 为什么需要 dynamicTransport

SDK 在构造期固定 `*http.Client`（`Options.SetHTTPClient`），而声明/治理链要用 driver
不该依赖的 bean 构建。因此 DefaultDriver 安装一个直通的 `dynamicTransport`
[driver.go:68-72] 并交给 `NewClient`，而 driver 调用 `NewClient` 时一并传入
`cloud.ClientParams` 包；`NewClient` 在构建 client 的同时换入真正的链
[client.go:102-113]——因此构造与治理之间没有空窗：driver 一返回链即已生效。不传入该
transport 的自定义 driver 得到的 client 没有传输层声明/治理——该 client 的 resilience
即不可用 [client.go:102-113]；逐调用的 `WritePoints` executor 仍然有效。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.influxdb.instances.<name>.`——经 `conf.BindEach` 的按实例前缀绑定。
完整清单（已用 `grep -rhoE 'value:"[^"]+"' --include='*.go'` 双向核对）：

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `server-url` | string | — | InfluxDB 基础 URL；同时派生 resilience 服务键 `influxdb:<server-url>`。⚠ HTTPS 由 URL scheme 表达——没有 `tls.*` 块。 | 空 → BindEach 启动失败（`expr:"$ != ''"` [config.go:26]）；host 错 → 首次使用时（`ping=true` 时为 fail-fast /health 探测）启动失败。 |
| `auth-token` | string | — | 传给 SDK 的 API token。 | 空 → 启动报错；token 错 → 写/查逐请求失败（/health 探测不做鉴权，可能仍绿）。 |
| `org` | string | `""` | `WritePoints`/`ManagedWriteAPI` 与 `Org()` 的默认 org。⚠ **调用期**才需要，装配期不校验：不配 org/bucket 的 client 照样能服务 Query/Delete API。 | 缺失 → `WritePoints` 返回 error，`ManagedWriteAPI` **panic**（失败方式不一致——设计嫌疑）。 |
| `bucket` | string | `""` | 写助手的目标 bucket 默认值。⚠ 与 `org` 同一调用期规则。 | 同 `org`。 |
| `ping` | bool | `false` | 启动连通探活：true 时构造期跑一次 `HealthCheck`（一趟 `/health` 往返），出错则启动失败，恢复 fail-fast。默认关闭，使尚未就绪的 server 不阻塞启动。 | `ping=true` 且 server 已挂 → 启动报 `failed to reach influxdb server …`。 |
| `health` | bool | `true` | 本实例是否贡献 `health.Indicator`（名 `influxdb:<name>`）供 actuator 就绪/启动探测。置 false 可把该实例排除在聚合健康报告之外。 | `health=false` → `/readyz` 无 `influxdb:<name>` 组件。 |

没有 `driver` key：client 装配由可选 `Driver` bean（见 §2.1）或内置 `DefaultDriver` 负责。
没有 `tls.*` 组、没有 `service-name`/服务发现、没有超时 key——未列出的一切都是
SDK 自身默认。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .      # components：influxdb:a、influxdb:b
docker stop influxdb-example     # 探测 = 每次检查一趟 /health 往返
curl -s :9370/readyz             # 503 OUT_OF_SERVICE，message 来自 server
docker start influxdb-example
```

探测只把 `status=pass` 映射为健康；`fail` 会携带 server 消息
（`influxdb: health status fail: <msg>`）[health.go:49-57]。

### 4.2 可观测性到底发什么

starter 只**声明**每个请求的身份；**resilience 层负责发射**（executor 链上唯一的发射点，
因此重试被并入一次调用）。你能看到：

| 信号 | 名称 / 形态 |
|------|-------------|
| Span | 名 = `"<METHOD> <path>"`，如 `POST /api/v2/write`；kind = internal（发射点的调用 span）；属性 `db.system=influxdb`、`db.operation=<method>`、`db.statement=<path>`（path 截断至 512 字节） |
| 指标（调用级） | `db.client.operation.duration`（直方图，s）与 `db.client.active_requests`（UpDownCounter）——与所有 DB 家族 starter 共用词汇；标签 `db.system`、`db.operation`、`status`（path 永不进标签） |
| 指标（尝试级） | `db.client.attempt.duration`（直方图，s）——每次下游尝试一条；被拒绝的调用（限流/熔断）不记录，因为下游根本没被触碰 |
| 访问日志 | tag `_app_influxdb_access`，每次调用一条，走 log 包原生分级：错误 → Warn；成功且带请求 path → Debug（惰性）；普通成功 → Info |
| 异步写失败 | 日志 tag `influxdb`（app tag），`influxdb: async write failed: <err>` [client.go:172]——异步批次不经过 executor，故这一行是发射点无法为其产出的唯一失败信号 |

```bash
curl -s :9370/metrics | grep db.client
grep _app_influxdb_access app.log | tail -2
```

### 4.3 server 宕机演练

```bash
docker stop influxdb-example
go run .   # 启动失败："failed to reach influxdb server http://..."——探测
           # 要求 pass，而不只是"可连通"
```

server 在启动**之后**挂掉：readiness 翻 DOWN（§4.1）；阻塞写返回 executor/transport
错误；若 governance 策略圈住了 `influxdb:<server-url>`，熔断达阈值后开启、不再拨号
即拒绝。

### 4.4 治理演练

配置治理规则源后，服务 `influxdb:http://127.0.0.1:8086` 上的限流/熔断策略
对经 transport executor 的**每一个**请求生效（写、查、健康探测）。压测 `WritePoints`
并观察拒绝以 `_app_influxdb_access` 记录与 resilience observer 的 outcome 计数器浮出。
运行期翻策略——executor 热更新，无需重启。注入故障（`spring.governance.client.fault.*`）也在同一
seam 生效。

### 4.5 异步 writer 排干

停容器、经 `ManagedWriteAPI` 写、再重启——SDK 批量重试耗尽后失败以
`influxdb: async write failed` 日志行出现，进程照常运行（异步失败永远不会变成调用方
error）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 `failed to reach influxdb server ...` | server 未起、server-url 错、或 OSS 首次启动 setup 未完成 | 等 `curl :8086/health` 报 `pass`；核对 URL scheme/host。 |
| `panic: influxdb: write helpers need org and bucket` | org/bucket 为空时调 `ManagedWriteAPI` | 配 `spring.influxdb.instances.<name>.org/.bucket`——或直接用内嵌的 `WriteAPI(org, bucket)`。 |
| `WritePoints` 返回 org/bucket 错误 | 同一调用期缺口的不 panic 形态 | 同上。 |
| 写失败但启动与健康都是绿的 | `auth-token` 错——/health 不做鉴权 | 用 `influx query --token ...` 验 token。 |
| 请求在跑却没有 span/指标 | 未 import starter-otel | 发射点挂在 OTel globals 上；import starter-otel（访问日志无 otel 也照发）。 |
| 写完立刻查询没有数据 | bucket 写路径落盘的短暂延迟 | 重试窗口——example 自身轮询至 15s [example/main.go:81-91]。 |
| 异步写无声消失 | 心智模型错位：`ManagedWriteAPI` 的失败是日志行，不是 error | grep `influxdb: async write failed`；需要错误就改用 `WritePoints`。 |

## 6. 设计体检

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 实例 6 个 |
| 其中必填 | 装配期 2（`server-url`、`auth-token`）+ 调用期 2（`org`、`bucket`） |
| quickstart 前置外部依赖 | 1（InfluxDB 2.x） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（审计台账——含存量与新增）：

- `org`/`bucket` 只在调用期校验；`WritePoints` 报 error、`ManagedWriteAPI` **panic**——
  同一缺口两种失败方式。
- 健康指示器每实例默认注册（`health=false` 可关）；启动探活为 opt-in（`ping=true`）——即
  redigo 拆成 `health.enabled`/`startup-ping` 的那两个旋钮。健康探测本身也走
  声明 transport，每次 readiness 检查多一条访问日志。
- `WritePoints` 之外的内嵌方法只有传输层治理、没有逐调用治理；`ManagedWriteAPI` 则
  完全没有——两档保护强度在调用点不可见。
- `WritePoints` 两次跨越 executor（逐调用 + 传输层）且共用一个服务键——熔断计数被
  放大，与 2026-08 修复的 http-client 同类 bug 同族。
- 无 `tls.*` / `service-name`，与兄弟 starter 不一致——HTTPS-only-via-scheme 面更小
  但属需文档化的不对称。
