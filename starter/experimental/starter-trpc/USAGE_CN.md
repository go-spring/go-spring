# starter-trpc 使用说明 — 参考手册

详细使用文档，概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`filter.go`、`fault.go`、`loadtest.go`、`internal/logger/logger.go`）与可运行的
[example/](example/) / [example-otel/](example-otel/) 核对。**tRPC-Go 自身语义**（filter、
服务配置、`trpc_go.yaml` 约定、生成 stub）见 [tRPC-Go 官方文档](https://trpc.group/trpc-go/trpc-go)——
以下只写 go-spring 的增量。

**激活条件**：只有配置了 `spring.trpc.server.addr` 才会注册 server bean——该 key 即开关
（没有 `enabled` key）。与原生 tRPC-Go 不同，**没有 trpc_go.yaml**：starter 用这些属性
编程式构建 `*trpc.Config` 并调用 `trpc.NewServerWithConfig`（见 `SimpleTrpcServer.Run`）。

---

## 1. 完整工程示例

一个带 tracing、metrics 与运行期 fault 注入的真实 tRPC greet 服务。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
├── idl/                      # 生成物：greet.pb.go / greet.trpc.go（trpc-go codegen）
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod**（关键依赖）：

```
require (
    trpc.group/trpc-go/trpc-go   latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-trpc   latest
    go-spring.org/starter-otel     latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest // 可选：运行期 fault 注入
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-trpc"
)

func main() { gs.Run() }
```

**service.go** —— 应用全部的 RPC 面：

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterTrpc "go-spring.org/starter-trpc"
    greet "demo/idl"
    "trpc.group/trpc-go/trpc-go/server"
)

func init() {
    // 应用提供且仅提供一个 ServiceRegister bean。starter 持有 *server.Server
    // 及其生命周期；你只需把生成的服务绑到它递给你的 server 上。
    gs.Provide(func() StarterTrpc.ServiceRegister {
        return func(s *server.Server) {
            greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
        }
    })
}

type GreetServiceImpl struct{}

func (s *GreetServiceImpl) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
    // trpclog 调用经日志桥进入 go-spring log（见 §4.1）。
    return &greet.GreetResponse{Greeting: "Hello, " + req.Name + "!"}, nil
}
```

**conf/app.properties** —— 上述代码实际用到的完整带注释配置面：

```properties
# --- trpc server -------------------------------------------------------------
# 关闭 gs 内建 HTTP server；本进程只暴露 tRPC 端口。
spring.http.server.enabled=false

# 激活 key：host:port，拆为 tRPC 的 IP/Port 字段。
spring.trpc.server.addr=127.0.0.1:8000

# 完整 tRPC 服务名（trpc.app.server.service）。必须与生成客户端 stub 里
# 烧入的 callee 名一致。
spring.trpc.server.service.name=trpc.demo.greet.GreetService

# 线协议 / 网络（默认值如下）。
spring.trpc.server.network=tcp
spring.trpc.server.protocol=trpc

# starter 自带的 server filter；默认全开（无 starter-otel 时为 no-op）。
spring.trpc.server.observer.tracing.enabled=true
spring.trpc.server.observer.metrics.enabled=true
spring.trpc.server.loadtest.enabled=true

# --- 可观测（starter-otel） ---------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- 治理（运行期 fault 注入） -------------------------------------------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml**（§4.4 的故障演练用）：

```yaml
govern:
  enabled: true
  fault:
    enabled: false        # 置 true 即免重启"放火"
    rate: 0.2
    error: timeout
    scope: loadtest       # 只影响带 x-loadtest 标记的流量
```

**验证**（与自断言的 example/check.sh 同构）：

```bash
go run . -manual          # 另开终端：
grep -c "trpc server starting" <stdout>   # 生命周期日志已出现
# example 内置 check 用 ip://127.0.0.1:8000 拨号并断言往返
./example/check.sh
```

example 的客户端是直连（`client.WithTarget("ip://127.0.0.1:8000")`）；tRPC 客户端语义
（target scheme、调用方式）属 tRPC-Go 自身。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-trpc
  └─ （副作用 import）internal/logger init(): trpclog.SetLogger(bridge)  [进程级，
      先于任何 tRPC 组件捕获默认 logger]
  └─ gs.Provide(NewSimpleTrpcServer, IndexArg(0, TagArg("${spring.trpc.server}")))
        .Export(gs.As[gs.Server]())
        .Condition(gs.OnProperty("spring.trpc.server.addr"))
        │
