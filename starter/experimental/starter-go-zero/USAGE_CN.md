# starter-go-zero 使用说明 — 参考手册

伞包使用参考，覆盖两个子 server 包（`rest/`、`zrpc/`）与内部日志桥。概览见
[README_CN.md](README_CN.md)。锚定可运行示例 [rest/example/](rest/example/example.go)、
[zrpc/example/](zrpc/example/example.go) 及 Jaeger 支撑的可观测示例 `rest/example-otel/`、
`zrpc/example-otel/`（各带 `docker-compose.yml`）。**go-zero 自身语义（rest 路由、zrpc 服务治理、
logx、DevServer）见 [go-zero 官方文档](https://go-zero.dev/docs/)** —— 本文只写 Go-Spring 增量：
激活 key、bean 接线、etcd 发布、日志桥、优雅退出。

**激活方式**：每个子 server 仅在其 key 存在时生效 —— key 即开关，没有 `enabled` key：

- `spring.go-zero.rest.server.port` → HTTP/API server（`rest.Server`）
- `spring.go-zero.zrpc.server.listen-on` → gRPC server（`zrpc.RpcServer`）

两者相互独立，可同进程共存（§1 给出完整双协议工程，含 metrics 端口冲突的规避写法）。
激活还需要应用提供该家族的 register bean —— 注入是不可空 autowire：配了 key 没有 bean
会容器报错；反之只有 bean 没有 key，starter 静默不装配。

---

## 1. 完整工程示例

一个双协议服务 —— 对外 REST API、对内 gRPC 服务 —— 跑在同一个 Go-Spring 进程里。
目录树（同 `rest/example/` + `zrpc/example/` 形态）：

```
demo/
├── go.mod
├── main.go
├── rest_handlers.go
├── rpc_services.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/zeromicro/go-zero   v1.10.1
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-go-zero  latest   // import 路径决定 rest/ 或 zrpc/
    go-spring.org/starter-otel     latest   // 可选：真实 trace 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-zero/rest" // 副作用 import 注册 REST bean
    _ "go-spring.org/starter-go-zero/zrpc" // 副作用 import 注册 gRPC bean
    _ "go-spring.org/starter-otel"         // 可选：安装全局 OTel provider
)

func main() { gs.Run() }
```

**rest_handlers.go** —— 应用全部 HTTP 面：

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/zeromicro/go-zero/rest"
    "go-spring.org/spring/gs"
    gozerorest "go-spring.org/starter-go-zero/rest"
)

func init() {
    // 恰好提供一个 HandlerRegister bean。rest.Server 归 starter 所有；
    // 你只往上面挂路由（AddRoute / rest.WithJwt / …… 均为 go-zero API）。
    gs.Provide(func() gozerorest.HandlerRegister {
        return func(server *rest.Server) {
            server.AddRoute(rest.Route{
                Method:  http.MethodGet,
                Path:    "/greet",
                Handler: func(w http.ResponseWriter, r *http.Request) {
                    name := r.URL.Query().Get("name")
                    w.Header().Set("Content-Type", "application/json")
                    _ = json.NewEncoder(w).Encode(map[string]string{"message": "Hi, " + name})
                },
            })
        }
    })
}
```

**rpc_services.go** —— 应用全部 gRPC 面（示例用 grpc 内置 health 服务，免去 protoc 步骤；
真实项目换成 `pb.RegisterGreeterServer(s, ...)`）：

```go
package main

import (
    "go-spring.org/spring/gs"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    gozerozrpc "go-spring.org/starter-go-zero/zrpc"
)

func init() {
    // 恰好提供一个 ServiceRegister bean；starter 构建 *grpc.Server 后回调它，
    // 在上面注册你 pb 生成的服务。
    gs.Provide(func() gozerozrpc.ServiceRegister {
        return func(s *grpc.Server) {
            healthpb.RegisterHealthServer(s, health.NewServer())
        }
    })
}
```

**conf/app.properties** —— 完整、带注释的配置面：

```properties
# --- gs 内建 HTTP server ----------------------------------------------------
# 关掉它：gs.Run() 启动的 HTTP 监听只剩下面的 go-zero server。不关的话
# gs.Run() 会绑定自己的 HTTP listener 并抱怨没有注册任何 handler。
spring.http.server.enabled=false

