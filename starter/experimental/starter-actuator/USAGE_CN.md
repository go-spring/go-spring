# starter-actuator 使用说明 — 参考手册

详细使用参考。概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`actuator.go`、`probes.go`、`endpoints.go`、`beans.go`）与自校验的 [example/](example/)
（`example/check.sh`）核实。Kubernetes 探针语义以
[kubelet 契约](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
为准；Spring Boot Actuator 命名对照见
[官方文档](https://docs.spring.io/spring-boot/reference/actuator/endpoints.html)——
本文写的都是 go-spring 的增量。

**激活方式**：只有配置了 `spring.actuator.addr`，actuator server bean 才会注册——这个 key
就是开关，没有 `enabled` key（actuator.go:93-95，`gs.OnProperty`）。单一管理端口模型：
探针、自省端点、贡献端点（如 `/metrics`）共享一个独立端口，与应用 HTTP 端口分开。

---

## 1. 完整工程示例

一个贴近真实的服务：echo 业务 server + 可切换的依赖健康 indicator + 管理端口上的探针，
Prometheus 指标也挂同一端口。文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
├── health.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod**（关键依赖）：

```
require (
    github.com/labstack/echo/v4  latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-echo       latest
    go-spring.org/starter-actuator   latest   // 探针 + 自省，:9370
    go-spring.org/starter-otel       latest   // 可选：真实指标导出、/metrics 挂载
    go-spring.org/starter-governance latest   // 可选：运行时故障注入
)
```

**main.go**：

```go
package main

import (
    _ "demo/health"
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**health.go**——健康贡献的全部故事。任何导出为 `health.Indicator` 的 bean 都会被折叠进
探针聚合，零逐组件接线；接口定义在 `cloud/actuator/health`，你的组件不需要 import 本
starter：

```go
package health

import (
    "context"
    "errors"
    "sync/atomic"

    "go-spring.org/cloud/actuator/health"
    "go-spring.org/spring/gs"
)

// DepDown 模拟"依赖坏了"（连接池死了、连接断了）。由 admin 路由翻转，
// 供 §4 演练在不杀进程的情况下强制 DOWN。
var DepDown atomic.Bool

// 可选缓存示例：NonCritical 表示 DOWN 只在 components 里逐项展示，
// readyz 保持 200 DEGRADED 而不是 503。
var CacheDown atomic.Bool

func init() {
    // 默认 critical，默认归属 readiness+startup 组。
    gs.Provide(health.NewIndicator("mysql:orders", func(ctx context.Context) error {
        if DepDown.Load() {
            return errors.New("dependency unavailable")
        }
        return nil // 真实探活：如 db.PingContext(ctx)
    })).Export(gs.As[health.Indicator]()).Name("mysql-orders-indicator")

    gs.Provide(health.NewIndicator("redis:cache", func(ctx context.Context) error {
        if CacheDown.Load() {
            return errors.New("cache unavailable")
        }
        return nil
    }, health.NonCritical())).Export(gs.As[health.Indicator]()).Name("redis-cache-indicator")
}
```

**router.go**——业务路由 + §4 演练用的 admin 开关：

```go
package router

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"

    healthpkg "demo/health"
    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.GET("/orders/:id", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
            })
            // 演练钩子（§4）：从外部翻转演示 indicator。
            e.POST("/admin/dep/:state", func(c echo.Context) error {
                healthpkg.DepDown.Store(c.Param("state") == "down")
                return c.NoContent(http.StatusOK)
            })
            e.POST("/admin/cache/:state", func(c echo.Context) error {
                healthpkg.CacheDown.Store(c.Param("state") == "down")
                return c.NoContent(http.StatusOK)
            })
        }
    })
}
```

**conf/app.properties**——与 actuator 相关的完整注释配置面：

```properties
# --- echo 业务 server --------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator（本 starter）---------------------------------------------------
# 激活 key：出现即注册管理 server。无默认值——绑全地址以便集群内 K8s 探针可达。
# 端口布局惯例：主 HTTP :9090、actuator :9370、pprof 127.0.0.1:9981。
spring.actuator.addr=:9370

