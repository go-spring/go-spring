# starter-kitex 使用说明 — 参考手册

详细使用文档，概览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`internal/logger/logger.go`、`schema.json`）与可运行的
[example/](example/) / [example-otel/](example-otel/) 核对。**Kitex 自身语义**（thrift/protobuf
代码生成、`kitex_gen`、middleware suite）见 [Kitex 官方文档](https://www.cloudwego.io/zh/docs/kitex/)——
以下只写 go-spring 的增量。

**激活条件**：只有配置了 `spring.kitex.server.addr` 才会注册 server bean——该 key 即开关
（没有 `enabled` key；example/conf 里 "默认 :8888" 的注释有误，不存在默认值）。starter 内联了
生成代码 `xxxservice.NewServer` 会做的事——构造裸 `server.Server`、把服务绑定推迟到你的
`ServiceRegister` bean——server 完全由 conf/app.properties 配置。

---

## 1. 完整工程示例

一个带 tracing、Prometheus 指标、可选 etcd 部署形态的 thrift echo 服务。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
├── kitex_gen/echo/…            # 由 echo.thrift 生成（kitex 工具）
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/cloudwego/kitex      latest
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-kitex     latest
    go-spring.org/starter-actuator  latest   // 可选：探针（本 server 未内建）
)
```

注意：本最小姿势不需要 starter-otel——没有它时 starter 回落到自有 OTel provider（§4.2）。
同时 import 亦可：kitex 会自动收敛挂上统一管道，两者不会双导出（§4.2）。

**main.go**：

```go
package main

import (
    _ "demo/service"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-kitex"
)

func main() { gs.Run() }
```

**service.go** —— 应用全部的 RPC 面：

```go
package service

import (
    "context"

    "github.com/cloudwego/kitex/server"
    "go-spring.org/spring/gs"
    StarterKitex "go-spring.org/starter-kitex"
    echo "demo/kitex_gen/echo"
    "demo/kitex_gen/echo/echoservice"
)

func init() {
    // 应用提供且仅提供一个 ServiceRegister bean。starter 持有裸 server.Server
    // 及其生命周期；你只需把生成的服务绑到它递给你的 server 上。
    gs.Provide(func() StarterKitex.ServiceRegister {
        return func(svr server.Server) error {
            return echoservice.RegisterService(svr, &EchoServiceImpl{})
        }
    })
}

type EchoServiceImpl struct{}

func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
    return &echo.EchoResponse{Message: req.Message}, nil
}
```

**conf/app.properties** —— 上述代码实际用到的完整带注释配置面：

```properties
# --- kitex server -------------------------------------------------------------
# 关闭 gs 内建 HTTP server；本进程只暴露 Kitex 端口。
spring.http.server.enabled=false

# 激活 key；解析为 TCP addr，经 server.WithServiceAddr 传入。
spring.kitex.server.addr=:8888

# 写入 EndpointBasicInfo 的服务名（开注册时也是 etcd 注册名）。
spring.kitex.server.service.name=echo

# thrift 生成服务必须开启（见 §3）：kitex thrift codegen 会在生成的 NewServer 里加
# WithCompatibleMiddlewareForUnary；本 starter 自建 server，须显式打开。
# protobuf 服务保持关闭。
spring.kitex.server.compatible-unary-middleware=true

# etcd 注册——可选。留空（不配）跑无注册直连服务；配置即发布服务供发现。
# spring.kitex.server.registry.etcd=127.0.0.1:2379

