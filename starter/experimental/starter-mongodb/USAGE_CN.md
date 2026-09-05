# starter-mongodb 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health/health.go`）与
可运行示例（[example/](example/)、[example-otel/](example-otel/)、
[example-cloudnative/](example-cloudnative/)、[example-load/](example-load/)）核实——文中方括号
为 file:line 抽查点。**MongoDB 语义与 mongo-driver v2 API 属于
[驱动官方文档](https://www.mongodb.com/docs/languages/go/go-driver/current/)**——本文只写
go-spring 的增量。

**激活条件**：出现任意 `spring.mongodb.*` key（模块为 `OnProperty("spring.mongodb")` 前缀
检查）。每个 `spring.mongodb.<name>` 条目创建一个名为 `<name>` 的 `*StarterMongoDB.Client`
bean（内嵌 `*mongo.Client`），并附带名为 `mongo:<name>` 的健康指示器。

---

## 1. 完整工程示例

单服务双实例——一个按 URI 直连、一个经服务发现解析——外加健康/actuator、otel、治理。
文件树：

```
demo/
├── go.mod
├── discovery.go
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go.mongodb.org/mongo-driver/v2  v2.6.0
    go-spring.org/spring            v1.3.x
    go-spring.org/cloud             latest
    go-spring.org/starter-mongodb   latest
    go-spring.org/starter-actuator  latest   // 可选：readiness + health
    go-spring.org/starter-governance latest  // 可选：resilience/fault 策略
    go-spring.org/starter-otel      latest   // 可选：真实 trace/metric 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-mongodb"
    _ "demo/service"
)

func main() { gs.Run() }
```

**discovery.go** —— 公司适配器只注册一次命名服务；此处用静态后端让示例自包含
（取自 [example/discovery.go](example/discovery.go)）：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default", discovery.NewStaticDiscovery(
        discovery.Endpoint{Addr: "127.0.0.1:27017", Healthy: true},
    ))
}
```

**service.go** —— 注入 wrapper，使用完整 mongo-driver 面：

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterMongoDB "go-spring.org/starter-mongodb"
    "go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
    // 永远注入 wrapper 类型 *StarterMongoDB.Client。它内嵌
    // *mongo.Client，Database/Collection/StartSession/ping 原样提升。
    // 不能按裸 *mongo.Client 注入。
    Main *StarterMongoDB.Client `autowire:"a"`   // URI 直连
    Disc *StarterMongoDB.Client `autowire:"disc"` // 服务发现（service-name）
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            coll := s.Main.Database("test").Collection("kv")
            _, _ = coll.InsertOne(ctx, bson.M{"key": "k", "value": "v"})
            // 发现客户端同样往返；其地址来自后端而非（占位）URI host。
            _ = s.Disc.Database("test").RunCommand(ctx, bson.M{"ping": 1}).Err()
        }
    })
}
```

**conf/app.properties** —— 上述用到的完整配置面：

