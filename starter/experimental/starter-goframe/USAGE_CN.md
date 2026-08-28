# starter-goframe 使用说明 — 参考手册

伞包使用文档，覆盖四个子 server（`http/`、`grpc/`、`tcp/`、`ws/`）与共享的
`internal/logger` 桥接。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照
starter 源码（`http/starter.go`、`grpc/starter.go`、`tcp/starter.go`、`ws/starter.go`、
`internal/logger/logger.go`）与可运行 example（`http/example`、`http/example-otel`、
`grpc/example`（含 `idl/`）、`grpc/example-otel`、`tcp/example`、`ws/example`）核验。
**goframe 自身语义（`ghttp`/`grpcx`/`gtcp`、路由、中间件、WebSocket）见
[goframe 官方文档](https://goframe.org/docs/)** —— 本文只写 go-spring 增量：生命周期接线、
激活 key、etcd 注册、日志桥接、metrics 挂载位置。

**激活条件**：每个子 server 的 bean 仅在其 `address` key 存在时注册
（`spring.goframe.<proto>.server.address`，`gs.OnProperty`）—— 该 key 即开关，没有
`enabled` key。应用还需提供恰好一个对应形态的 `ServiceRegister` bean。每个协议每进程
单实例。

---

## 1. 完整工程示例

一个运行 http 子 server 的真实服务：etcd 发现 + 原生 metrics + starter-otel tracing +
actuator 探活。文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/gogf/gf/v2              v2.x
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-goframe      latest   // 模块根；按子包 import
    go-spring.org/starter-actuator     latest   // 可选：探活
    go-spring.org/starter-otel         latest   // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/router"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-goframe/http"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go** —— 应用全部 HTTP 面：

```go
package router

import (
    "github.com/gogf/gf/v2/net/ghttp"
    "go-spring.org/spring/gs"

    goframehttp "go-spring.org/starter-goframe/http"
)

func init() {
    // 每个激活的子 server，应用提供恰好一个 ServiceRegister bean。http 形态收到
    // 的是 HTTPServer 建好的响应包装 RouterGroup —— 业务路由必须绑在组内，让
    // goframe 的 MiddlewareHandlerResponse JSON 信封生效（见 §2.2）。
    gs.Provide(func() goframehttp.ServiceRegister {
        return func(group *ghttp.RouterGroup) {
            group.ALL("/hello", func(r *ghttp.Request) {
                r.Response.Writeln("Hello World!")
            })
        }
    })
}
```

**conf/app.properties** —— 上述用到的完整注释配置面：

```properties
# --- goframe http server ------------------------------------------------------
# 让 goframe server 独占端口（关闭 gs 内置 HTTP server）。
spring.http.server.enabled=false

spring.goframe.http.server.name=goframe-http
spring.goframe.http.server.address=:8000

# 发布到 etcd 供发现。⚠ 进程级全局（见 §3.1 registry.etcd）。
# spring.goframe.http.server.registry.etcd=127.0.0.1:2379

# goframe 原生 OTel Prometheus 端点，由同一个 server 暴露。默认开启；
# /metrics 在 server 根路径，不进响应信封。
spring.goframe.http.server.metrics.enabled=true
spring.goframe.http.server.metrics.path=/metrics

# --- actuator（探活）----------------------------------------------------------
spring.actuator.addr=:9370

# --- 可观测（starter-otel）：这里只配 tracing ----------------------------------
# ⚠ goframe 原生 metrics 端点开启时不要配 spring.observability.metrics.* ——
# 两边都设置全局 OTel MeterProvider（见 §3.1 metrics 注）。
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**验证**（与 `http/example/check.sh`、`http/example-otel` 同构）：

```bash
curl -i :8000/hello          # 200，body "Hello World!"
curl -s :8000/metrics | head # goframe 原生 Prometheus 输出（合法文本格式）
curl -i :9370/healthz        # actuator liveness
```

grpc/tcp/ws 同形 —— 见可运行 example：`grpc/example`（把生成的
`echo.RegisterEchoServiceServer` 挂到 `grpc.ServiceRegistrar`，proto 在 `idl/echo.proto`）、
`tcp/example`（经 `s.SetHandler` 挂行回显 handler）、`ws/example`（在裸 `*ghttp.Server`
上绑 `r.WebSocket()` upgrade 路由）。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-goframe/<proto>
  └─ gs.Provide(New<Proto>Server, TagArg 配置参数)                 [条件：address key 存在]
        │    .Export(gs.As[gs.Server]())
  └─ blank-import internal/logger：glog → go-spring log 桥接在 init()
        自装 —— 早于任何 server 构建，因此第一条框架日志（含 gsvc 注册与启动错误）
        已经走桥接。

gs.Run()
  ├─ 配置绑定：${spring.goframe.<proto>.server} → Config（value tag）
  ├─ bean 装配：你的 ServiceRegister bean 注入 New<Proto>Server
  ├─ 构造期：
  │    ├─ http/grpc/ws：若设置 registry.etcd → gsvc.SetRegistry(etcdreg.New(...))，
  │    │    必须在 g.Server(name) / grpcx.Server.New 之前 —— ghttp/grpcx 在构造期
  │    │    快照 gsvc.GetRegistry()（顺序是承重的；见 NewHTTPServer/NewGRPCServer/
  │    │    NewWSServer 内注释）
  │    ├─ 仅 http：initMetrics → Prometheus exporter + otelmetric provider，
  │    │    provider.SetAsGlobal()，handler 绑在 server 根
  │    ├─ 仅 http：svr.Group("/") { MiddlewareHandlerResponse; reg(group) }
  │    ├─ ws：reg(svr) 挂裸 server（设计上不进信封组）
  │    └─ tcp：gtcp.NewServer(addr, nil)；reg(s) 经 SetHandler 挂 handler；
  │         registry 预创建但尚未调用
  ├─ Run()：<-sig.TriggerAndWait()（Go-Spring 就绪信号之后才启动）
  │    ├─ http/ws：svr.Start() —— 非阻塞 listen（设了 registry 则注册 etcd）
  │    ├─ grpc：svr.Start() —— 不用 grpcx 的 Run()，后者装自己的 gproc 信号
  │    │    handler，会与 Go-Spring 的信号处理打架；Start + park-on-done 让
  │    │    关停归 Go-Spring 生命周期所有（源码注释）
  │    └─ tcp：svr.Run() 放独立 goroutine（阻塞 Accept 循环，错误经 runErr
  │         channel 转发）；轮询 GetListenedPort() 最多 5s；随后按
  │         advertise.host:port 注册 etcd —— 先 bind 后 register
  └─ SIGTERM 时：Stop() →
       ├─ http：svr.Shutdown()（etcd 反注册）→ metricStop(ctx) flush
       ├─ grpc：svr.Stop()（反注册 + grpc.Server.GracefulStop）
       ├─ ws：svr.Shutdown()
       └─ tcp：先 Deregister（别让新消费者拿到垂死实例），再
            stopping.Store(true)，再 svr.Close() —— stopping 标志让 Run
            goroutine 吞掉预期中的 "use of closed network connection" Accept 错误
```

若缺少 `ServiceRegister` bean，server bean 的第二个构造参数注入失败，容器在启动期
报错 —— 子 server 不可能半接线启动。

### 2.2 路由落位 —— 各协议 register bean 为何不同

| 子 server | ServiceRegister 签名 | 落位理由（源码注释） |
|-----------|----------------------|----------------------|
| http | `func(group *ghttp.RouterGroup)` | 业务 controller 必须落在 `MiddlewareHandlerResponse` 组内（goframe JSON 响应信封）；starter 持有该组，信封不可能被漏掉。`/metrics` 刻意绑在 server 根，Prometheus 输出不进信封。 |
| grpc | `func(s grpc.ServiceRegistrar)` | 包一层生成的 `RegisterXxxServiceServer`；adapter 保持服务无关。 |
| tcp | `func(s *gtcp.Server)` | handler 经 `s.SetHandler` 挂载；gtcp 没有路由/中间件概念。 |
| ws | `func(s *ghttp.Server)` | **刻意用裸 server**：WebSocket upgrade 路由不能挂在响应包装中间件之下 —— 101 Switching Protocols 握手与帧流无法穿过 goframe 的 JSON 信封。 |

注（http）：`MiddlewareHandlerResponse` 不会重包已写入的响应 buffer —— 这就是 example
用 `r.Response.Writeln` 得到纯 body 而非 JSON 包装的原因。

### 2.3 一次启动逐步走读（http + etcd）

1. 包 init：日志桥接同时装在 `glog.SetDefaultHandler`（新建 `glog.New()` logger）与
   `g.Log().SetHandlers`（进程级单例 —— per-logger handlers 优先级更高，两个面都要盖）。
2. 配置绑定；`registry.etcd` 非空 → `gsvc.SetRegistry(etcdreg.New(...))`。
3. `g.Server(name)` 构造 server 并把全局 registry **快照**为自己的 registrar；
   `SetAddr(address)`。
4. metrics：Prometheus exporter → `otelmetric.MustProvider`（含内置指标）→
   `SetAsGlobal()`；根路径 `BindHandler("/metrics", otelmetric.PrometheusHandler)`。
5. 你的 `ServiceRegister` 在 `Group("/", ...)` 闭包内执行 → 路由注册。
6. 就绪信号触发 → `svr.Start()`：后台 listener + etcd Register。
7. `Run` 阻塞在 `done` 直到 `Stop`；SIGTERM → `Shutdown()`（反注册）→
   metric provider flush → `close(done)`。

---

## 3. 逐 key 行为参考

### 3.1 配置 key（四个子 server）

`spring.goframe.http.server.*` —— 激活：`address`

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `address` | string | — | **激活 key**。存在即注册 server bean（OnProperty）。 | 缺失 → 整个子 starter 静默不生效。 |
| `name` | string | `goframe` | ghttp server 名 / etcd 服务名。 | 与进程内另一个 ghttp 实例同名 → goframe 单例冲突。 |
| `registry.etcd` | string | `""` | 空 = 不注册。非空则在构造前**进程级**调用 `gsvc.SetRegistry(etcdreg.New(...))`。⚠ 两个 goframe 子 server 不能各用不同 etcd registry —— 最后构造者胜；"一个注册一个不注册"也会互相影响（第二个空 etcd 不会重置全局）。 | 跨 server registry 串台："未注册"的 grpc server 可能注册进 http server 的 etcd。 |
| `metrics.enabled` | bool | `true` | 在本 server 上装 goframe 原生 OTel Prometheus pull 端点。⚠ `provider.SetAsGlobal()` —— 无法与 starter-otel 的 metrics 管线统一；两边都配 metrics 就抢全局 MeterProvider。 | 全局 meter provider 冲突/重复；关掉一边。 |
| `metrics.path` | string | `/metrics` | 绑在 server 根，不进响应信封。 | 与业务路由撞路径 → 互相遮蔽。 |

`spring.goframe.grpc.server.*` —— 激活：`address`：`address`（必填）、`name`
（默认 `goframe`）、`registry.etcd`（同样的全局 registry 语义；grpcx 构造期快照）。
3 个 key，1 必填。

`spring.goframe.ws.server.*` —— 激活：`address`：key 集与 grpc 相同；register bean
收裸 `*ghttp.Server`。3 个 key，1 必填。

`spring.goframe.tcp.server.*` —— 激活：`address`

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `address` | string | — | **激活 key**；gtcp bind 地址。 | 缺失 → 子 starter 不生效。 |
| `name` | string | `goframe` | 手工注册用的 etcd 服务名。 | — |
| `advertise.host` | string | `127.0.0.1` | 发布进 etcd 的端点。⚠ 仅 `registry.etcd` 设置时使用；gtcp 不会探测公网 IP，必须自己给出可拨地址。⚠ 默认值与你实际 bind 的地址可能悄然不一致。 | 跨机消费者拨 127.0.0.1 → connection refused。 |
| `advertise.port` | int | `8003` | 同 host 的 ⚠ —— 必须与真实 `address` 端口一致，否则客户端拨错端口。 | etcd 里出现错端口端点。 |
| `registry.etcd` | string | `""` | gtcp **没有** gsvc 集成；starter 在 `Run` 里手工注册（先 bind 后 register），在 `Stop` 里先反注册再关 listener。 | — |

不存在其他 key：没有超时、没有 TLS、没有中间件开关 —— 那些属于 goframe 自身配置，
本 starter 刻意不代理。整个 starter 也没有任何 `${observability:=}` 式包装字段。

---

## 4. 验证与故障演练

### 4.1 基础（与各 example 自检同构）

```bash
# http（http/example）
curl -i http://127.0.0.1:8000/hello            # 200，"Hello World!"

# grpc（grpc/example）：用 grpcurl / 客户端打 :8001
grpcurl -plaintext -d '{"message":"hello"}' 127.0.0.1:8001 echo.EchoService/Echo

# tcp（tcp/example）：行回显
printf 'ping\n' | nc 127.0.0.1 8003            # -> ping

# ws（ws/example）
websocat ws://127.0.0.1:8002/echo              # 输入 ping，回 ping
```

### 4.2 原生 metrics 端点（http）

```bash
curl -s :8000/metrics | head          # 合法 Prometheus 文本，不是 JSON 包装
curl -s :8000/metrics | grep -c '^#'  # 存在 goframe 内置指标族
```

handler 在 server 根，输出是纯 Prometheus 文本 —— 若看到 JSON 信封，说明 handler 被
绑进了组里（是 bug，不是配置问题）。

### 4.3 经 starter-otel 的 tracing（http/example-otel、grpc/example-otel）

```bash
cd http/example-otel && docker compose up -d     # 开 OTLP 的 Jaeger
go run .                                          # 发 20 个请求并检查 Jaeger
# 然后：http://localhost:16686 —— 服务 "goframe-http-otel-example"
curl -s 'http://127.0.0.1:16686/api/traces?service=goframe-http-otel-example&limit=1'
```

ghttp/grpcx 基于 starter-otel 装好的全局 OTel TracerProvider 自动埋点 —— 没有
per-server key；缺 starter-otel 时静默 no-op，无告警。

### 4.4 日志桥接演练

任何 goframe 框架日志 —— server 生命周期行、gsvc 注册错误、你自己的
`g.Log().Info(...)` —— 都进 go-spring JSON 管线，tag 为 `_rpc_goframe`
（经 `log.RegisterRPCTag("goframe", "")` 注册；渲染出的 tag 字符串带前导下划线，见
`log.BuildTag`）。级别折叠：glog `NOTI`→Info、`CRIT`→Fatal（`PANI`/`FATA` 前缀标记
也有映射）。glog `Values` 里的结构化附加参数整体挂在单一 `"values"` 字段下 —— glog
对该切片没有 k/v 契约。glog 自身的 stdout/文件输出被完全抑制（handler 不调 `Next`）。

验证：启动 http example，在应用日志输出里 grep `goframe http server starting` ——
必须出现在 go-spring 管线，而不是裸 stdout。

### 4.5 关停演练（tcp 顺序）

给开了 `registry.etcd` 的 tcp example 发 SIGTERM，按日志观察顺序：etcd 反注册 →
listener 关闭 → 进程退出，且没有 "use of closed network connection" 报错
（`stopping` 标志已过滤）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| server 不启动、无 goframe 日志 | 缺 `spring.goframe.<proto>.server.address` | 设置它 —— 它就是激活开关。 |
| 容器报错：ServiceRegister 缺失/歧义 | 应用提供 0 个或 2+ 个该子 server register 类型 bean | 恰好提供一个。 |
| 端口被占用 | gs 内置 HTTP server 还占着端口 | `spring.http.server.enabled=false`。 |
| 没有 trace | 未 import starter-otel | blank-import 它；无它时埋点静默 no-op。 |
| `/metrics` 返回 JSON | handler 被绑进信封组（自定义改动） | 保持 server 根路径；检查 metrics.path。 |
| 两个子 server 意外写到了 etcd A/B | `gsvc.SetRegistry` 进程级全局，最后构造者胜 | 每进程一个 etcd registry，或拆进程。 |
| MeterProvider 冲突 / 指标重复 | goframe 原生 metrics 与 starter-otel metrics 同时开启 | 关一边：`metrics.enabled=false` 或去掉 `spring.observability.metrics.*`。 |
| etcd 消费者连不上 tcp server | `advertise.host/port` 还是默认 127.0.0.1:8003 | 设为与 `address` 匹配的可拨端点。 |
| 关停时刷 "use of closed network connection" | 非 starter 的 gtcp 用法，或绕过了 stopping 标志 | 正常情况下是内部已过滤的细节；若用户可见，按 bug 上报。 |
| 框架日志出现两份（go-spring + 裸 stdout） | 某包在 init 之后又改了 glog handlers | 重装桥接，或应用代码避免 `glog.Set*`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 5（http）/ 3（grpc）/ 3（ws）/ 5（tcp） |
| 必填 | 每子 server 1 个（`address`） |
| quickstart 前置外部依赖 | 0（etcd 可选；完整可观测需 collector） |
| "注意/坑" 条数 | 6 |

设计嫌疑（沿用上一版，交设计裁决）：

- ⚠ `gsvc.SetRegistry` 是**进程级全局突变** —— 两个 goframe 子 server 不能各用不同
  etcd registry（最后构造者胜）；连"一个注册一个不注册"都无法干净表达。候选：按
  server 注入 registrar。
- ⚠ http metrics 调用 `provider.SetAsGlobal()` —— 与 starter-otel 的 metrics 管线
  在两边都配置时冲突。候选：文档化或检测，或改显式 opt-out 默认。
- tcp `advertise.port` 默认值（8003）与实际 bind 的 `address` 可能悄然不一致。候选：
  默认取 bind 端口。
- ~~http metrics "无法与 starter-otel 管线统一"~~ —— **已修**（文档化 + 核验）：行为
  现在是 §3.1/§5 的显式 ⚠，附"关一边"的处置；结构性统一仍受 otel 边界阻塞。
- `http/example/conf/app.properties` 仍注释 "native metrics stay off by default"，
  而代码默认是 `metrics.enabled=true` —— example 注释过期，候选修复。
- grpc/tcp/ws 与 http 没有 metrics 对等（grpcx/gtcp 需各自接线）—— 不对称，候选跟进。