# --- kitex 可观测（全局优先，见 §4.2）-------------------------------------------
# 两者默认开。import 了 starter-otel 时，tracing suite 直接挂全局 OTel 管道
# （endpoint/insecure 被忽略）；没有全局时 starter 自建 provider 按 OTLP/gRPC
# 导出到 tracing.endpoint。
spring.kitex.server.tracing.enable=true
spring.kitex.server.tracing.endpoint=127.0.0.1:4317
spring.kitex.server.tracing.insecure=true
spring.kitex.server.metrics.enable=true
# 独立 kitex Prometheus 监听只在显式配置 port 时启动（无默认端口）。
# 没有 starter-otel 时配置它即得到 kitex /metrics；有 starter-otel 时
# kitex RPC 指标直接走全局管道。
# spring.kitex.server.metrics.port=9090
spring.kitex.server.metrics.path=/metrics
```

**验证**（与自断言的 example/check.sh 同构）：

```bash
go run . -manual           # 出现 "kitex server starting on :8888"
./example/check.sh         # 拨 :8888，断言 echo 往返，自 SIGTERM
```

example 的客户端直连（`echoservice.NewClient("echo", client.WithHostPorts(":8888"))`）；
kitex 客户端语义（resolver、重试）属 kitex 自身。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-kitex
  └─ （副作用 import）internal/logger init(): klog.SetLogger(bridge)  [进程级，
      先于任何 kitex 组件捕获默认 logger]
  └─ gs.Provide(NewSimpleKitexServer, IndexArg(0, TagArg("${spring.kitex.server}")))
        .Export(gs.As[gs.Server]())
        .Condition(gs.OnProperty("spring.kitex.server.addr"))
        │
gs.Run()
  ├─ 配置绑定：${spring.kitex.server} → Config（value tag）
  ├─ bean 装配：SimpleKitexServer ← 应用的 ServiceRegister bean（不可空）
  ├─ Server.Run():
  │    ├─ net.ResolveTCPAddr(addr)
  │    ├─ server 选项：WithServiceAddr、WithServerBasicInfo{ServiceName}
  │    ├─ registry.etcd 非空 → etcd.NewEtcdRegistry + WithRegistry
  │    ├─ compatible-unary-middleware → WithCompatibleMiddlewareForUnary
  │    ├─ tracing.enable → globalTracingActive() 探测：
  │    │    有全局管道（starter-otel）→ WithSuite(tracing.NewServerSuite())
  │    │      读 OTel 全局——不创建任何 kitex provider
  │    │    无全局 → provider.NewOpenTelemetryProvider(WithServiceName,
  │    │      WithExportEndpoint, WithEnableMetrics(false)) [留存到 Stop 用]
  │    │      + WithSuite(tracing.NewServerSuite())
  │    ├─ metrics.enable → metrics.port>0 → WithTracer(prometheus.
  │    │    NewServerTracer(":port", path))；port 未配 → 走全局管道
  │    │    （两者皆无则休眠并打 INFO 提示）
  │    ├─ server.NewServer(opts…) ; reg(svr) 绑定生成的服务
  │    └─ <-sig.TriggerAndWait()        # 挂起直到 gs 发就绪信号
  │         go svr.Run()                # 绑定监听、注册进 etcd 并阻塞
  │         select { Run-err | <-done }
  ├─ 就绪：TriggerAndWait 返回 → Run 启动
  └─ SIGTERM：gs 调 Stop → svr.Stop()（注销 etcd、优雅关停）
      → otelProvider.Shutdown(ctx)（冲刷 span）→ close(done) → Run 返回
```

`ServiceRegister` 缺失则容器装配失败（不可空注入）；两个 `ServiceRegister` bean 报
歧义错误。

### 2.2 starter 装了什么、没装什么

starter 组合的是 kitex 原生件——一个 tracing **suite**、一个 Prometheus **tracer**、
可选 etcd 注册、可选 compatible-unary middleware——且最后叠加（源码注释："provider 只改
conf/app.properties 就能点亮 metrics 与 tracing"）。它**不自建中间件链**：kitex middleware
在你的 `ServiceRegister` 里组合（或经 codegen 挂 suite）。与兄弟 RPC starter 相比的后果：

- 无 loadtest 识别 filter（本跳不会打 `traffic.IsLoadTest` 标）
- 无 fault 注入 filter（未接 `fault.InjectorFor`；治理中心在此处"放不了火"）
- 无 resilience 准入（限流/熔断）

