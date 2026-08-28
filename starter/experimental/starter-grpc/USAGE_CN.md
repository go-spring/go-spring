# starter-grpc 使用说明 — 参考手册

详细使用文档，概览见 [README_CN.md](README_CN.md)。本文所有行为声明均与源码核对
（`starter.go`、`admission.go`、`balancer.go`、`extension.go`、`fault.go`、`loadtest.go`、
`metrics.go`、`recover.go`、`tracing.go`）并与可运行示例对齐（[example/](example/)、
[example-lb/](example-lb/)、[example-otel/](example-otel/)，各自带自断言 `check.sh`）。
**grpc-go 自身语义**（流式、deadline、keepalive 行为、status code）见
[grpc-go 官方文档](https://grpc.io/docs/languages/go/)——本文只写 go-spring 的增量：装配接线、
拦截器链、可观测、治理准入、客户端负载均衡。

**激活条件**：只有配置了 `spring.grpc.server.addr` 才会注册 server bean
（`gs.OnProperty("spring.grpc.server.addr")`，见 starter.go 的 init）。该 key 即开关：没有
`enabled` key，也没有默认端口（go-spring 的端口一律不设默认值）。单 server 模型：一个
`grpc.Server`、一个监听端口。

---

## 1. 完整工程示例

一个带健康探测、tracing/metrics、运行期 fault 注入的 Echo 服务，外加一个经 discovery
后端做负载均衡的客户端。文件树：

```
demo/
├── go.mod
├── main.go
├── server.go
├── client.go
├── idl/echo.proto
└── conf/app.properties
```

**go.mod**（关键依赖）：

```
require (
    google.golang.org/grpc        latest
    go-spring.org/spring          v1.3.x
    StarterGrpc "go-spring.org/starter-grpc" latest
    go-spring.org/starter-otel      latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest // 可选：治理中心（准入 + fault）
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**server.go** — 服务注册（proto 生成代码假定在 `demo/idl/proto`）：

```go
package main

import (
    "context"

    "google.golang.org/grpc"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterGrpc "go-spring.org/starter-grpc"
    "demo/idl/proto"
)

func init() {
    // 应用侧提供恰好一个 ServiceRegister bean。starter 持有 *grpc.Server 及其
    // 生命周期；你在它交给你的 server 上注册服务。
    gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
        return func(svr *grpc.Server) {
            proto.RegisterEchoServiceServer(svr, &Controller{})
        }
    })
}

type Controller struct {
    proto.UnimplementedEchoServiceServer
}

func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
    // ctx 已携带 trace span 与 load-test 标记（若有）——
    // 可用 traffic.IsLoadTest(ctx) 在压测流量下降级功能。
    log.Infof(ctx, log.TagAppDef, "echo: %s", req.Message)
    return &proto.EchoResponse{Message: req.Message}, nil
}
```

**client.go** — 经 discovery 后端 + Go-Spring balancer 拨号（见 §2.4）：

```go
StarterGrpc.UseUnaryInterceptor(authGuard) // 用户 guard，整条链最外层

conn, err := grpc.NewClient(
    StarterGrpc.Scheme+":///echo-service", // gsdiscovery:///<service>，走 default 后端
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultServiceConfig(StarterGrpc.LoadBalancingConfig(loadbalance.RoundRobin)),
)
// 按调用附加亲和信息：
ctx = StarterGrpc.WithHashKey(ctx, req.UserId)  // consistent-hash 亲和
ctx = StarterGrpc.WithZone(ctx, "zone-a")       // zone 亲和
```

**conf/app.properties** — 完整注释配置面：

```properties
# --- grpc server --------------------------------------------------------------
# 让 gRPC server 独占端口；关掉 gs 内置 HTTP server。
spring.http.server.enabled=false
spring.grpc.server.addr=:9494

# 报文大小/并发上限（0 = 保持 grpc-go 默认）。
spring.grpc.server.maxRecvMsgSize=4194304
spring.grpc.server.maxSendMsgSize=4194304
spring.grpc.server.maxConcurrentStreams=100

# 服务端 keepalive 约束（keepalive.ServerParameters）。
spring.grpc.server.keepalive.time=2h
spring.grpc.server.keepalive.timeout=20s

# 标准 grpc_health_v1 服务（默认开），整体状态 SERVING。
spring.grpc.server.health.enabled=true

# 入站 load-test 识别（默认开）。
spring.grpc.server.loadtest.enabled=true

