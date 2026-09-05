# starter-cassandra 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`driver.go`、`health/health.go`）与可运行的
[example/](example/)（check.sh 自断言冒烟）核对，方括号内为 file:line 抽查点。
**CQL 语义与 gocql API 属于 [gocql 官方文档](https://pkg.go.dev/github.com/gocql/gocql)**
（ScyllaDB 说同一套原生协议，一个 starter 通吃 [config.go:26-28]）——下文只写 go-spring
的增量。

**激活**：任一 `spring.cassandra.*` key（模块为 `OnProperty("spring.cassandra")` 前缀
匹配 [starter.go:38]）。每个 `spring.cassandra.<name>` 条目创建一个名为 `<name>` 的
`*StarterCassandra.Client` bean（内嵌 `*gocql.Session`），并注册名为
`cassandra:<name>` 的健康指示器 [starter.go:47-49]。

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
    go-spring.org/starter-governance latest  // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cassandra"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入 wrapper；写走防护路径，读走原生路径：

```go
package service

import (
    "context"

    StarterCassandra "go-spring.org/starter-cassandra"
)

type Service struct {
    // 始终用 wrapper 类型 *StarterCassandra.Client。它内嵌
    // *gocql.Session，Query/Iter/Scan/Close 原样提升。
    Main *StarterCassandra.Client `autowire:"a"`
    Raw  *StarterCassandra.Client `autowire:"b"` // 第二个实例，同一集群
}

func (s *Service) Run(ctx context.Context) error {
    // 有防护 + 有观测：breaker/limiter/fault + span + metric + access log。
    if err := s.Main.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?) IF NOT EXISTS",
        1, "hello"); err != nil {
        return err
    }
    // 同样有防护 + 有观测：Client.Query 返回带防护的 wrapper（§2.3）——
    // Scan/Iter/ScanCAS/MapScan... 全部过治理执行器。
    var msg string
    return s.Main.Query("SELECT message FROM demo.greetings WHERE id = 1").
        WithContext(ctx).Scan(&msg)
}
```

**conf/app.properties** —— 上面实际用到的完整配置面：

```properties
# --- 实例 "a"：默认一致性级别、无认证（与本地容器一致）
spring.cassandra.a.hosts=127.0.0.1
spring.cassandra.a.consistency=local-quorum

# --- 实例 "b"：同一集群，预选 keyspace
spring.cassandra.b.hosts=127.0.0.1
spring.cassandra.b.keyspace=demo

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
go run .                          # 集群不可达时启动 fail fast
curl -s :9370/readyz | jq .       # components 含 "cassandra:a"、"cassandra:b"
curl -s :9370/metrics | grep -E 'db.client.(operation.duration|active_requests)'
grep _app_cassandra_access app.log | tail -3   # 每次 Exec 一条记录
docker exec -it cassandra-example cqlsh -e "SELECT * FROM demo.greetings"
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-cassandra
  └─ gs.Module(OnProperty("spring.cassandra"))：任一 spring.cassandra.* key 存在即触发
        └─ conf.BindEach(p, "${spring.cassandra}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient, IndexArg(1, ValueArg(c))).Name(<name>)
              │       .Init((*Client).Init).Destroy((*Client).Destroy)
              └─ Provide 名为 "cassandra:<name>" 的 health.Indicator，
                  按名注入上面的 client（TagArg）[starter.go:47-49]

gs.Run()
  ├─ 构造 newClient [starter.go:59]：
  │     username/password 成对校验 → driver 查找 → driver.CreateClient
  │     → HealthCheck 探活（fail fast，见下）
  ├─ Init [client.go:59]：newDBObserver("cassandra") → resource label
  │     → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │     → resilience.WrapExecutor(exec, "cassandra") —— exec 链就绪
  ├─ readiness：每个实例的 indicator 查询 system.local
  └─ SIGTERM → Destroy [client.go:75]：exec.Close（若已武装）→ Session.Close
```

driver 名配错、consistency 值未知、集群不可达都会中止启动——进程绝不会带着一个死掉的
Cassandra 进入 serving。

### 2.2 fail-fast 探活语义

`newClient` 以一次 `HealthCheck` 收尾：`SELECT release_version FROM system.local` 扫进
弃用变量 [starter.go:75-78, 84-87]。失败则关闭 session 并以 "failed to reach cassandra
cluster …" 中止启动。探活一回合验证协议版本、认证与集群状态——与运行期健康指示器同一
条查询。没有重试、没有跳过开关：配置里出现 cassandra 条目即意味着"启动时必须可用"。
探活由 gocql 自身的 `ConnectTimeout`/`Timeout`（此处默认各 11s）兜底，没有 starter 专属
超时。

### 2.3 哪些操作有/没有插桩 —— 逐操作走读

gocql 不提供可拒绝的中间件（没有 go-redis 那样的 hook 链），因此防护落在 query 对象
本身：`Client.Query`/`Client.Bind` 遮蔽内嵌 session 同名方法、返回带防护的 `*Query`
wrapper [query.go]，其执行方法全部过执行器 + 观察者。`Client.Exec` 是该 wrapper 上的
薄别名（为早前 opt-in 时代的调用方保留）。所有语句执行方法——`Exec`、`Iter`、`Scan`、
`ScanCAS`、`MapScan`、`MapScanCAS`——无需调用点 opt-in 即有防护+观测：

`Client.Exec(ctx, stmt, values...)` / `Client.Query(...).Exec()` [client.go, query.go]：

1. `obs.Start(ctx, "exec", stmt)` 打开名为 `exec` 的 client 型 span（语句作为有界
   `db.statement` 属性）、抬升 in-flight 计数、开启一条 access-log 记录。无 starter-otel 全局件时
   span/metric 为 no-op；access log 恒输出。
2. `exec.Execute(ctx, resource, call)` 向治理执行器申请许可 —— limiter/breaker 作用于
   resource label `cassandra:<hosts[0]>`（按实例，且**只取第一个 host**
   [client.go:66]）。拒绝时语句**根本不会执行**。治理关闭时执行器是透明 no-op。
3. `Session.Query(stmt, values...).WithContext(ctx).Exec()`（经 wrapper 内嵌的
   `*gocql.Query`）执行 —— 完整 gocql 语义（预编译缓存、gocql 配置的重试、实例配置
   的默认一致性级别）。
4. `sp.End(err)` 记录时长直方图、回落 in-flight、结束 span、输出 access-log 行
   （ok/error 状态；错误原样保留 gocql 错误——这里没有 go-redis 那种 nil 视作成功的
   特判）。

仍不覆盖的：(a) `Iter` 首页之后的翻页——守卫框住语句执行，后续页在返回的
`*gocql.Iter` 内部拉取（与 database/sql starter 同款语句级口径），且 `Iter` 自身的
错误因 gocql 未提供提前读取的手段而只能等 Scan/Close 暴露；(b) `Batch`
（NewBatch/ExecuteBatch）——batch 刻意保持原生，需要防护的批量写请走 `Query` 语句；
(c) 链式调用内嵌配置方法（`Consistency(...)` 等返回 `*gocql.Query`，会丢掉
wrapper）——请用 wrapper 自带的 `WithContext`/`Bind` 保持防护。后果：只要语句从
`Client.Query`/`Client.Bind`/`Client.Exec` 出发，读写都有 breaker/limiter/metrics/
access-log。

`Exec` 的分层顺序（由外向内）：observer start → fault 注入器（fault.WrapExecutor，
进程级）→ resilience 执行器（治理中心）→ gocql。resilience observer
（`resilience.WrapExecutor`）在执行器外再加 outcome 计数，因此注入故障与 breaker
拒绝都会被计数和记录。

### 2.4 Driver 构造缝

`Driver.CreateClient(ctx, Config) (*gocql.Session, error)` 拥有完整 session 装配 ——
hosts、PasswordAuthenticator、一致性级别、超时、CQL 版本、TLS [driver.go:58-93] —— 而
启动探活、resource label 与 resilience 接线留在 starter 生命周期里。自定义 driver 用
`RegisterDriver(name, Driver)` 注册（重名 panic），以 `driver` key 选择。与 starter-s3
的构造缝同构。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.cassandra.<name>.` 之下 —— 经 `conf.BindEach` 按实例绑定（构造
参数的 `Config`），不是 starter-Pool 的绝对属性规则。

### 3.1 连接与 session

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `hosts` | list | — | 初始接触点；条目可带 `:port`（默认端口 9042）；driver 自此发现集群其余节点。必填（expr `len($) > 0`，config.go:33）。 | 缺失/空 → BindEach 校验报错，启动失败。 |
| `keyspace` | string | — | session 默认 keyspace。留空则以无 keyspace 连接（比如先跑 `CREATE KEYSPACE`）。 | 名字错 → 每条带 keyspace 的语句报 "Keyspace … does not exist"。 |
| `username` / `password` | string | — | 两者都设 → `gocql.PasswordAuthenticator`（driver.go:73-74）；都空 → 无认证。⚠ 成对强制：只设其一 → 启动报 "username and password must be set together"（starter.go:62-64）。 | 只设一个 → 启动失败。 |
| `consistency` | string | `local-quorum` | session 默认一致性级别；精确匹配枚举 `any\|one\|two\|three\|quorum\|all\|local-quorum\|each-quorum\|local-one`（driver.go:96-119），无 kebab/camel 变体。 | 拼错 → 启动报错并列出合法值。 |
| `timeout` | duration | `11s` | gocql 单查询超时 —— 同时兜底 fail-fast 探活的查询回合。 | 过小 → 慢查询被客户端中断。 |
| `connect-timeout` | duration | `11s` | gocql 建连超时。 | 过小 → 慢网络下启动探活失败。 |
| `cql-version` | string | `3.0.0` | 传给 gocql 的 CQL 方言版本。 | 与服务端不匹配 → 握手报错。 |
| `tls.enabled` | bool | false | 开启整个 `tls.*` 组；映射 `gocql.SslOptions` [driver.go:76-87]。 | — |
| `tls.ca-file` | string | — | CA 路径（`CaPath`）。⚠ 除非 `insecure-skip-verify=true`，校验时必填。 | 缺 CA 且开校验 → 启动握手失败。 |
| `tls.cert-file` / `tls.key-file` | string | — | 客户端证书/私钥路径（双向 TLS）。⚠ 必须成对。 | 只配一个 → 握手失败。 |
| `tls.server-name` | string | — | 与 host 不同时的 SNI/校验名。 | 用 IP + 证书 CN 不一致 → 校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过主机校验（`EnableHostVerification = !值`，driver.go:85）。 | 生产开 = TLS 对 MITM 敞开。 |
| `driver` | string | `DefaultDriver` | 选择已注册 Driver（注册表 + 重名 panic，driver.go:31-53）。 | 未知名 → 启动报 "cassandra driver not found"。 |

### 3.2 观测

本 starter 没有观测类配置 key —— 插桩恒开启。也没有 `otel.*` key（与 starter-go-redis
不同）：observer 直接搭 OTel 全局件；不 import starter-otel 则 span/metric 为 no-op。
也没有服务发现类 key —— 没有 `service-name`、`scheme`、`discovery`。

### 3.3 指标 / 日志字段参考（Exec 产出）

- 指标：`db.client.operation.duration`（直方图，秒）、`db.client.active_requests`
  （UpDownCounter）—— 属性 `db.system=cassandra`、`db.operation`（observe.go）。
- access log：tag `_app_cassandra_access`（`log.RegisterAppTag("cassandra","access")`），
  每次 Exec 一条，走 log 包原生分级：错误 → Warn；成功且带语句参数（截断至 512 字节）
  → Debug；普通成功 → Info。
- span：名 `exec`，kind client，属性 `db.system` / `db.operation` / `db.statement`
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
# system=cassandra op=exec status=ok duration=...
curl -s :9370/metrics | grep db.client   # 时长直方图 + active_requests
```

再验证不对称性：跑一次 `Query(...).Scan` 读 —— 不会新增 access log 行、指标无变化、
无 span（该路径设计上无观测，§2.3）。

### 4.3 故障 / resilience 演练（需 starter-governance）

为 resource `cassandra:127.0.0.1` 在 `govern.*` 下配 breaker 或 limiter，压测 `Exec`
插入，观察拒绝以快速错误返回且语句**未执行**（Cassandra 侧无行），并有 resilience
observer 的 outcome 计数。运行期翻转策略 —— 执行器热生效，无需重启。注意 resource label
只取 hosts[0]：首 host 相同的实例 `b` 与实例 `a` 共用同一个 breaker 桶。

### 4.4 fail-fast 探活演练

```bash
docker stop cassandra-example && go run .
# 启动中止："failed to reach cassandra cluster [127.0.0.1]" —— 进程绝不进入
# serving。重启容器后同一配置正常启动。
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 "failed to reach cassandra cluster" | hosts 不可达 / 凭证错误 / TLS 不匹配 | 启动探活无条件执行（§2.2）；修连通性或认证；等 CQL 完全就绪（Cassandra 5 启动慢，check.sh 留 240s）。 |
| 启动报 "username and password must be set together" | 只配了成对校验的一边 | 两个都配或都不配 [starter.go:62-64]。 |
| 启动报 "unknown consistency" | `consistency` 拼错；枚举精确匹配 | 用九个合法值之一 [driver.go:117]。 |
| 启动报 "cassandra driver not found" | `driver` 未注册 | 在 init 里 `RegisterDriver`，或删掉该 key（DefaultDriver）。 |
| Exec 无 span/metric | 未 import starter-otel | observer 搭 OTel 全局件；import starter-otel（access log 仍会输出）。 |
| 完全没有 access log 行 | logger 级别过滤掉了 Debug/Info，或 `_app_cassandra_access` tag 被过滤 | 检查 logger 级别及其对 `_app_cassandra_access` tag 的过滤。 |
| breaker/limiter 永不触发 | 用的是裸 `*gocql.Session`（如别处取得的 session）、或 batch、或链式配置方法丢掉了 wrapper | 语句从 `Client.Query`/`Client.Bind`/`Client.Exec` 出发（§2.3）。 |
| 两个实例意外共用一个 breaker | resource label 是 `cassandra:<hosts[0]>` [client.go:66] | 设计行为（多 seed 折叠到首个 host）；需要隔离就拆接触点列表。 |
| Exec 可用但健康 DOWN | indicator 带自身 ctx 查 system.local；查权限/超时 | 看 /readiness 中该 component 的错误详情。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 10 个实例 key + tls 组 6 个 |
| 其中必填 | 1（`hosts`） |
| quickstart 前置外部依赖数 | 1（Cassandra） |
| 文档中"注意/坑"条数 | 4 |

设计嫌疑清单（审计台账 —— 沿自上一轮审计，仍然成立）：

- ~~只有 `Exec` 有防护/观测~~ 已修：带防护的 `*Query` wrapper 覆盖常规语句路径
  （§2.3）；batch 与 `Iter` 深翻页仍在守卫之外。
- resource label 只用 `hosts[0]`，多 seed 配置共享一个以首 host 为键的 resilience 桶。
- 健康指示器没有关闭 key（与 redigo 的 `health.enabled` 家族不对称）。