gs.Run()
  ├─ 配置绑定：${spring.trpc.server} → Config（value tag）
  ├─ bean 装配：SimpleTrpcServer ← 应用的 ServiceRegister bean（不可空）
  ├─ Server.Run():
  │    ├─ SplitHostPort(addr) → IP/Port（两段都必须有，见 §3）
  │    ├─ 代码内构建 *trpc.Config（单个 ServiceConfig；无 trpc_go.yaml，
  │    │  无 naming/registry 插件——直连 starter）
  │    ├─ trpc.NewServerWithConfig(cfg)
  │    ├─ filter.Register："tracing"/"metrics"/"loadtest"（各受配置控制，
  │    │  默认全开）与 "fault"（恒注册）
  │    ├─ reg(svr)：应用的 ServiceRegister 绑定生成的服务
  │    └─ <-sig.TriggerAndWait()        # 挂起直到 gs 发就绪信号
  │         go svr.Serve()              # 绑定监听并阻塞
  │         select { Serve-err | <-done }
  ├─ 就绪：TriggerAndWait 返回 → Serve 启动
  └─ SIGTERM：gs 调 Stop → svr.Close(nil) 解除 Serve 阻塞 → close(done) → Run 返回
```

`ServiceRegister` 缺失则容器装配失败（不可空注入）；两个 `ServiceRegister` bean 报
歧义错误。

**信号**（`SimpleTrpcServer` 源码注释）：tRPC 的 `Serve()` 会装自己的
SIGINT/SIGTERM/SIGSEGV/SIGUSR2 处理器。它与 gs 生命周期共存——gs 关停调 `Stop()` →
`server.Close(nil)`；直接 SIGTERM 两边都会捕获，无害：谁先到谁先把 server 拆掉。

### 2.2 Server filter —— 跑什么、什么顺序

starter **不装固定链**。它把命名 filter 注册进 tRPC 全局 filter 注册表
（`filter.Register(name, fn, nil)`）；是否执行、顺序如何遵循 tRPC 自身的 filter 组合
规则，即生成代码 / 服务配置引用了哪些 filter 名。各 filter 行为：

| Filter | 注册条件 | 行为 |
|--------|---------|------|
| `loadtest` | `loadtest.enabled`（默认 true） | 读入站 server metadata 的 `x-loadtest` 键；肯定值则经 `traffic.WithLoadTest(ctx, "trpc-metadata")` 打标，下游所有层（及你的 handler，用 `traffic.IsLoadTest(ctx)`）都能分支。无标记时 no-op。放**最前**，让标记先于 tracing/metrics/handler 落 ctx。 |
| `tracing` | `observer.tracing.enabled`（默认 true） | 起名为 `{calleeService}/{method}` 的 OTel server span，属性 `rpc.system=trpc`、`rpc.service`、`rpc.method`；出错置 span 状态 + RecordError。挂在 OTel 全局上——无 starter-otel 时 no-op。 |
| `metrics` | `observer.metrics.enabled`（默认 true） | `rpc.server.request_count` 计数器、`rpc.server.request.duration` 直方图（秒，显式桶 5ms..10s）、`rpc.server.active_requests` UpDownCounter，均带 `rpc.method` 属性。挂在 OTel 全局上。 |
| `fault` | 恒注册 | `fault.Apply(ctx, fault.InjectorFor(), "trpc", next)` —— 按治理规则注入延迟/错误；未配置时透明直通。 |

推荐链（starter 注释的处方）：`loadtest` 最前，随后 `tracing`、`metrics`、`fault`，
最后 handler —— 与 HTTP 系 starter 同理：标记须先落 ctx 才能被分支；fault 贴着 handler，
注入的错误仍会穿过 metrics/tracing 流出，保持可观测。

### 2.3 一次调用逐层走读

客户端 `Greet("world")`，metadata 带 `x-loadtest: 1`，fault scope=loadtest 已生效：

1. `loadtest` 读 `ServerMetaData()["x-loadtest"]` → ctx 打标 `traffic.IsLoadTest()==true`
2. `tracing` 起 server span `trpc.demo.greet.GreetService/Greet`
3. `metrics`：active_requests +1，时长观测开始
4. `fault`：`fault.Apply` 咨询治理 injector；scope=loadtest 且有标记时，约 `rate` 比例
   的调用被注入延迟/错误，其余直通
5. handler 执行；应答按 3→2 展开回收：计数器/直方图带 `rpc.method` 记录，
   span 结束（失败则 Error 状态 + 记录错误）

---

## 3. 逐 key 行为参考

全部 key 在 `spring.trpc.server.*` 下（共 8 个）。仅 `addr` 必填。

### 3.1 Server 核心

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `addr` | string | — | **激活 key**。存在即注册 server bean。用 `net.SplitHostPort` 拆分；host 必须存在（`:8000` 裸端口形式拆分失败 → Run 报错）。 | 缺失 → 整个 starter 静默不生效。裸 `:port` → 启动报 "failed to parse addr"。 |
| `service.name` | string | `trpc.helloworld.greet.GreetService` | 完整 tRPC 服务名（`trpc.app.server.service` 约定）。⚠ 默认值是 tRPC helloworld 示例的**演示**服务名——务必显式配置。 | 与客户端 stub 的 callee 不一致 → 客户端无法路由/反序列化（直连按地址拨号，可能表现为协议/callee 不匹配而非绑定错误）。 |
| `network` | string | tcp | 传给 tRPC ServiceConfig.Network。 | 非 tcp 需要 tRPC 侧对应传输支持。 |
| `protocol` | string | trpc | 传给 tRPC ServiceConfig.Protocol（线协议，非传输）。 | 客户端必须同协议。 |

### 3.2 Filter

| Key | 类型 | 默认值 | 行为 | 配错后果 |
|-----|------|--------|------|---------|
| `observer.tracing.enabled` | bool | true | 注册 `tracing` server filter。无 starter-otel 全局时 no-op——无任何告警。 | false（或未 import starter-otel）→ 静默无 span。 |
| `observer.metrics.enabled` | bool | true | 注册 `metrics` server filter。同样 no-op。 | false/无 starter-otel → 静默无 RPC 指标。 |
| `loadtest.enabled` | bool | true | 注册 `loadtest` server filter。 | false → 压测标记不再随这一跳传递，下游 `traffic.IsLoadTest` 恒 false。 |

⚠ **注册 ≠ 执行**：filter 进的是 tRPC 全局名字注册表；只有生成服务配置引用这些名字
才会执行（tRPC filter 组合规则）。若你的 codegen 产出自己的 filter 列表，把
`loadtest`/`tracing`/`metrics`/`fault` 加进去。

### 3.3 死的 `interceptor.*` key（已修复）

`example-otel/conf/app.properties` 原先配的是
`spring.trpc.server.interceptor.tracing.enabled` / `...interceptor.metrics.enabled`——
从兄弟 starter 抄来的陈旧配置。starter 里没有任何代码绑定 `interceptor.*` 前缀
（模块内 grep："interceptor" 仅出现在一行注释里）；example 之前能跑只因生效 key
默认即 true。**已修复**：配置改用生效的 `spring.trpc.server.observer.tracing.enabled` /
`spring.trpc.server.observer.metrics.enabled`（§3.2），`schema.json` 也补上了
`observer.*` / `loadtest.*` 子树。新配置不要再复制 `interceptor.*`——它不绑定任何代码。

---

## 4. 验证与故障演练

### 4.1 日志桥

import starter 即可：`internal/logger` 的 `init()` 在任何 tRPC 组件捕获默认 logger 之前
调用 `trpclog.SetLogger`，tRPC 的 server 接线、传输错误及 handler 里的 `trpclog.Infof`
全部流入 go-spring 日志管道。

- 每行桥接日志带 RPC 日志 tag `trpc`（经 `log.RegisterRPCTag("trpc", "")` 注册）——用
  常规 `logger.<tag>` 绑定即可路由到专属 logger。
- tRPC 的基础 Logger 不带 ctx：桥接行走 `context.Background()`，此路径无
  trace_id/span_id（与其他框架桥同样受限）。
- tRPC 的 `SetLevel`/`With` 变成 no-op——级别过滤与字段归 go-spring 管。

验证（example/conf 正是靠这一点）：未配 `${logging.logger}` sink 时运行 example——
tRPC 自身日志出现在 go-spring 默认控制台。

```bash
./example/check.sh 2>&1 | grep -i "trpc"   # 桥接的框架日志出现在 stdout
```

### 4.2 Trace 与指标

import starter-otel 后（example-otel）：

```bash
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :16686 / OTLP :4317
cd example-otel && go run .                               # 发 20 个 RPC 并自验
open http://localhost:16686    # 服务 "trpc-otel-example"：每 RPC 一个 span，名为
                               # {service}/{method}，属性 rpc.system=trpc
