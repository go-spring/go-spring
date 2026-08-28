# starter-influxdb 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health/health.go`）
与可运行的 [example/](example/) 核对——文中方括号为 file:line 抽查点。**InfluxDB 语义与
influxdb-client-go API 属于[客户端官方文档](https://docs.influxdata.com/influxdb/v2/api-guide/client-libraries/go/)**——
以下全部是 go-spring 的增量。

**激活条件**：出现任意 `spring.influxdb.*` key 即激活（模块是
`OnProperty("spring.influxdb")` 前缀检查 [starter.go:40]）。每个 `spring.influxdb.<name>`
条目创建一个名为 `<name>` 的 `*StarterInfluxdb.Client` bean，外加名为
`influxdb:<name>` 的健康指示器——两者均无关闭开关。

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
    go-spring.org/starter-governance latest   // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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
    // influxdb2.Client 接口，QueryAPI/WriteAPI/DeleteAPI/... 原样提升。
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

            // 2. 经内嵌 API 的 Flux 查询（传输层仍有观测——见 §2.3）。
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
spring.influxdb.a.server-url=http://127.0.0.1:8086
spring.influxdb.a.auth-token=go-spring-example-token
spring.influxdb.a.org=go-spring
spring.influxdb.a.bucket=example

# --- 实例 "b"：同 server 第二个客户端（本文作纯查询用）----------------------
spring.influxdb.b.server-url=http://127.0.0.1:8086
spring.influxdb.b.auth-token=go-spring-example-token

# --- 可观测：detailed 会在访问日志里附带请求 path ---------------------------
#（默认 brief；off 只关日志信号）
spring.influxdb.a.observability.level=detailed

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

go run .                          # server 不可达时启动快速失败（fail fast）
curl -s :9370/readyz | jq .      # components 含 influxdb:a 与 influxdb:b
curl -s :9370/metrics | grep -E 'db.client'   # duration 直方图 + active gauge
grep _app_influxdb_access app.log | tail -3   # 每个 HTTP 请求一条记录
docker exec influxdb-example influx query \
  'from(bucket:"example") |> range(start:-1m) |> filter(fn:(r) => r._measurement=="cpu")' \
  --org go-spring --token go-spring-example-token   # usage_idle=42.5
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-influxdb
  └─ gs.Module(OnProperty("spring.influxdb"))：出现任意 spring.influxdb.* key 即触发
        └─ conf.BindEach("${spring.influxdb}") → 每个 <name> 条目一个 Config
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)))
              │     .Name(<name>).Init((*Client).Init).Destroy((*Client).Destroy)
              └─ Provide health.Indicator，名为 "influxdb:<name>"
                    （经 TagArg 按名注入刚注册的 client，导出为 health.Indicator）

gs.Run()
  ├─ 构造 newClient [starter.go:61]：
  │    1. driver 查找——名字未知则启动失败
  │    2. driver.CreateClient → influxdb2 客户端，其 HTTP 请求经由
  │       dynamicTransport（Init 之前直通 http.DefaultTransport）
  │    3. 领取 DefaultDriver 安装的 dynamic transport [starter.go:77]
  │    4. fail-fast 探测：client.Health() → /health 必须报告 "pass"，
  │       否则关闭 client 并启动失败 [starter.go:80]
  ├─ gs 字段注入 Client.Observability（${observability:=}，顶层回退；
  │    实例前缀 spring.influxdb.<name>.observability.* 按字段覆盖——见
  │    Client.resolveObservability [client.go]）
  ├─ Init [client.go:76]：构建 Observer（NewDB "influxdb"）+ obsTransport；
  │    解析 executor = resilobserve.WrapExecutor(fault.WrapExecutor(
  │    resilience.ExecutorFor("influxdb:<server-url>")))；dyn.Swap 换入
  │    resilience round-tripper——观测+治理自此生效
  ├─ 就绪：指示器翻 UP（每次探测 = 一趟 /health 往返）
  └─ SIGTERM → Destroy [client.go:93]：Client.Close()——flush 异步 writer
       的残留批次——随后 exec.Close()