```properties
# --- 直连实例 ---------------------------------------------------------------
spring.mongodb.a.uri=mongodb://127.0.0.1:27017
spring.mongodb.a.max-pool-size=100
spring.mongodb.a.min-pool-size=1
spring.mongodb.a.max-conn-idle-time=5m
spring.mongodb.a.server-selection-timeout=10s

# --- 服务发现实例 -----------------------------------------------------------
# uri host 故意写成不可解析的占位：service-name 接管寻址，
# 连接成功即证明 discovery 已生效。directConnection=true 让驱动停在拨到的
# seed 上，不做自己的副本集拓扑发现（见 §3.1 ⚠ 说明）。
spring.mongodb.disc.uri=mongodb://nonexistent.invalid:27017/?directConnection=true
spring.mongodb.disc.service-name=mongo-cluster
spring.mongodb.disc.server-selection-timeout=10s

# --- 治理：作用于建连 seam 的策略（rate-limit 让建连保护可观测；
#     breaker/retry/timeout 同样生效）----------------------------------------
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5

# --- actuator + otel -------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**验证**（先起 MongoDB：`docker run -d -p 27017:27017 mongo:7`；启用 trace 导出时另需
:4317 上的 Jaeger/OTLP collector）：

```bash
go run .                          # server 不可达则启动直接失败（startup ping）
curl -s :9370/readyz | jq .       # components 含 "mongo:a" 与 "mongo:disc"
curl -s :9090/metrics | grep db.client   # db.client.operation.duration / db.client.active_requests
grep _app_mongodb_access app.log | tail -3   # 每条命令一条访问记录
# Jaeger UI（http://localhost:16686）：span 名为 insert/find/ping，属性 db.system=mongodb
```

[example-cloudnative](example-cloudnative/) 旗舰示例自校验同一能力集（健康、直连+发现
往返、限流建连爆发、热更新配置），失败即非零退出——其 `check.sh` 就是本节的可执行版本。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-mongodb
  └─ gs.Module(OnProperty("spring.mongodb")) 在任意 spring.mongodb.* key 存在时触发
        └─ conf.BindEach("${spring.mongodb}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient).Name(<name>).Init((*Client).Init).Destroy((*Client).Destroy)
              └─ Provide health.Indicator 名为 "mongo:<name>"，Export As[health.Indicator]

gs.Run()
  ├─ 构造 newClient [starter.go:80]：ApplyURI → 超时/连接池/认证 → tls.Build
  │   → SetMonitor(command monitor，惰性 observer)[starter.go:118]
  │   → newPickPool（设置 service-name 且非 mesh 时构建 loader-backed 端点池）[starter.go:121]
  │   → SetDialer(共享 dialerWrapper)[starter.go:147]
  │   → mongo.Connect → fail-fast Ping，由 connect-timeout 约束（兜底 10s）
  │     [starter.go:160-169] —— server 挂掉失败的是启动，不是第一条查询
  ├─ Init [client.go:90]：newDBObserver("mongodb") → 模块内建 observer（span + 指标 + 访问日志）
  │   → fault.WrapExecutor(resilience.ExecutorFor(resource)) → resilience.WrapExecutor(exec, "mongodb")
  │   → 换入 dialerWrapper.dial = resilience.NewDialer(base, exec, resource)
  ├─ 就绪：mongo:<name> 指示器对真实 server 跑 client.Ping
  └─ SIGTERM → Destroy [client.go:112]：exec.Close → client.Disconnect
      （loader 无资源，无需释放任何 discovery 相关的东西）
```

### 2.2 两个 seam —— 及塑造它们的家族不对称

mongo driver v2 **没有可类比 go-redis ProcessHook 的逐操作钩子**（[client.go:44-47]、
[command.go:18-21]）。因此 starter 把其他 client starter 在一条 hook 链里做的事拆成两个
seam：

- **观测走 command monitor**（[command.go:53]）：`event.CommandMonitor` 的
  Started/Succeeded/Failed 事件按 (connection id, request id) 关联。每条命令——insert、
  find、ping、hello——在 Started 开 span、抬 in-flight gauge，在 Succeeded/Failed 收尾。
  monitor 经 atomic pointer 惰性读 observer，因为它在构造期安装、早于 `Init` 构建
  observer；nil 保护覆盖启动 ping 路径（[command.go:39-45]）。
- **resilience/fault 走建连层**：`Init` 用 `resilience.NewDialer` 包裹拨号函数——
  breaker/limiter/bulkhead/timeout/fault 作用于**每次新连接**，不是每条命令。已建好的
  池内连接全速运行（[client.go:47-51]）。对家族不对称的诚实表述：这里**没有逐请求的
  resilience 埋点**——breaker 因连接失败而跳闸、限制的是建连抖动；慢而成功的命令永远
  触不到它。