### 2.3 一次调用逐层走读

客户端 `Echo("Hello")`，tracing+metrics 开（默认），无注册：

1. kitex 传输层在服务地址上接受连接
2. tracing suite（经 `WithSuite(tracing.NewServerSuite())` 装载）提取传播头并打开
   server span——经 §4.2 判定胜出的管道导出：starter-otel 的全局 exporter，或
   kitex 自有的回落 provider
3. suite 同时在 OTel meter 上记录 `kitex.server.duration` 直方图；有全局管道时该指标
   随之导出。显式配置 `metrics.port` 时，Prometheus server tracer 额外在其专属监听上
   记录 kitex RPC 统计
   （指标名见 kitex-contrib/monitor-prometheus 文档）
4. compatible-unary middleware（若开启）适配 thrift unary 调用形态
5. handler 执行；应答回收：span 结束（Stop 时冲刷）、统计落盘，错误（若有）
   反映到两路信号

---

## 3. 逐 key 行为参考

全部 key 在 `spring.kitex.server.*` 下（共 9 个）。仅 `addr` 必填。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `addr` | string | — | **激活 key**。`net.ResolveTCPAddr("tcp", addr)` 解析，经 `server.WithServiceAddr` 传入。 | 缺失 → starter 静默不生效。不可解析 → 启动报 "failed to resolve addr"。 |
| `service.name` | string | `kitex` | 写入 `EndpointBasicInfo`；也是 etcd 注册名与 kitex provider 的 OTel `service.name`。 | 名字错 → 发现找不到；trace 归属错服务。 |
| `registry.etcd` | string | —（空） | 非空 → `etcd.NewEtcdRegistry([]string{addr})` + `WithRegistry`；Run 时注册、`svr.Stop()` 时注销。空 = 无注册直连。 ⚠ 单地址字符串，不是列表。 | etcd 不可用时配置 → 启动报 "failed to create etcd registry"。 |
| `compatible-unary-middleware` | bool | false | 加 `server.WithCompatibleMiddlewareForUnary`。 ⚔ thrift 生成服务需要 `true`——kitex thrift codegen 在生成的 NewServer 里加、protobuf 不加；本 starter 自建 server，须在此显式补开。 | thrift + false → unary 调用行为异常（kitex 兼容语义）。 |
| `tracing.enable` | bool | true | 挂载 kitex tracing suite。⚠ key 是 `enable`，**不是** `enabled`（死 key——example-otel 笔误已修，勿再犯）。有全局管道（starter-otel）时 suite 直接挂全局、不建 kitex provider；无全局时建自有回落 provider。 | `tracing.enabled=…` → 静默忽略，tracing 保持开启。`false` → 完全不挂 suite——kitex 侧 span/指标被显式退出。 |
| `tracing.endpoint` | string | 127.0.0.1:4317 | kitex 自有**回落** provider 的 OTLP/gRPC 导出端点。有全局管道时被忽略。 | 那里没有 collector（回落姿势）→ 运行期导出报错/丢 span。 |
| `tracing.insecure` | bool | true | 回落 provider 加 `provider.WithInsecure()`（明文 OTLP）。有全局管道时被忽略。 | 对明文 collector 设 false → 导出失败。 |
| `metrics.enable` | bool | true | 门控 kitex 指标。同样注意是 `enable` 不是 `enabled`。 | `metrics.enabled=…` → 静默忽略。 |
| `metrics.port` / `metrics.path` | int / string | 0（未配）/ /metrics | 独立 monitor-prometheus HTTP 监听**只在 port 显式配置时启动**（此处从不默认绑端口）。port 未配且有全局管道时，kitex RPC 指标经 tracing suite 的 `kitex.server.duration` 直方图走全局；两者皆无则休眠（INFO 日志说明如何开启）。 | 端口被占 → 库内 `log.Fatal` 在首批流量时杀死进程。 |