```

`driver` 配错、server 不可达或未初始化都会导致启动失败——进程不会带着一个死的
InfluxDB 进入"服务中"。注意 OSS server 在一次性 setup 完成前报告状态不同，因此
bootstrap 顺序竞争会在启动期暴露，而不是表现为间歇性写失败（DESIGN.md §3）。

### 2.2 请求链——精确顺序与理由

SDK 发出的每个 HTTP 请求都经过换入的 transport：

```
influxdb-client-go → resilience round-tripper（exec.Execute）→ obsTransport
（span + 指标 + 访问日志）→ http.DefaultTransport → 网络
```

设计理由（源码注释 [client.go:76-88]、[command.go:30-44]）：

- **resilience 最外层**：executor 许可（限流/熔断/注入故障，按资源键
  `influxdb:<server-url>` 圈定——每实例一个 scope，而非每操作）在观测与发送之前
  检查；它的拒绝正是外层观测随后记录的东西。
- **obsTransport 承载全部三个信号**：influxdb-client-go 自身不带 OTel 插桩，因此
  不同于把 trace/metric 交给 redisotel 的 starter-go-redis，这里由 transport 一并
  负责 span + 时长指标 + 访问日志，没有 WithoutTraceAndMetric 拆分。
- 操作名是 `"<METHOD> <path>"`，如 `POST /api/v2/write` [command.go:39]——HTTP 面
  上唯一稳定的逐请求词汇。
- HTTP 5xx 在 round-tripper 内被映射为可重试失败，重试时逐次回卷请求体
  （`GetBody`）；无可回卷 body 的请求只跑一次（resilience/roundtripper.go:83-95）。

### 2.3 一次写与一次查的逐层走读

**`WritePoints`（阻塞）** [client.go:106]：

1. org/bucket 守卫——配置为空时返回带指引的错误
   `influxdb: write helpers need org and bucket`，不碰网络。
2. 创建 `WriteAPIBlocking(org, bucket)`，整个写入在**第二层** executor 运行内
   （`o.exec.Execute`）——过载敏感路径上的逐调用保护。
3. SDK 发出 `POST /api/v2/write`；该请求再穿过传输层 executor 与 obsTransport，
   因此一次 WritePoints 两次跨越 executor（两层共用同一资源键，故限流/熔断状态
   共享——内层的拒绝也计入外层的视野）。

**`QueryAPI(org).QueryRaw`（内嵌 SDK 方法）**：不加逐调用守卫（DESIGN.md §4——
查询路径的 resilience 刻意留白；日后加 GuardedQuery 是增量不破坏）。但请求在传输层
仍被 executor 门控并被观测，因为所有请求都是。

**`ManagedWriteAPI`（异步）** [client.go:126]：后台批量，**不**经过任何 executor——
由 SDK 自己的批量重试掌控；逐点加守卫会重复计数（DESIGN.md §2）。失败批次落到
`Errors()` channel，由 wrapper 排干进 go-spring 日志（`influxdb: async write
failed: ...`）——不排干会在首次失败时阻塞 writer。Destroy 的 `Client.Close()` 会
flush 残留批次。

### 2.4 为什么需要 dynamicTransport

SDK 在构造期固定 `*http.Client`（`Options.SetHTTPClient`），而可观测策略要到构造
返回**之后**才被字段注入。因此 DefaultDriver 安装一个直通的 `dynamicTransport`
[driver.go:66-72]，由 Init 换入真正的链 [client.go:83-86]。构造与 Init 之间竞争的
请求直接走 http.DefaultTransport。不自装它的自定义 driver 得到的 client 没有观测/
resilience transport——该 client 的 resilience 即不可用 [starter.go:74-79]；逐调用的
`WritePoints` executor 仍然有效。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.influxdb.<name>.`——经 `conf.BindEach` 的按实例前缀绑定。
完整清单（已用 `grep -rhoE 'value:"[^"]+"' --include='*.go'` 双向核对）：

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `server-url` | string | — | InfluxDB 基础 URL；同时派生 resilience 资源键 `influxdb:<server-url>`。⚠ HTTPS 由 URL scheme 表达——没有 `tls.*` 块。 | 空 → BindEach 启动失败（`expr:"$ != ''"` [config.go:26]）；host 错 → fail-fast /health 探测启动失败。 |
| `auth-token` | string | — | 传给 SDK 的 API token。 | 空 → 启动报错；token 错 → 写/查逐请求失败（/health 探测不做鉴权，可能仍绿）。 |
| `org` | string | `""` | `WritePoints`/`ManagedWriteAPI` 与 `Org()` 的默认 org。⚠ **调用期**才需要，装配期不校验：不配 org/bucket 的 client 照样能服务 Query/Delete API。 | 缺失 → `WritePoints` 返回 error，`ManagedWriteAPI` **panic**（失败方式不一致——设计嫌疑）。 |
| `bucket` | string | `""` | 写助手的目标 bucket 默认值。⚠ 与 `org` 同一调用期规则。 | 同 `org`。 |
| `observability` | group | 见下 | 逐请求 transport（span + 指标 + 访问日志）的 observe kit 配置。实例级 key 按字段覆盖顶层 `observability.*`；顶层是向后兼容的回退面。 | — |
| `observability.level` | string | `brief` | `off` / `brief` / `detailed`（detailed 额外附请求 path 作为参数）。`off` 只关日志信号；trace/metric 照发。 | 拼错 → 视为非 off 非 detailed，即 brief 式日志，无任何告警。实例值等于绑定默认值（`brief`）时不算"已设置"，无法覆盖非默认顶层值。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式下参数捕获上限。 | 过小 → 日志参数被截断。 |
| `observability.skipOps` | list | — | 对列出的操作名一并压制 span+指标+日志——条目须精确匹配 `"POST /api/v2/write"` 样式。 | 不匹配 → 无效果（名字很容易写错；无告警）。 |
| `driver` | string | `DefaultDriver` | 选择已注册的 Driver。⚠ `RegisterDriver` 重名注册 panic [driver.go:48]。 | 名字未知 → 启动报错 `influxdb driver not found: <name>` [starter.go:67]。 |

