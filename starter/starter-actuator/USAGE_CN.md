# starter-actuator 使用与设计 — 参考手册

详细参考。先读 [README_CN](README_CN.md) 有整体印象。文中一切行为都对照过源码
（`actuator.go`、`probes.go`）和自校验的 [example/](example/)。Kubernetes 探针语义以
[kubelet](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
的行为为准；Spring Boot Actuator 的名字只是照护熟悉度。

**怎么打开**：配了 `spring.actuator.addr`，server bean 才会注册。这个 key 就是开关——
没有 `enabled` key。探针、`/info`、贡献端点（如 `/metrics`）全在这一个端口上，和业务
端口分开。

---

## 1. 完整工程示例

一个服务：echo 业务 server + 可切换的依赖健康 indicator + 管理端口上的探针，
Prometheus 指标也在同一端口。

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
    go-spring.org/starter-actuator   latest   // 探针，:9370
    go-spring.org/starter-otel       latest   // 可选：/metrics 挂同一端口
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

**health.go**——健康的全部故事就这么多。把 bean 导出成 `health.Indicator`，它就自动进
探针聚合。接口在 `cloud/actuator/health`，你的组件不用 import 本 starter：

```go
package health

import (
    "context"
    "errors"
    "sync/atomic"

    "go-spring.org/cloud/actuator/health"
    "go-spring.org/spring/gs"
)

// DepDown 模拟"依赖坏了"。admin 路由翻转它，§3 演练就能在不杀进程的情况下
// 强制 DOWN。
var DepDown atomic.Bool

// CacheDown：NonCritical 的例子——DOWN 只在 components 里逐项展示，
// readyz 保持 200 DEGRADED 而不是 503。
var CacheDown atomic.Bool

func init() {
    // 默认 critical：算进 readiness+startup。
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

**router.go**——业务路由 + §3 演练用的 admin 开关：

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
            // 演练钩子（§3）：从外部翻转演示 indicator。
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

**conf/app.properties**——actuator 相关的每个 key，带注释：

```properties
# --- echo 业务 server --------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator（本 starter）---------------------------------------------------
# 开关：配了这个 key 管理服务器才注册。绑全地址，集群内 K8s 探针才够得着。
# 端口布局惯例：主 HTTP :9090、actuator :9370、pprof 127.0.0.1:9981。
spring.actuator.addr=:9370

# 整个端口的可选鉴权：bearer token（优先）或 HTTP Basic。
# 都不配 + 非 loopback 地址 = 启动打 WARN。
spring.actuator.token=
spring.actuator.username=
spring.actuator.password=

# --- 可观测（starter-otel）---------------------------------------------------
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics 只由 actuator 提供
```

**conf/govern.yaml**（非 actuator 专属，展示全貌用）：

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
  └─ gs.Provide(&Server{})  [仅当 spring.actuator.addr 已配置]
        │   以自己的名字导出 gs.Server，与应用主 HTTP server（同样导出
        │   gs.Server）共存
gs.Run()
  ├─ 配置绑定: addr / endpoints.include / token / username / password
  ├─ bean 装配: []health.Indicator、[]endpoint.Endpoint——全部可选
  ├─ Server.Run(): 监听、注册路由、包上鉴权 guard；
  │     没配鉴权且非 loopback → WARN——立刻开始 SERVING，不等应用就绪
  ├─ 就绪: sig.TriggerAndWait() → 全部 server（含本 server）报告 ready 后，
  │     s.ready 置 true
  ├─ 收到 SIGTERM: PreStop 置 draining=true
  │     → /readyz 应答 503 OUT_OF_SERVICE，但 server 继续服务，
  │       K8s 得以在 Stop() 之前把 pod 摘出 Service
  └─ Stop: http.Server.Shutdown 随停机上下文走
```

为什么先服务、后就绪：就绪探针要抓到「没就绪 → 就绪」的翻转，存活探针要在慢启动
全程有人应声——不然 Pod 会被白白重启。

### 2.2 端点注册顺序

```
探针:      /healthz /readyz /startupz（+ /health /readiness /startup 别名）
           ——无条件注册，过滤器碰不了它们（否则破坏 Kubernetes 契约）
自省:      /info——过 include 过滤器
贡献端点:  每个 []endpoint.Endpoint bean，过滤键 = pattern 的路径
           （去掉方法前缀，如 "/metrics"）；注册在内建之后，贡献者遮不了
           /health——路径重复时 ServeMux 启动即 panic，错误藏不住
```

### 2.3 一次 readiness 请求，从头走到尾

启动后的 `GET /readyz`，两个 indicator 都注册了，mysql 正常、redis 挂了：

1. 还没就绪、或正在排空？→ 直接 `503 {"status":"OUT_OF_SERVICE"}`。
2. 否则扫描 readiness 组里的每个 indicator。归属：显式 `HealthGroups()`，否则默认
   **readiness + startup，绝不含 liveness**——依赖检查永远不能触发 Pod 重启。
3. 扫描有 **3s 总超时**：单个慢依赖拖不过典型 kubelet 超时；迟到的 indicator 看到
   过期 ctx，以 deadline 错误报 DOWN。
4. 逐个：无错 → `components[name] = {status: UP}`；有错 →
   `{status: DOWN, error: err}`。
5. 裁决：全 UP → `UP`；仅非 critical DOWN → `DEGRADED`；任一 critical DOWN → `DOWN`。
6. 只有 `DOWN` 对应 HTTP 503。`DEGRADED` 保持 200——应用还在服务，一个可容忍的
   依赖在失败，而且看得出是哪个。

```json
{
  "status": "DEGRADED",
  "components": {
    "mysql:orders": {"status": "UP"},
    "redis:cache":  {"status": "DOWN", "error": "context deadline exceeded"}
  }
}
```

`/healthz` 只扫 **liveness** 组——通常没人声明 liveness，所以进程活着就是
`200 {"status":"UP"}`。`/startupz` 额外看 `s.ready`，且无视排空（启动成功后 kubelet
不再轮询它）。

### 2.4 端点响应形态

| 端点 | 响应 | 源码 |
|------|------|------|
| `GET /info` | `{"go","module":{"path","version"},"build":{"revision","time","modified"}}`，来自 `debug.ReadBuildInfo` | actuator.go |
| 贡献端点（如 `/metrics`） | 完全是贡献方的事（starter-otel 为 Prometheus 文本） | cloud/actuator/endpoint |

---

## 3. 演练

### 3.1 基线

```bash
curl -s :9370/readyz | jq .status          # "UP"（redis 挂了则 "DEGRADED"）
curl -s :9370/healthz                      # {"status": "UP"}
curl -s :9370/startupz | jq .status        # 启动完成后 "UP"
```

启动早期还能抓到 `curl -i :9370/readyz` → `503 OUT_OF_SERVICE`——就绪屏障跨越之前；
example 的自测断言的就是这个序列。

### 3.2 critical indicator DOWN → readyz 503、healthz 仍 200

```bash
curl -i -X POST :8002/admin/dep/down       # 翻转 critical 的 mysql:orders
curl -i :9370/readyz                       # 503 DOWN，components 里点名肇事者
curl -i :9370/healthz                      # 200——依赖挂了绝不能触发存活重启
curl -i -X POST :8002/admin/dep/up         # 恢复 → readyz 200 UP
```

### 3.3 非 critical indicator DOWN → DEGRADED、仍 200

```bash
curl -i -X POST :8002/admin/cache/down     # redis:cache 是 NonCritical
curl -s :9370/readyz | jq '.status, .components."redis:cache".status'
# "DEGRADED"        <- HTTP 200：仍在服务
# "DOWN"
curl -i -X POST :8002/admin/cache/up
```

### 3.4 Drain 翻转（优雅停机）

```bash
kill -TERM <pid>                           # 或 Ctrl+C
# 立刻、反复地：
curl -s -o /dev/null -w '%{http_code}\n' :9370/readyz   # 503 OUT_OF_SERVICE
curl -s -o /dev/null -w '%{http_code}\n' :9370/healthz  # 200——整个排空窗口内
                                                          # 探针持续可答
```

`PreStop` 在 server 停止**之前**置 `draining`，Kubernetes 趁 in-flight 请求收尾把 pod
摘出 Service。example 的 `runTest` 自动化了全程（SIGTERM + 轮询）。`/startupz` 按设计
无视排空。

### 3.5 端点过滤

```properties
spring.actuator.endpoints.include=/info,/metrics
```

```bash
curl -i :9370/info        # 200（默认开，也在 include 里）
curl -i :9370/metrics     # 200（贡献端点在白名单里）
curl -i :9370/readyz      # 200——探针不受过滤影响
```

不想让 `metrics` 这类贡献端点出现？去它自己的 starter 里关——没有 exclude 名单。
没配鉴权 + 非 loopback 地址还会打：
`WARN actuator listening on ":9370" without authentication; ...`。

### 3.6 鉴权

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

### 3.7 慢依赖封顶

整个 readiness 扫描共享一个 3s 预算。接一个 `CheckHealth` 睡 5s 的 indicator →
`readyz` 约 3s 内应答，该组件 `DOWN "context deadline exceeded"`——而不是探针超时。

---

## 4. 设计

actuator 是 Server 原型的 starter：自己端口上的管理 HTTP server——业务 server 的
运维对位物。

**它管什么**

- `/healthz`、`/readyz`、`/startupz`（K8s 探针；不带 z 的旧名是别名）和 `/info`。
- 收集所有 `health.Indicator` bean 汇进 `/readyz`——它不认识任何具体后端
  （redis、gorm……）；缝是 stdlib 接口。
- 收集所有 `endpoint.Endpoint` bean 挂到同一端口（今天是 otel 的 `/metrics`，
  明天是未来贡献者）——跨 starter 无 import。
- 与应用主 HTTP server（不同 bean 名）、`starter-pprof`、`starter-admin-ui` 共存
  ——各占一个端口。

**值得知道的决策**

- **先服务、后就绪。** 绑定即应答；`sig.TriggerAndWait` 只是旁观聚合。探针契约
  要求如此。
- **`health.Indicator` 在 `cloud/actuator/health`，不在 starter 里。** 每个贡献方
  （redis、gorm……）都要够得着它，又不必 import 本 starter。
- **`PreStop` 翻转 readiness。** `draining=true` → `/readyz` 503、in-flight 请求
  收完；端点控制器摘 pod，然后各 server 停止。
- **端点贡献。** 内建先注册、贡献者后挂；重复 pattern 启动即 panic——配置错误要
  响。敏感性是贡献方的事（`Endpoint.Sensitive`），actuator 不代裁。
- **每次 readiness 扫描一个 3s 预算。** 单个慢 indicator 拖不垮探针。

**否决过的备选**

- **复用 app 主 HTTP mux。** 探针必须在启动期与排空期可答——listener 的生命周期
  必须和应用就绪门解耦。
- **由后端 starter 推送 indicator。** 推送逼着每个后端 import actuator；按接口
  导出拉取，后端怎么组合都行。
- **未决：3s 预算是共享的。** indicator 一多单个预算被摊薄，慢的会饿死其余。
  候选：逐 indicator 上限。

---

## 5. 排障表

| 症状 | 为什么 | 怎么办 |
|------|--------|--------|
| 完全没有 actuator 端点 | `spring.actuator.addr` 没配 | 配上——这个 key 就是开关。 |
| 启动失败：`actuator: failed to listen on :9370` | 端口被占（另一个 actuator、pprof 撞了） | 换 `addr` 或放端口。 |
| 启动后 `/readyz` 一直 503 OUT_OF_SERVICE | 别的 `gs.Server` 一直没报 ready，就绪屏障没跨过去 | 查哪个 server 卡了就绪；`s.ready` 只在全部 server ready 后翻转。 |
| `/readyz` 503 DOWN、`/healthz` 200 | critical readiness indicator 挂了——行为正确 | 读 `components`，看是哪个依赖、什么错。 |
| `/metrics` 404 | 没 import starter-otel，或非空 `endpoints.include` 漏了 `metrics` | import starter-otel；白名单时记得贡献端点。 |
| 敏感贡献端点 404、日志无痕 | 默认关；过滤只打 Debug 日志 | 把它的路径加进 `endpoints.include`（大小写不敏感精确匹配）。 |
| 启动 panic：`http: multiple registrations for /...` | 贡献 pattern 撞了内建或其他贡献者 | 改贡献方 pattern——panic 就是设计好的快速失败。 |
| 所有端点 401 | 配了 token（或 Basic 对）——guard 连探针一起管 | 探针和抓取方带上头，或删掉这些 key 只绑 loopback。 |
| 探针间歇性 503 DOWN，报 `context deadline exceeded` | 某个 indicator 吃光了共享的 3s 预算 | 让 `CheckHealth` 尊重 ctx / 收紧检查。 |