# 内置可观测拦截器（默认全开；无 starter-otel 时为 no-op）。
spring.grpc.server.observer.tracing.enabled=true
spring.grpc.server.observer.metrics.enabled=true

# observe 层访问日志详细度（level: brief|full|off）。
spring.grpc.server.observability.level=brief
spring.grpc.server.observability.maxArgBytes=512

# --- 可观测（starter-otel；与 example-otel/conf 同构）-------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- 治理（准入 + fault；从该文件热加载）--------------------------------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml**（fault 演练见 §4.3；准入在 resilience 规则里）：

```yaml
govern:
  enabled: true
  # 按资源的 resilience 规则；资源 label = "grpc:{addr}"（见 admission.go）。
  rules:
    - resources: ["grpc::9494"]
      rate-limit: 100      # QPS 上限；超限 → codes.ResourceExhausted
      max-concurrent: 50   # 舱壁；超限 → codes.ResourceExhausted
  fault:
    enabled: false         # 改成 true 即可不重启"放火"
    scope: loadtest        # 只影响带 x-loadtest 标记的流量
    rules:
      - resources: ["grpc:/EchoService/Echo"]   # 规则 label = "grpc:{FullMethod}"
        rate: 0.2
        error: timeout
```

**验证**（与示例断言同构）：

```bash
cd example && ./check.sh                 # 自断言：echo 往返、x-handler 响应头、
                                         # grpc_health_v1 = SERVING，然后 SIGTERM
cd example-lb && ./check.sh              # 三阶段 LB 冒烟：均匀分布、熔断驱逐+恢复、摘除实例
cd example-otel && docker compose up -d && ./check.sh   # 经 Jaeger API 验证 trace
```

对运行中实例的 grpcurl 等价操作：`grpcurl -plaintext :9494 EchoService/Echo`、
`grpcurl -plaintext :9494 grpc.health.v1.Health/Check`。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-grpc
  └─ gs.Provide(NewSimpleGrpcServer)            [条件: 配置 spring.grpc.server.addr]
        └─ .Export(gs.As[gs.Server]())
gs.Run()
  ├─ 配置绑定: ${spring.grpc.server} → Config（value tag）
  ├─ bean 装配: ServiceRegister（不可空——缺失则容器失败）
  ├─ SimpleGrpcServer.Run(ctx, sig):
  │    ├─ buildOptions(): Config → []grpc.ServerOption，组装拦截器链
  │    ├─ grpc.NewServer(opts...)
  │    ├─ health: RegisterHealthServer + SetServingStatus("", SERVING)   [health.enabled]
  │    ├─ s.reg(svr): 你的服务在这里注册
  │    ├─ net.Listen(addr)
  │    ├─ <-sig.TriggerAndWait()   ← 就绪信号门控 Serve
  │    └─ svr.Serve(listener)
  └─ SIGTERM 时: StopContext → svr.GracefulStop()（排空在途 RPC；ctx 只标记关停日志）
```

`UseUnaryInterceptor`/`UseStreamInterceptor` 注册的用户拦截器必须在 `init()` 里（至少在容器
构建 server 之前）调用——extension.go 在 `buildOptions` 运行时做快照。

### 2.2 拦截器链 — 精确顺序与理由

Unary 链（`buildOptions`，starter.go:165-209；用 `grpc.ChainUnaryInterceptor` 组合，绝不用
单 setter 的 `UnaryInterceptor`——源码注释记录了历史上后调覆盖前调的 bug）：

```
用户拦截器 → LoadTest → Tracing → Metrics → Resilience(准入，仅 unary)
           → Fault → Recover → handler