# --- go-zero REST server ----------------------------------------------------
# 由 starter-go-zero/rest 绑定 ${spring.go-zero.rest.server}。
# `port` 是激活 key：不配置则整个子 server 不装配。
spring.go-zero.rest.server.name=greet
spring.go-zero.rest.server.host=0.0.0.0
spring.go-zero.rest.server.port=8888

# --- go-zero gRPC server ----------------------------------------------------
# `listen-on` 是激活 key。
spring.go-zero.zrpc.server.name=greet-rpc
spring.go-zero.zrpc.server.listen-on=0.0.0.0:8081

# 可选：把 ListenOn 端点发布进 etcd 供消费方发现。两者留空即直连
# （启动 0 外部依赖）。
#spring.go-zero.zrpc.server.etcd.addr=127.0.0.1:2379
#spring.go-zero.zrpc.server.etcd.key=greet.rpc

# --- metrics：6060 冲突规避（双协议同开时必改） -------------------------------
# rest 和 zrpc 各自起一个 go-zero DevServer 监听 metrics.port，且默认都是
# 6060 —— 第二个会绑定失败。给它们分配不同端口：
spring.go-zero.rest.server.metrics.enabled=true
spring.go-zero.rest.server.metrics.port=6060
spring.go-zero.zrpc.server.metrics.enabled=true
spring.go-zero.zrpc.server.metrics.port=6061

# --- tracing -----------------------------------------------------------------
# 默认（tracing.disabled=true）交给 starter-otel 的全局 provider —— 这里
# 无需任何配置。只有 disabled=false 才走 go-zero 原生 OTLP 上报
# （届时按子 server 配 endpoint/sampler/batcher）。

# --- logging -----------------------------------------------------------------
# go-spring 的 log 模块同时输出业务日志与 go-zero 框架日志（桥接，tag
# _rpc_gozero）。log.level（下行）是 logx 侧唯一的旋钮：
#spring.go-zero.rest.server.log.level=info
#spring.go-zero.zrpc.server.log.level=info

# --- 可观测（starter-otel，可选）---------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**前置依赖**：直连模式无任何依赖。配置 `etcd.addr` 需可达的 etcd（zrpc 示例形态：
`docker run -d -p 2379:2379 quay.io/coreos/etcd`）。otel 形态在 `example-otel/` 里
`docker compose up -d` 起 Jaeger（all-in-one:1.60，OTLP :4317，UI :16686）。

**验证**（与示例 `check.sh` 同构）：

```bash
go run .
# 启动日志："go-zero rest server starting on 0.0.0.0:8888"
#           "go-zero zrpc server starting on 0.0.0.0:8081"

curl -i 'http://127.0.0.1:8888/greet?name=world'        # {"message":"Hi, world"}
grpcurl -plaintext 127.0.0.1:8081 grpc.health.v1.Health/Check   # status: SERVING
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

每个子 server 家族同形（以 zrpc 为例，rest 仅 key 不同）：

```
import starter-go-zero/zrpc
  └─ init(): gs.Provide(NewZrpcServer, IndexArg(0, TagArg("${spring.go-zero.zrpc.server}")))
                .Export(gs.As[gs.Server]())
                .Condition(gs.OnProperty("spring.go-zero.zrpc.server.listen-on"))

gs.Run()
  ├─ 条件闸门：listen-on 配了吗？（否 → bean 不存在，starter 不介入）
  ├─ 配置绑定：${spring.go-zero.zrpc.server} → Config（value tag，相对前缀）
  ├─ bean 接线：NewZrpcServer(cfg, reg ServiceRegister)   ← 不可空 autowire
  ├─ Rooter Init → Run(ctx, sig)：
  │     ├─ 组装 zrpc.RpcServerConf（Name/Log.Level/Telemetry/DevServer/ListenOn）
  │     ├─ etcd.addr != "" → conf.Etcd = discov.EtcdConf{Hosts, Key}
  │     ├─ zrpc.MustNewServer(conf, func(g){ reg(g) })
  │     ├─ logx.SetWriter(logger.NewWriter())   ← 在 MustNewServer 之后（见 §3.3）
  │     ├─ <-sig.TriggerAndWait()               ← 挂起直到 gs 发出就绪信号
  │     ├─ 输出 "go-zero zrpc server starting on <listen-on>" 日志
  │     └─ go svr.Start()（绑定监听、向 Etcd.Key 注册、阻塞）
  │         / select on errCh | done
  └─ 收到 SIGTERM：Stop 关闭 done → Run 调 svr.Stop()
        （zrpc 从 etcd 反注册、排空传输层、gs 完成关停序列）