### 3.1 example-otel 中曾发现的死 key（已修复）

`example-otel/conf/app.properties` 曾配置 `tracing.enabled` / `metrics.enabled`——无任何
代码绑定（代码绑定的是 `enable`）。2026-08-28 已修复：conf 改为生效的 `…enable` key 并
注明全局路径语义。勿再使用 `enabled` 拼法。

### 3.2 schema.json 与代码不一致（已修复）

`schema.json` 曾声明 `tracing.enable` / `metrics.enable` 默认 **false**、`metrics.port`
默认 **9090**，与代码（true / true / 0）相反。2026-08-28 已修复：schema 与 `starter.go`
一致并补全局优先说明。

---

## 4. 验证与故障演练

### 4.1 日志桥

import starter 即可：`internal/logger` 的 `init()` 在任何 kitex 组件捕获默认 logger 之前
调用 `klog.SetLogger`，kitex 的 server 接线、etcd resolver 事件、传输错误及 handler 里的
`klog` 调用全部流入 go-spring 日志管道。

- 每行桥接日志带 RPC 日志 tag `kitex`（`log.RegisterRPCTag("kitex", "")`）——用常规
  `logger.<tag>` 绑定路由。
- handler 里优先用 `klog.CtxInfof(ctx, …)`：Ctx 路径会穿透你的 ctx，go-spring 的
  `FieldsFromContext` 钩子可从请求上提取 trace_id/span_id。普通 `Info`/`Infof` 路径无
  ctx，丢失 trace 关联。
- kitex 的 `Notice` 级折叠为 go-spring Info；`SetLevel`/`SetOutput` 为 no-op——过滤与
  sink 归 go-spring 管。

验证：不配 `${logging.logger}` sink 运行 example——kitex 框架日志出现在 go-spring
默认控制台。

### 4.2 Trace 与指标 —— 全局优先，一进程一管道

可观测**全局优先**：starter 探测进程全局是否装了真实 OTel TracerProvider
（`globalTracingActive()`——探测 span 的 `IsRecording()`；starter-otel 在其 setup 阶段、
任何 bean 运行前安装全局）。判定矩阵：

| 姿势 | Trace | 指标 |
|------|-------|------|
| import 了 starter-otel（全局管道在） | suite 直接挂**全局** provider/propagator；`tracing.endpoint`/`insecure` 被忽略；**不创建 kitex provider**——无重复 span、无第二条 OTLP 连接 | `kitex.server.duration` 经 suite 的 otel meter 走全局 metrics exporter；独立 kitex 监听只在显式 `metrics.port` 时另起 |
| 只用 kitex（无全局管道） | 回落：starter 自建 `OpenTelemetryProvider` 按 OTLP/gRPC 导出到 `tracing.endpoint`（零配置即有 span；INFO 日志提示 starter-otel） | 显式配置 `metrics.port` 时起独立 Prometheus 监听；未配则休眠并打 INFO 提示（不默认绑端口） |
| `tracing.enable=false` | 完全不挂 suite——显式退出，kitex 侧 span/指标全无 | 单独的 `metrics.enable` 仍尊重显式 `metrics.port` |

迁移说明（2026-08-28）：此前 starter 总是自建 provider 并默认绑 `:9090`，import
starter-otel 即重复 span + 端口冲突。若你依赖 kitex 自有的 `:9090`，请显式配置
`spring.kitex.server.metrics.port=9090`（或把抓取迁到
`spring.observability.metrics.port`）。

注意：全局 provider 若配了 always-off sampler，探测会判"无全局管道"而仍建回落
provider——但该配置本就丢弃所有 span，不会造成双导出。

已核实演练（example-otel 演示全局路径——starter-otel 配置、kitex 挂上去）：

```bash
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :16686 / OTLP :4317
cd example-otel && go run .                               # 20 个 RPC，自验 Jaeger 有 trace
curl -s :9090/metrics | grep kitex                        # starter-otel 的 prometheus exporter
```

