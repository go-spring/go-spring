# starter-cassandra 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`driver.go`、`health.go`）与可运行的
[example/](example/)（check.sh 自断言冒烟）核对，方括号内为 file:line 抽查点。
**CQL 语义与 gocql API 属于 [gocql 官方文档](https://pkg.go.dev/github.com/gocql/gocql)**
（ScyllaDB 说同一套原生协议，一个 starter 通吃 [config.go:26-28]）——下文只写 go-spring
的增量。

**激活**：任一 `spring.cassandra.instances.*` key（模块为 `OnProperty("spring.cassandra")` 前缀
匹配 [starter.go:36]）。每个 `spring.cassandra.instances.<name>` 条目创建一个名为 `<name>` 的
`*StarterCassandra.Client` bean（包装未导出的 `*gocql.Session`），并注册名为
`cassandra:<name>` 的健康指示器 [starter.go:46-60]。

---

## 1. 完整工程示例

一个集群两个实例：读写都走带防护的 `Query`/`Exec` 路径，外加探活/指标/trace。
文件树：

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖，模块名沿用 example 的 go.mod 布局）：

```
require (
    github.com/gocql/gocql          latest   // starter 传递引入
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-cassandra latest
    go-spring.org/starter-actuator  latest   // 可选：readiness + /metrics
    go-spring.org/starter-otel      latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance-file latest  // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cassandra"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入 wrapper；写与读都走带防护的路径：

```go
package service

import (
    "context"

    StarterCassandra "go-spring.org/starter-cassandra"
)

type Service struct {
    // 始终用 wrapper 类型 *StarterCassandra.Client。裸
    // *gocql.Session 是未导出字段；Query/Bind 返回带防护的 wrapper，
    // 其余 session 方法（Close、NewBatch、ExecuteBatch……）为显式委托。
    Main *StarterCassandra.Client `autowire:"a"`
    Raw  *StarterCassandra.Client `autowire:"b"` // 第二个实例，同一集群
}

func (s *Service) Run(ctx context.Context) error {
    // 有防护：breaker/limiter/fault。语句身份在此声明，span/指标/access log 由韧性层发射。
    if err := s.Main.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?) IF NOT EXISTS",
        1, "hello"); err != nil {
        return err
    }
    // 同样有防护 + 有声明：Client.Query 返回带防护的 wrapper（§2.3）——
    // Scan/Iter/ScanCAS/MapScan... 全部过治理执行器。
    var msg string
    return s.Main.Query("SELECT message FROM demo.greetings WHERE id = 1").
        WithContext(ctx).Scan(&msg)
}
```

**conf/app.properties** —— 上面实际用到的完整配置面：

```properties
# --- 实例 "a"：默认一致性级别、无认证（与本地容器一致）
spring.cassandra.instances.a.hosts=127.0.0.1
spring.cassandra.instances.a.consistency=local-quorum

# --- 实例 "b"：同一集群，预选 keyspace
spring.cassandra.instances.b.hosts=127.0.0.1
spring.cassandra.instances.b.keyspace=demo

# --- actuator + otel ------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**验证**（先起 Cassandra —— 用 [example 的 docker-compose.yml](example/docker-compose.yml)
`docker compose up -d`，Cassandra 5、端口 9042；启动很慢，check.sh 以
`cqlsh -e "DESCRIBE CLUSTER"` 最长等 240 秒）：

