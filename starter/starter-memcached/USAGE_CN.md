# starter-memcached 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`client.go`、`command.go`、`driver.go`、`config.go`、`health/health.go`、
`bytecache/bytecache.go`）与可运行的 [example/](example/)（`check.sh` 拉起 docker memcached
并自证 SET/GET/INCR 往返）。**gomemcache 自身语义（分片、文本协议、Item 字段）见
[gomemcache 文档](https://github.com/bradfitz/gomemcache)** —— 下文只写 go-spring 的增量。

**激活条件**：只有存在任意 `spring.memcached.*` key 时才有 bean —— `gs.OnProperty("spring.memcached")`
是前缀匹配（`starter.go:42`）。仅多实例：每个 `spring.memcached.<name>` map 项生成一个命名
client bean 加一个健康指示器；没有 `__default__` 单例。

---

## 1. 完整工程示例

文件树（对应 `example/`）：

```
demo/
├── go.mod
├── main.go
├── service.go
├── discovery.go        # 注册 discovery 后端（仅使用 service-name 时需要）
└── conf/
    └── app.properties
```

**前置依赖**（唯一外部依赖）：

```bash
docker run -d --name demo-memcached -p 127.0.0.1:11211:11211 memcached:1.6
```

**go.mod**：

```
require (
    github.com/bradfitz/gomemcache/memcache latest
    go-spring.org/spring                    v1.3.x
    go-spring.org/starter-memcached         latest
    go-spring.org/starter-actuator          latest   // 可选：/readiness 折入 memcache 健康
    go-spring.org/starter-governance        latest   // 可选：memcached 操作的 resilience/fault
)
```

**main.go**：

```go
package main

import "go-spring.org/spring/gs"

func main() { gs.Run() }
```

**service.go** —— 注入包装类型而非裸 client（见 §2.3）：

```go
package main

import (
    "github.com/bradfitz/gomemcache/memcache"
    "go-spring.org/spring/gs"
    StarterMemcached "go-spring.org/starter-memcached"
)

type Service struct {
    // autowire tag = spring.memcached.<name> 的 map key
    Memcached    *StarterMemcached.Client `autowire:"cache"`
    SessionCache *StarterMemcached.Client `autowire:"session"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}
```

**discovery.go** —— 仅 `service-name` 寻址方式需要（此处为静态后端；真实后端对接
Consul/Nacos 并推送新快照）：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:11211", Healthy: true}))
}
```

**conf/app.properties**（与 `example/conf/app.properties` 同面）：

```properties
spring.memcached.cache.servers=127.0.0.1:11211

spring.memcached.session.servers=127.0.0.1:11211
spring.memcached.session.timeout=100ms
spring.memcached.session.max-idle-conns=4

# 服务发现寻址：不配 `servers`；列表来自已注册后端。
spring.memcached.discovery.service-name=memcached-cluster
```

**验证**（example 在 :9090 暴露同样 handler；`go run . -manual` 运行）：

```bash
curl http://127.0.0.1:9090/set     # -> OK
curl http://127.0.0.1:9090/get     # -> value
curl http://127.0.0.1:9090/incr    # -> 1, 2, 3 ...
curl http://127.0.0.1:9090/get     # 删除后："memcache: cache miss"
```

或无头方式：`cd example && ./check.sh` —— 自证 SET/GET/INCR、删除后 miss、discovery client
往返，任一失败即非零退出。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-memcached
  ├─ init: RegisterDriver("DefaultDriver", ...)                     driver.go:35
  └─ init: gs.Module(OnProperty("spring.memcached"), BindEach)       starter.go:42-43
        对每个 spring.memcached.<name> 项：
          r.Provide(newClient, IndexArg(name,c))                     starter.go:46-50
              .Name(name).Init((*Client).Init).Destroy((*Client).Destroy)
          r.Provide(健康指示器 "memcache:"+name)                      starter.go:53
gs.Run()
  ├─ 配置绑定：${spring.memcached.<name>} → Config（value tag）       config.go:24-61
  ├─ newClient：校验、CreateClient、启动 PING（fail-fast）           starter.go:79-101
  ├─ gs 向包装 bean 字段注入 Observability                            client.go:50
  ├─ Client.Init：observer + resilience/fault executor               client.go:71-76
  ├─ 就绪：健康指示器把 Ping 折入 /readiness                          health/health.go:33-36
  └─ 停机：Client.Destroy —— 释放 executor、停 discovery watch        client.go:83-90
