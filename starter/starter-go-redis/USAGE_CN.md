# starter-go-redis 使用说明 — 参考手册

详细使用文档。概览见 [README.md](README_CN.md)。所有行为声明均已对照源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health/health.go`、
`bytecache/bytecache.go`）与可运行的 [example/](example/) 核实——下文括号内为 file:line 抽查点。
**Redis 语义与 go-redis API 见 [go-redis 官方文档](https://redis.io/docs/latest/develop/clients/go/)**
——本文只写 go-spring 的增量。

**激活条件**：任一 `spring.go-redis.*` key（模块为 `OnProperty("spring.go-redis")` 前缀匹配）。
每个 `spring.go-redis.<name>` 条目创建一个名为 `<name>` 的 `*StarterGoRedis.Client` bean，
并附带名为 `redis:<name>` 的健康指示器。

---

## 1. 完整工程示例

一个服务内演示三种拓扑 + cache 门面 + 探针/指标/trace。文件树：

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
    github.com/redis/go-redis/v9   latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-go-redis latest
    go-spring.org/starter-cache    latest   // cache 门面（spring.cache.*）
    go-spring.org/starter-actuator latest   // 可选：readiness + /metrics
    go-spring.org/starter-otel     latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance latest // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— 注入包装类型，使用完整 go-redis 命令面，并演示 cache driver：

```go
package service

import (
    "context"

    "go-spring.org/cloud/data/cache"
    "go-spring.org/spring/gs"
    StarterGoRedis "go-spring.org/starter-go-redis"
)