```bash
go run .                          # 集群不可达时启动 fail fast（需 ping=true；默认关闭）
curl -s :9370/readyz | jq .       # components 含 "cassandra:a"、"cassandra:b"
curl -s :9370/metrics | grep -E 'db.client.(operation.duration|attempt.duration|active_requests)'
grep _app_cassandra_access app.log | tail -3   # 每次 Exec 一条记录
docker exec -it cassandra-example cqlsh -e "SELECT * FROM demo.greetings"
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-cassandra
  └─ gs.Module(OnProperty("spring.cassandra.instances"))：任一 spring.cassandra.instances.* key 存在即触发
        └─ conf.BindEach(p, "${spring.cassandra.instances}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver),
              │            IndexArg(3, *resilience.Manager),
              │            IndexArg(4, *fault.Injector)).Name(<name>)
              │       .Destroy((*Client).Destroy)
              └─ Provide 名为 "cassandra:<name>" 的 health.Indicator，
                  按名注入上面的 client（TagArg）[starter.go:58-61]

gs.Run()
  ├─ 构造 newClient [starter.go:74]：
  │     username/password 成对校验 → 可选 Driver bean
  │     （无则用内置 DefaultDriver；多个并存时实例可按名指定：
  │     `spring.cassandra.instances.<name>.driver = <bean 名>`，留空 = 按类型注入
  │     唯一 Driver bean，指定的 bean 不存在则启动失败）→
  │     driver.CreateClient(c, cloud.ClientParams{Resilience: mgr, Fault: inj}) 返回 *Client
  │     （session + 身份 + 治理）：NewClient 设 exec = params.ExecutorFor("cassandra", service)，
  │     即 fault.WrapClientExecutor(mgr.ClientExecutorFor("cassandra", service), service, inj)，
  │     mgr/inj 为注入的 *resilience.Manager / *fault.Injector bean —— 构造期 exec 链即就绪
  ├─ 仅 ping=true：fail-fast 探活——newClient 调 HealthCheck [starter.go:100]，一条 system.local 查询直达裸 session
  ├─ readiness：indicator 委托 HealthCheck 查询 system.local [health.go:34]
  └─ SIGTERM → Destroy [client.go]：exec.Close → session.Close
```

没有 `Init` 钩子：`NewClient` 在 driver 装配期即固定 client 身份并应用治理包，`newClient`
返回时 bean 已完整。由于治理在构造期应用、先于探活，因此探活失败会销毁一个已受治理的
client（`Destroy`），不会泄漏执行器。

consistency 值未知、集群不可达都会中止启动——进程绝不会带着一个死掉的
Cassandra 进入 serving。

### 2.2 fail-fast 探活语义

`newClient` 以一次探活收尾——即 `HealthCheck`（`health.go:34`），该模块唯一的探活实现：
它在裸 session（`client.session`）上执行
`SELECT release_version FROM system.local` 扫进弃用变量，由 `newClient` 于 [starter.go:100] 调用。
Actuator 指示器的探针委托同一个 `HealthCheck`。探活直达裸
session 是有意为之——它是连通性检查、不是业务流量，不开 span、不耗 limiter/breaker 额度。
失败则销毁 client，并以 "failed to reach cassandra
cluster …" 中止启动。探活一回合验证协议版本、认证与集群状态——与运行期健康指示器同一
条查询。探活为 opt-in——只有实例置 `ping=true` 才执行；默认 `ping=false` 时配置里的
cassandra 条目不再意味着"启动时必须可用"，连通问题推迟到首次使用暴露。
探活由 gocql 自身的 `ConnectTimeout`/`Timeout`（此处默认各 11s）兜底，没有 starter 专属
超时。

### 2.3 哪些操作有/没有插桩 —— 逐操作走读

gocql 不提供可拒绝的中间件（没有 go-redis 那样的 hook 链），因此防护落在 query 对象
本身：`Client.Query`/`Client.Bind` 对应裸 session 同名方法、返回带防护的 `*Query`
wrapper [query.go]，其执行方法全部过守卫 + 执行器。`Client.Exec` 是该 wrapper 上的
薄别名（为早前 opt-in 时代的调用方保留）。所有语句执行方法——`Exec`、`Iter`、`Scan`、
`ScanCAS`、`MapScan`、`MapScanCAS`——无需调用点 opt-in 即有防护+声明：

`Client.Exec(ctx, stmt, values...)` / `Client.Query(...).Exec()` [client.go, query.go]：

1. `guard` 在 ctx 上声明该语句的身份（`observability.WithOperation`，[observe.go]）：
   span 名 `exec`、`db.system`/`db.operation` 标签、有界的 `db.statement` 明细
   （截断到 512 字节）。