拨号替换无需重建客户端：构造期交给驱动的是共享 `dialerWrapper`，`Init` 事后改写其
`dial` 字段（[starter.go:143-147]、[client.go:104-106]）。

为何手写而非 otelmongo：官方埋点只支持 v1 驱动，其 CommandMonitor 类型与 v2 不兼容；
这里的桥接是模块内建的（[observe.go]），使 MongoDB 与其他 client starter 同一套词汇。

### 2.3 一条命令逐层走读：直连实例上的 `FindOne`

1. `coll.FindOne(ctx, ...)` 在内嵌的 `*mongo.Client` 上执行——wrapper 不拦截；所有驱动
   方法原样提升。
2. 池中无空闲连接时，驱动调 `dialerWrapper.DialContext` → resilience executor 申请配额
   （资源标签 `mongodb:<service-name 或 uri>`，按实例，[client.go:99]）；超限的拨号被拒，
   操作浮出 `resilience.ErrRateLimited`。设置 service-name 时，底层拨号先经
   loader-backed `Pool` 选活端点、忽略 URI 地址（[starter.go:130-137]）。
3. 驱动发出 `find` 命令；monitor 的 `Started` 触发：`obs.Start(ctx, "find", "test")` ——
   span 名 = 命令名，参数 = 库名。
4. 应答触发 `Succeeded`（或 `Failed`）：span 收尾、记录 `db.client.operation.duration`、
   平衡 in-flight gauge，并输出一条 `_app_mongodb_access` 日志（失败 → Warn；带库名参数的
   成功 → Debug；普通成功 → Info）。
5. **复用的池内连接完全跳过第 2 步**——这正是 dial-only resilience seam 的实际表现。

---

## 3. 逐 key 行为参考

实例 key 位于 `spring.mongodb.<name>.`（经 `conf.BindEach` 绑定）。没有 observability
key——观测无条件开启（见 §3.3）。

### 3.1 连接与寻址

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `uri` | string | — | **必填**（expr `$ != ''`）。经 `ApplyURI` 解析；URI 内驱动选项优先，除非下方字段覆盖。 | 缺失/为空 → 绑定期启动报错。 |
| `username` | string | — | 非空时由 username/password/auth-source/auth-mechanism 组成 `options.Credential` [starter.go:97-104]。为空 → 凭据完全取自 URI。 | 设了 username 漏了 password/auth-source → 启动 ping 认证失败。 |
| `password` | string | — | 上述凭据一部分。⚠ 仅与 `username` 一起生效。 | — |
| `auth-source` | string | — | 凭据校验库，如 `admin`。⚠ 仅随 `username`。 | 库错 → 启动 ping 报 "Authentication failed"。 |
| `auth-mechanism` | string | — | 如 `SCRAM-SHA-256`；空 = 驱动自动协商。⚠ 仅随 `username`。 | 不支持的机制 → 启动 ping 报错。 |
| `connect-timeout` | duration | `10s` | 传给驱动，且限定 fail-fast 启动 ping（0 → 兜底 10s，[starter.go:182-187]）。 | 过小 → 慢网络下启动 ping 假超时。 |
| `server-selection-timeout` | duration | `0` | 0 = 驱动默认（30s）。驱动等待合适 server 的时长。 | 过小 + 发现延迟 → "server selection timeout"。 |
| `max-pool-size` | uint64 | `100` | 每 server 最大连接数。0 本意为用默认——但 starter 未设时显式传 100。 | 过小 → 操作排队等池位。 |
| `min-pool-size` | uint64 | `0` | 最小池内连接（恒应用，含 0）。 | — |
| `max-conn-idle-time` | duration | `0` | 0 = 不限；如 `5m` 清理空闲连接。⚠ 配 `service-name` 时有限值可在不重启的情况下把连接逐步迁到新端点。 | `0` + 发现 → 连接在被踢掉的端点上滞留到断开。 |
| `service-name` | string | — | 经已注册的发现后端解析地址；每次建连由 loader-backed（`Pool.Pick`）拨号替代 URI host [starter.go:126-137]。⚠ **绕过 MongoDB 自身拓扑发现**（副本集/mongos）——驱动拨命名服务给出的地址；需在 URI 配合 `directConnection=true`（[config.go:80-84]）。mesh 模式下忽略（sidecar 负责发现+LB）。 | 副本集 URI 不加 `directConnection=true` → "no such host"/拓扑报错；占位 URI 只有在 loader/pool 真被咨询时才能证明发现生效。 |
| `scheme` | string | — | 把发现端点收窄到一种传输 scheme（如 `tls`）。仅 service-name 生效时被咨询。 | — |
| `discovery` | string | `default` | 用哪个已注册发现后端解析 service-name。 | 未注册后端 → discovery.NewLoader 启动报错。 |
| `tls.*` | group | off | 共享 `tlsconf` 块（enabled/ca-file/cert-file/key-file/server-name/insecure-skip-verify）；`tls.Build` 报错直接失败启动 [starter.go:105-112]。enabled=false → 不启 TLS，除非 URI 自己要求（`mongodbs://` / `tls=true`）。 | 配一半 → 启动报 "mongodb: build TLS"。 |

