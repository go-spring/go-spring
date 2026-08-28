# starter-kratos 使用说明 — 参考手册

伞包使用参考，覆盖三个子 server 包（`http/`、`grpc/`、`ws/`）与内部日志桥。概览见
[README_CN.md](README_CN.md)。锚定 [contrib/go-kratos/](../../../contrib/go-kratos/) 可运行示例
（provider/consumer 成对，含 `scripts/smoke-test.sh`）。**kratos 自身语义（transport、
middleware、App 生命周期、proto 代码生成）见 [kratos 官方文档](https://go-kratos.dev/docs/)** ——
本文只写 Go-Spring 增量：激活 key、bean 接线、etcd 注册、可观测、日志桥接。

**激活方式**：每个子 server 仅在其 addr key 存在时装配 —— 该 key 即开关，没有 `enabled` key：

- `spring.kratos.http.server.addr` → HTTP transport（`khttp`）
- `spring.kratos.grpc.server.addr` → gRPC transport（`kgrpc`）
- `spring.kratos.ws.server.addr` → WebSocket transport（`kws`，tx7do fork）

三者互相独立，可在同一进程共存（各自构建独立的 `kratos.App`）。激活还要求应用提供该家族的
`ServiceRegister` bean —— 注入为非可空：配了 addr 没有 bean 容器报错（反之，有 bean 没有
addr 时 starter 静默不装配）。

---

## 1. 完整工程示例

一个贴近真实的 gRPC 形态服务（http/ws 同形）：一个 proto 服务、etcd 发布、JSON 日志、
OTel 指标/追踪。文件树（对应 `contrib/go-kratos/grpc/provider/`）：

```
demo/
├── go.mod
├── main.go
├── handler.go
├── idl/helloworld/v1/            # protoc 生成（greeter.pb.go、greeter_grpc.pb.go）
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/go-kratos/kratos/v2  v2.9.x
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-kratos    latest   // import 路径决定 http/ grpc/ ws/
    go-spring.org/starter-otel      latest   // 可选：真正的 metric/trace 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-kratos/grpc" // 副作用 import 注册 server bean
    _ "go-spring.org/starter-otel"        // 可选：点亮 tracing/metrics 导出
)

func main() { gs.Run() }
```

**handler.go** —— 应用的全部 gRPC 面：

```go
package main

import (
    "context"

    kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
    v1 "demo/idl/helloworld/v1"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    kratosgrpc "go-spring.org/starter-kratos/grpc"
)

func init() {
    // 提供且仅提供一个 ServiceRegister bean。starter 持有 kratos.App 与 transport
    // server；你在这里绑定生成的 RegisterXxxServer。
    gs.Provide(func() kratosgrpc.ServiceRegister {
        return func(s *kgrpc.Server) error {
            v1.RegisterGreeterServer(s, &GreeterService{})
            return nil
        }
    })
}

type GreeterService struct{ v1.UnimplementedGreeterServer }

func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
    log.Infof(ctx, log.TagBizDef, "SayHello name=%s", in.Name)
    return &v1.HelloReply{Message: "Hello " + in.Name}, nil
}
```

**conf/app.properties** —— 完整注释配置面：

```properties
# --- gs 内建 HTTP server ----------------------------------------------------
# 关掉它：gs.Run() 只启动下面的 kratos transport。不关的话 gs.Run() 会绑一个
# HTTP listener 并抱怨没有注册任何 handler。
spring.http.server.enabled=false

# --- kratos gRPC transport --------------------------------------------------
# 由 starter-kratos/grpc 从 ${spring.kratos.grpc.server} 绑定。
spring.kratos.grpc.server.name=kratos-grpc
spring.kratos.grpc.server.addr=0.0.0.0:9000
spring.kratos.grpc.server.timeout=1s

# etcd 注册：App 启动时把 {name, endpoints} 发布到这里、停止时反注册。
# 留空 = 纯直连 server。
spring.kratos.grpc.server.etcd.addr=127.0.0.1:2379

# 请求指标（默认开）。记录进全局 OTel meter；exporter 与抓取端点归 starter-otel
# 所有，不由本 starter 决定。
spring.kratos.grpc.server.metrics.enable=true

# --- 日志 --------------------------------------------------------------------
# go-spring 的 log 模块把业务日志与 kratos 框架日志（starter-kratos 桥接，tag
# _rpc_kratos）都写成结构化 JSON。
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=provider.log
logging.logger.root.layout.type=JSONLayout

# --- 可观测（starter-otel） --------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**前置依赖**：一个 `127.0.0.1:2379` 可达的 etcd（每个 contrib 示例自带
`docker-compose.yml`）；或留空 `etcd.addr` 走直连（0 个外部依赖）。

**验证**（与 `contrib/go-kratos/grpc/scripts/smoke-test.sh` 同构）：

```bash
# gs.Run() 之后日志出现启动行
grep 'kratos grpc server starting on 0.0.0.0:9000' ../logs/provider.log

# 用任意持有同一 etcd 的客户端做发现调用（consumer 示例）：
#   dial "discovery:///kratos-grpc" → SayHello → "Hello Kratos"
# 服务已发布进 etcd：
ETCDCTL_API=3 etcdctl get --prefix /microservices/kratos-grpc/
```

HTTP 形态只需把 import 换成 `starter-kratos/http`、key 换成
`spring.kratos.http.server.*`（默认 addr `0.0.0.0:8000`、name `kratos-http`），注册时用
`v1.RegisterGreeterHTTPServer`。WebSocket 见 §4.4。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

每个子 server 家族同形（以 grpc 为例）：

```
import starter-kratos/grpc
  └─ init(): gs.Provide(NewGrpcServer, IndexArg(0, TagArg("${spring.kratos.grpc.server}")))
                .Export(gs.As[gs.Server]())
                .Condition(gs.OnProperty("spring.kratos.grpc.server.addr"))

gs.Run()
  ├─ 条件门：addr 是否存在？（否 → bean 不存在，starter 静默失效）
  ├─ 配置绑定：${spring.kratos.grpc.server} → Config（value tag）
  ├─ bean 装配：NewGrpcServer(cfg, reg ServiceRegister)   ← 非可空注入
  ├─ Rooter Init → Run(ctx, sig)：
  │     ├─ 构建中间件链（§2.2）、kgrpc.NewServer、reg(srv)
  │     ├─ 可选 etcd 客户端（DialTimeout 5s）→ kratos.Registrar(etcd.New(cli))
  │     ├─ kratos.New(Name, Logger(桥接), Server(srv), [Registrar])
  │     ├─ <-sig.TriggerAndWait()          ← 等待 gs 就绪信号
  │     ├─ 日志行 "kratos grpc server starting on <addr>"
  │     └─ go app.Run()（发布进 etcd、开始服务）/ select done|errCh
  └─ SIGTERM：Stop → StopContext 关闭 done → Run 调 app.Stop()
        （kratos 从 etcd 反注册、收尾 transport，gs 完成关停序列）
```

设计说明（源码注释核对过）：

- `kratos.App.Run` 阻塞到 `Stop` 为止，因此它跑在 goroutine 里，`Run` 停在 `done`
  channel 上；`Stop` 关闭 `done` 把控制权交还 Go-Spring，由 Run 完成拆 App。
- `app.Stop()` 不接收 context —— `StopContext` 的 ctx 只用于给关停日志打标。
- 若 `app.Run()` 自身出错返回，`Run` 经
  `errutil.Explain(err, "kratos grpc app exited with error")` 上抛。

### 2.2 中间件链 —— 精确顺序与理由（http/grpc）

```
recovery.Recovery() → tracing.Server() → [kmetrics.Server()，metrics.enable 时]
→ 你的 service handler（经 ServiceRegister 注册）
```

理由（出自 `Run` 的源码注释）：

- **recovery 最外层**：下游任何一层的 panic 在跨越 transport 边界前被 recover。
- **tracing 在 metrics 之前**：tracing 开 span，随后 metrics 在活跃 span/context 下
  记录。`tracing.Server()` 读 starter-otel 安装的全局 OTel provider；没有它就是 no-op。
- **metrics 最内层**：counter/histogram 观测的是 handler 的结果与耗时。

本 starter 没有 fault/governance、loadtest、request-id、access-log 层 —— 也没有配置或
bean 缝隙可以追加 kratos 中间件（`khttp.Middleware`/`kgrpc.Middleware` 未暴露），
链是固定的（见设计嫌疑清单）。

**ws 完全没有中间件链** —— tx7do transport 不做埋点：无 tracing、无 metrics，recovery
只有 kratos-transport 内部机制。

### 2.3 一次请求逐层走读（grpc）

经 discovery endpoint 的 `SayHello("Kratos")`：

1. 客户端 dial `discovery:///kratos-grpc` → etcd watch 解析出活跃 endpoint → 建到
   `0.0.0.0:9000` 的 gRPC 连接。
2. recovery 布防（defer recover）。
3. tracing 从 gRPC metadata 提取 span context、开 server span（没有 starter-otel 的
   provider 时为 no-op）。
4. metrics 递增 `server_requests_code_total`，开始 `server_requests_seconds` 观测。
5. 你的 handler 执行；应答按 4→3 展开：耗时记录、span 结束。
6. 这条路径上的 kratos 框架事件（transport 错误等）经日志桥（§3.5）流进 go-spring
   log，tag `_rpc_kratos`。

---

## 3. 逐 key 行为参考

### 3.1 `spring.kratos.http.server.*`（激活：`addr`）— 6 个 key，1 必填

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `addr` | string | — | **激活 key**。存在即注册 server bean。 | 缺失 → starter 静默失效（handler 配了也永远不服务）。 |
| `name` | string | `kratos-http` | 发布进 etcd 的服务名；同时是 OTel **meter 名**（`otel.Meter(cfg.Name)`），指标序列按 server name 隔离。 | 与消费方 `discovery:///<name>` 不一致 → 调用时才发现解析失败，启动不报错。 |
| `network` | string | `""` | 空 = tcp；如 `unix` 走 unix domain socket。 | 填错 → Run 时 listen 报错。 |
| `timeout` | duration | `1s` | kratos server timeout（按请求传递 deadline）。`0` 跳过该选项。 | 过低 → 慢 handler 报 deadline exceeded。 |
| `etcd.addr` | string | `""` | 空 = 直连不注册。设置后 App 在 Run 时发布 `{name, endpoints}` 进 etcd、停止时反注册；etcd 客户端 `DialTimeout` 5s。⚠ etcd 不可达只在客户端构造失败时报 "failed to create etcd client"；可达但集群不对要更晚才暴露。 | 集群填错 → 发现侧消费方看不到服务。 |
| `metrics.enable` | bool | `true` | 装配 `kmetrics.Server()`（仅 http/grpc），记录进**全局** OTel meter；exporter/抓取归 starter-otel。 | 没有 starter-otel → 指标写进 no-op meter，无任何告警。 |

### 3.2 `spring.kratos.grpc.server.*`（激活：`addr`）— 6 个 key，1 必填

与 §3.1 完全同集；`name` 默认 `kratos-grpc`。中间件、etcd、meter 命名行为一致。

### 3.3 `spring.kratos.ws.server.*`（激活：`addr`）— 5 个 key，1 必填

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `addr` | string | — | **激活 key**。 | 同上。 |
| `name` | string | `kratos-ws` | etcd 服务名。⚠ kratos-transport 的 WS **客户端没有 discovery 钩子** —— 消费方直连 `ws://host:port/path`；注册仅为与 http/grpc 对齐（示例配置注释明确说明）。 | 注册进 etcd 并不能让 WS 可被发现。 |
| `network` | string | `""` | 空 = tcp。 | 同上。 |
| `path` | string | `/` | WebSocket upgrade 路径。 | 客户端 URL path 不一致 → 握手 404。 |
| `etcd.addr` | string | `""` | 语义同 http/grpc。 | 同上。 |

ws **没有** `timeout`、**没有** `metrics.*` key —— WebSocket transport 无中间件链、
不做埋点。

### 3.4 线上格式（ws，固定 —— 无 key）

每帧为 `<4 字节小端 uint32 messageType><JSON 编码 payload>`（binary payload type、
JSON codec）。message type 整数是应用间带外约定；payload 是普通 JSON struct，通常
**不**用 protoc 生成类型（理由见 ws 示例 handler.go 注释）。`github.com/tx7do/
kratos-transport` 依赖锁在 v1.3.1：v1.3.4 破坏了 session 注册（wsHandler 不再向
SessionManager 注册，`SendMessage` 报 "session not found"）。选 binary 而非 text 是因为
锁定版本的 text 模式不对称（接收时拆 `{"type","payload"}` 信封、回包却不加）。

### 3.5 日志桥（internal/logger）

kratos 的 `log.Logger` 签名不携带 `context.Context`，因此：

- 每条转发日志打 `_rpc_kratos` tag（`log.RegisterRPCTag("kratos", "")`）—— 可用
  go-spring 的 logger tag 配置过滤/路由；
- `log.FieldsFromContext` 的 trace-id 传播在这条路径上不可用；
- 记录的 caller（file:line）指向桥内部，而非真实发射点。

级别映射在共享的五级上一对一（kratos 没有 Trace/Panic）。kratos 的 `msg` keyval 提升为
事件消息；冗余的 `level` keyval 丢弃；其余全部作为结构化字段转发。

---

## 4. Beans 与 API

### 4.1 提供的 bean（按激活家族）

| Bean | 类型 | 说明 |
|------|------|------|
| HTTP server | `*StarterKratosHttp.HttpServer`，导出为 `gs.Server` | 包住 `kratos.App`；仅在 `spring.kratos.http.server.addr` 存在时创建。 |
| gRPC server | `*StarterKratosGrpc.GrpcServer`，导出为 `gs.Server` | grpc 前缀同理。 |
| WS server | `*StarterKratosWs.WsServer`，导出为 `gs.Server` | ws 前缀同理。 |

### 4.2 你必须提供的 bean（按家族）

| 家族 | Bean 类型 | 典型写法 |
|------|-----------|----------|
| http | `StarterKratosHttp.ServiceRegister = func(hs *khttp.Server) error` | `v1.RegisterGreeterHTTPServer(hs, svc)` |
| grpc | `StarterKratosGrpc.ServiceRegister = func(gs *kgrpc.Server) error` | `v1.RegisterGreeterServer(gs, svc)` |
| ws | `StarterKratosWs.ServiceRegister = func(ws *kws.Server) error` | `kws.RegisterServerMessageHandler(ws, msgType, handler)` |

注入为非可空：0 个 bean → 容器失败；同类型 2 个 → 歧义失败。每家族恰好一个。
starter 不认识你的服务类型 —— 注册完全在你的 bean 里。

### 4.3 程序化 API

只有上面两个 bean：没有导出的 client 帮助函数、没有导出的 middleware 构造器、没有
`ApplyMiddlewares` 等价物。中间件链在 Run 里固定（§2.2）。目前无法追加 kratos
middleware —— 见设计嫌疑清单。

### 4.4 验证与故障演练

```bash
# HTTP：一次往返 + 经 starter-otel prometheus exporter 读指标
curl -i :8000/helloworld/kratos
curl -s :9464/metrics | grep -E 'server_requests_code_total|server_requests_seconds'

# gRPC：任意客户端打 discovery endpoint；框架日志落在 _rpc_kratos
grep _rpc_kratos ../logs/provider.log | head

# WS：手工构造一帧（任意语言）；应答同格式
# 4 字节小端 msgType=1 || {"name":"Kratos"} → 应答 4 字节小端 1 || {"message":"Hello Kratos"}

# 关停演练：SIGTERM → 日志出现 "kratos grpc server shutting down on <addr>"，
# etcd key 被移除：
ETCDCTL_API=3 etcdctl get --prefix /microservices/kratos-grpc/   # 停止后为空
```

可观测面：

- **Metrics**（http/grpc，默认开）：counter `server_requests_code_total` 与 histogram
  `server_requests_seconds`，经 `otel.Meter(cfg.Name)` 创建 —— 属性跟随 kratos metrics
  中间件默认（`operation`、`code`）；按 server `name` 隔离。
- **Tracing**：每请求一个 span（`tracing.Server()`）；打流量后到 collector 看。
- **日志**：框架行在 `_rpc_kratos`；业务行用你自己的 tag；启停行在 app-def tag。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 什么都没起，没有 kratos 日志行 | addr key 缺失 | 设置 `spring.kratos.<family>.server.addr` —— 它就是开关。 |
| 容器失败：ServiceRegister 缺失 | 设了 addr 但没提供 bean | 为该家族提供恰好一个 `ServiceRegister` bean。 |
| 容器失败：bean 歧义 | 同类型两个 `ServiceRegister` | 只保留一个。 |
| gs 报 HTTP "no handlers registered" | 内建 HTTP server 未关 | `spring.http.server.enabled=false`。 |
| 发现侧消费方找不到服务 | `etcd.addr` 空/错，或 `name` 不匹配 | 设置 `etcd.addr`；`name` 与客户端 `discovery:///<name>` 对齐。 |
| Run 失败："failed to create etcd client" | 构造期 etcd 不可达 | 启动 etcd（示例 docker-compose）或留空 `etcd.addr`。 |
| 一切正常但没有 trace/metrics | 未 import starter-otel | 加上；OTel 钩子缺它是静默 no-op。 |
| WS 永远收不到应答 / "session not found" | 依赖漂移出 v1.3.1 锁，或按 text 模式拼帧 | 保持锁版本；说 binary 帧 `<4B 小端 type><JSON>`。 |
| WS 客户端握手 404 | 客户端 path ≠ `path` key | 对齐 URL path（默认 `/`）。 |
| kratos 日志缺 trace id / file:line 不对 | 桥的限制：kratos `Log` 无 ctx | 已接受的取舍（§3.5）；改用 `_rpc_kratos` 过滤。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 6（http）+ 6（grpc）+ 5（ws） |
| 必填 | 每家族 1 个（`addr`）+ 1 个 bean |
| quickstart 前置外部依赖 | 直连 0 个；注册 1 个（etcd） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（交设计裁决账本）：

- metrics 开关键此处叫 `enable`，而 goframe/go-zero 家族叫 `enabled`（跨家族不一致）。
- ws transport 锁死在 fork 的 v1.3.1 且上游未修；text 模式不对称迫使固定 binary 分帧、
  无配置逃生门。
- 所有子 server 均无 health indicator（除通用 gs.Server 信号外，不喂 gs actuator 就绪）。
- 中间件链硬编码在 Run —— 没有配置或 bean 缝隙追加 kratos middleware（与
  starter-echo/gin 相比缺 fault/governance/request-id/access-log 对等层）。
- ws 的 etcd 注册仅为对齐：WS 客户端没有 discovery 钩子，该 key 广告了一个 WS 侧
  无法被发现的服务。
- 日志桥丢失上下文（无 trace id、caller 指向桥）—— 源于 kratos `log.Logger` 签名，
  若 kratos 未来出 ctx 变体值得重评。