2. `exec.Execute(ctx, call)` 向治理执行器申请许可 —— limiter/breaker 作用于
   service label `cassandra:<hosts[0]>`（按实例，且**只取第一个 host**
   [client.go]）。拒绝时语句**根本不会执行**。治理关闭时 client 运行在仅观测的、
   一次性告警的 unmanaged 执行器上：语句仍会执行并被 trace/metric 记录，但不受任何
   限流/熔断/重试约束。
3. 裸 session 的 `Query(stmt, values...).WithContext(ctx).Exec()`（经 wrapper 内嵌的
   `*gocql.Query`）执行 —— 完整 gocql 语义（预编译缓存、gocql 配置的重试、实例配置
   的默认一致性级别）。
4. 调用返回后，韧性层（链上唯一发射点）记录调用级时长直方图、按尝试发射
   `db.client.attempt.duration`、回落 in-flight、结束 span、输出 access-log 行
   （ok/error 状态；错误原样保留 gocql 错误——这里没有 go-redis 那种 nil 视作成功的
   特判）。无 starter-otel 全局件时 span/metric 为 no-op；access log 恒输出。

仍不覆盖的：(a) `Iter` 首页之后的翻页——守卫框住语句执行，后续页在返回的
`*gocql.Iter` 内部拉取（与 database/sql starter 同款语句级口径），且 `Iter` 自身的
错误因 gocql 未提供提前读取的手段而只能等 Scan/Close 暴露；(b) `Batch`
（NewBatch/ExecuteBatch）——batch 刻意保持原生，需要防护的批量写请走 `Query` 语句；
(c) 链式调用内嵌配置方法（`Consistency(...)` 等返回 `*gocql.Query`，会丢掉
wrapper）——请用 wrapper 自带的 `WithContext`/`Bind` 保持防护。后果：只要语句从
`Client.Query`/`Client.Bind`/`Client.Exec` 出发，读写都有 breaker/limiter/metrics/
access-log。

`Exec` 的分层顺序（由外向内）：fault 注入器（`fault.WrapClientExecutor` 包住注入 manager 解析
出的执行器）→ resilience 发射器（`resilience.WrapClientExecutor`：语句的 span、调用级/尝试级
直方图、in-flight 计数、outcome 计数与 access log —— 它读取 guard 声明的 operation）→
resilience 执行器（限流/熔断/重试核心）→ gocql。fault 注入器
包裹的是核心执行的那个操作函数，因此注入故障与 breaker 拒绝都会走完
重试/熔断/超时并被计数和记录。

### 2.4 Driver 构造缝