### 3.2 resilience / fault（govern.*，不在实例前缀下）

策略 key 位于顶层 `govern.*`（starter-governance 治理中心）；本 starter 在 `Init` 里解析
`resilience.ExecutorFor("mongodb:<service-name|uri>")` 与 `fault.InjectorFor`
[client.go:97-102]。相关 key（全集见 starter-governance USAGE）：`govern.enabled`、
`govern.driver`、`govern.<driver>.rate-limit` / `error-threshold` / `open-duration` /
`max-retries` / `timeout`，以及 `govern.fault.*` 注入块（enable/rate/error）。⚠ 记住
seam 在**建连层**：breaker 策略表现为拒绝*连接*；故障注入按拨号触发，不按命令。

### 3.3 可观测性

本模块**没有 observability 配置 key**（没有 level、没有跳过名单、没有参数上限）：
观测恒开启，由模块内建埋点（[observe.go]）产出。trace 与 metric 搭乘 starter-otel
安装的 OTel 全局（`spring.observability.*`）；无 starter-otel 时为 no-op。

没有 `driver` key —— 本 starter 无 driver 注册表（[driver.go:17-21]）。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .      # components 含 "mongo:a"、"mongo:disc"
docker stop <mongo>              # 指示器跑 client.Ping → 组件翻 DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <mongo>
```

指示器无条件注册——没有关闭开关（[starter.go:55-57]）。

### 4.2 观测到底产出什么

```bash
curl -s :9090/metrics | grep db.client
# db.client.operation_duration_seconds...{db.operation="find",db.system="mongodb",status="ok"}
# db.client.active_requests{db.operation=...,db.system="mongodb"}
grep _app_mongodb_access app.log | tail -1
# db.operation=find status=ok duration_ms=1.2 ；带库名参数的成功为 Debug，
# 普通成功为 Info，失败时追加 error=... 并升为 Warn
```

- span：以 MongoDB 命令命名（`find`、`insert`、`ping`），属性 `db.system=mongodb`、
  `db.operation=<命令>`、`db.statement=<库名>`（截断至 512 字节）。
- 启动 ping 同样被观测——除非 observer 仍为 nil（Init 前），由 monitor 的 nil 保护
  覆盖（[command.go:44-45]）。

### 4.3 建连层 resilience 演练（取自 example-cloudnative）

```properties
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5
spring.mongodb.a.max-pool-size=100   # 给爆发留出强制新建连接的空间
```

冷池上并发打 40 个 `InsertOne`：超限的拨号以 `resilience.ErrRateLimited` 失败、浮出为
操作的连接错误；获准的照常成功（[example-cloudnative/example.go:197-225]）。反面同样
成立：池一旦焐热，同样的爆发全数通过——保护是连接级的。运行时翻转 `govern.*` ——
executor 热更新，无需重启（治理中心）。

### 4.4 故障注入 + 压测演练（example-load）

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic    # 或：timeout / reset
```

