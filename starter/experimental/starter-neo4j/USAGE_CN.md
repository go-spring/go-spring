# starter-neo4j 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health.go`）与
可运行示例（[example/](example/)、[example-otel/](example-otel/)、
[example-cloudnative/](example-cloudnative/)、[example-load/](example-load/)）核对——文中方括号
为 file:line 抽查点。**Cypher 语义与 neo4j-go-driver API 属于
[驱动官方文档](https://neo4j.com/docs/go-manual/current/)**——以下只写 go-spring 的增量。

**激活条件**：出现任意 `spring.neo4j.instances.*` key 即注册（模块为 `OnProperty("spring.neo4j")`
前缀检查）。每个 `spring.neo4j.instances.<name>` 条目创建一个名为 `<name>` 的
`*StarterNeo4j.Client` bean，并附带名为 `neo4j:<name>` 的健康指示器。

---

## 1. 完整工程示例

双实例（直连 + 连接池调优）、探针、指标/追踪、治理保护。文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/neo4j/neo4j-go-driver/v5 v5.28.4
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-neo4j        latest
    go-spring.org/starter-actuator     latest   // 可选：readiness + /metrics
    go-spring.org/starter-otel         latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance-file   latest   // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-neo4j"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入 wrapper，通过插桩接缝跑真实 Cypher：

```go
package service

import (
    "context"

    "github.com/neo4j/neo4j-go-driver/v5/neo4j"
    "go-spring.org/spring/gs"
    StarterNeo4j "go-spring.org/starter-neo4j"
)

type Service struct {
    // 恒为 wrapper 类型 *StarterNeo4j.Client。它内嵌裸
    // neo4j.DriverWithContext，每个接口方法均按原样提升，
    // 因此仍满足该接口。按实例名注入（"graph"、"analytics"）。
    Graph     *StarterNeo4j.Client `autowire:"graph"`
    Analytics *StarterNeo4j.Client `autowire:"analytics"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 经 Query 写 + 读：Query 只声明操作，span + 指标 + 访问日志由
            // resilience 层发射，外加韧性保护。与 neo4j.ExecuteQuery 同签名。
            _, err := StarterNeo4j.Query(ctx, s.Graph,
                "MERGE (p:Person {name: $name}) SET p.age = $age RETURN p",
                map[string]any{"name": "alice", "age": 30},
                neo4j.EagerResultTransformer)
            if err != nil {
                panic(err)
            }
            // 经 transformer 取单值结果。
            n, err := StarterNeo4j.Query[int](ctx, s.Graph,
                "MATCH (p:Person) RETURN count(p)",
                nil, neo4j.SingleResultTransformer[int]())
            _ = n // 1
            _ = err

            // 手写 session 代码：必须自己套韧性保护——
            // 它不会被自动保护（见 §2.3）。
            err = StarterNeo4j.RunWithResilience(ctx, s.Analytics, func(ctx context.Context) error {
                sess := s.Analytics.NewSession(ctx, neo4j.SessionConfig{})
                defer sess.Close(ctx)
                _, err := sess.Run(ctx, "MATCH (p:Person) RETURN p.name", nil)
                return err
            })
            _ = err
        }
    })
}
```

**conf/app.properties** —— 上述代码用到的完整配置面：

```properties
# --- 实例 "graph"：直连地址，其余走默认 --------------------------------------
spring.neo4j.instances.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.instances.graph.username=neo4j
spring.neo4j.instances.graph.password=password

# --- 实例 "analytics"：调优连接池，独立健康指示器 ----------------------------
spring.neo4j.instances.analytics.uri=bolt://127.0.0.1:7687
spring.neo4j.instances.analytics.username=neo4j
spring.neo4j.instances.analytics.password=password
spring.neo4j.instances.analytics.max-connection-pool-size=50
spring.neo4j.instances.analytics.connection-acquisition-timeout=30s

# --- observability（starter-otel：OTLP 导出 + prometheus）--------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus

# --- actuator：readiness 汇聚 neo4j:graph 与 neo4j:analytics ------------------
spring.actuator.addr=:9370