```

设计要点（源自源码注释，已核对）：

- `Start` 内部阻塞（rest 注释："Start binds the listener and blocks until Stop is called"），
  因此放到 goroutine 里跑，`Run` 挂在 `done` channel 上；`Stop` 关闭 `done`，把控制权交还
  Go-Spring 前先拆掉 server。
- `svr.Stop()` 不收 context —— `Stop` 的 ctx 只用来打关停日志。
- `Start` 自身正常返回时 errCh 收到 nil；Start 的真实失败（监听绑定失败等）在 go-zero
  内部以 panic/error 形式暴露 —— 见 §5。

### 2.2 中间件 / 拦截器走读

Go-Spring **不注入任何自己的中间件** —— 你拿到的是 go-zero 原生链，仅此而已：

- **rest**：`rest.MustNewServer(rc)` 未加任何额外 option → go-zero 内建 handler 链
  （recovery、logging、metrics、trace 中间件，遵循 `RestConf` 默认值；
  [go-zero rest 文档](https://go-zero.dev/docs/micro-service/restful-api)）。本 starter
  不注入 fault/governance、request-id、access-log 任何一层。
- **zrpc**：`zrpc.MustNewServer(conf, registerFn)` → go-zero 原生 server 拦截器
  （tracing、stat、breaker、prometheus —— 由 `RpcServerConf` 决定；
  [go-zero zrpc 文档](https://go-zero.dev/docs/micro-service/rpc-call)）。starter 传给
  zrpc 的注册函数就是你的 `ServiceRegister` bean。

trace 中间件行为（rest/starter.go Config 注释，已核对）：默认 `tracing.disabled=true`
时 go-zero **不**启动自己的 trace agent，中间件经当前**全局** OTel TracerProvider 出
span —— 导入了 starter-otel 就是它的 provider，否则是 no-op。置 `disabled=false` 改走
go-zero 原生 OTLP 上报。

### 2.3 一次请求逐层走读

REST `GET /greet?name=world`：

1. `host:port` 上的 net/http 监听接受连接；go-zero rest handler 链依次执行
   （recovery → logging → metrics → trace → 路由匹配）。
2. trace 中间件经全局 provider 抽取/开启 span（无 starter-otel 则 no-op）。
3. 你经 `HandlerRegister` 挂上的 handler 执行；返回 `{"message":"Hi, world"}`。
4. 该路径上的 go-zero 框架日志经日志桥（§4.3）流出，tag 为 `_rpc_gozero`。

gRPC `grpc.health.v1.Health/Check`（直连）：

1. 客户端以 plaintext 凭据拨号 `listen-on`。
2. go-zero server 拦截器链（stat/tracing/breaker/prometheus，随 RpcServerConf）。
3. 你经 `ServiceRegister` 注册的 Health 服务应答 `SERVING`。
4. 配了 `etcd.addr` 时，消费方改经 go-zero discov 解析 `Etcd.Key`，不再直连；
   Stop 时反注册。

---

## 3. 逐 key 行为参考

### 3.1 `spring.go-zero.rest.server.*`（激活：`port`）— 11 个 key，1 必填

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `port` | int | — | **激活 key**。存在即注册 server bean。 | 缺失 → starter 静默不生效。 |
| `host` | string | `0.0.0.0` | rest server 监听主机。 | 不可绑定 → Run 时监听报错。 |
| `name` | string | `go-zero` | 服务名（`RestConf.Name`）；同时是 logx `ServiceName`。 | 仅展示用途；rest 无发现耦合。 |
| `tracing.disabled` | bool | `true` | true = 交给全局 OTel provider（starter-otel）。⚠ `endpoint`/`sampler`/`batcher` 在此为 true 时全是**死 key**。 | 不翻转它：原生 OTLP 静默闲置。 |
| `tracing.endpoint` | string | `""` | go-zero 原生 OTLP collector 地址；仅 `disabled=false` 生效。 | 未翻转 disabled → 被忽略。 |
| `tracing.sampler` | float64 | `1.0` | 原生 OTLP 采样比；仅 `disabled=false` 生效。 | 同死 key 规则。 |
| `tracing.batcher` | string | `otlpgrpc` | 原生 batcher 种类；仅 `disabled=false` 生效。 | 同死 key 规则。 |
| `metrics.enabled` | bool | `true` | 启动 go-zero DevServer（Prometheus）。⚠ 与 starter-otel 的 metrics 管线相互独立、永不合并。 | 多出非预期的 6060 监听。 |
| `metrics.port` | int | `6060` | DevServer 监听端口。⚠ **与 zrpc 的同名默认值冲突** —— 双子 server 同开时第二个 DevServer 绑定失败。 | rest+zrpc 共存且都用默认 → 启动失败（见 §5）。 |
| `metrics.path` | string | `/metrics` | DevServer 上 Prometheus 抓取路径。 | 抓取端指错 → 404。 |
| `log.level` | string | `info` | 日志桥之后**唯一**生效的 logx 字段：logx 在委托给桥之前按级别过滤。取值 `debug`/`info`/`error`/`severe`（go-zero 口径）。 | 大小写/取值错误 → logx 用默认值。 |

DevServer 的 `EnablePprof` 在 starter 里硬编码 false，没有对应 key。

### 3.2 `spring.go-zero.zrpc.server.*`（激活：`listen-on`）— 12 个 key，1 必填

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `listen-on` | string | — | **激活 key**；同时是 gRPC 监听地址（`host:port`）。 | 缺失 → starter 静默不生效。 |
| `name` | string | `go-zero` | 服务名；同时是 logx `ServiceName`。 | 展示用途。 |
| `etcd.addr` | string | `""` | 空 = 直连。配置后 Start 时把 `ListenOn` 发布到 `etcd.key` 下，Stop 时反注册。⚠ 仅单 host —— starter 构造 `discov.EtcdConf{Hosts: []string{addr}}`，无集群列表。 | etcd 不可达 → Run 时 zrpc 启动报错。 |
| `etcd.key` | string | `""` | 消费方解析的发现 key。⚠ 不配 `etcd.addr` 则无意义。 | 有 addr 无 key → go-zero discov 默认行为。 |
| `tracing.disabled` | bool | `true` | 策略同 rest §3.1（zrpc trace 拦截器用环境全局 provider）。 | 同 rest。 |
| `tracing.endpoint` / `.sampler` / `.batcher` | string/float/string | `""`/`1.0`/`otlpgrpc` | 原生 OTLP 旋钮；`disabled=false` 前是死 key。 | 同死 key 规则。 |
| `metrics.enabled` | bool | `true` | 与 rest 同一 DevServer。⚠ 同样的 6060 冲突。 | 同 rest。 |
| `metrics.port` | int | `6060` | ⚠ rest 也激活时必须配**不同**端口（如 6061）。 | 启动时端口占用失败。 |
| `metrics.path` | string | `/metrics` | 抓取路径。 | 不一致 → 404。 |
| `log.level` | string | `info` | 同 rest §3.1 —— 过滤日志桥的 logx 侧。 | 同 rest。 |

### 3.3 日志桥（internal/logger）

go-zero 的 `logx.Writer` 方法不带 `context.Context`，因此（internal/logger/logger.go，已核对）：

- 每条转发日志打 tag `_rpc_gozero`（`log.RegisterRPCTag("gozero", "")`）—— 用 go-spring
  的 logger tag 配置过滤或路由；
- `log.FieldsFromContext` 的 trace-id 传播无法触发，但 go-zero 经 `logx.WithContext`
  注入的 trace/span 字段会作为结构化字段透传，链路关联仍在；
- 记录到的 caller（file:line）落在桥里，不是真实发出点；
- **安装时机**：`MustNewServer` 会跑 `ServiceConf.SetUp()` → `logx.SetUp()`，装上 logx
  自己的 writer，因此每个子 starter 在构建 server **之后**用 `logx.SetWriter` 重装桥。
  后果：双子 server 同开时，实际生效的 logx 级别取决于最后执行 `Run` 的那个
  （writer + 级别均为进程级）—— 两个前缀下的 `log.level` 请保持一致。

级别映射：Debug/Info/Error 一一对应；`Alert`→Error，`Severe`→Fatal（仅级别信号 —— 桥
绝不调 os.Exit），`Slow`→Warn，`Stat`→Info，`Stack`→Error。

### 3.4 包装 tag 是前缀相对的（已核对）

嵌套结构体的 `value:"${tracing}"` / `${metrics}` / `${log}` / `${etcd}` 绑定在 ctor 参数
前缀之下（`gs.IndexArg(0, gs.TagArg("${spring.go-zero.<family>.server}"))`），**不是**
顶层绝对引用。本 starter 不存在任何 `${observability:=}` 式包装字段（已 grep 全部 value
tag 核实）。

---

## 4. Beans、可观测与演练

### 4.1 提供的 bean（每个激活家族）

| Bean | 类型 | 说明 |
|------|------|------|
| REST server | `*StarterGoZeroRest.RestServer`，导出为 `gs.Server` | 仅当 `...rest.server.port` 配置时存在。 |
| gRPC server | `*StarterGoZeroZrpc.ZrpcServer`，导出为 `gs.Server` | 仅当 `...zrpc.server.listen-on` 配置时存在。 |

### 4.2 你必须提供的 bean（每个家族）

| 家族 | bean 类型 | 典型写法 |
|------|-----------|----------|
| rest | `StarterGoZeroRest.HandlerRegister = func(*rest.Server)` | `server.AddRoute(rest.Route{...})` |
| zrpc | `StarterGoZeroZrpc.ServiceRegister = func(*grpc.Server)` | `pb.RegisterGreeterServer(s, svc)` |

不可空 autowire：零 bean → 容器失败；同型两个 → 歧义失败。各家族恰好一个。
starter 永远不知道你的路由表或 pb 类型。

### 4.3 可观测面

- **日志**：框架日志在 `_rpc_gozero` 下；业务日志在你自己的 tag 下；启动/关停日志在
  app-def tag 下。
- **Tracing**：默认搭乘 starter-otel 的全局 provider —— go-zero 侧零配置。example-otel
  一对示例端到端验证了该路径（20 个请求 → Jaeger `http://127.0.0.1:16686`，服务名
  `gozero-rest-otel-example` / `gozero-zrpc-otel-example`）。
