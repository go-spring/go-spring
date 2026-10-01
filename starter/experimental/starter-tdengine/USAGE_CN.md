# starter-tdengine 使用说明（参考手册）

详细使用文档。概述见 [README_CN.md](README_CN.md)。所有行为声明均与源码核对
（`starter.go`、`config.go`、`client.go`、`driver.go`、`health/health.go`、`tdengine_test.go`）
并锚定可运行的 [example/](example/)——文内以方括号给出 file:line 抽查点。**TDengine SQL 语义、
DSN 格式与 driver-go WebSocket 连接器属于 [TDengine 官方文档](https://docs.taosdata.com/reference/connector/go/)**
（WebSocket 端点由 [taosAdapter](https://docs.taosdata.com/reference/taosadapter/) 提供）——
本文只写 go-spring 的增量。

**激活条件**：出现任意 `spring.tdengine.instances.*` key（模块为 `OnProperty("spring.tdengine")` 前缀匹配）。
每个 `spring.tdengine.instances.<name>` 条目创建一个名为 `<name>` 的 `*StarterTdengine.Client` bean，
并注册名为 `tdengine:<name>` 的健康指示器。

---

## 1. 完整工程示例

一个服务配两个 TDengine client 指向同一实例，外加探针、指标与 trace。文件树：

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
    github.com/taosdata/driver-go/v3 latest      // 由 starter 间接引入
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-tdengine latest
    go-spring.org/starter-actuator latest        // 可选：readiness + /metrics
    go-spring.org/starter-otel     latest        // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance-file latest      // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-tdengine"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入 wrapper，用超级表范式跑真实 SQL：

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterTdengine "go-spring.org/starter-tdengine"
)

type Service struct {
    // 永远注入 wrapper 类型 *StarterTdengine.Client。它内嵌裸 *sql.DB，
    // ExecContext/QueryContext/QueryRowContext/PingContext 等整个 database/sql
    // 方法集都按原样提升 [client.go]。
    Admin *StarterTdengine.Client `autowire:"a"` // 不带默认库
    Power *StarterTdengine.Client `autowire:"b"` // DSN 固定 /power
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            for _, q := range []string{
                "CREATE DATABASE IF NOT EXISTS power",
                "CREATE STABLE IF NOT EXISTS power.meters (ts TIMESTAMP, current FLOAT) TAGS (location BINARY(24))",
                "INSERT INTO power.d001 USING power.meters TAGS('beijing') VALUES (NOW, 10.5)",
            } {
                if _, err := s.Admin.ExecContext(ctx, q); err != nil {
                    panic(q + ": " + err.Error())
                }
            }
            var n int
            // 走 "b" client，其 DSN 已选定 power 库。
            if err := s.Power.QueryRowContext(ctx, "SELECT COUNT(*) FROM meters").Scan(&n); err != nil {
                panic(err)
            }
        }
    })
}
```

**conf/app.properties** —— 上述用到的完整配置面（DSN 取自
[example/conf/app.properties](example/conf/app.properties)）：

```properties
# --- 两个 client，同一实例 -----------------------------------------------------
# DSN 为驱动统一格式；ws() = 经 taosAdapter:6041 的 WebSocket。
spring.tdengine.instances.a.dsn=root:taosdata@ws(127.0.0.1:6041)/
spring.tdengine.instances.a.max-open-conns=4
spring.tdengine.instances.a.max-idle-conns=2

spring.tdengine.instances.b.dsn=root:taosdata@ws(127.0.0.1:6041)/power

# --- 可观测恒开启：resilience 层在 starter-otel 的全局 provider 上发射 span ----
# 与 db.client.* 指标，访问日志走 log 包原生分级。starter 只声明每条语句的身份。

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**验证**（先起 TDengine——[example 的 docker-compose.yml](example/docker-compose.yml)
自带 3.3.6.0，taosAdapter 在 6041）：

```bash
docker compose -p gs-tdengine-demo up -d          # 等约 30s；taosAdapter 启动偏慢
go run .                        # DSN 不可达则启动 fail fast（§2.1）
curl -s :9370/readyz | jq .     # components 含 tdengine:a 与 tdengine:b
curl -s :9370/metrics | grep -E 'db_client_(operation|attempt)_duration'   # 逐语句直方图
grep _app_tdengine_access app.log | tail -3        # 每条语句一条记录
docker exec -it <container> taos -s "SELECT COUNT(*) FROM power.meters"   # 1
```