### 4.3 注册演练（etcd）

```bash
docker run -d -p 2379:2379 quay.io/coreos/etcd …
# 配 spring.kitex.server.registry.etcd=127.0.0.1:2379，-manual 启动
etcdctl get --prefix "" | grep -A1 echo                   # 服务键已按 service.name 发布
# 消费方按同名解析；SIGTERM 时条目注销（svr.Stop）
```

### 4.4 生命周期验证

```bash
go run . -manual &     # 日志："kitex server starting on :8888"
kill -TERM %1          # 日志："kitex server shutting down on :8888"；etcd 条目消失；退出 0
```

`Stop` 是优雅的：`svr.Stop()` 注销并按 kitex 语义排空，随后
`otelProvider.Shutdown(ctx)` 冲刷未落 span，最后 `close(done)` 解除 Run 阻塞。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| server 不启动、无 kitex 日志 | 缺 `spring.kitex.server.addr` | 配上——该 key 即激活开关（不存在默认 :8888，example 注释有误）。 |
| 启动报 "failed to resolve addr" | addr 不是可解析的 TCP 地址 | 用 `net.ResolveTCPAddr` 接受的 `:8888` / `host:port` 形式。 |
| 容器失败：ServiceRegister 缺失/歧义 | 应用提供 0 个或 2+ 个 bean | 提供且仅提供一个。 |
| thrift unary 调用行为异常 | `compatible-unary-middleware=false` | thrift 服务置 true。 |
| 启动报 "failed to create etcd registry" | 配了 `registry.etcd` 但 etcd 不可达 | 起 etcd 或清掉该 key（无注册直连）。 |
| `tracing.enabled` / `metrics.enabled` 不生效 | 死 key——生效 key 是 `…enable`（§3.1，example 已修） | 改 key 名。 |
| collector 里 span 重复 | 一进程内出现两个 SDK provider（如 starter-otel 之外又自建全局 + kitex 回落） | 确保管道归属唯一；有 starter-otel 时 starter 自动收敛（§4.2）。 |
| :9090 被占用 | 显式配置的 kitex `metrics.port` 或 starter-otel exporter 端口冲突 | 错开端口 / 去掉 kitex `metrics.port` 走全局。 |
| 升级后 kitex 指标端点没了 | `metrics.port` 不再默认 9090（§4.2 迁移） | 显式配 `spring.kitex.server.metrics.port`，或抓 `spring.observability.metrics.port`。 |
| 完全没有 trace | tracing 关闭，或无全局管道且 collector 不在 `tracing.endpoint` | 查 key 名（`enable`）、端点、`insecure`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 9 |
| 必填 | 1（`addr`） |
| quickstart 外部依赖 | 0（仅当配置 `registry.etcd` 时需要 etcd） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（已修条目留档追溯）：

1. `tracing.enable`/`metrics.enable`（不是 `enabled`）——与所有兄弟 starter 不一致；
   `…enabled` 拼法静默不绑定任何东西（**example-otel 笔误已于 2026-08-28 修复**）。
2. ~~默认监听 `:9090` 的 metrics~~——**已修复（2026-08-28）**：`metrics.port` 默认 0（未配）；
   独立监听须显式配置端口。
3. ~~自建 OTel provider 而非复用 starter-otel 全局——双管道、无去重路径~~——
   **已修复（2026-08-28，USAGE_FINDINGS P2 #18）**：全局优先收敛；有全局管道时直接挂上、
   不建 kitex provider，仅在无全局时回落自建（§4.2）。`tracing.enable=false` 即显式退出。
4. example/conf 注释声称 ":8888 默认值"——不存在。
5. 与 starter-grpc / starter-trpc 相比缺 fault/准入/loadtest 对等能力。
6. ~~schema.json 默认值与代码相反~~——**已修复（2026-08-28）**：schema 与代码一致
   （enable 默认 true、metrics.port 默认 0）并补全局优先说明。