- **Metrics**：仅 go-zero 原生 Prometheus，在 DevServer 上（`metrics.port`），与
  starter-otel 的 metrics 管线相互独立。

### 4.4 验证与故障演练

```bash
# REST 往返（与 rest/example/check.sh 同一断言）：
curl -i 'http://127.0.0.1:8888/greet?name=world'    # 期望 {"message":"Hi, world"}

# gRPC 往返（grpcurl，同 zrpc/example 的 health check）：
grpcurl -plaintext 127.0.0.1:8081 grpc.health.v1.Health/Check   # status: SERVING

# DevServer metrics（打过流量后）：
curl -s :6060/metrics | grep -E 'prometheus|metric' | head

# 6060 冲突演练：注释掉 conf/app.properties 里任意一行独立的 metrics.port，
# 让两边都用默认 6060，再 `go run .` → 第二个 DevServer 绑定失败
# （listen :6060 in use）。恢复拆分端口即可。

# 框架日志经桥流出：
grep _rpc_gozero <你的日志文件> | head

# SIGTERM 优雅关停演练：
kill -TERM <pid>   # 期望同时出现两行：
                   # "go-zero rest server shutting down on 0.0.0.0:8888"
                   # "go-zero zrpc server shutting down on 0.0.0.0:8081"
# 配了 etcd 时，停机后 key 消失：
ETCDCTL_API=3 etcdctl get --prefix ''   # 无 greet.rpc 条目

# Tracing 演练（example-otel 形态）：
docker compose -f rest/example-otel/docker-compose.yml up -d
go run ./rest/example-otel   # 发 20 个请求、断言 Jaeger 有该服务、自退出
```