# 敏感自省端点（loggers、env、configprops、threaddump、beans）默认关闭；
# 要暴露必须显式列入 include。/info 与贡献端点（如 metrics）默认开启；
# 探针永远注册。
spring.actuator.endpoints.include=loggers,env,configprops,threaddump
spring.actuator.endpoints.exclude=

# 整个管理端口的可选鉴权：bearer token（优先）或 HTTP Basic。
# 两者都不配且绑非 loopback 地址时启动打 WARN。
spring.actuator.token=
spring.actuator.username=
spring.actuator.password=

# --- 可观测（starter-otel）---------------------------------------------------
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 只由 actuator 提供

# --- 日志（让 /loggers 有内容可列）-------------------------------------------
logging.logger.root.type=Logger
logging.logger.root.level=INFO
```

**conf/govern.yaml**（非 actuator 专属；为全栈姿势保留）：

```yaml
govern:
  enabled: true
  fault:
    enabled: false
    rate: 0.2
    error: timeout
```

**验证**：

```bash
curl -i :8002/orders/7          # 200 业务路由
curl -i :9370/healthz           # 200 {"status": "UP"}
curl -i :9370/readyz            # 200 UP（带 components，见 §2.3）
curl -i :9370/startupz          # 启动完成后 200
curl -i :9370/info              # 构建/版本元数据
curl -i :9370/metrics           # starter-otel 贡献，同一端口
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-actuator
  └─ gs.Provide(&Server{})  [条件: spring.actuator.addr 已配置]  (actuator.go:93-95)
        │   以独立名字导出 gs.Server——与应用主 HTTP server（同样导出 gs.Server）
        │   共存
gs.Run()
  ├─ 配置绑定: addr / endpoints.include / endpoints.exclude /
  │               token / username / password（value tag）
  ├─ bean 装配: []health.Indicator、[]endpoint.Endpoint、
  │             *gs.PropertiesRefresher、BeanLister——全部 autowire:"?" 可选
  ├─ Server.Run(): net.Listen、mux 注册、立即开始 SERVING
  │     （先于应用就绪——readiness 探针必须能观察到 OUT_OF_SERVICE → UP 迁移；
  │      actuator.go:22-25、265-273）
  ├─ 就绪: sig.TriggerAndWait() → goroutine 在全部 server（含本 server）
  │     报告 ready 后置 s.ready（actuator.go:269-273）
  ├─ 收到 SIGTERM: PreStop(ctx) 置 draining=true  (actuator.go:305-307)
  │     → /readyz 翻为 503 OUT_OF_SERVICE，但 server 继续服务，
  │       端点控制器在 Stop() 之前把 pod 摘出 Service
  └─ StopContext(ctx): http.Server.Shutdown(ctx) 随停机上下文排空
```

为什么先服务后就绪（源码注释，actuator.go:21-25）："a readiness probe must be able to
reach the endpoint *before* the app is ready so it can observe the OUT_OF_SERVICE -> UP
transition, and a liveness probe must answer throughout a long startup so the pod is not
killed prematurely."

### 2.2 端点注册顺序

```
探针路由:   /healthz /readyz /startupz（+ /health /readiness /startup 别名）
            ——无条件注册（actuator.go:228-235）；actuator.go:169-171 源码注释：
            过滤它们 "would break the Kubernetes contract"
自省端点:   info、loggers、env、configprops、threaddump、beans——逐个先过
            include/exclude 过滤器（actuator.go:238-243）
贡献端点:   每个 []endpoint.Endpoint bean，名字 = 去掉 "/" 的路径，
            同一过滤器（actuator.go:251-258）；注册在内建之后，贡献者无法遮蔽
            /health——路径重复时 ServeMux 在启动期 panic，快速暴露配置错误