`Driver.CreateClient(ctx, Config, cloud.ClientParams) (*Client, error)` 拥有完整 session
装配 —— hosts、PasswordAuthenticator、一致性级别、超时、CQL 版本、TLS [driver.go:63-95] ——
并在构建期应用治理包，返回已完整的 `*Client` wrapper（身份与治理均经 `NewClient` 固定
[driver.go:95]）；只有启动探活留在 starter 生命周期里（`newClient`）。session 装配是
**可选容器 bean**：公司/伞包 starter 可把自己的 `Driver` 作为 bean 提供
（`gs.Provide(func() StarterCassandra.Driver{...})`，因为是 bean，可在装配期注入从配置
文件绑定的配置）；`spring.cassandra` 下每个实例都经它构建。没有该 bean 时 starter 在
装配内回退到内置 `DefaultDriver`（`driver.go:56`，`starter.go:82-84`）。没有
per-config 的 `driver` key。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.cassandra.instances.<name>.` 之下 —— 经 `conf.BindEach` 按实例绑定（构造
参数的 `Config`），不是 starter-Pool 的绝对属性规则。

### 3.1 连接与 session

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `hosts` | list | — | 初始接触点；条目可带 `:port`（默认端口 9042）；driver 自此发现集群其余节点。必填（expr `len($) > 0`，config.go:33）。 | 缺失/空 → BindEach 校验报错，启动失败。 |
| `keyspace` | string | — | session 默认 keyspace。留空则以无 keyspace 连接（比如先跑 `CREATE KEYSPACE`）。 | 名字错 → 每条带 keyspace 的语句报 "Keyspace … does not exist"。 |
| `username` / `password` | string | — | 两者都设 → `gocql.PasswordAuthenticator`（driver.go:74-76）；都空 → 无认证。⚠ 成对强制：只设其一 → 启动报 "username and password must be set together"（starter.go:77-79）。 | 只设一个 → 启动失败。 |
| `consistency` | string | `local-quorum` | session 默认一致性级别；精确匹配枚举 `any\|one\|two\|three\|quorum\|all\|local-quorum\|each-quorum\|local-one`（driver.go:99-120），无 kebab/camel 变体。 | 拼错 → 启动报错并列出合法值。 |
| `timeout` | duration | `11s` | gocql 单查询超时 —— `ping=true` 时同时兜底探活的查询回合。 | 过小 → 慢查询被客户端中断。 |
| `connect-timeout` | duration | `11s` | gocql 建连超时。 | 过小 → 慢网络下启动探活失败。 |
| `cql-version` | string | `3.0.0` | 传给 gocql 的 CQL 方言版本。 | 与服务端不匹配 → 握手报错。 |
| `tls.enabled` | bool | false | 开启整个 `tls.*` 组；映射 `gocql.SslOptions` [driver.go:77-88]。 | — |
| `tls.ca-file` | string | — | CA 路径（`CaPath`）。⚠ 除非 `insecure-skip-verify=true`，校验时必填。 | 缺 CA 且开校验 → 启动握手失败。 |
| `tls.cert-file` / `tls.key-file` | string | — | 客户端证书/私钥路径（双向 TLS）。⚠ 必须成对。 | 只配一个 → 握手失败。 |
| `tls.server-name` | string | — | 与 host 不同时的 SNI/校验名。 | 用 IP + 证书 CN 不一致 → 校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过主机校验（`EnableHostVerification = !值`，driver.go:86）。 | 生产开 = TLS 对 MITM 敞开。 |
| `ping` | bool | `false` | 启动连通探活：true 时构造期扫一次 `system.local`（`HealthCheck`），不可达则启动失败，恢复 fail-fast。默认关闭，使尚未就绪的集群不阻塞启动。 | `ping=true` 且集群已挂 → 启动报 "failed to reach cassandra cluster …"。 |
| `health` | bool | `true` | 本实例是否贡献 `health.Indicator`（名 `cassandra:<name>`）供 actuator 就绪/启动探测。置 false 可把该实例排除在聚合健康报告之外。 | `health=false` → `/readyz` 无 `cassandra:<name>` 组件。 |

### 3.2 观测

本 starter 没有观测类配置 key —— 插桩恒开启。也没有 `otel.*` key（与 starter-go-redis
不同）：发射的信号直接搭 OTel 全局件；不 import starter-otel 则 span/metric 为 no-op。
也没有服务发现类 key —— 没有 `service-name`、`scheme`、`discovery`。

### 3.3 指标 / 日志字段参考（Exec 声明，韧性层发射）

- 指标：`db.client.operation.duration`（直方图，秒）与 `db.client.attempt.duration`
  （直方图，秒，每次下游尝试一条），以及 `db.client.active_requests`（UpDownCounter）
  —— 属性 `db.system=cassandra`、`db.operation`、`status`（observe.go 声明；`status`
  与 `resilience.client.calls` 计数由发射器补充）。
- access log：tag `_app_cassandra_access`（`log.RegisterAppTag("cassandra","access")`），
  每次 Exec 一条，走韧性层的分级：错误 → Warn；成功且带语句参数（截断至 512 字节）
  → Debug；普通成功 → Info。
- span：名 `exec`，属性 `db.system` / `db.operation` / `db.statement`
  （语句截断至 512 字节）。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .          # components 含 "cassandra:a"
docker stop cassandra-example        # indicator 重查 system.local → DOWN
curl -s :9370/readyz                 # 503
docker start cassandra-example
```

