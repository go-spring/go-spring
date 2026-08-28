# starter-otel 使用说明 — 参考手册

详细使用文档,概览见 [README.md](README.md)。全部行为声明已对照源码核实(`starter.go`、
`config.go`、`exporters.go`、`trace/`、`metric/`),并锚定可运行的 [example/](example/)
(`example/check.sh` 已验证通过)。**OTel 概念(span、tracer/meter provider、exporter、采样、
[W3C trace context](https://www.w3.org/TR/trace-context/))是 OpenTelemetry 自己的**,见
[OTel Go 文档](https://opentelemetry.io/docs/languages/go/)与
[Collector 文档](https://opentelemetry.io/docs/collector/)。本文只写 go-spring 的增量:
一个扁平配置前缀、进程级 provider 安装、actuator 接缝、停机 flush。

**激活方式**:引入 starter 即激活 OTel SDK——没有"是否引入"的开关 key;import 之后的总开关是
`spring.observability.enable`(默认 `true`)。provider **不是 bean**:在 module setup 阶段、任何
bean 构造之前安装为 OTel 进程全局对象(见 §2.1)。

---

## 1. 完整工程示例

一个真实服务:starter-echo 提供 HTTP,actuator 提供探针与 Prometheus 指标挂载,trace 走 OTLP
发给 collector。文件树:

```
demo/
├── go.mod
├── main.go
├── loghook.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
require (
    github.com/labstack/echo/v4    latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-echo     latest
    go-spring.org/starter-actuator latest   // 探针 + /metrics 挂载
    go-spring.org/starter-otel     latest
    go.opentelemetry.io/otel       v1.45.x  // 仅当直接触碰 OTel API(loghook.go)
)
```

**main.go**:

```go
package main

import (
    _ "demo/loghook"
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**loghook.go** —— trace↔log 关联刻意由应用自己接(starter-otel 不随包提供 installer,见 §6):

```go
package loghook

import (
    "context"

    "go-spring.org/log"
    "go.opentelemetry.io/otel/trace"
)

func init() {
    // log.FieldsFromContext 是 go-spring log 每条记录都会调用的唯一钩子;
    // 从 context 抬出 trace_id/span_id 即可让日志与 trace 关联。
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        sc := trace.SpanContextFromContext(ctx)
        if !sc.IsValid() {
            return nil
        }
        return []log.Field{
            log.String("trace_id", sc.TraceID().String()),
            log.String("span_id", sc.SpanID().String()),
        }
    }
}
```

**router.go**:

```go
package router

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"

    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.GET("/hello/:name", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"msg": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties** —— 完整、带注释的可观测配置面:

```properties
# --- echo server -------------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator(探针 + 指标挂载)-----------------------------------------------
spring.actuator.addr=:9370

# --- observability(starter-otel)---------------------------------------------
spring.observability.enable=true
# service-name 默认取 ${spring.application.name},再回落 go-spring-app——两者都
# 不设的话,后端会把所有"默认名"服务的流量混在一起,务必设其一。
spring.observability.service-name=demo

# trace:批量推送,明文 gRPC 发给本地 OTLP collector。w3c 传播器
# (TraceContext + Baggage)安装为进程全局,所有被埋点的出站请求自动带 traceparent。
spring.observability.trace.enable=true
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.trace.propagator=w3c

# 指标:pull 型 Prometheus。port=0 关掉独立抓取 server,/metrics 只由
# actuator 管理端口服务——探针与指标共用一个端口(example 验证的整合形态)。
spring.observability.metrics.enable=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics

# Go 运行时指标(GC、heap、goroutine、GOMAXPROCS)进同一 MeterProvider,
# 与上面的 echo HTTP 指标一起被抓取。
spring.observability.metrics.runtime.enable=true
```

**验证**:

```bash
curl -i :8002/hello/world                                  # 200
curl -s :9370/metrics | grep -E 'go_goroutine_count|http_server_request_duration'
curl -i :9370/health                                       # 探针与 /metrics 同端口存活
```

注意:只 import 不配置,starter 并非惰性——默认 trace/metrics 都是 `otlp-grpc` + 空 endpoint,
会静默瞄准 `localhost:4317`——好在 2026-08 起连通性探针会在拨不通时 WARN 一次(见 §5)。
每个环境仍要显式配置或显式关闭。

---

## 2. 装配与时序

### 2.1 bean 生命周期时间线——以及 setup 为何先于 bean 构造

```
import starter-otel
  └─ init():blank-import 注册 exporter(exporters.go:27-33)
  └─ 注册 gs.Module(nil, setup)                            [starter.go:62]
gs.Run()
  ├─ 配置加载 + .env(pre-config)
  ├─ RefreshPrepare → applyModules → setup()                [starter.go:69-93]
  │     ├─ 绑定 ${spring.observability} → Config
  │     ├─ enable=false → 打日志直接返回(全局对象保持 OTel no-op) [starter.go:74-77]
  │     ├─ trace.NewResource(service-name)                  [trace/provider.go:32-36]
  │     ├─ setupTrace:TracerProvider + 传播器 → otel.Set*Globals
  │     │     └─ gs.RegisterStopper("otel-trace", tp.Shutdown)        [starter.go:121]
  │     └─ setupMetrics:MeterProvider → otel.SetMeterProvider
  │        ├─ runtime 指标(进程内仅一次)                              [starter.go:184-189]
  │        ├─ prometheus:RegisterStopper("otel-metrics-scrape-server", ...)
  │        └─ Provide(metric.NewEndpoint).Export(As[endpoint.Endpoint])
  ├─ bean 构造(gorm client、echo engine、http transport……)
  │     —— 任何读 otel.GetTracerProvider()/GetMeterProvider() 的组件
  │        看到的都是已生效的全局对象
  ├─ 各 server Run → 就绪
  └─ SIGTERM:server 停止 → 容器关闭 → runStoppers flush       [gs/stopper.go:86-100]
```

这个顺序是承重的,源码注释说明了原因(starter.go:56-61):

> This must be a `gs.Module`, not a plain bean: its body executes during applyModules in the
> RefreshPrepare phase, i.e. BEFORE any bean is instantiated. Setting the OTel globals here
> therefore guarantees they are live before component beans (e.g. a gorm client calling `db.Use`)
> are constructed. Building the providers lazily inside a bean constructor would break that
> ordering.

对用户的含义:没有需要注入的东西、没有初始化顺序坑——但也没有运行期可覆盖的口子。
`enable=false` 时全局对象保持 SDK 的 no-op provider,引入而未启用完全无效果(starter.go:66-68)。

### 2.2 exporter 注册表

两条支柱用同一套 driver-registry 惯例(`trace/registry.go`、`metric/registry.go`,基于共享的
泛型 `internal/registry`)。内置项经 `exporters.go` 的 blank import 在 init 自注册:

| 支柱 | 注册名 | 类型 | endpoint 默认 | 说明 |
|------|--------|------|---------------|------|
| trace | `otlp-grpc` | push(batcher) | `localhost:4317` | 默认 exporter |
| trace | `otlp-http` | push(batcher) | `localhost:4318` | |
| trace | `stdout` | push | — | 本地调试 |
| trace | `none` | — | — | 整条 trace 支柱跳过(starter.go:103) |
| metrics | `otlp-grpc` | push(PeriodicReader,`interval`) | `localhost:4317` | 默认 exporter |
| metrics | `otlp-http` | push(PeriodicReader,`interval`) | `localhost:4318` | |
| metrics | `prometheus` | **pull** | 在 `port`/actuator 上服务 `path` | `otelprom.New` 自身即 Reader;handler 渲染独立 registry(`metric/prometheus/exporter.go`) |
| metrics | `stdout` | push(PeriodicReader) | — | |
| metrics | `none` | — | — | 支柱跳过(starter.go:134) |

未知名会让 setup 响亮失败,错误信息列出已注册的 exporter 自诊断(`unknownExporterErr`,
trace/registry.go:58-61)。应用通过 `trace.RegisterSpanExporter(name, factory)` /
`metric.RegisterMeterExporter(name, factory)` 增加后端——与内置项同一条路;重名/nil 注册在
init 直接 panic。

### 2.3 停机:stopper flush 顺序

SIGTERM 后,待所有 server 停止、IoC 容器关闭,`runStoppers` 逐个调用已注册的 stopper
(gs/stopper.go:26-30、86-100)。这里注册的有:`otel-trace`(tp.Shutdown)、
`otel-metrics`(mp.Shutdown),以及——仅 prometheus 且 port>0 时——
`otel-metrics-scrape-server`(srv.Shutdown)。

- **没有确定的 flush 顺序。** stopper 按 map 迭代序执行,契约上彼此独立
  (gs/stopper.go:51-55):"Stoppers must be independent: like servers, they run in no defined
  order and must not rely on one another's cleanup having run." 不要推断"trace 先于 metrics
  flush"——两者都可能先走。
- **没有超时包装。** 传给 stopper 的 context 是 `context.WithoutCancel(ctx)`
  (gs/stopper.go:92)——flush 要等 exporter 自己的重试/超时让步。collector 连接打结时,
  停机会被一直挂住。
- **跳过会丢什么**:trace 侧用 `sdktrace.WithBatcher`(trace/provider.go:55),批间隔内
  尚未导出的 span 缓存在队列里,`tp.Shutdown` 正是负责冲刷的那一下。`kill -9`(或没跑
  stopper)会丢掉队列里全部 span。metrics 的 PeriodicReader 同样在 `mp.Shutdown` 冲刷最后
  一次采集。Prometheus 是 pull 型——没有 flush,stopper 只关抓取 server。
- 某个 stopper 失败只记日志,不阻塞其余(gs/stopper.go:94-97);注册表会被清空,二次调用
  是 no-op。

### 2.4 一次被追踪的请求,端到端走读

对 §1 工程发 `GET /hello/world`:

1. echo 的 Tracing 中间件用 setupTrace 安装的全局传播器解析入站 `traceparent`(无则起新
   trace)——`w3c` 对应 `propagation.TraceContext{}` + `Baggage{}`(trace/provider.go:80-84)。
2. 采样决策:`sampler-ratio` 映射为 `ParentBased(Always|Ratio|Never)`(见 §3),入站已采样的
   trace 跟随父决策,否则按比例。
3. 服务端 span `{method} {route}` 运行;请求级 context 携带 SpanContext。
4. handler 打日志——`log.FieldsFromContext` 钩子把 `trace_id`/`span_id` 抬到日志记录上。
5. 任何被埋点的出站调用(http-client transport、gorm 桥接……)从同一 context 起子 span,
   并经同一全局传播器注入 `traceparent`。
6. span 结束 → SDK batcher 入队;导出按批节奏,不是每请求一次。
7. 指标(时长 histogram、in-flight gauge)经全局 MeterProvider 记录;Prometheus reader 在
   下一次抓取 `:9370/metrics` 时吐出。
8. SIGTERM:server 排空 → 容器关闭 → `otel-trace`/`otel-metrics` stopper 把最后一批冲到
   collector(§2.3)。

---

## 3. 逐 key 行为参考

全部在 `spring.observability.*` 下,共 17 个 key(已用 `grep -rhoE 'value:"[^"]+"'` 与文档
双向核对)。`trace.*`/`metrics.*` 是 struct 子配置(config.go:32-33 的 `${trace}`/`${metrics}`)。

| Key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|----------|
| `enable` | bool | true | setup 内的总开关;false 时全局对象保持 SDK no-op(starter.go:74-77)。 | false + 被埋点组件 → 一切照跑、什么都不导出、无告警。 |
| `service-name` | string | `${spring.application.name:=go-spring-app}` | 唯一的 resource 属性 `service.name`(schemaless resource,trace/provider.go:32-36)。 | 不设 → 静默 `go-spring-app`;所有默认名服务在后端混流。 |
| `trace.enable` | bool | true | false(或 `exporter=none`)整条 trace 支柱跳过——连传播器也不装(starter.go:103)。 | ⚠ 关 trace 同时丢 W3C 传播:跨服务 trace 上下文不再转发,组件还在(空转地)调全局对象。 |
| `trace.exporter` | string | otlp-grpc | 注册表查找(§2.2):otlp-grpc \| otlp-http \| stdout \| none。 | 未知名 → 启动期 setup 失败,错误列出合法名。 |
| `trace.endpoint` | string | "" | otlp exporter 的 host:port;空回落 SDK 默认 localhost:4317/:4318(trace/otlp/exporter.go)。stdout/none 忽略。 | 配错 → 启动时连通性探针(internal/probe,3s TCP 拨号)WARN 一次并点名 endpoint——绝不阻断启动(collector 可能晚于应用起);之后 span 静默排队丢弃直到恢复。 |
| `trace.insecure` | bool | true | 明文 OTLP(WithInsecure)——本地/sidecar collector 的常态。 | true 对 TLS collector → 仅运行期导出失败;反向同理。 |
| `trace.sampler-ratio` | float | 1.0 | `ParentBased` 映射:≥1 全采,(0,1) 按 TraceID 比例(trace/provider.go)。 | **≤0 启动即报错**(2026-08):非正比例过去等于永不采样——全部 span 静默丢弃而表面一切健康。要关 trace 用 `trace.exporter=none` / `trace.enable=false`(传播仍在)。 |
| `trace.propagator` | string | w3c | w3c = TraceContext+Baggage 组合;none 不动进程默认(返回 nil,不是清空)(trace/provider.go:78-90)。自 2026-08 起**独立于 trace 导出生效**(过去 trace 支柱关闭时被整体忽略)。 | 拼错 → setup 报 `unknown propagator (want w3c|none)`。 |
| `metrics.enable` | bool | true | false(或 `exporter=none`)跳过指标支柱(starter.go:134)。 | 与 trace.enable 同样的静默不导出形态。 |
| `metrics.exporter` | string | otlp-grpc | otlp-grpc \| otlp-http \| prometheus \| stdout \| none(§2.2)。 | 未知名 → setup 失败并列出合法名。 |
| `metrics.endpoint` | string | "" | 仅 otlp;与 trace 同样的 SDK 默认回落。对 prometheus/stdout 是死 key。 | 与 trace.endpoint 同样的惰性失败形态。 |
| `metrics.insecure` | bool | true | 仅 otlp。 | 对 prometheus/stdout 是死 key——设了无效。 |
| `metrics.port` | int | **9090** | 仅 prometheus:>0 在 setup 期**同步**起一个独立第二 HTTP server(metric/prometheus/exporter.go 的 `serveMetrics`);0 = 抓取 handler 只经 actuator 挂载服务。 | ⚠ 默认 9090 + actuator → `/metrics` 两个端口都有(endpoint bean 无条件贡献)。0 + 未引 actuator → handler 导出了却没人服务,静默。端口占用 → 启动响亮失败。 |
| `metrics.path` | string | /metrics | 仅 prometheus;独立 server 与 actuator 挂载**共用**(starter.go:169)。 | 自定义 path 同时改两处——Prometheus 抓取配置要跟着改。 |
| `metrics.interval` | duration | 10s | otlp/stdout PeriodicReader 的推送节奏(metric/provider.go:101-107)。prometheus/none 死 key。 | 0/负数回落 reader 自身默认(不是"越快越好")。 |
| `metrics.runtime.enable` | bool | true | 经 OTel contrib 喂 Go 运行时指标(GC、heap、goroutine、GOMAXPROCS),进程内仅启动一次(starter.go:184-189)。 | 关掉丢 `go_goroutine_count` 等——example 冒烟就断言这些。 |
| `metrics.runtime.min-read-mem-stats-interval` | duration | 15s | 限制 `runtime.ReadMemStats`(stop-the-world)频率;0 = 仪表自身默认(metric/provider.go:57-62)。 | 过低 → 采集频繁带来可测的 STW 开销。 |

⚠ **按 exporter 的死 key**(任何阶段都无告警):`endpoint`/`insecure` 对
stdout/prometheus/none 无效;`port`/`path`/`interval` 对 otlp/stdout/none 无效。

---

## 4. 验证与故障演练

### 4.1 自包含冒烟(无外部服务)

仓内 example 在进程内证明两个核心结论:

```bash
cd starter/starter-otel/example && ./check.sh
# 期望:stdout 打出 span、"log correlation OK: trace_id=... span_id=..."、
# "actuator /metrics OK: runtime metrics exposed on :9370"、"actuator /health OK"
```

### 4.2 验证 span 落进 collector(真实导出演练)

起一个往 console 打印的 OTLP collector,把应用的 trace exporter 指过去,造流量,看 span:

```bash
# 1. console exporter 的 collector(存为 otel-config.yaml):
# receivers:
#   otlp:
#     protocols:
#       grpc: { endpoint: 0.0.0.0:4317 }
#       http: { endpoint: 0.0.0.0:4318 }
# processors: { batch: {} }
# exporters:
#   debug: { verbosity: detailed }
# service:
#   pipelines:
#     traces:  { receivers: [otlp], processors: [batch], exporters: [debug] }
#     metrics: { receivers: [otlp], processors: [batch], exporters: [debug] }
docker run --rm -p 4317:4317 -p 4318:4318 \
  -v "$PWD/otel-config.yaml:/etc/otelcol/config.yaml" otel/opentelemetry-collector

# 2. 应用指过去(§1 配置已有 trace.endpoint=127.0.0.1:4317;
#    把指标也切到 otlp-grpc 一并验证 metrics pipeline):
#    spring.observability.metrics.exporter=otlp-grpc
#    spring.observability.metrics.endpoint=127.0.0.1:4317

# 3. 造流量:
curl -s :8002/hello/world >/dev/null; curl -s :8002/hello/again >/dev/null

# 4. 看 collector console:detailed 模式下出现名为 "GET /hello/:name" 的 span,
#    resource 属性 service.name=demo;metrics pipeline 每 interval(10s)打印
#    go_goroutine_count / http server histogram。
# 5. 发 SIGTERM(Ctrl-C),确认最后一次 flush 仍吐出尾部 span——
#    那就是 otel-trace stopper(§2.3)。
```

换真实后端时按现行 Collector 文档把 exporter 换成 jaeger 类后端,打开 Jaeger UI
(:16686)按 service `demo` 检索。采样演练:设
`spring.observability.trace.sampler-ratio=0.05`,打 100 个请求,数 collector 输出的
trace——约 5 条存活;设 `0` 则全部消失。

### 4.3 port=0 + actuator 挂载演练(指标只走 actuator)

```bash
# 用 §1 配置(metrics.exporter=prometheus、metrics.port=0):
curl -s :9370/metrics | grep go_goroutine_count        # actuator 在服务
curl -s --max-time 2 :9090/metrics                     # connection refused——没有第二个 server
curl -i :9370/health                                   # 探针同端口共存
```

改回 `metrics.port=9090` 重启:`/metrics` 在 `:9090`(独立 server,日志
`prometheus scrape server listening`)与 `:9370`(actuator 挂载)**两处**都应答——endpoint
bean 与 port 无关地贡献(starter.go:164-172)。再去掉 starter-actuator import 且 `port=0`:
`/metrics` 哪都不在——但 2026-08 起该组合会在启动时 WARN 并给出修复建议。

### 4.4 日志↔trace 关联演练

装上 §1 的 `loghook.go` 后:

```bash
curl -s :8002/hello/world >/dev/null
# handler 的业务日志行带 trace_id/span_id,与 collector 为同一请求打印的 span
# 一致——把 id 粘进 Jaeger 检索即可互跳。
```

### 4.5 运行时指标 + 传播演练

- 运行时指标:`curl -s :9370/metrics | grep -E '^go_'` → GC、heap、goroutine、GOMAXPROCS
  序列(持续型——与 starter-pprof 的按需 profile 互补)。
- 传播:`curl -H 'traceparent: 00-<32位hex-traceid>-<16位hex-spanid>-01' :8002/hello/world`
  → collector 里服务端 span 挂在该 trace id 之下(ParentBased 尊重上游 sampled 标志)。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 一切照跑,哪儿都没有 trace/指标 | `enable=false`、某支柱 `enable=false` 或 `exporter=none` | 查三个开关;默认全开——多半是有人显式设了。 |
| 无 span、启动 WARN `OTLP endpoint ... is not reachable`、collector 日志空 | OTLP exporter 惰性连接;启动探针(internal/probe)3s TCP 拨号失败时按 (信号,endpoint) 各 WARN 一次——不阻断启动(collector 可能晚于应用起) | 核对 endpoint:端口(4317 grpc 对 4318 http)与 `insecure` 对 TLS;exporter 会在后台持续重试。 |
| 别的服务有 span、唯独本服务没有 | `sampler-ratio` 比例过低 | 调高比例;≤0 自 2026-08 起启动即报错(要完全不采就用 exporter=none/enable=false)。 |
| 启动失败:`unknown exporter` / `unknown propagator` | `trace.exporter` / `metrics.exporter` / `trace.propagator` 拼错 | 错误信息已列出注册名;改用合法名。 |
| 启动失败:prometheus scrape server 绑定报错 | `metrics.port` > 0 且端口占用(同步绑定是刻意的,metric/prometheus/exporter.go) | 腾端口、换端口,或设 0 走 actuator 挂载。 |
| `/metrics` 也在 :9090 应答,只想走 actuator | 默认 `metrics.port=9090` 会起独立 server,即使 actuator 挂载存在 | 设 `spring.observability.metrics.port=0`。 |
| `/metrics` 哪都不在 + 启动 WARN `prometheus exporter has no place to serve` | `port=0` 且进程内没链入 starter-actuator(endpoint.IsServing()==false)——2026-08 起启动即检测 | 引入 starter-actuator(并配 `spring.actuator.addr`)或改用正端口 `metrics.port`。 |
| 跨服务链路在本跳断开、日志丢 trace_id | trace 支柱关闭(`trace.enable=false`/`none`)同时不装传播器、无有效 span context | 保持 trace 开启(传播器是横切关注点——§6 嫌疑 5),并装 log.FieldsFromContext 钩子(§1)。 |
| SIGTERM 后停机挂住 | stopper 的 context 是 `WithoutCancel`(gs/stopper.go:92);collector 连接打结会拖住 flush | 修 collector 可达性/TLS;导出超时由 OTLP exporter 自身决定。 |
| 后端里所有服务都叫 `go-spring-app` | `service-name` 与 `spring.application.name` 都没设 | 设其一;回落是静默的。 |

---

## 6. 设计体检表 + 嫌疑清单

| 指标 | 数值 |
|------|------|
| 配置 key | 17 |
| 必填 | 0 |
| quickstart 前置外部依赖 | 0(stdout)/ 1(真实导出需 collector) |
| "注意/坑"条数(§3 ⚠ + §5) | 8 |

设计嫌疑清单(待设计裁决):

1. ~~README 称 provider 是"带 destroy hook 的 bean"、`endpoint` 对 otlp "必填"~~
   **已修复** —— README 现在如实写进程级 stopper 与可选 endpoint/SDK 默认回落
   (README.md "Graceful Shutdown"、exporter 表)。
2. ~~静默死角:prometheus + `port=0` + 无 actuator → `/metrics` 哪都不在,无日志。~~
   2026-08 已修:经 endpoint.IsServing() 启动检测并 WARN 修复建议。
3. 按 exporter 的死 key 无告警(`endpoint`/`insecure` 对 `port`/`path`/`interval` 各管各的)。
4. `insecure=true` 默认;`service-name` 静默回落 `go-spring-app`。
5. propagator key 在 trace 支柱内部——`trace.enable=false` 会静默关掉跨服务上下文传播,
   而传播是横切关注点。
6. ~~example/ 里提交了 Mach-O 二进制~~ **已修复** —— 该二进制已被 git-ignore(未跟踪的
   本地构建产物);`git ls-files` 显示 example/ 只跟踪源码。
7. (本次审计新增)stopper flush 无超时包装——exporter 端点打结会让进程停机无限期挂住
   (`context.WithoutCancel`,gs/stopper.go:92)。