可观测项：app-def tag 下的启动/关停日志行；span 命名遵循 go-zero rest/zrpc 中间件
默认；Prometheus 序列来自 go-zero DevServer 注册表。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 什么都没起，没有 go-zero 日志 | 激活 key 缺失（`port` / `listen-on`） | 配上 —— 它就是开关。 |
| 容器失败：register bean 缺失 | 配了 key 但没有 `HandlerRegister`/`ServiceRegister` bean | 给该家族恰好提供一个。 |
| 容器失败：bean 歧义 | 同型 register bean 有两个 | 只留一个。 |
| gs 报 HTTP "no handlers registered" | 内建 HTTP server 还开着 | `spring.http.server.enabled=false`。 |
| 启动绑定 :6060 失败 | rest+zrpc 同开、都用默认 `metrics.port=6060` | 配不同端口（6060/6061），或一侧 `metrics.enabled=false`。 |
| 配了 etcd 后 zrpc 启动报错 | `etcd.addr` 不可达 | 起 etcd，或留空 `etcd.addr`（直连）。 |
| 一切正常但没有 trace | 未导入 starter-otel（全局 provider 是 no-op） | 加上；或 `tracing.disabled=false` + endpoint 走原生 OTLP。 |
| `tracing.endpoint` 似乎不生效 | `disabled=true`（默认）时它是死 key | 先置 `tracing.disabled=false`。 |
| go-zero 日志缺 trace id / file:line 不对 | 桥的固有限制：logx.Writer 无 ctx | 关联信息走 trace/span 字段；用 `_rpc_gozero` 过滤（§3.3）。 |
| 两个家族日志级别相互覆盖 | `logx.SetWriter` + 级别是进程级；最后 `Run` 的说了算 | 两个前缀的 `log.level` 保持一致。 |
| 配了 "port" 但 zrpc 静默不生效 | key 用错：zrpc 绑 `listen-on`，不是 `port` | 改用 `spring.go-zero.zrpc.server.listen-on=host:port`。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 11（rest）+ 12（zrpc） |
| 必填 | 每家族 1 个（`port` / `listen-on`）+ 1 个 bean |
| quickstart 前置外部依赖 | 直连 0；etcd 注册 1（etcd）；otel 演练 +1（Jaeger） |
| "注意/坑" 条数 | 6 |