没有 `tls.*` 组、没有 `service-name`/服务发现、没有超时 key——未列出的一切都是
SDK 自身默认；需要定制就走自定义 Driver。

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
（`influxdb: health status fail: <msg>`）[health/health.go:48-57]。

### 4.2 可观测性到底发什么

| 信号 | 名称 / 形态 |
|------|-------------|
| Span | 名 = 操作名，如 `POST /api/v2/write`；kind = client；属性 `db.system=influxdb`、`db.operation=<op>`、`db.<arg>=<path>`（detailed） |
| 指标 | `db.client.operation.duration`（直方图，s）与 `db.client.active_requests`（UpDownCounter）——与所有 DB 家族 starter 共用词汇 |
| 访问日志 | tag `_app_influxdb_access`，每请求一条：`system=influxdb op=<METHOD+path> status duration`（detailed 附 path） |
| 异步写失败 | 日志 tag `influxdb`（app tag），`influxdb: async write failed: <err>` [client.go:137] |

```bash
curl -s :9370/metrics | grep db.client
grep _app_influxdb_access app.log | tail -2
# 压掉健康探测自身的噪音（探测同样走 transport）：
#   spring.influxdb.a.observability.skipOps=GET /health
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

配置 starter-governance 后，资源 `influxdb:http://127.0.0.1:8086` 上的限流/熔断策略
对经 transport executor 的**每一个**请求生效（写、查、健康探测）。压测 `WritePoints`
并观察拒绝以 `_app_influxdb_access` 记录与 resilience observer 的 outcome 计数器浮出。
运行期翻策略——executor 热更新，无需重启。注入故障（`govern.fault.*`）也在同一
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
| 启动失败 `influxdb driver not found` | `driver` 指向未注册的名字 | 在 init 里 `StarterInfluxdb.RegisterDriver`，或删掉该 key。 |
| `panic: influxdb: write helpers need org and bucket` | org/bucket 为空时调 `ManagedWriteAPI` | 配 `spring.influxdb.<name>.org/.bucket`——或直接用内嵌 `WriteAPI(org, bucket)`。 |
| `WritePoints` 返回 org/bucket 错误 | 同一调用期缺口的不 panic 形态 | 同上。 |
| 写失败但启动与健康都是绿的 | `auth-token` 错——/health 不做鉴权 | 用 `influx query --token ...` 验 token。 |
| 请求在跑却没有 span/指标 | 未 import starter-otel | observe kit 挂在 OTel globals 上；import starter-otel（访问日志无 otel 也照发）。 |
| skipOps 似无效 | 条目与 `"METHOD /path"` 不精确匹配 | 按字面匹配如 `GET /health`。 |
| 写完立刻查询没有数据 | bucket 写路径落盘的短暂延迟 | 重试窗口——example 自身轮询至 15s [example/example.go:81-91]。 |
| 异步写无声消失 | 心智模型错位：`ManagedWriteAPI` 的失败是日志行，不是 error | grep `influxdb: async write failed`；需要错误就改用 `WritePoints`。 |

## 6. 设计体检

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 9（实例 6 + observability.level/.maxArgBytes/.skipOps） |
| 其中必填 | 装配期 2（`server-url`、`auth-token`）+ 调用期 2（`org`、`bucket`） |
| quickstart 前置外部依赖 | 1（InfluxDB 2.x） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（审计台账——含存量与新增）：

- `org`/`bucket` 只在调用期校验；`WritePoints` 报 error、`ManagedWriteAPI` **panic**——
  同一缺口两种失败方式。
- 健康指示器无关闭 key（redigo 有 `health.enabled`——家族不对称）；健康探测本身也走
  observe transport，除非 skipOps 过滤 `GET /health`，否则每次 readiness 检查多一条
  日志。
- `WritePoints` 之外的内嵌方法只有传输层治理、没有逐调用治理；`ManagedWriteAPI` 则
  完全没有——两档保护强度在调用点不可见。
- `WritePoints` 两次跨越 executor（逐调用 + 传输层）且共用一个资源键——熔断计数被
  放大，与 2026-08 修复的 http-client 同类 bug 同族。
- 无 `tls.*` / `service-name`，与兄弟 starter 不一致——HTTPS-only-via-scheme 面更小
  但属需文档化的不对称。
- `driver=DefaultDriver` 魔法串 + 重名注册 panic 的注册表，相对单一实现现实偏重。