# --- governance：Query / RunWithResilience 的保护 ----------------------------
# NOTE: governance RULES go in conf/governance.properties, referenced by spring.governance.source.file.path in app.properties (see starter-governance-file USAGE).
spring.governance.enabled=true
spring.governance.driver=default
spring.governance.client.default.rate-limit=100
spring.governance.client.default.max-retries=1
spring.governance.client.default.attempt-timeout=500ms
```

**验证**（先启动 Neo4j —— `docker run -d -e NEO4J_AUTH=neo4j/password -p 7687:7687 -p 7474:7474 neo4j:5`，
或用 [example/docker-compose.yml](example/docker-compose.yml)）：

```bash
go run .                          # Neo4j 不可达时启动 fail fast
curl -s :9370/readyz | jq .       # components 含 "neo4j:graph"、"neo4j:analytics"
curl -s :9370/metrics | grep -E 'db.client.operation'   # 每次查询的 duration 直方图
grep _app_neo4j_access app.log | tail -3   # 每次 Query 一条访问记录
cypher-shell -a bolt://127.0.0.1:7687 -u neo4j -p password \
  'MATCH (p:Person {name:"alice"}) RETURN p.age'        # 30
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-neo4j
  └─ gs.Module(OnProperty("spring.neo4j.instances"))：出现任意 spring.neo4j.instances.* key 即触发
        └─ conf.BindEach("${spring.neo4j.instances}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient).Name(<name>)
              │    .Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator，名 "neo4j:<name>"，Export
                 （按名注入 wrapper；探测其持有的裸 driver）
                 [starter.go:67-69]

gs.Run()
  ├─ 构造 newClient [starter.go:107]：记录实例创建日志
  │   ├─ 若设置 service-name 且 mesh 关闭：resolveURI → 选一个端点，
  │   │  地址拼进 URI host [starter.go:103-110, driver.go:185-213]；同一个 resolver
  │   │  还喂给 driver 的 AddressResolver，种子主机挂掉后 neo4j:// client 可重新找回集群
  │   ├─ 可选 Driver bean——没有则回退内置 DefaultDriver（d == nil
  │   │  回退 [starter.go:113-115]）；多个并存时实例可按名指定：
  │   │  `spring.neo4j.instances.<name>.driver = <bean 名>`（留空 = 按类型注入唯一
  │   │  Driver bean；指定的 bean 不存在则启动失败）
  │   ├─ d.CreateClient(ctx, c, backend, params)（auth + 连接池参数 + TLS）返回 *Client
  │   │  已完整——身份（session + 服务标签）与治理均已应用
  │   │  [starter.go:123, driver.go:77]；params = cloud.ClientParams{Resilience: mgr,
  │   │  Fault: inj}，mgr/inj 为注入的 *resilience.Manager / *fault.Injector bean
  │   ├─ NewClient [client.go:96-103]：service = resilience.ServiceLabel("neo4j",
  │   │  ServiceName, URI) → params.ExecutorFor("neo4j", service)——bundle 有值时是受治理的
  │   │  executor（fault.WrapClientExecutor 包住 mgr.ClientExecutorFor），零值 bundle 时
  │   │  为 resilience.Unmanaged（被观测、仅告警一次）
  │   └─ fail-fast HealthCheck（直达裸 driver 的 VerifyConnectivity），受
  │      socket-connect-timeout 或 5s 约束；失败时销毁 client，启动中止
  │      [starter.go:130-136]
  ├─ readiness：指示器每次探测跑 HealthCheck [health.go:32-34]
  └─ SIGTERM → Destroy [client.go:115-120]：exec.Close →
      driver.Close(context.Background())
```

没有 `Init` 钩子：`newClient` 返回时 wrapper 已完整——治理在构造期（`NewClient`）内应用，
不是事后补打。因此探活失败会销毁一个已受治理的 client（`Destroy`），不会泄漏执行器。
```

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:66-68`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterNeo4j.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.neo4j` 下每个实例都经它构建。
`CreateClient(ctx, c, backend, params)` 还会收到该实例 `${discovery}` label 解析出的发现后端
（无后端 bean 时为 nil）。没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`
（`driver.go:71`，`starter.go:113-115`）。没有 per-config 的 `driver` key。

服务器不可达、TLS 材料坏——启动即失败，进程不会带着一个死 Neo4j 进入
"服务中"状态。

销毁方法叫 `Destroy` 而非 `Close`：`Close(context.Context)` 是 `neo4j.DriverWithContext` 的一部分，
已被按原样提升，因此 wrapper 仍满足该接口（可传给 neo4j.ExecuteQuery / Query）；`Destroy` 是
另一条 gs 生命周期方法（client.go）。

### 2.2 实际存在的插桩——与不存在的部分

对家族不对称要诚实：**没有透明的按请求插桩**。neo4j-go-driver 走二进制 Bolt 协议、
无官方 OpenTelemetry 插桩，且 `ExecuteQuery` 是包级泛型函数——不是 driver 的方法——
因此没有 transport/dialer/hook 可拦截（starter.go:79-84 与 command.go:31-44 注释明确
称这是 documented gap，非疏漏）。实际存在的：

| 辅助函数 | 增量 | 级别 |
|--------|--------------|-------|
| `StarterNeo4j.Query[T]` | `neo4j.ExecuteQuery` 的替换（同签名）：声明操作的语义身份，由 resilience 层发射 span + 时长指标 + 访问日志，外加调用点韧性保护 | 可选，逐调用点 |
| `StarterNeo4j.RunWithResilience` | 把任意 session/事务代码套进韧性保护；配合 `StartSpan` 后该调用也带上声明的身份 | 可选 |
| `StarterNeo4j.StartSpan` | 在 ctx 上声明手工操作的语义身份（Cypher 作为 `db.statement`）；它自身不启动任何东西——把操作放进 `RunWithResilience` 运行，信号由 resilience 层发射 | 可选 |
| 健康指示器 `neo4j:<name>` | 每次 actuator 探测跑 `HealthCheck`（直达裸 driver 的 `VerifyConnectivity`） | 自动，恒注册 |
| 由 manager 的执行器（`resilience.WrapClientExecutor`）应用的 observe 层 | 唯一发射点：读 ctx 上声明的操作，发射 span + `db.client.*` 指标 + 访问日志；未声明操作时为 outcome 指标（`resilience.*`） | 用了辅助函数后自动 |

`Query` 的 span/指标/日志**不**由 starter 发射。starter 只声明操作（[observe.go]），
信号由 resilience 层发射——`NewClient` 从治理 bundle 构建 client 的 executor
（`params.ExecutorFor`，内部用 `resilience.WrapClientExecutor`），因此发射器是链条上唯一看得见整次调用（含重试）
的点。声明没有任何配置开关；信号在 starter-otel 安装的 OTel globals 上为空操作，
访问日志则恒按 log 包原生级别输出。

`Query` 使用时的实际产出：

- span：名称 = `op`（`Query` 为 `"query"`），属性 `db.system=neo4j`、
  `db.operation=<op>`、`db.statement=<Cypher，截断至 512 字节>`；span 由 resilience
  发射器开启（kind 为 internal），因而覆盖每一次 attempt
- 指标：call 级 `db.client.operation.duration` 直方图、attempt 级
  `db.client.attempt.duration` 直方图，以及 `db.client.active_requests`（在飞 gauge），
  均带 db.system/operation 标签
- 访问日志：日志 tag `_app_neo4j_access` 下每次调用一条（`log.RegisterAppTag`），
  按原生级别——失败 → Warn；带捕获 Cypher 参数的成功 → Debug；普通成功 → Info

### 2.3 一次查询的真实走读：`Query(... "MATCH ...")`

1. `Query` 在 ctx 上声明操作的语义身份
   （`observability.WithOperation(ctx, operation("query", cypher))`，[command.go:65]）
   ——此处不碰 span、gauge 或日志。
2. `queryResilience(driver)` 把 driver 断言回 `*Client` [command.go:105-117]。是 wrapper 时
   executor 在构造期已绑定，调用走
   `resilience.Run(ctx, exec, fn)`——限流/熔断/重试/bulkhead/超时，作用于服务标签
   `neo4j:<service-name|uri>` [client.go:100]；且因 executor 已被
   `resilience.WrapClientExecutor` 包裹，发射器会从 ctx 读出声明的操作。传入**裸**
   `neo4j.DriverWithContext` 时交给 `resilience.Unmanaged("neo4j", "neo4j")`
   [command.go:116]：被观测、且仅告警一次，而不是静默无保护。
3. 发射器开启 call span，随后 `neo4j.ExecuteQuery[T]` 执行 Cypher（驱动自身对瞬时错误
   重试至 `max-transaction-retry-time`——驱动语义，见
   [驱动手册](https://neo4j.com/docs/go-manual/current/)）。
4. 发射器记录 call 级与 attempt 级时长直方图、平衡 gauge、结束 span、按结果发访问日志记录。

直接调 `neo4j.ExecuteQuery`、或未经 `RunWithResilience`/`StartSpan` 驱动
`NewSession`/`session.Run` 的代码，绕过声明与发射——无观测也无保护。这是缺失
接缝的既定代价（§6）。

### 2.4 服务发现寻址 —— 种子 + 路由模式下的自愈

设置 `service-name` 且 mesh 关闭时，`resolveURI` 在 `discovery` 后端上建 Resolver、
选一个端点、把地址拼进 URI host [driver.go:185-213]。neo4j driver 无 dialer 注入点，
所以这个地址是**启动种子**——运行期不会按查询重挑。真正继续跟随命名服务的是 driver 自己的
`AddressResolver` 钩子，由同一个 resolver 驱动 [driver.go:92-96]：路由驱动（`neo4j://` scheme）
在无法从拨号地址建立路由表时会咨询它——也就是种子主机消失的那一刻。所以路由模式的 client 能
**不重启**恢复到当前集群；而 **`bolt://` 直连没有路由表，会一直停在启动地址上**。注册中心抖动时
该钩子回吐 driver 手中已有的地址，绝不把一次读失败变成空路由表。mesh 模式（`GS_MESH_MODE=on`）下
sidecar 负责发现+LB，URI 原样使用 [starter.go:103]。

⚠ **刻意不接受管的端点选择。** 没有"每次查询挑选"可管：启动那次挑选被固化进 URI 字符串，池在同一
个函数里建了就用、用完就丢。因此 `spring.governance.client.rules[N].balancer` / `outlier-threshold` 对 neo4j 无效；
它的治理止于保护策略（label 为 `neo4j:<service-name|uri>` 的 timeout / retries / breaker）。写在这里
是为了让这个缺口读起来是**决策**而不是遗漏。（上面的 `AddressResolver` 管的是**哪个集群**，
不是**哪个节点**——节点仍然由 driver 自己选。）

---

## 3. 逐 key 行为参考

全部 key 位于 `spring.neo4j.instances.<name>.`——经 `conf.BindEach` 按实例绑定（ctor
IndexArg(1)），不是 starter Pool 的绝对属性规则。

### 3.1 寻址与发现

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `uri` | string | — | **必填**（`expr:"$ != ''"`）。scheme 决定路由+加密：`bolt`/`neo4j` 明文，`neo4j+s`/`bolt+s` TLS，`+ssc` 自签。⚠ 设置 `service-name` 时 host 被发现结果替换（example 故意用哑地址 `bolt://0.0.0.0:0`）。 | 缺失 → BindEach 报错并点名实例；scheme 非法 → 构造期 driver 报错。 |
| `service-name` | string | — | 经发现后端解析**启动地址**，并为 `neo4j://` client 保留一份活的路由集用于自愈（§2.4）。⚠ 需有匹配的命名后端 bean。 | 后端未注册 → 启动报错 "neo4j: resolve service …"。 |
| `scheme` | string | — | 把发现收窄到单一传输 scheme 的端点；仅 `service-name` 生效时被读取。 | — |
| `discovery` | string | — | 用哪个已注册后端解析 `service-name`。未配置时回退 `${spring.neo4j.default.discovery}`。 | service-name 已设但两层都未配置或名字无对应 bean → 启动报错。 |

### 3.2 认证与连接池

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `username` | string | — | 与 `password`/`realm` 组成 BasicAuth。⚠ `username` 为空 ⇒ NoAuth 匿名连接。 | 在启用认证的服务器上留空 → 启动期 fail-fast 报错。 |
| `password` | string | — | 见上。 | 错误 → fail-fast 报错。 |
| `realm` | string | — | 传给 BasicAuth 的 realm。 | — |
| `max-connection-pool-size` | int | 100 | 每 host 最大连接数（驱动语义）。 | 过小 → 突发下报 `connection-acquisition-timeout` 错。 |
| `max-connection-lifetime` | duration | 1h | 连接退役重连窗口。 | — |
| `connection-acquisition-timeout` | duration | 1m | 从池里取连接的最长等待。 | 过小 → 突发下查询假性失败。 |
| `socket-connect-timeout` | duration | 5s | TCP 建连超时；⚠ 同时约束启动 fail-fast 探测（starter.go:130-136）。 | 0/负值时探测静默回退 5s。 |
| `max-transaction-retry-time` | duration | 30s | 驱动级瞬时错误重试预算。⚠ 与 `spring.governance.*.max-retries` 叠加——两层重试相乘。 | 大值 + 治理重试 → 延迟放大。 |

### 3.3 TLS

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `tls.ca-file` | string | — | CA bundle 装入 `RootCAs`。⚠ 仅对 `+s`/`+ssc` scheme 生效——加密由 scheme 决定；`tls.enabled` 只是形状对齐的占位符，不会开启加密（driver.go:143-145 注释）。 | 配了明文 `bolt://` → 静默忽略。 |
| `tls.cert-file` / `tls.key-file` | string | — | 双向 TLS 客户端证书（static provider）。⚠ 两者成对。 | 不可读/非法 → 启动报错 "neo4j: load client certificate"。 |
| `tls.server-name` | string | — | 覆盖对端名校验。 | — |
| `tls.insecure-skip-verify` | bool | false | 跳过校验。 | 仅为便利；风险自明。 |

### 3.4 插桩

本模块**没有插桩配置 key**（没有 level、没有跳过名单、没有参数上限）：辅助函数的
声明无条件开启（[observe.go]），信号由 resilience 层发射；trace/指标搭乘 starter-otel
安装的 OTel globals（`spring.observability.*`）。

核对：starter 内 14 个 `value:"..."` tag（Config 字段）
与上表一一对应——`grep -rhoE 'value:"[^"]+"'` 双向无多余。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .      # components 含 "neo4j:graph"、"neo4j:analytics"
docker stop starter-neo4j        # 指示器跑 HealthCheck → 组件翻 DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start starter-neo4j       # 下次探测翻回 UP——无需重启
```


### 4.2 可观测性 —— 声明的操作产出什么

```bash
grep _app_neo4j_access app.log | tail -1
# db.system=neo4j db.operation=query status success duration_ms=...（仅
# StarterNeo4j.Query / StartSpan+RunWithResilience；带 Cypher 的成功为 Debug，
# 普通成功为 Info，失败为 Warn）——由 resilience 层发射
curl -s :9090/metrics | grep -E 'db.client.(operation|attempt).duration|db.client.active_requests'
# call 级 + attempt 级时长直方图 + 在飞 gauge，db.system=neo4j
# Jaeger（example-otel compose）：span "query" 带 db.statement=<Cypher>
```

反向演练：直接调 `neo4j.ExecuteQuery`——无 span、无指标、无访问日志。这份静默是
未被拦截的路径，不是管线坏了（§2.2）。

### 4.3 韧性演练（example-cloudnative / example-load 形态）

```properties
# NOTE: governance RULES go in conf/governance.properties, referenced by spring.governance.source.file.path in app.properties (see starter-governance-file USAGE).
spring.governance.enabled=true
spring.governance.client.default.rate-limit=5
```

```bash
go run ./example-cloudnative -manual   # 自校验：15 连发 → 部分放行、
                                       # 部分以 resilience.ErrRateLimited 拒绝
```

故障注入（热切换，example-load）：压测进程运行中把 `conf/app.properties` 的
`spring.governance.client.fault.enabled=true`、`spring.governance.client.fault.rate=0.5`、`spring.governance.client.fault.error=timeout`
打开——错误分布即时变化，无需重启。

### 4.4 发现演练

注册后端 bean（见 example/discovery.go），设
`spring.neo4j.instances.graph.service-name=neo4j-cluster` 与哑 `uri=bolt://0.0.0.0:0`。启动日志
`neo4j client initialized, uri=bolt://127.0.0.1:7687`——拼接后的地址，不是哑值。
杀掉该实例：查询持续失败——地址只在启动时解析过一次（§2.4）；重启应用（或交给平台）
才会重新解析。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|---------|--------------|-----|
| 启动报 "failed to verify neo4j connectivity" | 服务器不可达 / 凭证错误 / TLS 不匹配 | fail-fast 探测无条件执行 [starter.go:130-136]；修连通性或认证。 |
| 启动报 "neo4j: resolve service X" | 设了 `service-name` 但 `discovery` 名下无后端 | 注册后端（example/discovery.go）或去掉 service-name。 |
| 查询正常但无 span/指标/访问日志 | 代码直调 `neo4j.ExecuteQuery`，绕过接缝 | 换 `StarterNeo4j.Query` / 用 `StartSpan` 声明后放进 `RunWithResilience` 运行（§2.2）；真实导出需 import starter-otel。 |
| 治理已开却没有保护 | session 代码未走 `Query`/`RunWithResilience`，或传了裸 driver（断言落空） | 走辅助函数；恒传 `*Client` wrapper [command.go:105]。 |
| TLS 配置似乎不起作用 | URI scheme 是明文 `bolt://`/`neo4j://` | 把 scheme 换成 `neo4j+s://`/`bolt+s://`；tls.* 只定制加密 scheme 的信任 [driver.go:143-145]。 |
| 发现端点已变，client 仍拨旧地址 | 一次性解析——driver 无 dialer 钩子 | 重建/重启 client（§2.4）；或前置 mesh/sidecar LB。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 13 实例 key + tls 组（4） |
| 其中必填 | 1（`uri`） |
| quickstart 前置外部依赖 | 1（Neo4j） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单（审计台账——保留上一轮条目，另加新条目）：

- 保护只经可选的 `Query`/`RunWithResilience`——直接用 session 静默失去治理与观测。
  2026-08-28 守卫统一化复核再次确认这是 **SDK 阻塞而非 starter 偷懒**：
  `neo4j.SessionWithContext` 含未导出方法，driver 包之外没有 wrapper 能实现它
  （封死了"重写 NewSession 返回带守卫 session"的路）；`neo4j.ExecuteQuery` 是包级
  泛型函数（Go 禁止带类型参数的方法，也无法在 `*Client` 上覆写）。辅助函数就是可达的
  最深接缝；一旦使用，治理自动生效（无 resilience 开关）。测试见 resilience_test.go。
- `Query` 通过把 driver 参数断言回 `*Client` 找 executor——类型不同的自定义 driver
  没有配置可推导标签，因而只跑在"仅观测、且大声告警"的 unmanaged executor 上，而非受治理
  （command.go:105-117）。
- `tls.enabled` 在这里是死占位 key（scheme 才管加密），候选收敛 tls 形状
  （driver.go:143-145）；一次性发现解析（vs 其它 client starter 的活性重解析）是被
  缺失的 dialer 钩子所迫（§2.4）。
- fail-fast 探测复用 `socket-connect-timeout` 作为上限——把"用户的 TCP 预算"和
  "启动探测预算"混在一个 key 里（starter.go:130-136）。