### 4.2 观察 Exec 路径

```bash
grep _app_cassandra_access app.log | tail -1
# db.system=cassandra db.operation=exec db.statement="INSERT ..." status=ok duration_ms=...
curl -s :9370/metrics | grep db.client   # 调用级/尝试级直方图 + active_requests
```

再验证守卫之外的部分：跑一次 `NewBatch`/`ExecuteBatch`，或链式调用裸配置方法
（`Consistency(...)` 返回 `*gocql.Query`）—— 不会新增 access log 行、指标无变化、
无 span（这些路径设计上不声明身份，§2.3）。

### 4.3 故障 / resilience 演练（需 starter-governance-file）

为 service `cassandra:127.0.0.1` 在 `spring.governance.*` 下配 breaker 或 limiter，压测 `Exec`
插入，观察拒绝以快速错误返回且语句**未执行**（Cassandra 侧无行），并有 resilience
发射器的 outcome 计数。运行期翻转策略 —— 执行器热生效，无需重启。注意 service label
只取 hosts[0]：首 host 相同的实例 `b` 与实例 `a` 共用同一个 breaker 桶。

### 4.4 fail-fast 探活演练（`ping=true`）

```bash
docker stop cassandra-example && go run .   # 实例需 ping=true
# 启动中止："failed to reach cassandra cluster [127.0.0.1]" —— 进程绝不进入
# serving。重启容器后同一配置正常启动。
# 默认 ping=false 时启动成功，改为首条语句才失败。
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 "failed to reach cassandra cluster" | hosts 不可达 / 凭证错误 / TLS 不匹配 | 启动探活仅在 `ping=true` 时执行（默认关闭，§2.2）；修连通性或认证，或去掉 `ping` 把失败推迟到首次使用；等 CQL 完全就绪（Cassandra 5 启动慢，check.sh 留 240s）。 |
| 启动报 "username and password must be set together" | 只配了成对校验的一边 | 两个都配或都不配 [starter.go:77-79]。 |
| 启动报 "unknown consistency" | `consistency` 拼错；枚举精确匹配 | 用九个合法值之一 [driver.go:120]。 |
| Exec 无 span/metric | 未 import starter-otel | 发射的信号搭 OTel 全局件；import starter-otel（access log 仍会输出）。 |
| 完全没有 access log 行 | logger 级别过滤掉了 Debug/Info，或 `_app_cassandra_access` tag 被过滤 | 检查 logger 级别及其对 `_app_cassandra_access` tag 的过滤。 |
| breaker/limiter 永不触发 | 语句走了 batch（`NewBatch`/`ExecuteBatch`），或链式配置方法返回裸 `*gocql.Query` 丢掉了防护 | 语句从 `Client.Query`/`Client.Bind`/`Client.Exec` 出发（§2.3）。 |
| 两个实例意外共用一个 breaker | service label 是 `cassandra:<hosts[0]>` [client.go:101] | 设计行为（多 seed 折叠到首个 host）；需要隔离就拆接触点列表。 |
| Exec 可用但健康 DOWN | indicator 带自身 ctx 查 system.local；查权限/超时 | 看 /readiness 中该 component 的错误详情。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 11 个实例 key + tls 组 6 个 |
| 其中必填 | 1（`hosts`） |
| quickstart 前置外部依赖数 | 1（Cassandra） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单（审计台账 —— 沿自上一轮审计，仍然成立）：

- ~~只有 `Exec` 有防护/观测~~ 已修：带防护的 `*Query` wrapper 覆盖常规语句路径
  （§2.3）；batch 与 `Iter` 深翻页仍在守卫之外。
- service label 只用 `hosts[0]`，多 seed 配置共享一个以首 host 为键的 resilience 桶。
- 健康指示器每实例默认注册（`health=false` 可关）；启动探活为 opt-in（`ping=true`）——即
  redigo 拆成 `health.enabled`/`startup-ping` 的那两个旋钮。