```

### 2.3 一次 readiness 请求，逐层走读

启动后的 `GET /readyz`，两个 indicator 均已注册，mysql 正常、redis DOWN：

1. `s.ready.Load()` 为 true 且 `s.draining.Load()` 为 false——否则立即返回
   `503 {"status":"OUT_OF_SERVICE"}`（probes.go:120-126）。
2. `checkGroup(GroupReadiness)` 扫描所有组内含 readiness 的 indicator。组归属：显式
   `HealthGroups()`，否则默认 **readiness + startup，绝不含 liveness**（probes.go:34-39
   ——依赖检查绝不能触发 pod 重启）。
3. 扫描带 **3s 总超时**（`checkTimeout`，actuator.go:98-100）：单个慢依赖不能把探针
   拖过典型 kubelet 超时；超时的 indicator 会看到过期 ctx 并以 deadline 错误报 DOWN。
4. 逐 indicator：`CheckHealth(ctx)` 返回 nil → `components[name] = {status: UP}`；
   出错 → `{status: DOWN, error: err}`（probes.go:74-83）。
5. 聚合（probes.go:55-62、85-88）：全 UP → `UP`；仅非 critical DOWN → `DEGRADED`；
   任一 critical DOWN → `DOWN`。
6. `writeProbe`：仅 DOWN 映射 HTTP 503；DEGRADED 保持 200——"the app is still serving
   traffic, only a tolerable dependency is failing"（probes.go:91-94）。

```json
{
  "status": "DEGRADED",
  "components": {
    "mysql:orders": {"status": "UP"},
    "redis:cache":  {"status": "DOWN", "error": "context deadline exceeded"}
  }
}
```

`/healthz` 对 **liveness** 组做同样扫描——没有 indicator 声明 liveness（常态）时只要
进程在服务就平凡地返回 `200 {"status":"UP"}`。`/startupz` 额外以 `s.ready` 为门，且
不受 drain 影响（probes.go:131-135：startup 成功后 kubelet 不再轮询它）。

### 2.4 自省 handler（精确响应形态）

| 端点 | 响应形态 | 源码 |
|------|----------|------|
| `GET /info` | `{"go","module":{"path","version"},"build":{"revision","time","modified"}}`，来自 `debug.ReadBuildInfo` | actuator.go:312-336 |
| `GET /loggers` | `{"loggers": {"root": {"configuredLevel": "INFO"}, ...}}`——只读；`POST /loggers/{name}` 有意不实现 | endpoints.go:54-62 |
| `GET /env` | `{"propertySources": [{"name", "properties": {key: {"value": masked}}}]}`——按优先级排序（高在前）、**不合并** | endpoints.go:64-84 |
| `GET /configprops` | 数组 `[{"name", "config": <嵌套树>}]`——同样数据源、树视图、同一掩码 | endpoints.go:90-100 |
| `GET /threaddump` | `text/plain` goroutine 堆栈（debug 级别 2） | endpoints.go:104-108 |
| `GET /beans` | `{"beans": [{"name","type"}]}`；无 `BeanLister` bean 时为 `{"beans": [], "note": "bean registry not available..."}` | beans.go:51-63 |
| 贡献端点（如 `/metrics`） | 完全由贡献方决定（starter-otel 为 Prometheus 文本格式） | cloud/actuator/endpoint |

掩码（endpoints.go:29-71）：值替换为 `"******"` 的条件——**key** 命中大小写不敏感正则
`password|passwd|secret|token|credential|api-?key|private-key|access-key`（子串匹配，
`spring.datasource.password` 与 `auth.access-key` 都命中）；或 key 的**最后一个**
`.`/`_`/`-` 分段恰好是 `key` / `api-key` / `api_key` / `apikey`（结尾锚定 + 前置分隔
符：`some.key`、`aws.key` 命中，`monkey`、`keyword`、`keynote` 不命中）；或**值**是
配置加密产生的 `ENC(...)` 占位符。值是**内嵌凭据的 URL**（`scheme://user:pass@host`，
含空用户名 `scheme://:pass@host`）时只把 userinfo 部分掩为
`scheme://******@host`，URL 其余部分保持可读。其余原样透出。