```

启动 PING 时机：它在**构造函数内部**执行、bean 尚不存在 —— 服务器不可达会以
`memcached: startup ping failed` 中止容器装配（`starter.go:98-100`）；不是懒加载、不重试。
gomemcache 的 `Ping` 探测全部已配置服务器，`servers` 里一个死节点即令整个实例失败。

### 2.2 服务发现寻址流程

设置 `service-name`（且非 mesh 模式）时，`DefaultDriver.CreateClient` 用 `discovery` 指名的
后端（默认 `"default"`）、按 `scheme` 过滤构建 `discovery.Resolver`（`driver.go:70`、
`driver.go:111-112`）。**仅初始快照**成为 client 的 server 列表：空快照使启动失败并报
`memcached: discovery returned no endpoints for %q`（`driver.go:75-79`）。Resolver 的后台
Watch 仅用于持有 watch 生命周期 —— **成员变更不会热应用**，因为 gomemcache 在创建时把 key
哈希到固定 server 集（`driver.go:60-66`、`driver.go:90-96`）。集群成员变化需要重启——没有动态成员机制；若拓扑动态扩缩，请把 `servers` 指向 serverless/代理类端点（单一稳定地址），由代理层管理成员。mesh
模式下完全跳过 discovery，`servers` 原样使用（sidecar 负责发现+LB，`driver.go:69-70` 注释）。

### 2.3 一次 Set 调用逐层走读

`Client.Set(item)`（`command.go:70-75`）：

1. `instrument("set", item.Key)` 开启 observe span。gomemcache API 无 context，span 是用
   `context.Background()` 的**根 span** —— 不与调用方请求 trace 关联（`command.go:38-40`，
   局限 documented 于 `client.go:37-41`）。
2. `guardErr` 经 `resilience.Run` 在 resilience executor 下执行操作（`command.go:174-181`）：
   limiter/breaker 以资源 `memcached:<instance-name>` 隔离（`client.go:73`）；
   `memcache.ErrCacheMiss` 计为成功，miss 不会触发熔断（`command.go:168`）；executor 先包
   fault（`fault.WrapExecutor`，`client.go:74`）再包 observe（`resilience.WrapExecutor`，
   `client.go:75`）。governance 关闭时为透明 no-op。
3. 内嵌的 `*memcache.Client` 执行实际写入；end 回调以错误收尾 span。

全部 17 个操作（get/get_and_touch/get_multi/touch/set/add/replace/append/prepend/cas/delete/
delete_all/increment/decrement/ping/flush_all）同构（`command.go:45-162`）。未覆写的方法
（生命周期里只有 `Close`）从内嵌 client 原样提升。

### 2.4 starter-cache 桥

第二个 `init` 向 starter-cache 注册 `"memcached"` driver（`starter.go:66-73`）：
`spring.cache.<name>.driver = memcached:<memcached实例名>` 经 `bytecache.NewByteCache` 暴露
类型化 `cache.Cache`（`bytecache/bytecache.go:33-36`）。TTL 转换：`toExp` 把 ttl 映射为
int32 秒 —— **0/负值 = 永不过期**，亚秒向上取整为 1s，避免被静默当成 forever
（`bytecache/bytecache.go:42-50`）。`GetBytes` 把 `ErrCacheMiss` 映射为 `cache.ErrMiss`；
删除不存在的 key 不算错（`bytecache/bytecache.go:55-79`）。

---

## 3. 逐 key 行为参考

starter 共 9 个 value tag（grep 审计核实；输出中的 `demo.label` 属于 example 应用而非
starter）。

| key（`spring.memcached.<name>` 下） | 类型 | 默认值 | 行为与联动 | 配错后果 |
|---|---|---|---|---|
| `servers` | []string | 空 | 静态 server 列表；请求按其分片（config.go:28）。与 `service-name` 二选一 | 两者皆空 → 构造错误 `one of servers or service-name must be set`（starter.go:83）；地址死 → 启动 ping fail-fast |
| `service-name` | string | 空 | 服务发现寻址：经 `discovery` 指名后端解析 server 列表（config.go:39）。设置后（非 mesh）忽略 `servers` | 后端缺失 → 启动报 `discovery resolve %q failed`；空快照 → 启动报错（driver.go:76-79） |
| `scheme` | string | 空 | 把 discovery 收窄到单一传输 scheme 的端点；仅在设 `service-name` 时生效（config.go:45） | 过滤过度 → "no endpoints" 启动错误 |
| `discovery` | string | `default` | 用哪个已注册的 `discovery.Discovery` 解析 `service-name`（config.go:50） | 未知后端名 → 启动报错 |
| `timeout` | duration | 0 | 每请求 socket 读/写超时；0 = gomemcache 默认 100ms（config.go:54） | 过低 → 高压下伪超时 |
| `max-idle-conns` | int | 0 | 每 server 保留的空闲连接数；0 = driver 默认 2（config.go:58） | 过低 → 重连抖动 |
| `driver` | string | `DefaultDriver` | 选择已注册 `Driver`（config.go:61）。自定义 driver 用 `RegisterDriver` 注册；重名 panic（driver.go:45-49） | 未知名 → 构造错误 `memcached driver not found`（starter.go:87-88） |
| `observability`（在包装 bean 上，顶层） | ObserveConfig | 空 | 共享 observe kit 的 span/metric/log 配置，由 gs 字段注入（client.go:50）。⚠ 它解析为**顶层共享 key** —— 所有 memcached 实例（及读取同一表达式的其他 starter）共享一份值，无法按实例配置 | 缺省 → 默认 observe 行为 |

无 `resilience` key：resilience/fault 来自治理中心（starter-governance 的 `govern.*` 配置），
按资源 `memcached:<instance-name>` 隔离。

---

## 4. 验证与故障演练

1. **SET/GET 往返**：`curl :9090/set` → `OK`；`curl :9090/get` → `value`
   （handler 见 `example/example.go`；check.sh 无头断言）。
2. **cache-miss 语义**：删除 key 后 `curl :9090/get` → `memcache: cache miss`；开 governance
   时反复 miss **不会**熔断（ErrCacheMiss 计为成功，`command.go:168`）。
3. **server-down fail-fast**：停掉 memcached（`docker stop demo-memcached`）再启动应用 →
   容器装配以 `memcached: startup ping failed` 中止（`starter.go:98-100`）。坏 `servers`
   地址表现相同 —— 这是 fail-fast 姿态，没有懒模式。
4. **discovery 坏地址演练**：设 `service-name` 但未注册后端 → 启动报
   `memcached: discovery resolve %q failed`（`driver.go:71-72`）。注册的后端返回空端点集 →
   启动报 `discovery returned no endpoints`（`driver.go:78`）。
5. **成员变更局限演练**：discovery 实例运行中，从后端快照移除端点 —— client 持续拨旧地址
   直到重启（§2.2）。这是 documented 的 gomemcache 约束，不是接线 bug。
6. **健康/就绪**：import starter-actuator；每个实例贡献名为 `memcache:<name>` 的指示器，
   探测即一次真实 `Ping`（`starter.go:53`、`health/health.go:34-36`）。杀掉服务器后
   `curl :9370/readiness` 翻 DOWN。注意探测无 deadline —— gomemcache 的 `Ping` 无 context，
   由 client 超时兜底（`health/health.go:30-32`）。
7. **多实例**：`cache` 与 `session` 实例并存（bean 名 = map key，`starter.go:47-50`）；两个
   项指向同一服务器也是独立 bean、独立 executor（资源标签 `memcached:cache` 与
   `memcached:session`）。
8. **可观测**：import starter-otel 后，每个操作出现名为 `memcached.get`/`memcached.set`/...
   的 span（根 span，§2.3）；时长/在飞指标与访问日志来自共享 observe kit
   （`observe.NewDB("memcached", ...)`，`client.go:72`）。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|---|---|---|
| 启动报 `one of servers or service-name must be set` | 实例块两者皆未配 | 配其一（`starter.go:83`） |
| 启动报 `memcached: startup ping failed` | 服务器宕机/启动期地址错误 | 启动 memcached、修 `servers`；ping 是 fail-fast（`starter.go:98-100`） |
| 启动报 `memcached driver not found: X` | `driver` 指向未注册 driver | 在 `init` 里 `RegisterDriver(X, ...)`，或删掉该 key（`starter.go:87-88`） |
| 启动报 `discovery resolve "..." failed` | 设了 `service-name` 但 `discovery` 名下无后端 | 启动前注册后端（`discovery.RegisterDiscovery`） |
| 启动报 `discovery returned no endpoints` | 后端健康但服务无实例（或 `scheme` 过滤过度） | 拉起实例/清空 `scheme`（`driver.go:75-79`） |
| 集群扩缩容后 server 列表不更新 | gomemcache 创建即固定 server 集；watch 仅管生命周期 | 重启进程重新解析（`driver.go:60-66`） |
| trace 里 memcached span 与请求 trace 断联 | gomemcache API 无 context；span 为根 span | 已知局限（`client.go:37-41`）；按 key/时间关联 |
| 熔断在 cache miss 下永不触发 | 设计如此：ErrCacheMiss 计为成功 | 熔断演练须用真实故障而非 miss（`command.go:168`） |
| `RegisterDriver` panic `already registered` | 两个 init 注册同名（如自定义 driver 命名 `DefaultDriver`） | 改名（`driver.go:45-49`） |
| readiness 持续 UP 但操作失败 | 指示器只探 `Ping`；慢而活着的服务器照样通过 | 看 observe 指标的真实时延/错误 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|------|
| 配置 key | 9（7 连接 + 1 共享 observability + 1 经 cache 桥命名） |
| 必填 | 1（`servers` 与 `service-name` 二选一） |
| quickstart 前置外部依赖 | 1（memcached，docker） |
| "注意/坑"条数 | 4 |

设计嫌疑清单（沿自上一版，已更新）：

- ~~无健康指示器~~ —— **已解决**：每个实例现已注册导出的 `health.Indicator`
  `memcache:<name>`（`starter.go:53`、`health/health.go:33-36`）；配合 starter-actuator
  自动折入 `/readiness`，无需额外接线。
- `observability` 解析为顶层共享 key —— 所有实例共享一份配置，无法按实例调可观测
  （`client.go:50`）。跨家族嫌疑（沿自上一版）。
- discovery watch 仅管生命周期：成员变更需重启（gomemcache 结构性约束，`driver.go:60-66`）
  —— 可考虑 rebuild seam 或 client 原子替换模式（参照 dubbo 动态超时的 swap 方案）。
- span 为根 span（gomemcache API 无 context）—— trace 关联弱于 redis/gorm starter
  （`client.go:37-41`）。
