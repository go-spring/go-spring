# starter-gin 使用说明

详细使用文档,概览见 [README.md](README.md)。锚定 [example/](example/)(`example/check.sh`
已验证通过;另有 example-resilience/、example-otel/)。**gin 路由器自身的语义(路由、handler、
绑定)见 [gin 官方文档](https://gin-gonic.com/docs/)**——本文只写 go-spring 增量:装配、中间件
链、可观测、治理准入。

**激活条件**:仅当设置 `spring.gin.server.addr` 时 server bean 才存在(⚠ README 声称有
`spring.gin.server.enabled` key——不存在)。单 server 模型:一个 bean、一份配置。

## 1. 快速开始

```properties
spring.gin.server.addr=:8001
```

```go
package main

import (
	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-gin"
)

func init() {
	// 应用提供恰好一个 RouterRegister bean。
	gs.Provide(func() StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			e.GET("/hello/:name", func(c *gin.Context) { c.String(200, "hi %s", c.Param("name")) })
		}
	})
}

func main() { gs.Run() }
```

可选的 `EngineMiddleware func(*gin.Engine)` bean(可空)作为最外层包住一切——但
`middleware.enabled=false` 时被静默忽略。

## 2. 全量配置参考

key 在 `spring.gin.server.*` 下(含 tls/observability 约 57 个)。只有 `addr` 必填,其余全有默认。

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `addr` | string | — | 必填;存在即激活 starter |
| `readTimeout` / `writeTimeout` / `idleTimeout` | duration | 5s / 5s / 60s | readTimeout 兼作 ReadHeaderTimeout |
| `tls.enabled` / `tls.cert-file` / `tls.key-file` / `tls.ca-file` | | false / — / — / — | 走 `tlsconf.BuildServer`(与 starter-grpc 同语义);配置 `ca-file` 即开启 **mTLS**(ClientCAs + `RequireAndVerifyClientCert`,客户端必须出示该 CA 签发的证书)。`server-name`/`insecure-skip-verify` 是客户端 key,server 侧无效。 |
| `health.enabled` / `health.path` | bool / string | false / /healthz | |
| `observability.level` / `maxArgBytes` / `skipOps` | | brief / 512 / — | 韧性访问日志 |
| `middleware.enabled` | bool | true | 总开关;false = 手动模式,自己调导出的中间件函数 |

中间件子 key(均在 `middleware.*` 下,各自带 `.enabled`):

| 分组 | 主要 key(默认值) |
|------|------------------|
| `loadtest` | header `X-LoadTest`(开)——README 表里没有 |
| `requestId` | header `X-Request-Id`(开)——自实现(uuid),不是 gin-contrib/requestid |
| `accessLog` | `skipPaths`;`payload.enabled`(开!)`payload.limit` 524288;`metrics.sseDistributions` 开、`metrics.activeRequests` 关 |
| `cors` | allowAllOrigins/allowedOrigins/allowedMethods(空时代码默认全动词集)/allowedHeaders/exposeHeaders/allowCredentials/maxAge——默认关 |
| `gzip` | level 5、minLength 0——默认关 |
| `secureHeaders` | frameOptions DENY、referrerPolicy no-referrer、hsts.*——默认关 |
| `admission` / `fault` | (无 key——治理驱动) |

⚠ 报文捕获**默认开启**:最大 512 KiB 的请求/响应体会进访问日志。

## 3. 中间件链与扩展

顺序(开启时,最外→最内):

```
LoadTest → RequestID → Observe(Recovery+Tracing+Metrics+AccessLog) → admission(韧性)
→ fault → SecureHeaders → CORS → Gzip → ResponseCapture → 健康路由 → 应用路由
```

(⚠ README 的顺序表过期——漏了 LoadTest/admission/fault/ResponseCapture,且把 RequestID 画在
Observe 之内。)手动模式导出 `ApplyMiddlewares`、`LoadTest`、`RequestID`、`Observe`、
`SecureHeaders`、`CORS`、`Gzip`、`ResponseCapture`、`RequestIDFromContext`。

## 4. 可观测与治理

- 访问日志 tag `_app_gin_access`;按状态定级(≥500 Error、≥400 Warn);字段含
  `duration_ms`、`request_id`、`trace_id`/`span_id`、`req.body`/`resp.body`、`panic`+stack。
- tracing/metrics 走 OTel 全局对象——**无 starter-otel 时静默空操作**:server span
  `{method} {route}`、SSE 子 span、`http.server.request.duration` 等。
- 治理:入口准入经 `resilience.ExecutorFor("gin::{addr}")`——限流/舱满 429、熔断开 503;
  故障注入经 `fault.InjectorFor()`。均由治理中心热更新;恒安装,未配置时透传。

## 5. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | ~57(47 + tls/observe) |
| 必填 | 1(`addr`) |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 6 |

设计嫌疑清单(待设计裁决):

1. README 激活声明(`spring.gin.server.enabled`、"默认 :8001")有误——真实条件是 `addr`
   存在;中间件顺序表过期;requestId 归属写错。
2. 已修复——server 侧改用 `tlsconf.BuildServer`(原来 `ServeTLS` 只吃 cert/key):`ca-file`
   即开启 mTLS(`RequireAndVerifyClientCert`),与 starter-grpc 对齐;`server-name`/
   `insecure-skip-verify` 仍为客户端 key,server 侧无效果。
3. 报文捕获默认开(512 KiB 进日志)——隐私/体量惊讶点。
4. `middleware.enabled=false` 时 `EngineMiddleware` 被静默忽略。
5. 指标创建错误被丢弃(`_, _ =`)。
6. example-resilience 的文档注释仍引用不存在的 `resilience.enabled` key。