---

## 3. 逐 key 行为参考

全量：这六个就是 starter 里全部的 `value` tag（已用
`grep -rhoE 'value:"[^"]+"'` 交叉核对，两边无多余项）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `spring.actuator.addr` | string | — | **激活 key + 监听地址。** 出现即注册 `gs.Server` bean（actuator.go:93-95）。按策略无默认值（starter server 端口必须用户显式配置）；文档布局 `:9370`、全地址以便集群内探针可达。 | 缺失 → 整个 starter 静默不激活（无探针、无 `/metrics` 挂载）。地址非法 → `Run` 以 `actuator: failed to listen on <addr>` 失败。 |
| `spring.actuator.endpoints.include` | string（逗号列表） | `""` | 非空 = **白名单模式**：只有点名的自省/贡献端点注册。名字为去掉 `/` 的路径：`info、loggers、env、configprops、threaddump、beans`，以及贡献端点自身路径（如 `metrics`）。匹配精确、大小写不敏感、容忍空白。⚠ **敏感端点默认关闭**：`loggers、env、configprops、threaddump、beans` 只有显式列入本 key 才注册（列表其余部分为空也一样）。⚠ **探针端点豁免**——`/healthz` 等无条件注册。 | 名字写错 → 该端点静默关闭（仅 Debug 日志）；白名单漏写 `metrics` 会丢掉唯一的指标端口。**迁移（2026-08-28）**：原先依赖默认全开自省端点的配置需显式加 `spring.actuator.endpoints.include=env,configprops,loggers,threaddump,beans`。 |
| `spring.actuator.endpoints.exclude` | string（逗号列表） | `""` | **黑名单，始终生效——白名单内也一样，显式 include 敏感端点也会被它压掉**（测试 `TestEndpointFilter_ExcludeBeatsSensitiveInclude`）。名字语法相同。 | 误排除实际要抓取的 `metrics`/`info`；静默（仅 Debug 日志）。 |
| `spring.actuator.token` | string | `""` | 整个管理端口的 bearer token 鉴权（共享 `stdlib/httpauth` guard）：每个请求都要带 `Authorization: Bearer <token>`。优先于 username/password。常数时间比较。 | 头缺失/错误 → 所有端点（含探针）401——K8s 探针也得带头，或把 actuator 只绑 loopback。 |
| `spring.actuator.username` | string | `""` | 与 `spring.actuator.password` 成对启用 HTTP Basic（两者都设才生效；同时设 token 则 token 优先）。失败返回 401 + `WWW-Authenticate: Basic`。 | 只设一半 → guard 不生效；非 loopback 监听会打 WARN。 |
| `spring.actuator.password` | string | `""` | 见 `spring.actuator.username`。 | 见 `spring.actuator.username`。 |

没有任何 key 与主 HTTP server 联动：actuator 永远独占自己的端口。没有 TLS / 路径前缀
key——见 §5/§6。未配置任何鉴权且地址非 loopback 时，启动打 WARN
（`listening on %q without authentication`）。

---

## 4. 验证与故障演练

所有演练假设 §1 工程已运行（`go run .`）。

### 4.1 基线探针

```bash
curl -s :9370/readyz | jq .status          # "UP"（redis DOWN 时为 "DEGRADED"）
curl -s :9370/healthz                      # {"status": "UP"}
curl -s :9370/startupz | jq .status        # 启动完成后 "UP"
```

启动早期可抓到 `curl -i :9370/readyz` → `503 {"status": "OUT_OF_SERVICE"}`（就绪屏障
跨越之前；example 的自测断言了这一序列）。

### 4.2 critical indicator DOWN → readyz 503、healthz 仍 200