设计嫌疑（供设计裁决台账）：

- **端口冲突**：rest 与 zrpc 的 `metrics.port` 默认都是 6060 —— 两者同开且都用默认会
  双重绑定 DevServer 端口；考虑按家族给默认（6060/6061）或共享单例 DevServer。
- **进程级全局量**：`logx.SetWriter` 与 tracing 全局量是进程级的，两个子 server 无法在
  日志/trace 上各行其是；桥还会被子 server 的每次 `Run` 重装（最后一个生效）。
- **仅 log.level**：桥装好后 `log.level` 是唯一生效的 logx 旋钮 —— logx 丰富的
  `LogConf`（encoding、rotation、stat 周期）既未暴露也被部分覆盖。
- DevServer 的 pprof 开关被硬编码关闭，无配置逃生门。
- etcd 配置仅单 host（`Hosts: []string{addr}`）—— 无集群列表、无 TLS。
- 两个子 server 都没有 health indicator bean（就绪信号只有泛化的 gs.Server 信号）；
  zrpc 的 grpc health 服务要应用自己手动注册。
- 跨家族不一致：metrics 开关此处叫 `enabled`，kratos 家族叫 `enable`。
- 文档过期风险：两个普通示例的配置注释声称 metrics 默认关闭，但代码默认
  `enabled=true`；zrpc/example-otel 的配置写的是 `spring.go-zero.zrpc.server.port`
  （zrpc 不存在该 key —— 激活要 `listen-on`），该示例按此配置大概率起不来 server。