运行 [example-load](example-load/)：upsert/FindOne 闭环输出吞吐、延迟分位与错误分布；
故障在建连 seam（新连接）触发，因此 `max-conn-idle-time` 越短故障暴露窗口越大。

### 4.5 发现地址回收

扩缩/迁移后端实例；每条**新**连接都向 loader-backed `Pool` 要活端点。配 `max-conn-idle-time=5m`，
池在该窗口内无重启完成迁移。用 `_app_mongodb_access` 记录验证，或停掉旧端点看
`mongo:<name>` 健康保持 UP。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `mongodb: ping <uri>: ...` | server 不可达 / 凭据错 / TLS 不匹配——fail-fast ping 无条件执行 | 修连通性/凭据；`connect-timeout` 约束探测时长。 |
| 绑定期对 `uri` 启动失败 | `uri` 为空——expr 校验非空 | 设置 `spring.mongodb.<name>.uri`。 |
| 启动报 "build TLS" / "build discovery resolver" | `tls.*` 配了一半；`discovery` 指向未注册后端 | 补全 tlsconf 块；`discovery.RegisterDiscovery` 注册后端。 |
| 发现客户端报 "no such host" / 拓扑错误 | `service-name` 绕过驱动拓扑发现 | URI 加 `directConnection=true`；副本集/mongos URI 则放弃 service-name。 |
| 爆发时操作报 `ErrRateLimited` | 治理 rate-limit 作用在建连 seam | 调高 `govern.<driver>.rate-limit` 或 `max-pool-size`/`min-pool-size`（焐热的池免拨号）。 |
| 查询很慢 breaker 却从不跳闸 | 符合设计——resilience 仅建连层；慢而连通的命令它看不见 | 改为对 `db.client.operation.duration` 告警；见 §2.2。 |
| 命令正常但无 span/metric | 未导入 starter-otel——monitor 搭乘 OTel 全局 | `_ "go-spring.org/starter-otel"` + `spring.observability.*`。 |
| 注入 `*mongo.Client` 失败 | bean 是 wrapper `*StarterMongoDB.Client` | 注入 wrapper 类型；驱动方法原样提升。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 14 个实例 key + tls 组 |
| 其中必填 | 1（`uri`） |
| quickstart 前置外部依赖 | 1（MongoDB） |
| "注意/坑" 条数 | 4（directConnection、焐热池绕过、username 门控凭据） |

设计嫌疑清单（审计台账；保留上一版条目，新增标 NEW）：

- `service-name` 静默关闭驱动拓扑发现，还需用户配合 `directConnection=true`——starter
  无法替用户注入（URI 不透明）。
- 健康指示器无关闭开关（家族不对称：redigo 有 `health.enabled`）。
- resilience 仅建连层；期待逐命令 breaker 语义（如 starter-go-redis）的用户对焐热池
  的命令失败得到的是静默不保护。2026-08-28 守卫统一化复核确认这是**命令级 SDK 阻塞**：
  v2 驱动唯一的逐命令 hook 是 `event.CommandMonitor`，只能观测（事件无拒绝能力）；
  内部 `driver.Deployment` seam 在驱动包外无法构造；`ClientOptions` 也没有命令执行器
  覆盖点。因此建连层 + 命令监视器（观测）就是可达的最深接缝；若 v2 驱动未来出现
  可拒绝的 hook 再升级。
- （NEW）健康指示器 Provide 依赖 `gs.TagArg(name)` 按名取 wrapper——今天正确，但再出现
  一个 Client 类型 bean 家族会让该 tag 产生歧义。