```bash
curl -i -X POST :8002/admin/dep/down       # 翻转 critical 的 mysql:orders
curl -i :9370/readyz                       # 503 {"status":"DOWN","components":{"mysql:orders":{"status":"DOWN","error":"..."}}}
curl -i :9370/healthz                      # 200 {"status":"UP"}——依赖劣化绝不能
                                           # 触发 liveness 重启（probes.go:107-110）
curl -i -X POST :8002/admin/dep/up         # 恢复 → readyz 200 UP
```

### 4.3 非 critical indicator DOWN → DEGRADED、仍 200

```bash
curl -i -X POST :8002/admin/cache/down     # redis:cache 是 NonCritical
curl -s :9370/readyz | jq '.status, .components."redis:cache".status'
# "DEGRADED"        <- HTTP 200：仍在服务（probes.go:91-94）
# "DOWN"
curl -i -X POST :8002/admin/cache/up
```

### 4.4 Drain 翻转（优雅停机）

```bash
kill -TERM <pid>                           # 或 Ctrl+C
# 立刻、反复地：
curl -s -o /dev/null -w '%{http_code}\n' :9370/readyz   # 503 OUT_OF_SERVICE
curl -s -o /dev/null -w '%{http_code}\n' :9370/healthz  # 200——整个 drain 窗口内
                                                          # 探针持续可答
```

`PreStop` 在 `Stop` 关停 server **之前**置 `draining`（actuator.go:299-307），Kubernetes
得以在 in-flight 请求完成期间把 pod 摘出 Service 端点。example 的 `runTest` 自动化了
这个序列（example.go，SIGTERM + 轮询循环）。注意 `/startupz` 按设计不受 drain 影响
（probes.go:131-135）。

### 4.5 端点过滤

```properties
spring.actuator.endpoints.include=info,env,metrics
spring.actuator.endpoints.exclude=configprops
```

```bash
curl -i :9370/info        # 200（默认开启，也在 include 内）
curl -i :9370/metrics     # 200（贡献端点在白名单内）
curl -i :9370/env         # 200（敏感端点：必须显式 include）
curl -i :9370/configprops # 404（敏感且被 exclude）
curl -i :9370/threaddump  # 404（敏感、未 include——即默认状态）
curl -i :9370/readyz      # 200——探针豁免于过滤器
```

未配鉴权且 addr 非 loopback 时，启动还会打：
`WARN actuator listening on ":9370" without authentication; set ${spring.actuator.token} ...`。

### 4.6 鉴权

```properties
spring.actuator.token=s3cret           # 或：
spring.actuator.username=admin
spring.actuator.password=pw
```

```bash
curl -i :9370/healthz                   # 401
curl -i -H 'Authorization: Bearer s3cret' :9370/healthz      # 200
curl -i -u admin:pw :9370/info          # 200（Basic，未设 token 时）
```

### 4.7 机密掩码

```bash
curl -s :9370/env | grep -E 'password|token|key|redis'
# "demo.datasource.password": {"value": "******"}
# "demo.api.token":          {"value": "******"}
# "demo.aws.key":            {"value": "******"}   # 末段恰为 "key"
# "demo.redis.url":          {"value": "redis://******@127.0.0.1:6379/0"}
curl -s :9370/configprops | grep ENC      # 嵌套树，叶子同样是 "******"
```

### 4.8 慢依赖封顶