```

Stream 链相同，但没有 Resilience（准入只覆盖 unary）。

理由（引自源码注释，已核对）：

- **用户拦截器最外层**（extension.go）："app guard 在内置栈之前看到请求，可在任何工作被
  观测前短路"——对齐 starter-gin 的 EngineMiddleware 模型。
- **LoadTest 是内置链最外层**（starter.go:173）：标记在 tracing、metrics、resilience、
  handler 之前进入 context，所有下游层都能基于 `traffic.IsLoadTest(ctx)` 分支；无标记时
  为 no-op。
- **Tracing 在 Metrics 之前**：span 同时包住 metrics 观测，时长与状态落在同一 trace 上下文。
- **Resilience 在 Fault/Recover 之前**：准入控制在做事之前裁决；executor 被
  `resilobserve.WrapExecutor` 包裹，熔断/拒绝自身会打 span + counter + histogram。
- **Fault 位于策略最内层**（fault.go）："安装在最内层，让注入的错误回穿 tracing/metrics/
  resilience 被观测到"——你放的火自己看得见。
- **Recover 最内层**（recover.go）："grpc-go 自身对 handler panic 不做任何 recover"；转换出的
  `codes.Internal` 沿正常错误路径回穿所有观测层——统一 panic 策略在请求侧的落位。

### 2.3 一次 unary RPC 逐层走读

带 metadata `x-loadtest: 1`、fault scope 生效、配置了限流的 `Echo(msg)`：

1. 用户拦截器——auth/guard 可在任何观测发生前拒绝。
2. LoadTest：`extractLoadTest` 从入站 metadata 读 `x-loadtest` → ctx 打标
   （`traffic.IsLoadTest(ctx) == true`）。
3. Tracing：从 metadata 抽取 W3C 上下文（`extractTraceContext`），启动 server span，
   名字 = FullMethod，属性 `rpc.system=grpc`、`rpc.service`、`rpc.method`。
4. Metrics：in-flight +1（仅 `rpc.method` 属性）；时长计时开始。
5. Resilience：`exec.Execute(ctx, "grpc::9494", handler)`——超限则 handler 不执行，
   `mapAdmissionError` 映射为 `ResourceExhausted`；熔断开启 → `Unavailable`。
6. Fault：`fault.Apply(ctx, InjectorFor(), "grpc:/EchoService/Echo", handler)`——被标记
   流量按 ~`rate` 概率失败或增加注入延迟；未标记/scope 关闭则直通。
7. Recover 布防；handler 执行。panic 经 `goutil.ReportPanic` 上报并转换为
   `codes.Internal`。
8. 响应/错误回穿 6→5→4→3：带 `rpc.grpc.status_code` 记录时长、count +1、in-flight −1、
   失败时 span 记状态码 + Error 状态 + RecordError，span 结束。

### 2.4 客户端：gsdiscovery resolver + balancer（balancer.go）

- 拨号 `gsdiscovery:///<service>`（默认 discovery 后端）或 `gsdiscovery://<backend>/<service>`
  （命名后端）。resolver 在 watch 之前先推送初始快照，首个 RPC 不会与空地址列表竞速。
- 仅靠 service config 选策略：`grpc.WithDefaultServiceConfig(
  StarterGrpc.LoadBalancingConfig(strategy))`。balancer 名为 `gs_round_robin`、`gs_least_conn`、
  `gs_consistent_hash`、`gs_weighted`、`gs_zone_aware`（init 预注册，驱逐默认值：连续失败
  5 次 → 驱逐 30s，之后半开试探）。
- 按调用提示：`WithHashKey`（consistent-hash 亲和）、`WithZone`（zone 亲和）。
- Weight=0 的实例由策略本身过滤（Pool 全策略统一的摘流语义）。
- `RegisterBalancer(name, strategy, trackerConfig)` 注册自定义名字以隔离驱逐状态；对未知
  策略或重复名字会 panic，与 grpc-go 自己的 balancer.Register 契约一致。
- example-lb/main.go 是可执行证明：均匀轮询分布、驱逐 + 半开恢复、discovery 摘除实例——
  全部进程内断言。

---

## 3. 逐 key 行为参考

