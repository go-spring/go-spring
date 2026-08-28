# starter-neo4j 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health/health.go`）与
可运行示例（[example/](example/)、[example-otel/](example-otel/)、
[example-cloudnative/](example-cloudnative/)、[example-load/](example-load/)）核对——文中方括号
为 file:line 抽查点。**Cypher 语义与 neo4j-go-driver API 属于
[驱动官方文档](https://neo4j.com/docs/go-manual/current/)**——以下只写 go-spring 的增量。

**激活条件**：出现任意 `spring.neo4j.*` key 即注册（模块为 `OnProperty("spring.neo4j")`
前缀检查）。每个 `spring.neo4j.<name>` 条目创建一个名为 `<name>` 的
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
    go-spring.org/starter-governance   latest   // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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
    // 恒为 wrapper 类型 *StarterNeo4j.Client。它内嵌
    // neo4j.DriverWithContext，NewSession / VerifyConnectivity / target
    // 等方法原样提升。按实例名注入（"graph"、"analytics"）。
    Graph     *StarterNeo4j.Client `autowire:"graph"`
    Analytics *StarterNeo4j.Client `autowire:"analytics"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 经插桩 Query 写 + 读（span + 指标 + 访问日志 + 韧性保护）。
            // 与 neo4j.ExecuteQuery 同签名。
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
spring.neo4j.graph.uri=bolt://127.0.0.1:7687
spring.neo4j.graph.username=neo4j
spring.neo4j.graph.password=password

# --- 实例 "analytics"：调优连接池，独立健康指示器 ----------------------------
spring.neo4j.analytics.uri=bolt://127.0.0.1:7687
spring.neo4j.analytics.username=neo4j
spring.neo4j.analytics.password=password
spring.neo4j.analytics.max-connection-pool-size=50
spring.neo4j.analytics.connection-acquisition-timeout=30s

# --- observability -----------------------------------------------------------
# Query 的访问日志默认 level=brief（包级 observer）；
# 实例级 spring.neo4j.<name>.observability.* 驱动的是 resilience
# executor 的观测（resilience.* 指标），不是查询访问日志。
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus

# --- actuator：readiness 汇聚 neo4j:graph 与 neo4j:analytics ------------------
spring.actuator.addr=:9370

# --- governance：Query / RunWithResilience 的保护 ----------------------------
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=100
govern.default.max-retries=1
govern.default.timeout=500ms
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
  └─ gs.Module(OnProperty("spring.neo4j"))：出现任意 spring.neo4j.* key 即触发
        └─ conf.BindEach("${spring.neo4j}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient).Name(<name>)
              │    .Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator，名 "neo4j:<name>"，Export
                 （按名注入 wrapper；把内嵌 driver 交给指示器）
                 [starter.go:44-53]

gs.Run()
  ├─ 构造 newClient [starter.go:77]：记录实例创建日志
  │   ├─ 若设置 service-name 且 mesh 关闭：resolveURI → 选一个端点，
  │   │  地址拼进 URI host [starter.go:81-89, driver.go:128-145]
  │   ├─ driverRegistry 查找（未命中报 "neo4j driver not found"）[starter.go:91-98]
  │   ├─ driver.CreateClient（auth + 连接池参数 + TLS）[driver.go:61-82]
  │   └─ fail-fast VerifyConnectivity，受 socket-connect-timeout 或 5s 约束；
  │      失败时关闭 client 与 resolver，启动中止
  │      [starter.go:109-119, 132-137]
  ├─ gs 字段注入 Client.Observability（${observability:=}）
  ├─ Init [client.go:74-79]：resource = resilience.ResourceLabel("neo4j",
  │   ServiceName, URI) → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │   → resilobserve.WrapExecutor(exec, "neo4j", Observability)——治理关闭时
  │   executor 为透明 no-op
  ├─ readiness：指示器每次探测跑 VerifyConnectivity
  └─ SIGTERM → Destroy [client.go:89-95]：exec.Close → stopLiveResolver →
      driver.Close(context.Background())
```

`driver` 配错、服务器不可达、TLS 材料坏——启动即失败，进程不会带着一个死 Neo4j 进入
"服务中"状态。

销毁方法刻意叫 `Destroy` 而不是 `Close`：内嵌的 `neo4j.DriverWithContext` 已暴露
`Close(context.Context)`，用不同签名遮蔽它会让 wrapper 不再满足该接口（也就无法传给
neo4j.ExecuteQuery / Query）（client.go:81-88 注释）。

### 2.2 实际存在的插桩——与不存在的部分

对家族不对称要诚实：**没有透明的按请求插桩**。neo4j-go-driver 走二进制 Bolt 协议、
无官方 OpenTelemetry 插桩，且 `ExecuteQuery` 是包级泛型函数——不是 driver 的方法——
因此没有 transport/dialer/hook 可拦截（starter.go:63-68 与 command.go:31-44 注释明确
称这是 documented gap，非疏漏）。实际存在的：

| 辅助函数 | 增量 | 级别 |
|--------|--------------|-------|
| `StarterNeo4j.Query[T]` | `neo4j.ExecuteQuery` 的替换（同签名）：span + 时长/在飞指标 + 访问日志，外加调用点韧性保护 | 可选，逐调用点 |
| `StarterNeo4j.RunWithResilience` | 仅把任意 session/事务代码套进韧性保护（无 span/指标/日志） | 可选 |
| `StarterNeo4j.StartSpan` / `EndSpan` | 为手工 `driver.NewSession` 操作补 span + 指标 + 访问日志 | 可选 |
| 健康指示器 `neo4j:<name>` | 每次 actuator 探测跑 `VerifyConnectivity` | 自动，恒注册 |
| Init 里的 `resilobserve.WrapExecutor` | 受保护执行的 outcome 指标（`resilience.*`），由实例级 `observability.*` 块控制 | 治理开启时自动 |

`Query` 的 span/指标/日志挂在**包级**默认 observer 上（`observe.NewDB("neo4j",
Level: brief)`，command.go:46），随 starter-otel 安装的 OTel globals——实例级
`observability.*` 块不能改它的档位（command.go:42-44 注释：kit 无法绑定到自由函数
调用路径）。

`Query` 使用时的实际产出（observe kit，`cloud/observe/observer.go:58-63, 184-195`）：

- span：kind=client，名称 = `op`（`Query` 为 `"query"`），属性 `db.system=neo4j`、
  `db.operation=<op>`、detailed 模式下 `db.statement=<Cypher，截断约束>`
- 指标：`db.client.operation.duration`（直方图，秒）与
  `db.client.active_requests`（在飞 gauge），均带 db.system/operation 标签
- 访问日志：日志 tag `_app_neo4j_access` 下每次调用一条（`log.RegisterAppTag`），
  默认 `brief`（system/op/status/duration/error）

### 2.3 一次查询的真实走读：`Query(... "MATCH ...")`

1. `defaultObs.Start(ctx, "query", cypher)` 开 span、加在飞 gauge、开访问日志记录
   [command.go:66]。
2. `queryResilience(driver)` 把 driver 断言回 `*Client` [command.go:111-116]。是
   wrapper 时 executor 恒已解析（治理关闭为 no-op），调用走
   `exec.Execute(ctx, resource, fn)`——限流/熔断/重试/bulkhead/超时，作用于资源标签
   `neo4j:<service-name|uri>` [client.go:75]。传入**裸** `neo4j.DriverWithContext`
   则得 `(nil, "")`，静默无保护运行。
3. `neo4j.ExecuteQuery[T]` 执行 Cypher（驱动自身对瞬时错误重试至
   `max-transaction-retry-time`——驱动语义，见
   [驱动手册](https://neo4j.com/docs/go-manual/current/)）。
4. `sp.End(err)` 记录时长直方图、平衡 gauge、结束 span、按结果发访问日志记录。

直接调 `neo4j.ExecuteQuery`、或未经 `RunWithResilience`/`StartSpan` 驱动
`NewSession`/`session.Run` 的代码，绕过第 1-2 步的一切——无观测也无保护。这是缺失
接缝的既定代价（§6）。

### 2.4 服务发现寻址 —— 一次性

设置 `service-name` 且 mesh 关闭时，`resolveURI` 在 `discovery` 后端上建 Resolver、
选一个端点、把地址拼进 URI host [driver.go:128-145]。neo4j driver 无 dialer 注入点，
因此是**启动时一次性解析**——启动后的地址变化不会感知，除非重建 client
（config.go:79-83 注释）。Resolver 存活仅为与其它 client starter 的生命周期统一，
停机时 Stop。mesh 模式（`GS_MESH=on`）下 sidecar 负责发现+LB，URI 原样使用
[starter.go:70-76]。

---

## 3. 逐 key 行为参考

全部 key 位于 `spring.neo4j.<name>.`——经 `conf.BindEach` 按实例绑定（ctor
IndexArg(1)），不是 starter Pool 的绝对属性规则。

### 3.1 寻址与发现

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `uri` | string | — | **必填**（`expr:"$ != ''"`）。scheme 决定路由+加密：`bolt`/`neo4j` 明文，`neo4j+s`/`bolt+s` TLS，`+ssc` 自签。⚠ 设置 `service-name` 时 host 被发现结果替换（example 故意用哑地址 `bolt://0.0.0.0:0`）。 | 缺失 → BindEach 报错并点名实例；scheme 非法 → 构造期 driver 报错。 |
| `service-name` | string | — | 经发现后端解析地址，启动时一次（§2.4）。⚠ 需有经 `discovery.RegisterDiscovery` 注册的匹配后端。 | 后端未注册 → 启动报错 "neo4j: resolve service …"。 |
| `scheme` | string | — | 把发现收窄到单一传输 scheme 的端点；仅 `service-name` 生效时被读取。 | — |
| `discovery` | string | `default` | 用哪个已注册后端解析 `service-name`。 | 名字错 → 发现层启动报错。 |
| `driver` | string | `DefaultDriver` | 选择已注册的 `Driver`（注册表在 driver.go:37）。 | 未知名 → 启动报错 "neo4j driver not found"；`RegisterDriver` 重名 panic。 |

### 3.2 认证与连接池

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `username` | string | — | 与 `password`/`realm` 组成 BasicAuth。⚠ `username` 为空 ⇒ NoAuth 匿名连接。 | 在启用认证的服务器上留空 → 启动期 fail-fast 报错。 |
| `password` | string | — | 见上。 | 错误 → fail-fast 报错。 |
| `realm` | string | — | 传给 BasicAuth 的 realm。 | — |
| `max-connection-pool-size` | int | 100 | 每 host 最大连接数（驱动语义）。 | 过小 → 突发下报 `connection-acquisition-timeout` 错。 |
| `max-connection-lifetime` | duration | 1h | 连接退役重连窗口。 | — |
| `connection-acquisition-timeout` | duration | 1m | 从池里取连接的最长等待。 | 过小 → 突发下查询假性失败。 |
| `socket-connect-timeout` | duration | 5s | TCP 建连超时；⚠ 同时约束启动 fail-fast 探测（starter.go:110,132-137）。 | 0/负值时探测静默回退 5s。 |
| `max-transaction-retry-time` | duration | 30s | 驱动级瞬时错误重试预算。⚠ 与 `govern.*.max-retries` 叠加——两层重试相乘。 | 大值 + 治理重试 → 延迟放大。 |

### 3.3 TLS

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `tls.ca-file` | string | — | CA bundle 装入 `RootCAs`。⚠ 仅对 `+s`/`+ssc` scheme 生效——加密由 scheme 决定；`tls.enabled` 只是形状对齐的占位符，不会开启加密（driver.go:84-89 注释）。 | 配了明文 `bolt://` → 静默忽略。 |
| `tls.cert-file` / `tls.key-file` | string | — | 双向 TLS 客户端证书（static provider）。⚠ 两者成对。 | 不可读/非法 → 启动报错 "neo4j: load client certificate"。 |
| `tls.server-name` | string | — | 覆盖对端名校验。 | — |
| `tls.insecure-skip-verify` | bool | false | 跳过校验。 | 仅为便利；风险自明。 |

### 3.4 插桩

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|---------|-------------------------|------------------------------|
| `observability.level` | string | `brief` | 控制 **resilience-executor** 的访问日志（Init 里的 `resilobserve.WrapExecutor`），不是 Query 的访问日志（那是包级 `brief`，§2.2）。`off` 只静默日志信号。 | 指望它调 Query 日志 → 无效果，反直觉。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式下参数捕获上限。 | — |
| `observability.skipOps` | list | — | 对列出的 op 名同时抑制 span+指标+日志。 | — |

核对：starter 内 15 个 `value:"..."` tag（Config 14 字段 + wrapper 的 `observability`）
与上表一一对应——`grep -rhoE 'value:"[^"]+"'` 双向无多余。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .      # components 含 "neo4j:graph"、"neo4j:analytics"
docker stop starter-neo4j        # 指示器跑 VerifyConnectivity → 组件翻 DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start starter-neo4j       # 下次探测翻回 UP——无需重启
```


### 4.2 可观测性 —— 本 starter 实际产出什么

```bash
grep _app_neo4j_access app.log | tail -1
# brief 记录：system=neo4j op=query status ok duration=...（仅 StarterNeo4j.Query）
curl -s :9090/metrics | grep -E 'db.client.(operation.duration|active_requests)'
# 每次 Query 的时长直方图 + 在飞 gauge，db.system=neo4j
# Jaeger（example-otel compose）：span "query" 带 db.statement=<Cypher>
```

反向演练：直接调 `neo4j.ExecuteQuery`——无 span、无指标、无访问日志。这份静默是
未被拦截的路径，不是管线坏了（§2.2）。

### 4.3 韧性演练（example-cloudnative / example-load 形态）

```properties
govern.enabled=true
govern.default.enabled=true
govern.default.rate-limit=5
```

```bash
go run ./example-cloudnative -manual   # 自校验：15 连发 → 部分放行、
                                       # 部分以 resilience.ErrRateLimited 拒绝
```

故障注入（热切换，example-load）：压测进程运行中把 `conf/app.properties` 的
`govern.fault.enabled=true`、`govern.fault.rate=0.5`、`govern.fault.error=timeout`
打开——错误分布即时变化，无需重启。

### 4.4 发现演练

注册后端（`discovery.RegisterDiscovery`，见 example/discovery.go），设
`spring.neo4j.graph.service-name=neo4j-cluster` 与哑 `uri=bolt://0.0.0.0:0`。启动日志
`neo4j client initialized, uri=bolt://127.0.0.1:7687`——拼接后的地址，不是哑值。
杀掉该实例：查询持续失败——地址只在启动时解析过一次（§2.4）；重启应用（或交给平台）
才会重新解析。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|---------|--------------|-----|
| 启动报 "failed to verify neo4j connectivity" | 服务器不可达 / 凭证错误 / TLS 不匹配 | fail-fast 探测无条件执行 [starter.go:109-119]；修连通性或认证。 |
| 启动报 "neo4j driver not found: X" | `driver` 名未注册 | 在 init 里 `StarterNeo4j.RegisterDriver`，或用 `DefaultDriver`。 |
| 启动报 "neo4j: resolve service X" | 设了 `service-name` 但 `discovery` 名下无后端 | 注册后端（example/discovery.go）或去掉 service-name。 |
| 查询正常但无 span/指标/访问日志 | 代码直调 `neo4j.ExecuteQuery`，绕过接缝 | 换 `StarterNeo4j.Query` / 套 `StartSpan`（§2.2）；真实导出需 import starter-otel。 |
| 治理已开却没有保护 | session 代码未走 `Query`/`RunWithResilience`，或传了裸 driver（断言落空） | 走辅助函数；恒传 `*Client` wrapper [command.go:111-116]。 |
| `observability.level=off` 访问日志仍在发 | 它控制的是 resilience-executor 日志，不是 Query 的包级 `brief` observer | 已知不对称（§3.4）；用日志 tag 配置静默 `_app_neo4j_access`。 |
| TLS 配置似乎不起作用 | URI scheme 是明文 `bolt://`/`neo4j://` | 把 scheme 换成 `neo4j+s://`/`bolt+s://`；tls.* 只定制加密 scheme 的信任 [driver.go:84-89]。 |
| 发现端点已变，client 仍拨旧地址 | 一次性解析——driver 无 dialer 钩子 | 重建/重启 client（§2.4）；或前置 mesh/sidecar LB。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 14 实例 key + tls 组（4）+ observability（3） |
| 其中必填 | 1（`uri`） |
| quickstart 前置外部依赖 | 1（Neo4j） |
| "注意/坑" 条数 | 7 |

设计嫌疑清单（审计台账——保留上一轮条目，另加新条目）：

- 保护只经可选的 `Query`/`RunWithResilience`——直接用 session 静默失去治理与观测。
  2026-08-28 守卫统一化复核再次确认这是 **SDK 阻塞而非 starter 偷懒**：
  `neo4j.SessionWithContext` 含未导出方法，driver 包之外没有 wrapper 能实现它
  （封死了"重写 NewSession 返回带守卫 session"的路）；`neo4j.ExecuteQuery` 是包级
  泛型函数（Go 禁止带类型参数的方法，也无法在 `*Client` 上覆写）。辅助函数就是可达的
  最深接缝；一旦使用，治理自动生效（无 resilience 开关）。测试见 resilience_test.go。
- `Query` 通过把 driver 参数断言回 `*Client` 找 executor——类型不同的自定义 driver
  静默丢失保护（command.go:111-116）。
- 访问日志用包级默认 observer 而非实例级 `observability` 块，同名配置 key 控制两件
  不同的事（command.go:42-46 与 client.go:77）——配置语义陷阱。
- `tls.enabled` 在这里是死占位 key（scheme 才管加密），候选收敛 tls 形状
  （driver.go:84-89）；一次性发现解析（vs 其它 client starter 的活性重解析）是被
  缺失的 dialer 钩子所迫（§2.4）。
- fail-fast 探测复用 `socket-connect-timeout` 作为上限——把"用户的 TCP 预算"和
  "启动探测预算"混在一个 key 里（starter.go:110,132-137）。