[example/](example/) 本身（单 client `a`，`check.sh` 自断言往返）是冒烟锚点；
上面的 otel/actuator 块是 client 家族的标准组合配置。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-tdengine
  └─ gs.Module(OnProperty("spring.tdengine"))：出现任意 spring.tdengine.instances.* key 触发
        └─ conf.BindEach("${spring.tdengine}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient).Name(<name>)
              │       .Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator，命名 "tdengine:<name>"，导出为
                  health.Indicator（恒注册；无关闭 key）

gs.Run()
  ├─ 构造 newClient [starter.go]：可选 Driver bean——没有则回退内置
  │     DefaultDriver（d == nil）；容器中存在多个 Driver bean
  │     时，实例可按名指定：`spring.tdengine.instances.<name>.driver = <bean 名>`
  │     （留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的 bean 不存在则启动失败）
  │     → d.CreateClient(ctx, c, params)：构造器把注入的 bean 收进
  │       params = cloud.ClientParams{Resilience: mgr, Fault: inj}
  │       （*resilience.Manager / *fault.Injector）并传入 →
  │       ParseDSN → taosws.NewConnector → NewClient
  │     → NewClient [client.go]：建 slot、包 guardedConnector、
  │       开 *sql.DB 池、应用连接池参数——身份 + 守卫一次装好，无 Init 钩子
  │     → NewClient 算出 serviceLabel（"tdengine:<dsn addr>"）与
  │       exec = params.ExecutorFor("tdengine", serviceLabel)，并装到 slot 上——
  │       client 返回时即已完整。零值包（无 *resilience.Manager）退化为
  │       resilience.Unmanaged：仅观测，并一次性告警提示不受任何保护
  │     → fail-fast 探测：HealthCheck(ctx, cl)——一次 PingContext，10s 上限，在装配
  │       完成之后跑；失败则 Destroy 半成品 client，启动失败
  ├─ readiness：指示器对每实例跑 HealthCheck（即 db.PingContext）
  └─ SIGTERM → Destroy：exec.Close → db.Close
```

DSN 写错、凭据不对或 server 低于驱动下限都会导致启动失败——进程不会带着死的
TDengine 进入 "serving"（版本下限：driver-go v3.8.2 在 WebSocket 路径要求
server ≥ 3.3.6.0 [DESIGN.md §3]）。

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:45-46`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterTdengine.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.tdengine` 下每个实例都经它构建。
没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`（`driver.go:50`，`starter.go:65-67`）。
没有 per-config 的 `driver` key。

### 2.2 语句 seam —— 精确顺序与理由

`database/sql` 没有拦截器链，因此 guard 落在 `driver.Conn` 层：DefaultDriver 把
taosWS connector 包进 `guardedConnector`，池内每条连接都是 `guardedConn`
[driver.go:96-124]。这是 gorm callback 链与 HTTP RoundTripper 适配器在 database/sql
里的对应物——意味着治理是**逐语句且透明的**：调用点无需 opt-in，架在池上的 ORM
同样被覆盖 [driver.go:115-124, DESIGN.md §4]。

一条语句，例如 `QueryContext("SELECT COUNT(*) ...")`：

```
*sql.DB 池
  └─ guardedConn.QueryContext [driver.go]
        ├─ 声明：observability.WithOperation(ctx, operation("query", sql))——语句身份
        │    在 executor 运行**之前**放到 ctx 上，因为 executor 在 Execute 入口读它：
        │    在它内部（每次尝试）声明则无人读取 [observe.go]。
        ├─ run：resilience.Run(ctx, exec, call)
        │    （外层：限流/熔断/故障注入在语句执行**之前**裁决；被拒绝的语句
        │     根本到不了连接——有单测 [tdengine_test.go:62-76]。
        │     无容器时 executor 为 resilience.Unmanaged：调用被观测但不受保护，
        │     声明仍能到达发射器。）
        │    └─ resilience executor **发射**（starter 自身不发射任何东西）：一个 call
        │         span（"query"）、call 级 db.client.operation.duration、每次下游尝试一条
        │         db.client.attempt.duration、resilience.client.calls 计数，以及一条访问
        │         日志——重试一并包含。
        └─ taosWS conn → websocket → taosAdapter
```

与 starter-go-redis 对比：那边的声明挂在熔断器上方的拦截器里；这里声明在调用缝上、
executor 在最外层，因此发射器看得见整次调用。driver-go 自身不带埋点，所以本 starter
只声明语句身份（见 observe.go），由 resilience 层作为三信号（span + 指标 + 日志）的
唯一发射点。

**不在 guard 覆盖内**的路径 [driver.go:189-198]：

- `Prepare` 透传给原始连接——经 prepared `*sql.Stmt` 执行的语句绕过
  resilience/observability。请用 `ExecContext`/`QueryContext`（`database/sql`
  本来也更偏好它们，源码注释同此）。
- `Begin` 同样透传：TDengine 无事务，底层 driver 会报错。
- 健康探测（`PingContext`）不经过声明/executor。

槽位执行器为 nil 时语句原样透传——零配置直通，在槽位层有单测覆盖
[tdengine_test.go:51-58]。经构造器出来的槽位一定有执行器（由 [NewClient] 装入），
所以实际中语句总被观测：有容器时受治理，无容器时走 `Unmanaged`（仅观测，一次性告警）。

### 2.3 service label

`serviceLabel` 从 DSN 提取展示安全地址
（"root:taosdata@ws(127.0.0.1:6041)/power" → "127.0.0.1:6041"）并拼出
`tdengine:<addr>` [client.go:91-94, starter.go:89-96]。限流/熔断状态按 TDengine
实例划分，而非按语句或按库：指向同一 host:port 的两个 client（上面的 `a` 与 `b`）
即使 DSN 不同也共享一个桶。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.tdengine.instances.<name>.` 下——经 `conf.BindEach` 按实例绑定
（不是 starter-Pool 的绝对属性规则）。完整清单，已与
`grep -rhoE 'value:"[^"]+"'` 双向核对：

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `dsn` | string | — | **必填**（expr `$ != ''` [config.go:33]）。驱动统一 DSN `[user[:password]@]ws(host:port)/[dbname][?params]`。也是 resilience service label（§2.3）与启动错误信息的来源。TLS 在这里表达（`wss(...)`、证书参数）——没有 `tls.*` 块。 | 空 → BindEach 启动报错。地址/凭据错 → fail-fast ping 报 "failed to reach tdengine at <addr>"。 |
| `max-open-conns` | int | 8 | 内嵌池的 `db.SetMaxOpenConns` [driver.go:81]。 | 过小 → 语句排队等连接。 |
| `max-idle-conns` | int | 2 | `db.SetMaxIdleConns`。⚠ 应 ≤ max-open-conns（database/sql 会静默封顶，但超出即是配置坏味道）。 | 大于 open conns → 被钳制，idle 抖动。 |
| `conn-max-lifetime` | duration | 0s | `db.SetConnMaxLifetime`；0 = 永不退役。⚠ 与 redis（默认 2m）不同，这里没有 discovery 需要跟随，0 是安全的。 | — |

没有 `driver` key：client 装配由可选 `Driver` bean（见 §2.1）或内置 `DefaultDriver` 负责。
没有观测类 key——插桩恒开启（§4.2）。

---

## 4. 验证与故障演练

### 4.1 经 actuator 的健康检查

```bash
curl -s :9370/readyz | jq .      # components：tdengine:a、tdengine:b（探针 = PingContext）
docker stop <tdengine>           # 指示器 ping 失败 → component 翻 DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <tdengine>
```

探针会真实拉起一条 websocket 连接并走一遍 taosAdapter 的 action 链
[health/health.go:33-37]。

### 4.2 starter 声明什么，resilience 层发射什么

starter 逐调用不发射任何东西：它只在 ctx 上声明每条语句的身份（`observe.go`）——
语句种类（`exec`/`query`）、`db.system`/`db.operation` 标签，以及作为 `db.statement`
detail 的 SQL（截断至 512 字节）。resilience 层是 executor 链条上唯一看得见整次调用
的点，它据此发射全部信号。

| 信号 | 名称 / 形态 | 属性 |
|--------|--------------|------------|
| Span | `exec` / `query`（操作名，由 resilience 层开启） | `db.system=tdengine`、`db.operation=exec\|query`、`db.statement=<sql，截断至 512 字节>` |
| 指标 | `db.client.operation.duration`（直方图，s）——每次调用，含重试 | `db.system`、`db.operation`、`status` |
| 指标 | `db.client.attempt.duration`（直方图，s）——每次下游尝试 | `db.system`、`db.operation`、`status` |
| 指标 | `db.client.active_requests`（up-down counter） | `db.system`、`db.operation` |
| 指标 | `resilience.client.calls`（counter） | `system=tdengine`、`service`、`status=ok\|error`（拒绝时另有 `resilience.outcome`） |
| 指标 | `resilience.client.breaker.state_change`（counter） | from/to 属性 |
| 日志 | tag `_app_tdengine_access` | db.system=tdengine db.operation=… status duration_ms；错误 → Warn，成功且带 SQL → Debug，普通成功 → Info |
| 日志 | tag `_app_resilience` | 熔断状态跃迁（已声明的访问日志——含失败——走上面的语句 tag） |

未引入 starter-otel 时，span/指标是 no-op（全局 provider 为空）——只有访问日志产出，
且不带 trace_id。

```bash
curl -s :9370/metrics | grep -E 'db_client_(operation|attempt)_duration|db_client_active'
grep _app_tdengine_access app.log | tail -1
# db.system=tdengine db.operation=exec status=ok duration_ms=... "INSERT INTO power.d001 ..."
```

### 4.3 resilience 演练

引入 starter-governance-file 后，为 service `tdengine:127.0.0.1:6041`（§2.3）配策略——
例如限流。压测 `ExecContext`；超限语句以 `resilience.ErrRateLimited` 拒绝，
**不会到达连接**（有单测 [tdengine_test.go:62-76]），体现在
`resilience.client.calls{resilience.outcome="rate_limited"}` 与 `_app_tdengine_access` tag 下的一条
Warn 日志。运行期切换策略——executor 经 governance 中心热加载，无需重启。

### 4.4 fail-fast 演练

```bash
docker stop <tdengine> && go run .    # 退出并报 "failed to reach tdengine at 127.0.0.1:6041"
```

启动 ping 无条件执行、10s 封顶 [starter.go:98-108]——启动成功即证明凭据、DSN 与
server 版本全部正常。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "failed to reach tdengine at ..." | 地址不可达 / 凭据错 / taosAdapter 未就绪 | 修 DSN；等 6041 端口（镜像要起多个服务——check.sh 等 90s + 5s）。 |
| 启动失败，driver 版本报错 | WebSocket 路径 server < 3.3.6.0（driver-go v3.8.2 下限） | 升级 server 镜像。 |
| BindEach 在 `dsn` 上启动失败 | `spring.tdengine.instances.<name>.dsn` 缺失或为空 | expr tag 强制非空——补上。 |
| SQL 正常但健康检查 DOWN | 池被占满（max-open-conns 过低），探针拉不到新连接 | 调大 max-open-conns；看 /readiness 里 component 的错误详情。 |
| 无 span/指标 | 未引入 starter-otel | resilience 层在 OTel 全局 provider 上发射；import starter-otel。 |
| 无访问日志 | logger 级别过滤掉 Debug/Info，或日志 tag 被过滤 | 检查 logger 级别及对 `_app_tdengine_access` 的配置。 |
| 经 db.Prepare 的语句无治理无观测 | `Prepare` 设计上绕过 slot [driver.go] | 改用 ExecContext/QueryContext。 |
| 熔断状态跨"库"共享 | service label 按 host:port 划分，DSN 参数不参与 | 有意为之（按实例划分）；要分桶就分开 host。 |
| `Begin` 报错 | TDengine 无事务 | 设计如此——driver 会报错。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 4 个实例 key |
| 其中必填 | 1（`dsn`） |
| quickstart 前置外部依赖 | 1（TDengine 及其自带 taosAdapter） |
| "注意/坑" 条数 | 4 |

设计嫌疑（保留上一轮审计条目，另加新增）：
- DSN 是不透明字符串——resilience service label 靠从中解析地址，仅参数或库名不同的
  两个 DSN 共享同一个桶；无 `tls.*`/`service-name`，与兄弟 starter 不一致（家族不对称）。
- `Prepare` 完全绕出 guard seam——走 prepared statement 的 ORM 会静默失去
  resilience + observability 覆盖。
- 健康指示器无关闭 key（与 starter-go-redis 相同的家族不对称；redigo 有 `health.enabled`）。
