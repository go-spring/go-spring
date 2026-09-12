# starter-echo 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`middleware.go`、`metrics.go`、`tracing.go`、`recover.go`、`config.go`)并锚定
可运行的 [example/](example/)。**echo 自身语义(路由、context、绑定、中间件写法)见
[echo 官方文档](https://echo.labstack.com/docs)**——以下全部是 go-spring 增量。

**激活条件**:仅当设置 `spring.echo.server.addr` 时 server bean 才存在——该 key 就是开关,
没有 `enabled` key。单 server 模型:一个引擎、一个端口。

---

## 1. 完整工程示例

一个带健康探针、指标、trace 与运行期故障注入的真实服务。文件树:

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
require (
    github.com/labstack/echo/v4  latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-echo   latest
    go-spring.org/starter-actuator latest   // 可选:探针 + /metrics
    go-spring.org/starter-otel     latest   // 可选:真实 trace/指标导出
    go-spring.org/starter-governance latest // 可选:运行期故障注入
)
```

**main.go**:

```go
package main

import (
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go**——应用的全部 HTTP 面:

```go
package router

import (
    "context"
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    // 可选:让每行业务日志带上 RequestID 中间件传播的请求 id(见 §4.1)。
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        if id := StarterEcho.RequestIDFromContext(ctx); id != "" {
            return []log.Field{log.String("request_id", id)}
        }
        return nil
    }

    // 应用提供恰好一个 RouterRegister bean。starter 拥有 *echo.Echo 和
    // HTTP server;你往它递给你的引擎上接路由——此时内置链已装好(见
    // §2.2),你的路由跑在最内层。
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
                return func(c echo.Context) error {
                    c.Response().Header().Set("X-App", "demo")
                    return next(c)
                }
            })
            e.GET("/echo/:name", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"msg": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties**——上面用到的完整注释配置面:

```properties
# --- echo server -------------------------------------------------------------
# 让 echo server 独占端口(关掉 gs 内置 HTTP server)。
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# starter 自带的存活端点;访问日志自动跳过。
spring.echo.server.health.enabled=true
spring.echo.server.health.path=/healthz

# 拒绝 > 1 MiB 的请求体(413,像普通响应一样记日志)。
spring.echo.server.maxBodySize=1048576

# 超时(默认值;按负载调)。
spring.echo.server.readTimeout=5s
spring.echo.server.writeTimeout=5s
spring.echo.server.idleTimeout=60s

# --- middleware --------------------------------------------------------------
# 默认开:loadtest、recovery、requestId、tracing、metrics、accessLog。
# 可选开启:
spring.echo.server.middleware.secureHeaders.enabled=true

# --- actuator(探针 + 指标挂载)----------------------------------------------
spring.actuator.addr=:9370

# --- observability(starter-otel)--------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 只走 actuator

# --- governance(运行期故障注入)----------------------------------------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml**(§4.4 的故障演练用它):

```yaml
govern:
  enabled: true
  fault:
    enabled: false        # 改成 true 即"点火",无需重启
    rate: 0.2
    error: timeout
    scope: loadtest       # 只有带 X-LoadTest 标记的流量受影响
```

**验证**:

```bash
curl -i :8002/echo/world        # 200,X-App: demo,X-Request-Id: ...
curl -i :9370/healthz           # actuator 存活探针
curl -s :9370/metrics | grep http_server_request_duration
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-echo
  └─ gs.Provide(echo 版 server 构造)                        [条件:设置了 addr]
        │
gs.Run()
  ├─ 配置绑定:${spring.echo.server} → Config(value tag + expr 校验)
  ├─ bean 装配:RouterRegister 注入——echo 唯一 app 缝(无 gin 那种 EngineMiddleware
  │             外层槽),app 中间件跑在内置链内侧
  ├─ 引擎组装:applyMiddlewares() 先装内置链,
  │            再跑你的 RouterRegister(路由最内层)
  ├─ Server.Run():net.Listen 后开始服务
  ├─ 就绪:sig.TriggerAndWait() → readyz 翻 UP
  └─ SIGTERM:PreStop/Stop → http.Server.Shutdown(ctx) 排空在途请求
```

缺 `RouterRegister` 容器直接失败(非空注入);两个则因歧义失败。

### 2.2 中间件链——精确顺序与设计理由

```
LoadTest → Recovery → RequestID(+propagate) → Tracing → Metrics → AccessLog
→ SecureHeaders → CORS → Gzip → [BodyLimit] → admission → fault → 健康路由 → 应用路由
```

理由(源自源码注释,已核对):

- **LoadTest 最外层**:标记在任何东西之前落到请求 context 上,后续每一层——以及 handler
  调用的每个出站 client——都能用 `traffic.IsLoadTest(ctx)` 分流。单次头查找;无标记即空操作。
- **Recovery** 兜住所有内层的 panic,并上报到共享的 goutil panic 链(统一 panic 策略),
  不是裸用 echo 自带 Recover。
- **RequestID 在 AccessLog 之前**:每条访问记录都带请求 id;id 同时存进请求 context
  (`RequestIDFromContext`)供业务日志关联。
- **Tracing 包住 Metrics 和 AccessLog**:span 能同时捕获两者的时序与属性。
- **AccessLog 包住策略中间件**:短路响应(BodyLimit 的 413、CORS 的 403、204)也会被记录。
- **admission 在 fault 外层**:被入站限流/隔离/熔断拦下(或放行)的请求不会再被放火;它产生的
  429/503 照样过 AccessLog/Tracing/Metrics。**无条件安装**——治理关着时 executor 是透明透传,
  只多一帧调用,别的不变。资源 label 是 `echo:<address>`(如 `echo::8080`),与治理规则一致:
  `govern.rules[N].resources=echo::8080`,配 `rate-limit` / `max-concurrent` / `error-threshold`
  等旋钮。拒绝映射为 **429**(限流、隔离满)与 **503**(熔断打开);handler 返回错误或已提交的
  5xx 会作为本次调用的失败回喂给 executor,熔断因此能看到服务端错误。入站准入**从不重试**
  ——handler 已产生副作用就不能重放——所以 `max-retries` 请留 0;真有重试策略也有重入守卫兜住
  (handler 仍只跑一次)。
- **fault 最内层**:注入的 503 出来时照样过 AccessLog/Tracing/Metrics——你放的火可观测。
  只有注入错误(`*fault.InjectedError`)渲染为 503 "service unavailable";handler 自己的错误
  原样交给 echo 的 HTTPErrorHandler。

### 2.3 一次请求,逐层走读

带 `X-LoadTest: 1` 的 `GET /echo/world`,fault scope 已启用:

1. LoadTest 打标(`traffic.IsLoadTest(ctx) == true`)
2. Recovery 布防
3. RequestID 生成/透传 id → 响应头
4. Tracing 开 server span `{method} {route}`(无 OTel provider 时空操作)
5. Metrics 加 in-flight、起耗时观测
6. AccessLog 布防(字段在出口捕获)
7. SecureHeaders/CORS/Gzip 按配置;BodyLimit 执行 `maxBodySize`
8. fault:`fault.Apply(ctx, InjectorFor(), "echo", handler)`——`scope: loadtest` 且有标记时,
   约 `rate` 比例的请求拿到注入错误 → 503;其余放行
9. 你的路由执行;响应按 6→5→4→3 解栈:访问记录落盘(按状态定级)、
   带 `http.request.method`/`http.route`/`http.response.status_code` 属性的耗时入库、
   span 结束、id 头写回。

---

## 3. 逐 key 行为参考

### 3.1 server 核心

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `addr` | string | — | **激活 key**。存在即注册 server bean。 | 缺失 → 整个 starter 静默不生效(路由配了也永远不服务)。 |
| `maxBodySize` | int | 0 | `>0` 装 BodyLimit;超限 413,与普通响应同路记录/恢复。 | 过低 → 合法上传 413;错误按请求暴露,不在启动期。 |
| `readTimeout` | duration | 5s | 兼限 header 读取。 | 过低会掐死慢客户端。 |
| `writeTimeout` / `idleTimeout` | duration | 5s / 60s | 透传 `http.Server`。 | idleTimeout 过低 → keep-alive 频繁重建。 |
| `health.enabled` / `health.path` | bool / string | false / `/healthz` | starter 服务的存活路由;路径自动并入访问日志跳过集。 | 自定义路径只有在用 starter 服务的那条才自动跳过。 |
| `tls.enabled` + `cert-file`/`key-file` | — | 关 | 切到 `tlsconf.BuildServer()` 构建的 TLS listener(与 starter-grpc 同语义);配置 `ca-file` 即开启 **mTLS**(ClientCAs + `RequireAndVerifyClientCert`,客户端必须出示该 CA 签发的证书)。`server-name`/`insecure-skip-verify` 是客户端 key,此处绑定但无效。 | 随手配 `ca-file` → 没有证书的客户端全部被拒。 |

### 3.2 middleware 组

每组都有 `.enabled`;各层语义见 §2.2/§2.3。

| Key | 默认 | 说明 |
|-----|------|------|
| `middleware.loadtest.enabled` / `.header` | 开 / `X-LoadTest` | 标记头名;空则回落 traffic 包默认。 |
| `middleware.requestId.enabled` / `.header` | 开 / `X-Request-Id` | 缺失时生成、存在时透传。 |
| `middleware.tracing.enabled` / `metrics.enabled` | 开 / 开 | 无 starter-otel 的 OTel 全局对象时空操作——没有任何告警。 |
| `middleware.accessLog.skipPaths` | — | 与健康路径合并。 |
| `middleware.accessLog.payload.*` | 与 gin 对齐 | 请求体捕获——echo 侧默认值见 config.go;捕获体进日志字段。 |
| `middleware.cors.*` | 关 | `allowedMethods` 空 → 代码默认全动词集;`allowAllOrigins` 与显式 `allowedOrigins` 是互斥姿态。 |
| `middleware.gzip.enabled` / `.level` | 关 / 5 | `minLength` 类调优以 config.go 为准。 |
| `middleware.secureHeaders.*` | 关 | frameOptions DENY、referrerPolicy no-referrer;`hsts.*` 子 key(关)。 |
| `observability.level` / `maxArgBytes` / `skipOps` | brief / 512 / — | observe 层访问日志详略。 |

---

## 4. 验证与故障演练

### 4.1 请求 id 透传

```bash
curl -sD- -o/dev/null :8002/echo/a | grep -i x-request-id    # 生成
curl -sD- -o/dev/null -H 'X-Request-Id: fixed-42' :8002/echo/a | grep -i x-request-id  # 透传:fixed-42
```

配合 §1 的 `log.FieldsFromContext` 钩子,handler 内每行业务日志都带 `request_id`。

### 4.2 观测中间件链

- 访问日志(tag `_app_echo_access`):每请求一条结构化记录——路由、状态、耗时、request id、
  tracing 生效时的 trace/span id。定级:≥500 Error、≥400 Warn。
- 指标:`http.server.request.duration` 直方图,属性 `http.request.method`、`http.route`
  (*路由模板*,不是原始路径)、`http.response.status_code`;同属性维度的 in-flight 量表:

```bash
curl -s :9370/metrics | grep -E 'http_server_request_duration|in_flight'
```

- Trace:每请求一个 `{method} {route}` span;打完流量去 collector(Jaeger UI 等)看。

### 4.3 探针

```bash
curl -i :9370/readyz     # 就绪前 OUT_OF_SERVICE,之后 UP;摘流中 503
curl -i :8002/healthz    # echo 侧存活(starter 服务)
```

### 4.4 故障演练(免重启)

1. 按 §1 启动(`fault.enabled: false`)。
2. 打基线流量:`curl :8002/echo/x` → 全 200。
3. 把文件里 `fault.enabled` 改 `true`——治理 source 热更新。
4. 标记流量着火、正常流量无感:

```bash
curl -i :8002/echo/x                          # 200(scope=loadtest,无标记)
curl -i -H 'X-LoadTest: 1' :8002/echo/x       # ~20% → 503 service unavailable
```

5. 在可观测面看火:访问日志 503 的 Warn 记录、耗时直方图的 503 桶、被注入请求的 span。
   改回 `false` 灭火。

### 4.5 压测标记演练

把 `scope` 换成 `real` 则注入作用于无标记流量——只在专用环境用。handler 里
`traffic.IsLoadTest(ctx)` 分流同一标记,业务代码可在合成流量下自动降级功能。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| server 不启、无日志 | 缺 `spring.echo.server.addr` | 补上——该 key 即激活开关。 |
| 启动即端口冲突 | gs HTTP server 或其他 starter 占端口 | `spring.http.server.enabled=false` 或换 `addr`。 |
| 容器失败:RouterRegister 缺失/歧义 | 应用提供了 0 个或 2+ 个 bean | 恰好提供一个。 |
| 一切正常但无 trace/指标 | 未引入 starter-otel | 加上;OTel 钩子无它即静默空操作。 |
| 上传 413 | `maxBodySize` < 载荷 | 调大或去掉。 |
| 探针刷爆访问日志 | 自定义健康路径 | starter 服务的路径自动跳过;自己的路径加进 `skipPaths`。 |
| app 中间件要跑到内置链外层 | echo 无 EngineMiddleware 槽(gin 有) | `RouterRegister` 最内层跑;每个内置组用各自 `middleware.<group>.enabled` 开关,无总开关 `middleware.enabled`、无导出 `ApplyMiddlewares`。 |
| 客户端在 TLS 握手被拒(证书错误) | 配了 `tls.ca-file`——即开启 **mTLS**(`RequireAndVerifyClientCert`) | 去掉 `ca-file` 走单向 TLS,或给客户端发证书。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | ~37 |
| 必填 | 1(`addr`) |
| quickstart 前置外部依赖 | 0(完整可观测需 collector) |
| "注意/坑"条数 | 7 |

设计嫌疑(待设计裁决):~~无韧性准入(限流/熔断)~~ —— 已补齐,与 gin 对齐(label 与状态码映射见 §2);
TLS 已改用 `BuildServer()`,
`ca-file` 即开 mTLS(已修复,原 `Build()` 会忽略);无外层 `EngineMiddleware` 槽(gin 有,echo 的 app 中间件跑在内置链内侧);
与 gin 的请求体捕获配置无对齐文档。