整个 readiness 扫描共享一个 3s 预算（actuator.go:100）。接一个 `CheckHealth` 睡 5s 的
indicator → `readyz` 约 3s 内返回，该组件 `DOWN "context deadline exceeded"`，而不是
探针超时。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 完全没有 actuator 端点 | `spring.actuator.addr` 缺失 | 配上——该 key 是激活开关（actuator.go:93-95）。 |
| 启动失败：`actuator: failed to listen on :9370` | 端口被占（另一个 actuator、与 pprof 重叠） | 改 `addr` 或释放端口。 |
| 启动后 `/readyz` 一直是 503 OUT_OF_SERVICE | 就绪屏障未跨越——某个 `gs.Server` 一直没报 ready | 排查哪个 server 阻塞了就绪；actuator 只在全部 server ready 后置 `s.ready`（actuator.go:269-273）。 |
| `/readyz` 503 DOWN、`/healthz` 200 | critical readiness indicator 失败——这是正确行为 | 读响应体 `components` 里失败依赖的名字与错误串。 |
| `/metrics` 404 | 未 import starter-otel，或非空 `endpoints.include` 漏了 `metrics` | import starter-otel；使用白名单时记得贡献端点。 |
| 自省端点 404、日志无痕 | 敏感端点默认关闭（或被过滤器关掉）——只打 Debug 日志 | 把端点加进 `endpoints.include`（大小写不敏感的精确名字）。 |
| 所有端点 401 | 配了 `spring.actuator.token`（或 Basic 对）——guard 对探针同样生效 | 探针/抓取方带上头或 token；或去掉这些 key 只绑 loopback。 |
| 启动 panic：`http: multiple registrations for /...` | 贡献 `endpoint.Endpoint` 路径与内建或其他贡献者冲突 | 改贡献方 `Path()`；ServeMux 在注册期 panic（actuator.go:249-253）——设计上的快速失败。 |
| `/env` 数据源比预期多 / 值"不对" | `/env` 按源展示、**不合并**、高优先级在前 | 自行跨源判断聚合（endpoints.go:64-67）；树视图用 `/configprops`。 |
| `/env` 里出现疑似机密的明文 | key 不匹配掩码正则（如 `demo.license` 不命中；URL 内嵌凭据**不**掩码） | 改 key 命名，或作为设计问题上报——掩码只对 key 做正则（endpoints.go:32-45）。 |
| 探针间歇性 503 DOWN，报 `context deadline exceeded` | 某个 indicator 吃掉了共享的 3s 扫描预算 | 让 `CheckHealth` 尊重 ctx / 收紧依赖检查。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 3 |
| 其中必填 | 1（`addr`，兼作开关） |
| quickstart 前置外部依赖数 | 0 |
| "注意/坑"条数 | 7（探针豁免过滤、鉴权对探针同样生效；敏感端点默认关闭；过滤仅 Debug 日志；/env 不合并；/beans 需贡献者；掩码不含 query 串凭据；共享 3s 扫描预算） |

设计嫌疑清单（交设计裁决）：

1. ~~死 key `spring.actuator.enabled`~~ — FIXED 2026-08-27：已从 README、DESIGN 和约
   20 份 example 配置中移除。
2. ~~过期的 `POST /loggers` 文档~~ — FIXED 2026-08-27：README_CN/DESIGN 现声明只读；
   删除了幽灵字段 `"levels"`。
3. ~~`:9370` 默认值表述~~ — FIXED 2026-08-27：文档/注释现说明"无默认值；该 key 激活
   starter"。
4. ~~`/env` 被写成合并视图~~ — FIXED 2026-08-27：现按源展示、不合并。
5. ~~全地址管理端口无安全层~~ — FIXED 2026-08-28：`spring.actuator.token` /
   `.username` / `.password` 用共享 `stdlib/httpauth` guard 保护整个端口（与
   starter-pprof 同款），敏感自省端点默认关闭、须显式 include。
6. ~~掩码正则漏掉泛化敏感 key、不掩值内嵌凭据~~ — FIXED 2026-08-28：末段裸 `key`/
   `api-key` 命中（边界锚定：`aws.key` 命中、`monkey`/`keyword` 不命中），URL 内嵌
   userinfo 掩为 `scheme://******@host`。
7. 整个扫描共享 3s `checkTimeout`：indicator 一多单个预算被摊薄，慢的会饿死其余
   （无逐 indicator 超时）。→ 候选：逐 indicator 上限。
8. ~~example 配置的 POST 注释~~ — FIXED 2026-08-27。
9. `/beans` 开箱永远列不出（gs 核心不导出 bean 枚举，beans.go:29-43）——端点以边界
   说明的形式存在。可接受，但用户不应期待 Spring 同等能力。
