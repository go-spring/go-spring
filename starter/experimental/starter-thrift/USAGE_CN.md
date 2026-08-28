# starter-thrift 使用说明 — 参考手册

详细使用文档，概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`middleware.go`——注意包名为 `StarterThrift`，属 experimental/ 模块，
即**未审核标记，不是质量分级**）与可运行的 [example/](example/)（`example/check.sh`
自断言）和 [example-otel/](example-otel/) 核对。**Thrift 自身语义**（IDL、代码生成、
protocol/transport 线上规则、TSimpleServer 行为）见 [apache/thrift 官方文档](https://thrift.apache.org/)
——本文只写 go-spring 的增量：装配、生命周期与可观测包装。按项目决策，go-spring 不追求
thrift 协议建设的完善；协议层事务依赖成熟框架，本 starter 有意做成 RPC 家族中最薄的一个。

**激活条件**：只有配置了 `spring.thrift.server.addr` 才注册 server bean
（`starter.go` init 里的 `gs.OnProperty("spring.thrift.server.addr")`）——该 key 即开关；
没有 `enabled` key，也**没有默认端口**（必须显式配置；端口是启动条件）。server 是
apache/thrift 的 `TSimpleServer`（单 accept 循环、每连接一个 goroutine）——Go 版 Thrift
库唯一的 server 模型。

---

## 1. 完整工程示例

一个真实 Thrift 服务：IDL → 生成的 processor → 包装（中间件）processor → server，
外加 trace/metrics 导出。目录树（与 `example/` 同构）：

```
demo/
├── go.mod
├── main.go
├── handler.go
├── middleware.go
├── idl/
│   └── echo.thrift
└── conf/
    └── app.properties
```

**idl/echo.thrift**（用 Apache Thrift 编译器生成，
`thrift -r --gen go:package_prefix=demo/idl/ idl/echo.thrift`；生成代码落在
`idl/gen-go/`，像 example 的 `idl/proto/` 一样入库）：

```thrift
namespace go proto

struct EchoRequest {
1: required string message
}

struct EchoResponse {
1: required string message
}

service EchoService {
  EchoResponse echo(1: EchoRequest req)
}
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"    // 可选：真实 trace/metric 导出
    _ "go-spring.org/starter-thrift"
)

func main() { gs.Run() }
```

**handler.go** —— 服务实现 + processor bean：

```go
package main

import (
    "context"

    "github.com/apache/thrift/lib/go/thrift"
    "go-spring.org/spring/gs"

    "demo/idl/proto" // 你生成的包
)

func init() {
    // 应用提供任意实现 thrift.TProcessor 的 bean。每应用恰好一个（见 §3）。
    gs.Provide(&Controller{})
    gs.Provide(func(c *Controller) thrift.TProcessor {
        return newLoggingProcessor(proto.NewEchoServiceProcessor(c))
    })
}

// Controller 实现 proto.EchoService。
type Controller struct{}

func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
    return &proto.EchoResponse{Message: req.Message}, nil
}
```

**middleware.go** —— 扩展点：TProcessor 装饰器。⚠ 不能天真地"读头、打日志、再调
inner.Process"：输入协议的消息头是一次性的，装饰器必须基于 inner 的 `ProcessorMap()`
重写那几行分发循环——example 的 `loggingProcessor` 正是这么做的：

```go
package main

import (
    "context"

    "github.com/apache/thrift/lib/go/thrift"
    "go-spring.org/log"
)

type loggingProcessor struct{ inner thrift.TProcessor }

func newLoggingProcessor(inner thrift.TProcessor) *loggingProcessor {
    return &loggingProcessor{inner: inner}
}

func (p *loggingProcessor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (bool, thrift.TException) {
    name, _, seqId, err := iprot.ReadMessageBegin(ctx)
    if err != nil {
        return false, thrift.WrapTException(err)
    }
    log.Infof(ctx, log.TagAppDef, "thrift middleware: method=%s seq=%d", name, seqId)
    if fn, ok := p.inner.ProcessorMap()[name]; ok {
        return fn.Process(ctx, seqId, iprot, oprot)
    }
    // 未知方法：复刻生成代码的行为，保证线上协议仍然完好。
    _ = iprot.Skip(ctx, thrift.STRUCT)
    _ = iprot.ReadMessageEnd(ctx)
    x := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "Unknown function "+name)
    _ = oprot.WriteMessageBegin(ctx, name, thrift.EXCEPTION, seqId)
    _ = x.Write(ctx, oprot)
    _ = oprot.WriteMessageEnd(ctx)
    _ = oprot.Flush(ctx)
    return false, x
}

func (p *loggingProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
    return p.inner.ProcessorMap()
}
func (p *loggingProcessor) AddToProcessorMap(name string, fn thrift.TProcessorFunction) {
    p.inner.AddToProcessorMap(name, fn)
}
```

**conf/app.properties** —— 上面用到的完整注释配置（复制自 example/conf）：

```properties
# 关掉 gs 内置 HTTP server，只暴露 Thrift 端口。
spring.http.server.enabled=false

# Thrift server 绑定地址。必填——也是激活开关；没有默认值。
spring.thrift.server.addr=:9292

# server socket 的每连接超时（0 = 无超时）。
spring.thrift.server.clientTimeout=30s

# 线上协议：binary（默认）/ compact / json。客户端必须用匹配的 protocol factory
#（example 用 compact 演练非默认值）。
spring.thrift.server.protocol=compact

# 传输包装：none（裸 socket，默认）/ buffered / framed。客户端必须用匹配的 transport
#（framed：客户端把 TSocket 包进 TFramedTransport）。
spring.thrift.server.transport=framed

# 缓冲大小（buffered）/ 最大帧长（framed），字节。
spring.thrift.server.bufferSize=4096

# observer（tracing/metrics）默认开启；此处显式写出便于发现：
spring.thrift.server.observer.tracing.enabled=true
spring.thrift.server.observer.metrics.enabled=true

# --- 可观测（starter-otel）--------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# TLS（此处关闭以便明文客户端连接）：
# spring.thrift.server.tls.enabled=true
# spring.thrift.server.tls.cert-file=server.crt
# spring.thrift.server.tls.key-file=server.key
```

**客户端**（example 里是进程内调用；独立进程完全相同）——必须匹配服务端的
`protocol=compact` + `transport=framed`：

```go
socket := thrift.NewTSocketConf(":9292", nil)
transport := thrift.NewTFramedTransportConf(socket, nil)
defer transport.Close()
client := proto.NewEchoServiceClientFactory(transport, thrift.NewTCompactProtocolFactoryConf(nil))
if err := transport.Open(); err != nil { /* ... */ }
resp, err := client.Echo(ctx, &proto.EchoRequest{Message: "Hello, Thrift!"})
```

**验证**（与 example 断言同构）：

```bash
cd example && ./check.sh          # 自断言：echo 两次往返 + 装饰器计数 == 2，退出码 0
# 或手动：
cd example && go run .            # 打印 "Response from server: Hello, Thrift!" 等，成功后自 SIGTERM
cd example && go run . -manual    # server 保持运行，供交互探测
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-thrift
  └─ init(): gs.Provide(NewSimpleThriftServer, IndexArg(0, TagArg("${spring.thrift.server}")))
             .Export(As[gs.Server]()) .Condition(OnProperty("spring.thrift.server.addr"))
gs.Run()
  ├─ 配置绑定：${spring.thrift.server} → Config（value tag；NewSimpleThriftServer 以
  │   Debug 打 "thrift server created addr=… protocol=… transport=…"）
  ├─ bean 装配：你的 thrift.TProcessor bean 注入 NewSimpleThriftServer
  │   （缺失/重复 processor bean → 容器失败）
  ├─ gs.Server 阶段 —— SimpleThriftServer.Run(ctx, sig)：
  │   ├─ newTransport()：监听（tls.enabled 时经 tlsconf.Build() 建 TLS）← socket 在此打开
  │   ├─ protocolFactory()/transportFactory()：校验——protocol/transport 名字非法则启动失败
  │   ├─ tracing 或 metrics 开启时 WrapProcessor(proc) —— 最外层 processor
  │   ├─ <-sig.TriggerAndWait()  ← 就绪信号之后才开始 serve
  │   └─ svr.Serve() 阻塞；打 Info "thrift server starting on :9292"（TagAppDef）
  └─ SIGTERM：StopContext → 打 "thrift server shutting down" → s.svr.Stop()
```

### 2.2 Processor 链 —— 精确顺序与理由

```
TSimpleServer → WrapProcessor (observedProcessor) → 你的装饰器 → 生成的 processor
```

- **WrapProcessor 在最外层**（`Run` 中套在你提供的 bean 之外）：每一次调用——包括
  你的装饰器短路或写坏的调用——都被计数、计时、建 span。metric instrument 在
  `WrapProcessor` 时一次性创建并存放在 struct 上（源码注释：observedProcessor
  "avoiding any lazy init on the hot path"）。
- **你的装饰器在内层**：它消费消息头拿到方法名，再经 `ProcessorMap()` 分发
  （为何必须重写分发见 §1）。
- starter 刻意**没有内置中间件链**——无 recover、无 request-id、无 access log、无准入、
  无 fault 注入。装饰器是唯一的横切缝隙（设计口径：不建共享拦截器协议；各家族自带
  最小缝隙）。

### 2.3 一次调用，逐层走读

`client.Echo(ctx, req)`，服务端 `protocol=compact transport=framed`：

1. 客户端把一帧 compact 消息写到 :9292。
2. `TSimpleServer` accept 循环把连接交给一个 goroutine。
3. `observedProcessor.Process`：起 OTel server span `thrift.process`
   （`rpc.system=thrift`；**新根**——见 §4.2），`rpc.server.active_requests` +1，开始计时。
4. 你的装饰器：`ReadMessageBegin` → 打方法名/seq 日志 → 经 ProcessorMap 分发 `Echo`。
5. 生成的 processor 解码参数、调 `Controller.Echo`、编码响应。
6. 回程：`observeEnd` 记 `rpc.server.request_count` +1 与
   `rpc.server.request.duration`（秒；桶 5ms…10s），维度
   `rpc.thrift.status_code=ok|error`；in-flight −1；span 结束——有 TException 时 span
   额外带 `thrift.error_code`（数字 `TExceptionType`）并置 Error 状态。

---

## 3. 逐 key 行为参考

全部 key 在 `spring.thrift.server.*` 下。已与
`grep -rhoE 'value:"[^"]+"' … | sort -u` 核对（10 个 tag；tls 子 key 来自 cloud/tlsconf）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `addr` | string | — | **激活 key 且必填。** 配置即注册 server bean（`OnProperty`）。也是监听地址；TLS 路径换用 `NewTSSLServerSocketTimeout`。 | 缺失 → starter 静默不激活。地址非法 → `Run` 在 listen 失败（"failed to listen on …"）。 |
| `clientTimeout` | duration | 0 | 传给 `NewTServerSocketTimeout`/SSL 变体的 socket 超时——约束**每连接**读写。0 = 无超时。 | 过低 → 慢客户端/慢请求被中途杀掉。 |
| `protocol` | string | binary | 枚举：`binary` / `compact` / `json` / `header`（空串 = binary），`protocolFactory()` 映射。`header`（THeaderProtocol）自带帧与消息头——唯一支持 W3C trace 传播的模式（§4.2），搭配 `transport=none`。⚠ 客户端 protocol factory **必须**匹配。 | 未知值 → 启动失败（`unknown thrift protocol %q`）。客户端不匹配 → 读乱/死锁，不是干净报错。 |
| `transport` | string | none | 枚举：`none`（裸 socket，恒等 factory——历史默认）/ `buffered` / `framed`（framed 时 `MaxFrameSize = bufferSize`），`transportFactory()` 映射。⚠ 客户端 transport 必须匹配；跨语言客户端通常要求 `framed`。 | 未知值 → 启动失败。不匹配 → 挂起/乱码。`framed` 且 `bufferSize` 过小 → 帧被拒。 |
| `bufferSize` | int | 4096 | 仅对 `buffered`（缓冲大小）与 `framed`（最大帧长）有意义。⚠ `transport=none` 时是死 key。 | 小于实际负载 → framed transport 运行时报错。 |
| `tls.enabled` | bool | false | `newTransport()` 切到 SSL server socket。 | 期望 TLS 却得到明文 server。 |
| `tls.cert-file` / `tls.key-file` | string | — | ⚠ `tls.enabled=true` 时必须成对提供（否则 `tlsconf.Build()` 报错 → listen 失败）。 | 启动期 listen 报错 "thrift: build TLS"。 |
| `tls.ca-file` / `tls.server-name` / `tls.insecure-skip-verify` | — | — | **服务端死 key**：它们是 tlsconf 的客户端校验项；服务端路径只调 `Build()`，从不校验客户端证书（无 mTLS）。 | 期望 mTLS → 静默缺失。 |
| `observer.tracing.enabled` | bool | true | 给 processor 套 OTel span 层。无 starter-otel 的 provider 时是 no-op（静默）。⚠ **不可热切换**——`Run` 中一次性判定。 | — |
| `observer.metrics.enabled` | bool | true | 给 processor 套 metrics 层（同样注意事项）。⚠ example-otel 的 conf 历史上写的是 `…server.interceptor.*` key——那是死 key；只是因为默认值是 `true` 才碰巧生效。请用 `observer.*`。 | 前缀写错（`interceptor.*`）→ 静默忽略，observer 仍然开启。 |

---

## 4. 验证与故障演练

### 4.1 装配 + 装饰器

```bash
cd example && ./check.sh    # 断言：echo 两次往返体一致、装饰器恰好触发 2 次
```

example 成功后自 SIGTERM，顺带验证了关停路径（`StopContext`）。

### 4.2 可观测读数

配 starter-otel + Prometheus 导出（`example-otel/conf/app.properties`）：

```bash
cd example-otel && docker compose up -d      # Jaeger（OTLP :4317，UI :16686）
go run .                                     # 发 20 个 Echo RPC，校验 Jaeger API，退出码 0
curl -s :9090/metrics | grep -E 'rpc_server_request_(count|duration)|rpc_server_active_requests'
```

- **Span**：名称 `thrift.process`，kind server，属性 `rpc.system=thrift`；出错时另有
  `thrift.error_code` + Error 状态。trace 传播取决于 protocol：
  - `protocol=header`（THeaderProtocol）：W3C trace-context 传播**可用**——客户端把
    `traceparent` 注入消息头（Go 客户端用 `THeaderProtocol.SetWriteHeader`），服务端经
    全局 OTel propagator 提取，`thrift.process` 挂到远端 span 之下（实现在 `middleware.go`
    `observedProcessor.Process`；未 import starter-otel 时为 no-op，与 starter-kitex 同样的
    global-first 模式）。
  - `binary` / `compact` / `json`：线上格式没有 header 通道——每个 span 都是**新根**，
    跨服务 trace 互不相连。要加就得自定义协议信封 = 破坏性线上变更，已否决（见 §6）。
- **指标**：`rpc.server.request_count`（counter）、`rpc.server.request.duration`
  （秒直方图，显式桶 0.005…10）、`rpc.server.active_requests`（up-down gauge），
  均带 `rpc.system=thrift`；count/duration 另加 `rpc.thrift.status_code=ok|error`。
- **日志**：starter 只打 app-def 行（"thrift server created/starting/shutting down"，
  tag `app-def`）加你的装饰器自己打的——没有 access log。

### 4.3 故障演练

**没有——且是有意为之。** 本 starter 无 fault 注入、无 resilience 准入、无 loadtest
识别、无 governance 挂点，也不会加：thrift starter 是仓内最薄的协议家族。生产级
thrift 治理请用成熟框架（如 contrib/kitex）。能做的演练只有 protocol/transport
不匹配（§5）与 kill -9。

### 4.4 关停

`SIGTERM` → `StopContext` 打日志并调 `thrift.TSimpleServer.Stop()`，后者关闭 server
transport、打断 accept 循环。thrift 的 `TSimpleServer.Stop` 并不等待每连接 goroutine
——**没有优雅排空**；把关停当作"停止 accept、丢弃滞留者"看待。这是声明的边界而非
待修 bug：需要连接排空的 thrift 服务请用成熟框架。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| starter 完全不生效、无 thrift 日志 | 缺 `spring.thrift.server.addr` | 配上——它既是端口也是开关（无默认值）。 |
| 容器装配失败 | 提供了 0 个或多个 `thrift.TProcessor` bean | 恰好提供一个（`NewSimpleThriftServer` 的不可空 ctor 参数）。 |
| 启动失败：`unknown thrift protocol/transport %q` | `protocol`/`transport` 拼写错误 | 用 binary/compact/json/header 与 none/buffered/framed。 |
| 客户端连上后挂起 / 乱码报错 | 客户端↔服务端 protocol 或 transport 不匹配 | 对齐 factory：framed↔TFramedTransport、compact↔TCompactProtocol（example 两侧都钉死）。 |
| framed transport 运行期帧错误 | `bufferSize` < 实际帧大小 | 调大 `spring.thrift.server.bufferSize`。 |
| 启动失败：`thrift: build TLS` | `tls.enabled=true` 但没有证书对（或文件坏） | 提供 `tls.cert-file` + `tls.key-file`。 |
| 期望 mTLS 但客户端没被校验 | 服务端走 `tlsconf.Build()`；`ca-file` 等是死 key | 不支持；需要就作为设计问题提出。 |
| observer "开着"却无 trace/metrics | 未 import starter-otel，或 key 前缀写错（`interceptor.*`） | import starter-otel；用 `observer.tracing.enabled`/`observer.metrics.enabled`。 |
| trace 表现为互不相连的根 | 用了 binary/compact/json protocol，本身无传播 carrier | 预期行为；两侧都换 `protocol=header` 即有 W3C 传播，否则靠时间/服务名关联。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | starter 侧 10 个（另有 3 个死的 tls 客户端 key） |
| 必填 | 1（`addr`；`tls.cert-file`/`key-file` 条件必填） |
| quickstart 外部依赖 | 0（可观测 example 才需要 Jaeger） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单：

1. example/conf 注释声称 ":9292 默认值"——不存在（仍在；是 conf 文件，非 Go 源码）。
2. ~~缺 trace 传播~~ **已解决**：`protocol=header` 时服务端从 THeaderProtocol 消息头
   提取 W3C trace context，`thrift.process` 挂到远端 span 之下（middleware.go）。
   binary/compact/json 要传播必须自定义协议信封（破坏性线上变更）——声明边界，不做。
3. ~~无优雅排空~~ **声明边界**：`TSimpleServer.Stop` 只关 transport；此处不建排空，
   生产 thrift 请用成熟框架。
4. 无 fault 注入 / resilience 准入 / health 服务——与 starter-grpc、gin 不对称，
   已拍板为有意如此（"最薄协议家族"；生产 thrift 依赖成熟框架）。
5. example-otel conf 用了死的 `spring.thrift.server.interceptor.*` key——只因 observer
   默认为 true 才碰巧生效。
6. tlsconf 客户端侧 key（`ca-file`/`server-name`/`insecure-skip-verify`）绑定了但在
   服务端是死 key；无 mTLS 姿态。
