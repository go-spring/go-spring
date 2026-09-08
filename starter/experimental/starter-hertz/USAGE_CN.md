# starter-hertz 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`middleware.go`、`metrics.go`、`tracing.go`、`recover.go`、`config.go`)
并锚定可运行的 [example/](example/)（自断言 `check.sh`，无外部依赖）与
[example-otel/](example-otel/)（docker-compose Jaeger）。**Hertz 自身语义（路由、
`app.RequestContext`、hertz-contrib）见 [Hertz 官方文档](https://www.cloudwego.io/zh/docs/hertz/)**
——以下全部是 go-spring 增量。

**激活条件**：仅当设置 `spring.hertz.server.addr` 时 server bean 才存在——该 key 就是
开关，没有 `enabled` key。单 server 模型。与 gin/echo 不同，Hertz 自持 listener：starter
通过 `WithHostPorts` 传地址，read/write/idle 超时与 `maxBodySize` 走引擎选项
（`server.New(opts...)`，非标准 `http.Server`、非中间件——见 `NewSimpleHertzServer`）。

---

## 1. 完整工程示例

一个带健康端点、指标、trace 与运行期故障注入的真实服务。文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod**（关键依赖）：

```
require (
    github.com/cloudwego/hertz   v0.10.x
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-hertz  latest
    go-spring.org/starter-actuator latest   // 可选：探针 + /metrics
    go-spring.org/starter-otel     latest   // 可选：真实 trace/指标导出
    go-spring.org/starter-governance latest // 可选：运行期故障注入
)
```

**main.go**：

```go
package main

import (
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-hertz"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go**——应用的全部 HTTP 面（handler 签名是 hertz 的
`func(ctx context.Context, c *app.RequestContext)`）：

```go
package router

import (
    "context"
    "net/http"

    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/app/server"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterHertz "go-spring.org/starter-hertz"
)

func init() {
    // 可选：让每行业务日志带上 RequestID 中间件传播的请求 id
    //（见 §4.1，propagateRequestID）。
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        if rid := StarterHertz.RequestIDFromContext(ctx); rid != "" {
            return []log.Field{log.String("request_id", rid)}
        }
        return nil
    }

    // 应用提供恰好一个 RouterRegister bean。starter 拥有 *server.Hertz 和
    // listener；你往它递给你的引擎上接路由——此时内置链已装好（见 §2.2），
    // 你的中间件跑在最内层。
    gs.Provide(func() StarterHertz.RouterRegister {
        return func(h *server.Hertz) {
            h.Use(func(ctx context.Context, c *app.RequestContext) {
                c.Response.Header.Set("X-App", "demo")
                c.Next(ctx)
            })
            h.GET("/echo/:name", func(ctx context.Context, c *app.RequestContext) {
                c.JSON(http.StatusOK, map[string]string{"message": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties**——上面用到的完整注释配置面：

```properties
# --- hertz server -------------------------------------------------------------
# Hertz 自持 listener：先关掉 gs 内置 HTTP server。
spring.http.server.enabled=false
spring.hertz.server.addr=127.0.0.1:8003

# starter 自带的存活端点；访问日志自动跳过。
spring.hertz.server.health.enabled=true
spring.hertz.server.health.path=/healthz

# 请求体上限 1 MiB（WithMaxRequestBodySize，引擎层 413）。
spring.hertz.server.maxBodySize=1048576

# 超时（默认值；引擎选项，按负载调）。
spring.hertz.server.readTimeout=5s
spring.hertz.server.writeTimeout=5s
spring.hertz.server.idleTimeout=60s

# --- middleware --------------------------------------------------------------
# 默认开：loadtest、recovery、requestId、tracing、metrics、accessLog。
# （此处没有 middleware.enabled 总开关——每组一个 key。）
# 可选开启：
spring.hertz.server.middleware.secureHeaders.enabled=true

# --- actuator（探针 + 指标挂载）----------------------------------------------
spring.actuator.addr=:9370

# --- observability（starter-otel）--------------------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- governance（运行期故障注入）----------------------------------------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml**（§4.4 的故障演练用它）：

```yaml
govern:
  enabled: true
  fault:
    enabled: false        # 改成 true 即"点火"，无需重启
    rate: 0.2
    error: timeout
    scope: loadtest       # 只有带 X-LoadTest 标记的流量受影响
```

**验证**（与 `example/check.sh` 断言同构——X-App 头、X-Request-Id 头、JSON 体、
/healthz "ok"）：

```bash
cd example && ./check.sh                          # 自断言冒烟，exit 0
curl -i 127.0.0.1:8003/echo/world                 # 200,X-App: demo,X-Request-Id: ...
curl -i 127.0.0.1:8003/healthz                    # 200 "ok"（starter 服务）
curl -s 127.0.0.1:9090/metrics | grep http_server_request_duration
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-hertz
  └─ gs.Provide(NewSimpleHertzServer, IndexArg(1, TagArg(${spring.hertz.server})))
        │  .Export(gs.As[gs.Server]()), Condition(OnProperty("spring.hertz.server.addr"))
gs.Run()
  ├─ 配置绑定：${spring.hertz.server} → Config（value tag）
  ├─ bean 装配：应用唯一的 RouterRegister 注入（非空）
  ├─ NewSimpleHertzServer：
  │    引擎选项（WithHostPorts/Read/Write/Idle/MaxRequestBodySize[/WithTLS]）
  │    → server.New(opts...)           // 非 server.Default：Recovery 保持可配
  │    → applyMiddlewares(h, cfg)      // 内置链在此安装
  │    → 健康路由（先于 register，通配路由无法遮蔽）
  │    → register(h)                   // 你的路由，最内层
  ├─ SimpleHertzServer.Run(ctx, sig)：<-sig.TriggerAndWait() 后 h.Run() 阻塞
  ├─ 就绪：TriggerAndWait 返回 → readyz 翻 UP → 引擎开始服务
  └─ SIGTERM：Stop/Stop → h.Shutdown(ctx) 排空在途请求
```

缺 `RouterRegister` 容器直接失败（非空注入）；两个则因歧义失败。注意引擎在
**就绪信号之后**才开始服务——`Run` 先阻塞在 `sig.TriggerAndWait()`。

### 2.2 中间件链——精确顺序与设计理由

```
LoadTest → Recovery → RequestID(+propagate) → Tracing → Metrics → AccessLog
→ SecureHeaders → CORS → Gzip → fault → 健康路由 → 应用路由
```

理由（源自 `applyMiddlewares` 注释，已核对）：

- **LoadTest 最外层**：标记在任何东西之前落到请求 context 上，后续每一层——以及
  handler 调用的每个出站 client——都能用 `traffic.IsLoadTest(ctx)` 分流。单次头查找
  （`Header.Peek`）；无标记即空操作。
- **Recovery**（starter 自有 `Recover()`，非 hertz contrib 中间件）兜住所有内层 panic，
  上报共享 goutil panic 链（统一 panic 策略），然后 500 中止。
- **RequestID 在 AccessLog 之前**：每条访问记录都带请求 id；id 同时经
  `propagateRequestID` 存进请求 context（`RequestIDFromContext`）供业务日志关联。
- **Tracing 包住 Metrics 和 AccessLog**：span 能同时捕获两者的时序与属性。
- **AccessLog 包住策略中间件**：短路响应（CORS 403、204）也会被记录。请求体限制是
  引擎选项 `WithMaxRequestBodySize`，超限 413 同样被记录。
- **fault 最内层、恒安装**：注入的 503 出来时照样过 AccessLog/Tracing/Metrics——
  你放的火可观测。`buildFault` 只在响应未被触碰时（`!IsBodyStream() &&
  StatusCode()==0`）写 503；handler 自己产出的响应原样放行。

### 2.3 一次请求，逐层走读

带 `X-LoadTest: 1` 的 `GET /echo/world`，fault scope 已启用：

1. LoadTest 打标（`traffic.WithLoadTest(ctx, "http-header")`）
2. Recovery 布防（`defer`/`recover`）
3. RequestID：hertz-contrib/requestid 生成/透传 id → 响应头；
   `propagateRequestID` 把 id 复制到请求 ctx
4. Tracing 提取调用方 context，开 server span `HTTP <method>`（无 OTel provider 时
   空操作）
5. Metrics 加 in-flight 量表、起耗时观测
6. AccessLog 布防（字段在出口捕获）
7. SecureHeaders/CORS/Gzip 按配置（引擎独立执行 `maxBodySize`）
8. fault：`fault.Apply(ctx, fault.InjectorFor(), "hertz", handler)`——`scope: loadtest`
   且有标记时，约 `rate` 比例的请求拿到注入错误 → 503；其余放行
9. 你的路由执行；响应按 6→5→4→3 解栈：访问记录落盘（按状态定级）、耗时/计数入库、
   in-flight 减一、span 结束（5xx 标 Error）、id 头写回。

---

## 3. 逐 key 行为参考

全部 key 在 `spring.hertz.server.*` 下（含 tls 共 37 个叶子）。已用
`grep -rhoE 'value:"[^"]+"' --include='*.go'` 核对。

### 3.1 server 核心

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `addr` | string | — | **激活 key**。`OnProperty("spring.hertz.server.addr")` 注册 server bean；经 `WithHostPorts` 传入。 | 缺失 → 整个 starter 静默不生效。 |
| `readTimeout` | duration | 5s | 引擎选项 `WithReadTimeout`。 | 过低掐死慢客户端。 |
| `writeTimeout` / `idleTimeout` | duration | 5s / 60s | `WithWriteTimeout` / `WithIdleTimeout`。 | idleTimeout 过低 → keep-alive 频繁重建。 |
| `maxBodySize` | int | 0 | `>0` → `WithMaxRequestBodySize`；超限由引擎 413，与普通响应同路记录。0 = Hertz 默认。 | 过低 → 合法上传 413；按请求暴露，不在启动期。 |
| `health.enabled` / `health.path` | bool / string | false / `/healthz` | starter 服务的存活路由 `GET path → "ok"`；在应用路由**之前**注册，通配路由无法遮蔽；路径自动并入访问日志跳过集（`accessLogSkipSet`）。 | 自定义路径仅在 health.enabled 为 true 时被自动跳过。 |
| `tls.enabled` + `tls.cert-file`/`key-file` | — | 关 | `cfg.TLS.BuildServer()` → `WithTLS`（与 starter-grpc 同语义）。配置 `tls.ca-file` 即开启 **mTLS**（ClientCAs + `RequireAndVerifyClientCert`，客户端必须出示该 CA 签发的证书）。`server-name`/`insecure-skip-verify` 是客户端 key，此处绑定但**无效**。 | 随手配 `ca-file` → 没有证书的客户端全部被拒。 |

### 3.2 middleware 组

没有 `middleware.enabled` 总开关——每组各自的 `.enabled`（与 echo/gin 不同）。
各层语义见 §2.2/§2.3。

| Key | 默认 | 说明 |
|-----|------|------|
| `middleware.loadtest.enabled` / `.header` | 开 / `X-LoadTest` | 标记头名；空则回落 `traffic.HeaderLoadTest`。 |
| `middleware.recovery.enabled` | 开 | 关掉后请求 goroutine 的 panic 会打崩整个进程（hertz 核心行为）。 |
| `middleware.requestId.enabled` | 开 | hertz-contrib/requestid 默认头 `X-Request-Id`；缺失生成、存在透传。⚠ 无可配置头 key（loadtest 有）。 |
| `middleware.tracing.enabled` / `metrics.enabled` | 开 / 开 | 无 starter-otel 的 OTel 全局对象时空操作——没有任何告警。 |
| `middleware.accessLog.enabled` / `.skipPaths` | 开 / — | skip 列表与健康路径合并。 |
| `middleware.cors.enabled` + 7 个子 key（`allowAllOrigins`、`allowedOrigins`、`allowedMethods`、`allowedHeaders`、`exposeHeaders`、`allowCredentials`、`maxAge`） | 全关/空 | `allowedMethods` 空 → 代码默认全动词集（`corsMiddleware`）；配置启动期经 `c.Validate()` 校验——坏策略启动即失败报 `hertz: invalid cors config`，而非首个请求 panic。`allowAllOrigins` 与显式 `allowedOrigins` 互斥。 |
| `middleware.gzip.enabled` / `.level` | 关 / 5 | level 遵循 compress/gzip 语义：1=BestSpeed … 9=BestCompression，-1=DefaultCompression。⚠ 无 minLength 调优 key。 |
| `middleware.secureHeaders.enabled` | 关 | 打 X-Content-Type-Options: nosniff、X-Frame-Options: DENY、Referrer-Policy: no-referrer（自实现；刻意避开 hertz-contrib/secure 的 10 年 HSTS + SSL redirect 默认）。 |
| `middleware.secureHeaders.hsts.enabled`/`.maxAge`/`.includeSubDomains`/`.preload` | 关 / 0s / 关 / 关 | 仅当 hsts 开 **且** tls.enabled **且** maxAge>0 才发头（`secureHeaders` 函数）。 |

---

## 4. 验证与故障演练

### 4.1 请求 id 透传

```bash
curl -sD- -o/dev/null 127.0.0.1:8003/echo/a | grep -i x-request-id    # 生成
curl -sD- -o/dev/null -H 'X-Request-Id: fixed-42' 127.0.0.1:8003/echo/a | grep -i x-request-id  # 透传：fixed-42
```

配合 §1 的 `log.FieldsFromContext` 钩子，handler 内每行业务日志都带 `request_id`
（example-otel 的 handler 演示了该模式）。

### 4.2 观测中间件链

- 访问日志（tag `_app_hertz_access`，经 `log.RegisterAppTag("hertz","access")` 注册）：
  每请求一条结构化记录——`method`、`path`、`status`、`size`、`ip`、`latency`、
  `request_id`。定级：≥500 Error、≥400 Warn、其余 Info。
- 指标（meter `go-spring.org/starter-hertz`）：
  - 计数 `http.server.request_count`
  - 直方图 `http.server.request_duration`（秒；OTel HTTP semconv 桶 0.005…10）——
    属性 `http.request.method`、`http.route`、`http.response.status_code`
  - 上下量表 `http.server.active_requests`——属性 method + `http.route`。
  ⚠ 此处 `http.route` 是**原始请求路径**（`c.Request.URI().Path()`），非路由模板——
  高基数，与 starter-echo/gin 不一致。读取：

```bash
curl -s 127.0.0.1:9090/metrics | grep -E 'http_server_request_(count|duration)|active_requests'
```

- Trace（tracer `go-spring.org/starter-hertz`）：每请求一个 **`HTTP <method>`** span
  （如 `HTTP GET`）——不是 echo 的 `{method} {route}`。属性：`http.request.method`、
  `url.path`、`server.address`、`http.response.status_code`；≥500 置 span Error。
  example-otel 端到端验证 Jaeger API：

```bash
cd example-otel && docker compose up -d && go run .
# 断言：Jaeger 中找到 service 'hertz-otel-example' 的 trace
```

### 4.3 探针

```bash
curl -i 127.0.0.1:8003/healthz   # starter 服务的存活 "ok"，访问日志自动跳过
curl -i 127.0.0.1:9370/readyz    # actuator 就绪（需加 starter-actuator）
```

### 4.4 故障演练（免重启）

1. 按 §1 启动（`fault.enabled: false`）。
2. 打基线流量：`curl 127.0.0.1:8003/echo/x` → 全 200。
3. 把文件里 `fault.enabled` 改 `true`——治理 source 热更新。
4. 标记流量着火、正常流量无感：

```bash
curl -i 127.0.0.1:8003/echo/x                          # 200（scope=loadtest，无标记）
curl -i -H 'X-LoadTest: 1' 127.0.0.1:8003/echo/x       # ~20% → 503 service unavailable
```

5. 在可观测面看火：访问日志 503 的 Error 记录、耗时直方图的 503 桶、被注入请求的
   Error span（`HTTP GET`）。改回 `false` 灭火。

### 4.5 压测标记演练

把 `scope` 换成 `real` 则注入作用于无标记流量——只在专用环境用。handler 里
`traffic.IsLoadTest(ctx)` 分流同一标记，业务代码可在合成流量下自动降级功能。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| server 不启、无日志 | 缺 `spring.hertz.server.addr` | 补上——该 key 即激活开关。 |
| 启动即端口冲突 | gs HTTP server 还开着 | `spring.http.server.enabled=false`（Hertz 自持 listener）。 |
| 容器失败：RouterRegister 缺失/歧义 | 应用提供了 0 个或 2+ 个 bean | 恰好提供一个。 |
| server 迟迟不服务、readyz 卡住 | `Run` 按设计阻塞等就绪信号 | 查其他 bean 的就绪；引擎在 `TriggerAndWait` 之后才启动。 |
| 一切正常但无 trace/指标 | 未引入 starter-otel | 加上；OTel 钩子无它即静默空操作。 |
| 启动失败 `hertz: invalid cors config` | cors 子 key 不兼容（如 allowAllOrigins 搭配 allowedOrigins/credentials） | 修策略——启动期校验是设计行为（`corsMiddleware`）。 |
| 上传 413 | `maxBodySize` < 载荷 | 调大或去掉（引擎选项，非中间件）。 |
| 客户端在 TLS 握手被拒（证书错误） | 配了 `tls.ca-file`——即开启 **mTLS**（`RequireAndVerifyClientCert`） | 去掉 `ca-file` 走单向 TLS，或给客户端发证书。 |
| 指标按路径基数爆炸 | `http.route` 属性是原始路径 | ⚠ 与 echo/gin 已知分叉；提设计问题。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 37（含 5 个无效 tls key） |
| 必填 | 1（`addr`） |
| quickstart 前置外部依赖 | 0（完整可观测需 collector） |
| "注意/坑"条数 | 6 |

设计嫌疑（待设计裁决）：

1. **README 陈旧表述（部分已修）**——中间件表已列 loadtest/tracing/metrics/fault，但
   结尾注记 "Metrics and tracing are not built in either - use starter-actuator and
   starter-otel for those" 不实：两者都是默认开的中间件（`metrics.go`、`tracing.go`）。
   写文档时仍未修。
2. 指标属性 `http.route` 是原始请求路径非路由模板——基数无界，且与
   starter-echo/gin 不一致（`metrics.go` `metricsMiddleware`）。
3. tracing span 名 `HTTP <method>` 不含路由——不同端点的 span 不可区分
   （`tracing.go` `tracingMiddleware`）。
4. 已修复：server 侧改用 `tlsconf.BuildServer()`（原 `Build()` 是客户端语义）——`ca-file`
   即开启 mTLS（`RequireAndVerifyClientCert`），与 starter-grpc 对齐；`server-name`/
   `insecure-skip-verify` 仍为客户端 key，server 侧无效果。
5. 入站路径无韧性准入（限流/熔断）——与 starter-gin 不对称；fault 注入已接线、防护没有。
6. `requestId` 组无可配置头 key（loadtest 有）——轻微不对称。
7. 无 `middleware.enabled` 总开关，与 echo/gin 不同——只有逐 key 开关（可能更安全；
   记录备跨家族一致性评审）。