type Service struct {
    // 始终注入包装类型 *StarterGoRedis.Client。它内嵌 redis.UniversalClient，
    // Get/Set/Incr/Pipeline/PoolStats 原样提升，single/sentinel/cluster 通吃。
    Main     *StarterGoRedis.Client `autowire:"main"`     // single
    Sentinel *StarterGoRedis.Client `autowire:"sentinel"` // sentinel（仍是 *redis.Client）
    Cluster  *StarterGoRedis.Client `autowire:"cluster"`  // cluster（*redis.ClusterClient）

    // spring.cache.main.driver=go-redis:main 暴露的 cache 门面。
    // 注意：bean 名取自 REDIS 实例名（"main"），不是 spring.cache 的 map key
    // ——见 starter-cache 的 USAGE §3。
    Cache *cache.Cache `autowire:"main"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            _ = s.Main.Set(ctx, "key", "value", 0).Err()
            var v string
            _ = s.Main.Get(ctx, "key").Scan(&v)
            _ = s.Cache.Set(ctx, "user:1", map[string]string{"name": "demo"}, 0)
        }
    })
}
```

**conf/app.properties** —— 上面用到的完整配置面：

```properties
# --- single ---------------------------------------------------------------
spring.go-redis.main.addr=127.0.0.1:6379
spring.go-redis.main.pool-size=20
spring.go-redis.main.conn-max-lifetime=2m

# --- sentinel：经 sentinel 节点解析 master 组 ------------------------------
spring.go-redis.sentinel.mode=sentinel
spring.go-redis.sentinel.master-name=mymaster
spring.go-redis.sentinel.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380

# --- cluster：种子节点；客户端自行学习全量拓扑 ------------------------------
spring.go-redis.cluster.mode=cluster
spring.go-redis.cluster.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
spring.go-redis.cluster.route-by-latency=true

# --- cache 门面：把 "main" 暴露为带类型的 cache.Cache bean ------------------
spring.cache.main.driver=go-redis:main

# --- 可观测 -----------------------------------------------------------------
# 访问日志详略（tag _app_redis_access）。注意：observability.* 是顶层 key，
# 全部 client starter/实例共享（wrapper 字段按绝对引用绑定 ${observability:=}），
# 不在 spring.go-redis.<name> 之下。默认 "brief"；"detailed" 附带命令+key。
observability.level=detailed
# redisotel 的 span/连接池指标默认开启，挂在 starter-otel 的全局管线上。

# --- actuator + otel ------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**验证**（先起 Redis：`docker run -d -p 6379:6379 redis`；要用全三种拓扑则用
[example/docker-compose.yml](example/docker-compose.yml)）：

```bash
go run .                          # 任一拓扑不可达则启动期 fail-fast
curl -s :9370/readyz              # readiness 汇入 redis:main / redis:sentinel / redis:cluster
curl -s :9370/metrics | grep -E 'redis'   # redisotel 连接池指标
grep _app_redis_access app.log | tail -3  # 每条命令一条访问记录
redis-cli GET key                 # "value"——经 hook 链写入
redis-cli GET user:1              # 经 cache 门面写入的 JSON
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-go-redis
  └─ 任一 spring.go-redis.* key 存在时 gs.Module(OnProperty("spring.go-redis")) 触发
        └─ conf.BindEach("${spring.go-redis}") → 每个 <name> 条目一份 Config
              ├─ mode single/sentinel → Provide(newClient).Name(<name>)
              │                          .Init((*Client).Init).Destroy((*Client).Destroy)
              ├─ mode cluster          → Provide(newClusterClient).Name(<name>)（同一包装类型）
              └─ Provide 名为 "redis:<name>" 的 health.Indicator（由 health.enabled 控制，默认开）

gs.Run()
  ├─ 构造 newClient [starter.go:97]：validateConfig → 查 driver → driver.CreateClient
  │   → instrument()（redisotel tracing+metrics，由 otel.* 开关门控）
  │   → failFastPing（无条件执行，上限 dial-timeout 或 5s）[starter.go:218]
  ├─ gs 字段注入 Client.Observability（${observability:=}）
  ├─ Init [client.go:58]：resourceLabel → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │   → resilobserve.WrapExecutor → applyObservability（访问日志 hook）
  │   → AddHook(resilienceHook)——命令链装配完成
  ├─ 就绪：探针翻转 UP（指示器执行 client.Ping）
  └─ SIGTERM → Destroy [client.go:82]：exec.Close → 停 discovery watch → client.Close
```

mode 配错或启动 ping 失败都会导致启动失败——进程不会带着一个死 Redis 进入"服务中"状态。

### 2.2 命令 hook 链 —— 精确顺序与理由

go-redis 的 hook 是 FIFO：先加的在最外层。装配顺序：

```
redisotel（span + 连接池指标）→ observeHook（访问日志）→ resilienceHook（熔断/...）
→ go-redis 核心 → 网络
```

理由（源码注释 [client.go:63-73]、[command.go:17-27]）：

- **redisotel 最外层**：在构造期由 `instrument()` 添加，早于 Init 加的其余层。span 因此
  覆盖 starter 加的全部层，访问日志也借用 redisotel 的 span 上下文做 trace_id 关联。
- **observeHook 在熔断器之外**：一条访问日志覆盖整个重试循环——记录的是最终结果而非
  每次尝试。它以 `WithoutTraceAndMetric()` 构建 [command.go:112]，不重复发 span/指标
  （redisotel 已负责），只补日志缺口。
- **resilienceHook 最内层**：保护决策贴近网络；其拒绝正是外层随后要观测的对象。
- 两个 hook 的 `DialHook` 都不动——建连是 discovery 的职责，不是命令级保护 [command.go:39-41]。

### 2.3 一条命令逐层走读：miss 场景下的 `GET user:1`

1. redisotel 开 client span（无 starter-otel 时为 no-op）。
2. observeHook 开一条名为 `get` 的访问日志记录（cmd.FullName()）。
3. resilienceHook 向 executor 申请许可（限流/熔断作用域是 resource 标签，如
   `redis:127.0.0.1:6379`——按实例而非按命令 [client.go:94-100]）。
4. go-redis 执行；key 不存在，返回 `redis.Nil`。
5. `run()` 通过 nil-as-success 谓词把 `redis.Nil` 判为成功 [command.go:94]——
   **cache miss 永不触发熔断**，也不会为此重试。
6. observeHook 以 `nilAsSuccess(err)` 结束记录 [command.go:147]——miss 记为成功操作而非错误。
7. redisotel 结束 span；调用方拿到的仍是普通 go-redis 那样的 `redis.Nil`。

**pipeline** 场景下 resilienceHook 把整批包进一次 executor 运行；只有当批次根本没执行
（被拒绝或注入故障）时才对每条命令 `cmd.SetErr`——真实的逐命令错误已由 go-redis 记录，
不会被覆盖 [command.go:84-90]。

### 2.4 服务发现寻址与连接回收（single 模式）

设置 `service-name` 后，DefaultDriver 替换 go-redis 的 dialer：每次新建连接都调用
`resolver.Pick()` 取一个存活端点 [driver.go:141-147]，resolver 后台保鲜端点集。配合
`conn-max-lifetime`（默认 2m），连接**无需重建客户端**就能换到更新后的地址——这也是默认值
取较短 2m 而非"无限"的原因 [config.go:95-97]。此时 `addr` 不生效——两者都配置时启动会打 WARN 点名被忽略的 `addr`（example 故意配 dummy
`0.0.0.0:0` 来证明这点）。sentinel/cluster 模式下设置 `service-name` 会在启动期被拒绝：
这两种拓扑自己发现节点 [starter.go:170-194]。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.go-redis.<name>.` 下——这里是 `conf.BindEach` 的实例前缀绑定
（不是 starter Pool 的绝对属性引用规则）。

### 3.1 拓扑与寻址

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `mode` | string | `single` | `single`/`sentinel` → 内嵌 `*redis.Client`；`cluster` → `*redis.ClusterClient`。其他值 BindEach 报 "invalid mode"。 | 拼错 → 启动报错并指名实例。 |
| `addr` | string | — | single 模式目标。⚠ single 模式下 `addr` / `service-name` 至少其一（RequireAny）。 | 都缺 → 启动报错；都配 → service-name 生效，启动 WARN 点名被忽略的 `addr`。 |
| `master-name` | string | — | sentinel 模式必填。 | 缺失 → 启动报 "master-name and sentinel-addrs are required"。 |
| `sentinel-addrs` | list | — | sentinel 模式必填。 | 同上。 |
| `sentinel-password` | string | — | sentinel 节点自身的认证（区别于 `password`）。 | sentinel 开 ACL → 启动探测报错。 |
| `addrs` | list | — | cluster 种子节点；仅种子，客户端自学习拓扑。cluster 必填。 | 缺失 → 启动报 "addrs is required in cluster mode"。 |
| `max-redirects` | int | 0 | cluster 的 MOVED/ASK 跟随次数；0 → go-redis 默认（3）。 | — |
| `route-by-latency` / `route-randomly` | bool | false | cluster 只读路由。 | — |
| `service-name` | string | — | 仅 single：经服务发现解析地址。⚠ 与 sentinel/cluster 组合被拒。 | 组合错 → 启动报错；见 §2.4。 |
| `scheme` | string | — | 把发现端点收窄到单一传输 scheme。仅在 service-name 设置时生效。 | — |
| `discovery` | string | `default` | 用哪个已注册的 discovery 后端。 | 后端未注册 → discovery 启动报错。 |

### 3.2 连接与认证

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `password` / `username` | string | — | 服务端/ACL 认证（sentinel 下为 master）。 | 配错 → 启动期 failFastPing 失败。 |
| `db` | int | 0 | 连接时 SELECT（仅 single/sentinel）。Redis Cluster 无库概念：`mode=cluster` 且 `db != 0` → 启动报 "db is not supported in cluster mode"。 | 越界 → 首条命令报错。 |
| `pool-size` | int | 10 | 最大连接数。 | 过小 → 突发下排队。 |
| `max-idle` | int | 5 | 最大空闲连接（go-redis MaxIdleConns）。 | — |
| `max-retries` | int | 0 | go-redis 命令重试。⚠ resilience 侧重试也要保持 0——双重重试放大时延，且可能重发非幂等命令（config.go:152-154 注释）。 | 调大 + resilience 重试 → 尝试次数相乘。 |
| `dial-timeout` / `read-timeout` / `write-timeout` | duration | 5s / 3s / 3s | 直传；dial-timeout 同时限定启动 ping [starter.go:229]。 | — |
| `conn-max-lifetime` | duration | 2m | 连接复用窗口；较短值利于发现流量切换。 | 很大 + discovery → 老端点连接滞留。 |
| `tls.*` | group | off | `tlsconf` 客户端 TLS（enabled/ca-file/cert-file/key-file/server-name/insecure-skip-verify）。 | 配一半 → `tls.Build` 启动报错。 |
| `driver` | string | `DefaultDriver` | 选择已注册 Driver。 | 未知名 → 启动报 "redis driver not found"。 |
| `health.enabled` | bool | true | 为实例注册 `redis:<name>` 健康指示器——与 starter-redigo 同名开关。 | false → 该实例无指示器 bean，不再上报就绪。 |

### 3.3 观测

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `otel.tracing.enabled` | bool | true | 挂 redisotel span。无 starter-otel 时为 no-op。 | 关掉又期待 trace → 静默无告警。 |
| `otel.metrics.enabled` | bool | true | 挂 redisotel 连接池/命中指标。同上。 | — |
| `observability.level` | string | `brief` | 访问日志：`off` / `brief` / `detailed`（附带命令+key）。 | `off` 只静默日志；trace/metric 照发。 |
| `observability.maxArgBytes` | int | 512 | detailed 模式参数截断上限。 | 过小 → 参数被截断。 |
| `observability.skipOps` | list | — | 对列出的操作名同时抑制 span+metric+log。 | — |

### 3.4 cache driver 引用语法

`spring.cache.<name>.driver = go-redis:<redis-实例名>` 注册一个 `*cache.Cache` bean
（包 `bytecache.NewByteCache(c.UniversalClient)`），**bean 名取自 redis 实例名**
[starter.go:80-89]。`redis.Nil` 在该边界被映射为 `cache.ErrMiss`
[bytecache/bytecache.go:42-51]。

---

## 4. 验证与故障演练

### 4.1 经 actuator 验证健康

```bash
curl -s :9370/readyz | jq .      # components 含 "redis:main"、"redis:cluster"...
docker stop <redis>              # 指示器跑 client.Ping → 组件翻 DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <redis>
```

### 4.2 验证访问日志与指标

```bash
redis-cli SET probe 1
grep _app_redis_access app.log | tail -1
# brief：system=redis op=set status ok duration=...；detailed 附 "set probe"
curl -s :9370/metrics | grep -E 'redis.*pool|hits'   # redisotel 指标
```

### 4.3 验证发现地址回收

```properties
spring.go-redis.main.service-name=redis-cluster
spring.go-redis.main.conn-max-lifetime=30s
```

扩缩/迁移后端实例；在 conn-max-lifetime 内新连接即拨到更新端点（每次拨号 resolver.Pick）。
观察 `PoolStats()`（TotalConns/Hits）或 redisotel 连接池指标确认无需重启的回收。

### 4.4 验证 cache driver 接线（门面 SET，裸客户端 GET）

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)   // 带类型，JSON codec
val, _ := s.Main.Get(ctx, "k").Result()       // 裸客户端读同一 key："v"（JSON 引号字符串）
```

门面存的是 JSON 字节（`cache.Cache` 默认 codec）——裸 GET 拿到编码后的形态。门面 miss
返回 `cache.ErrMiss`，不是 `redis.Nil`。

### 4.5 故障/弹性演练

配置 starter-governance 后给资源 `redis:<addr>` 设熔断/限流策略；压测并观察拒绝如何出现在
`_app_redis_access` 记录与 resilience observer 的 outcome 计数里。策略可运行时热切换——
executor 无需重启即刷新。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动报 "startup ping failed" | 地址不可达/密码错/TLS 不匹配 | failFastPing 无条件执行；修连通性或凭据。 |
| 启动报 "invalid mode ... (want single/sentinel/cluster)" | `mode` 拼错 | 改正——mode 精确匹配。 |
| 启动报 "service-name is not supported in sentinel/cluster mode" | 发现与自发现拓扑组合 | 删 service-name；sentinel/cluster 自行发现节点。 |
| 启动报 "redis driver not found" | `driver` 指向未注册名 | 在 init 里 `StarterGoRedis.RegisterDriver`，或用 DefaultDriver。 |
| 启动报 "db is not supported in cluster mode" | Cluster 无 database select | 删掉 `db`（cluster 只有 0 号库）。 |
| 启动 WARN "addr ... is ignored" | single 模式同时配了 `addr` 和 `service-name` | 无害；删 `addr` 或留着当标签——寻址归服务发现。 |
| 命令正常但健康 DOWN | 指示器带 ctx ping；查 ACL/只读副本 | 看 /readiness 里组件的错误详情。 |
| 注入的 bean 无 span/指标 | 未引入 starter-otel | redisotel 挂 OTel 全局；补 import。 |
| 没有访问日志 | `observability.level=off`，或日志 tag 被过滤 | 设 `detailed`；检查 `_app_redis_access` 的 logger 配置。 |
| 怀疑 GET miss 触发熔断 | 不会——redis.Nil 判为成功 [command.go:94] | 找真实后端错误；miss 已排除。 |
| cache bean 注入失败 | 门面 bean 名取自 redis 实例名而非 spring.cache key | 按 `<redis-实例名>` 注入；见 starter-cache USAGE。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 实例 25 个 + tls 组 + otel(2) + observability(3) |
| 其中必填 | 每种模式 1 组（addr/service-name、master-name+sentinel-addrs 或 addrs） |
| quickstart 前置外部依赖 | 1（Redis） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单：~~健康指示器无关闭 key~~（已修：`health.enabled` 与 redigo 对齐）；cache 门面
bean 名取后端实例名而非 `spring.cache` map key（注入名反直觉；同一实例被两个 spring.cache
条目引用会撞名）；`max-retries`（go-redis）与 resilience 重试的双重重试隐患仅写在配置注释里。