所有 key 挂在 `spring.grpc.server.*` 下。已与
`grep -rhoE 'value:"[^"]+"' . --include='*.go' | sort -u` 核对。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `addr` | string | — | **激活 key**（必填）。监听地址；同时是 resilience 资源 label 后缀（`grpc:{addr}`）与唯一实例区分符。 | 缺失 → 整个 starter 静默不生效；ServiceRegister bean 随之令容器失败。 |
| `connectionTimeout` | duration | 0 | >0 时作为 `grpc.ConnectionTimeout`。 | 0 = grpc-go 默认。 |
| `maxRecvMsgSize` / `maxSendMsgSize` | int | 0 | 字节；>0 时生效。 | 过小 → 大报文按次收到 `ResourceExhausted`（grpc 的 "received message larger than max"），启动时不报。 |
| `maxConcurrentStreams` | uint32 | 0 | 每连接上限。 | 0 = grpc-go 默认。 |
| `keepalive.time` | duration | 0 | `keepalive.ServerParameters.Time`。⚠ 四项中任一 >0 才会应用整块配置；只配 `keepalive.timeout` 会静默让其余项落默认。 | 激进的 `time` + 更低频 ping 的客户端 → GOAWAY 风暴。 |
| `keepalive.timeout` | duration | 0 | `ServerParameters.Timeout`。 | |
| `keepalive.maxConnectionIdle` / `maxConnectionAge` | duration | 0 | 连接生命周期边界。 | |
| `tls.enabled` | bool | false | `credentials.NewTLS(TLS.BuildServer())`。 | 期望明文却开 TLS → 握手失败。 |
| `tls.cert-file` / `tls.key-file` | string | — | 服务端证书对。⚠ `tls.enabled` 后两者需成对出现。 | 启动时报 "build TLS"（来自 `Run`）。 |
| `tls.ca-file` | string | — | **服务端语义 = mTLS**：设 ClientCAs + `RequireAndVerifyClientCert`（tlsconf.BuildServer）。 | 随手一配 → 所有无证书客户端被拒。 |
| `tls.server-name` / `tls.insecure-skip-verify` | | — | 客户端旋钮；**此处为死 key**（BuildServer 忽略）。 | 虚假安全感；无任何效果。 |
| `health.enabled` | bool | true | 注册 `grpc_health_v1`，整体状态 SERVING。 | false → 探针/LB 健康检查得到 Unimplemented。 |
| `loadtest.enabled` | bool | true | 安装 LoadTest 拦截器，读 `x-loadtest` metadata（小写——grpc metadata key 一律小写）。 | false → load-test 标记不可见；fault `scope: loadtest` 永不触发。 |
| `observer.tracing.enabled` | bool | true | 安装 tracing 拦截器，依附 OTel 全局；无 starter-otel 时为 no-op（无告警）。⚠ example-otel 的 conf 用的是 `interceptor.tracing.*`——死路径，全靠默认 true 兜住。 | 前缀写错 → 静默落默认值。 |
| `observer.metrics.enabled` | bool | true | 安装 metrics 拦截器；同样 no-op 陷阱。 | 同上。 |
| `observability.level` | string | brief | cloud/observe ObserveConfig：`brief`/`full`/`off`，控制 observe-resilience 访问日志侧的详细度。 | |
| `observability.maxArgBytes` | int | 512 | observe 层参数捕获上限。 | |
| `observability.skipOps` | []string | — | observe 访问日志要跳过的 op。 | |

---

## 4. 验证与故障演练

### 4.1 基本往返 + 健康（与 example/check.sh 断言同构）

```bash
go run ./example          # 自断言：echo 体往返、x-handler 响应头、
                          # grpc_health_v1 → SERVING；退出码 0
grpcurl -plaintext -d '{"message":"hi"}' :9494 EchoService/Echo
grpcurl -plaintext :9494 grpc.health.v1.Health/Check   # {"status":"SERVING"}
```

### 4.2 指标名与属性（metrics.go，meter `go-spring.org/starter-grpc`）

Unary：`rpc.server.request_count`（counter）、`rpc.server.request.duration`（直方图，秒，
显式桶 5ms…10s）、`rpc.server.active_requests`（UpDownCounter，仅 `rpc.method` 属性）。
Stream：`rpc.server.stream_count`、`rpc.server.stream_duration`。时长/计数属性：
`rpc.method` = FullMethod、`rpc.grpc.status_code` = 如 `OK`、`ResourceExhausted`。

```bash
curl -s :9090/metrics | grep -E 'rpc_server_request_(duration|count)|active_requests'
```

Span（tracing.go）：名字 = FullMethod；`rpc.system=grpc`、`rpc.service`（从路径提取）、
`rpc.method`；出错时 `rpc.grpc.status_code` + Error 状态 + 记录的错误事件。
example-otel 端到端验证走 Jaeger API
（`http://127.0.0.1:16686/api/traces?service=grpc-otel-example`）。

### 4.3 fault 演练 — 热切换、不重启（fault.go）

1. 以 `govern.yaml` 中 `fault.enabled: false` 启动。
2. 打基线流量 → 全部正常。
3. 把文件里 `fault.enabled` 改为 true——`fault.InjectorFor()` 每次**调用时**解析，改动在
   下一个 RPC 生效，无需重启。
4. 规则 label 是 `grpc:{FullMethod}`（如 `grpc:/EchoService/Echo`）——可以只烧一个方法。
   `scope: loadtest` 时只烧带标记的流量：