```

指标（经 starter-otel 的 Prometheus 导出时）：

```bash
curl -s :9090/metrics | grep -E 'rpc_server_request_duration|rpc_server_request_count|rpc_server_active_requests'
```

### 4.3 生命周期验证

```bash
go run . -manual -conf=conf &   # 日志："trpc server starting on 127.0.0.1:8000"
kill -TERM %1                   # 日志："trpc server shutting down ..." 后进程退出 0
```

`Stop` 调 `svr.Close(nil)`；tRPC 关内部 closeCh 解除 Serve 阻塞。在途请求**不排空**——
见 §6。

### 4.4 故障演练（免重启）

1. 按 §1 的 `govern.yaml` 启动（`fault.enabled: false`），filter 链里有 `fault`。
2. 打基线流量 → 全部成功。
3. 把文件里 `fault.enabled` 置 true —— 治理 source 热加载；filter 每次调用都重新解析
   `fault.InjectorFor()`，无需重启。
4. 带标流量着火（客户端调用 metadata 带 `x-loadtest: 1`），正常流量不受影响：约 20% 的
   带标调用被注入 `timeout`。
5. 观测着火：`rpc.server.request.duration` 桶位移动、被注入调用的 span 带 Error 状态、
   `rpc.server.request_count` 照常计数。置回 false 灭火。

### 4.5 压测标记录演练

`scope: real` 反转过滤——不带标的流量着火；只在专用环境使用。handler 里
`traffic.IsLoadTest(ctx)` 分支同一个标记（由 `loadtest` filter 从入站 metadata 打上），
业务代码可在合成压力下降级功能。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| server 不启动、无 trpc 日志 | 缺 `spring.trpc.server.addr` | 配上——该 key 即激活开关。 |
| 启动报 "failed to parse addr" | 裸 `:port` 形式 | 用 `host:port`（如 `127.0.0.1:8000`）；SplitHostPort 要求 host 段。 |
| 容器失败：ServiceRegister 缺失/歧义 | 应用提供 0 个或 2+ 个 bean | 提供且仅提供一个 `StarterTrpc.ServiceRegister`。 |
| 一切正常但没有 trace/指标 | 未 import starter-otel，或 `observer.*.enabled=false` | import starter-otel；filter 静默挂在其 OTel 全局上。 |
| filter 已注册但不执行 | 生成服务配置未引用这些名字 | 把 `tracing`/`metrics`/`loadtest`/`fault` 加进服务 filter 链（§3.2 ⚠）。 |
| 直连地址正确但客户端调不通 | `service.name` 与 stub callee 不一致 | 两端对齐；别依赖演示默认值。 |
| 关停挂起或丢在途请求 | 预期行为：`Close(nil)` 无排空 | 暂且接受；见 §6 嫌疑。 |
| 配了 `spring.trpc.server.interceptor.*` 没反应 | 死前缀（§3.3） | 改用 `spring.trpc.server.observer.*`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 8 |
| 必填 | 1（`addr`） |
| quickstart 外部依赖 | 0（完整可观测需 collector） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单：

1. `service.name` 的默认值是**示例**服务名（`trpc.helloworld.greet.GreetService`）——
   借来的演示值，对真实应用不是合理默认。
2. filter 按名字全局注册，生成配置未引用就不会执行——激活语义是隐式的。
3. `addr` 必须是 `host:port`；HTTP starter 常见的裸 `:port` 形式会失败。
4. 关停无优雅排空（`svr.Close(nil)`）；无 resilience 准入（限流/熔断）——与 starter-grpc
   不同，此处未接线。
5. 死 key：`example-otel/conf` 的 `spring.trpc.server.interceptor.*` 前缀——已修复，
   配置改用生效的 `spring.trpc.server.observer.*` key（§3.3）。
6. `schema.json` 缺 `observer.*` / `loadtest.*` 子树——已修复，两个子树均已补上
   （`enabled` 默认 true）。