```bash
grpcurl -plaintext -H 'x-loadtest: 1' -d '{"message":"x"}' :9494 EchoService/Echo  # ~20% 失败
grpcurl -plaintext -d '{"message":"x"}' :9494 EchoService/Echo                      # 持续 200
```

5. 观测火情：`rpc_server_request_count{rpc.grpc.status_code!="OK"}`、错误 span；改回 false
   灭火。

### 4.4 准入演练（admission.go）

资源 label 为 `grpc:{addr}` → `grpc::9494`。加一条规则
（`govern.rules[n].resources=grpc::9494`、`rate-limit=...`）把限流压到流量之下：拒绝以
`codes.ResourceExhausted`（限流/舱壁）或 `codes.Unavailable`（熔断开启）呈现——
`mapAdmissionError` 保证消费方可按 code 分支。被包裹的 observe-resilience executor 自身
对熔断/拒绝打 counter/histogram。**不要**给入站配 retry：已产生副作用的 handler 无法重放
（admission.go 自带的警告）。

### 4.5 负载均衡演练

```bash
cd example-lb && ./check.sh    # 断言：三实例均匀分布、3 次失败驱逐 + 2s 冷却、
                               # 恢复后重新接纳、被杀实例的流量摘除
```

### 4.6 panic 路径

handler panic → `codes.Internal` "panic in {FullMethod}: ..."，并经共享 goutil panic 链
（`goutil.ReportPanic`）出结构化报告——日志与 span 都可见，进程不倒。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 无监听；容器在 ServiceRegister 上失败 | 缺 `spring.grpc.server.addr`——starter 未激活，你的 bean 悬空 | 配上；它是激活开关。 |
| tracing/metrics "开了"但没有导出 | 未 import starter-otel——OTel 全局是无声 no-op | 加 import（照 example-otel）。 |
| 拦截器配置不生效 | 前缀写错：是 `observer.*`，不是 `interceptor.*`（example-otel 的 conf 就带这个死 key） | 用 `spring.grpc.server.observer.tracing/metrics.enabled`。 |
| 客户端 TLS 握手被拒、证书报错 | 配了 `tls.ca-file`——那会开启 **mTLS**（`RequireAndVerifyClientCert`） | 单向 TLS 就删掉它，否则给客户端发证。 |
| 一切正常但 fault/准入无效果 | 未 import starter-governance 或未配 `govern.source`——seam 直通 | 加 import 并把 `govern.source.file.path` 指向文件。 |
| `ResourceExhausted` "received message larger than max" | `maxRecvMsgSize` 低于报文 | 调大上限。 |
| stream RPC 绕过限流 | 准入设计上只覆盖 unary | 用用户拦截器防护 stream（`UseStreamInterceptor`）。 |
| GOAWAY / 连接抖动 | 激进的 `keepalive.time` 对上低频 ping 的客户端 | grpc keepalive 语义；放宽服务端参数。 |
| LB 客户端启动即 `ErrNoSubConnAvailable` | discovery 后端缺失/无健康实例；或 service config 里 balancer 名不对 | 注册后端（`discovery.RegisterDiscovery`）并用 `BalancerName`/`LoadBalancingConfig`。 |
| discovery 出错后 LB 不再更新 | `watchLoop` 收到 `WatchResult.Err` 后永久退出 | 重启客户端；已记入设计嫌疑。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key（叶子，含 tls/observability） | 21 |
| 必填 | 1（`addr`） |
| quickstart 前置外部依赖 | 0（完整可观测才需要 collector） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（承接上轮审计并更新）：

1. README 陈旧说法：拦截器组合需要 handler 层包装——`UseUnaryInterceptor`/
   `UseStreamInterceptor` + 链式拦截器已存在（example.go 的 `interceptedEchoServer` 是
   遗留写法）。*（仍开放——README 文本）*
2. example/conf 注释声称 ":9494 默认值"——不存在；`addr` 必填。
3. resilience 准入只覆盖 unary；stream RPC 跳过（admission.go 未构建 stream 拦截器）。
4. `observability.*` 绑定 cloud/observe 配置，但只有 observe-resilience 包装器消费它；
   访问日志侧基本未用。
5. 新增：example-otel conf 使用死前缀 `spring.grpc.server.interceptor.*`（只因默认 true
   才看起来生效）——配置面陷阱。
6. 新增：balancer.go 的 `watchLoop` 在 watch 错误后永久退出（注释承认推送路径是
   best-effort）；瞬时 discovery 错误会冻结地址集。
