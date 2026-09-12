# 治理中心配置指南

本文讲**怎么写治理配置**。治理配置是**它自己的一份文档**——本地规则文件（本模块的 file source 盯着它）、远程控制台，或配置中心——经 `governance.Source` 契约进入治理中心。它**不写进 `app.properties`**：改一条规则只刷新治理，不触发全应用的属性重绑。

设计原理（为什么是 Source 契约、per-label Register 怎么分发）见 [DESIGN_CN.md](./DESIGN_CN.md)；资源标签（label）格式表见 [DESIGN_CN.md §6](./DESIGN_CN.md#6-资源标签label约定)。

适用于一个项目里**同时用多个 starter**（redis + gorm + http-client + gin 入站……）的场景。

---

## 1. 前置：引入 starter-governance + 指定规则来源

治理不是自动生效的，两步：

1. 程序入口 blank import `starter-governance`——它注册接线 bean，把规则来源交给治理中心并启动它。
2. 在 `app.properties` 里用**一个引导 key** 告诉它规则从哪来（本地文件为例）：

```properties
# app.properties —— 治理的“引导”配置，只有这一行
# 规则内容不在这里，在 conf/govern.properties
govern.source.file.path=conf/govern.properties
```

```go
import (
    _ "go-spring.org/starter-governance" // 启动时接线治理中心，装载规则来源
    _ "go-spring.org/starter-redigo"     // 你的业务 starter
    // ... 其他 starter
)
```

> **client starter 不需要 import（也不注入）治理。** 每个 client（redis/gorm/http/…）只调中立函数 `resilience.ExecutorFor(系统名, 资源label)` 拿到它的 executor——不知道治理的存在。`governance.Source` 契约把治理核心和“规则从哪来”解耦，因此控制台推流、专用配置中心 listener、静态注入都能驱动它，而本地文件只是其中一种。

**不 import starter-governance 时**：没有 provider 注册，`ExecutorFor` 返回透传的 noop executor，resilience 完全旁路（直连后端），不会报错。所以“没配治理”和“不能用 starter”是两回事。

**其它规则来源**：

- `govern.source.http.*`（轮询远程控制台/规则 API，见 [starter-governance README](../../starter/starter-governance/README.md)）；
- config 中心的 source 适配器（nacos/etcd）各自独立成模块（`starter-governance-nacos`、`starter-governance-etcd`），内容同样是这份 `govern.*` 文档；
- 代码里 `governance.SetSource(...)` 静态注入或推流。

优先级：显式 `governance.SetSource` > 由 bean 注入的 source（如上面 file/http 的）。

**例外：两个直接走门面的消费方**（center 类型本身不导出，它们只用门面函数）：

- **starter-dubbo**：走自己的 URL-param 治理模型（timeout/retries 是 dubbo 参数，不走 resilience executor），所以直接调 `governance.PolicyFor` 读策略字段。
- **starter-gateway**：它的路由池是**每次路由表重编译重建**的，订阅必须能随旧池一起撤销——用 `governance.Register` 拿回 `Subscription`，重建时 `Cancel()`。保护策略仍走中立的 `resilience.ExecutorFor` seam（同其它 client），只有**端点选择**这一半走门面，因为选择策略没有对应的中立 seam，为单一实现硬造一个不划算。

---

## 2. 最小可用：一份默认策略管全部

最常见用法——全进程所有资源共享同一份韧性策略。规则写在**规则文件**里（下面是 `properties`，YAML 键同构）：

```properties
# conf/govern.properties —— 治理规则，独立文档
govern.enabled=true
govern.driver=default          # 或 sentinel；全进程统一后端，一处切换处处生效

govern.default.enabled=true
govern.default.timeout=500ms
govern.default.max-retries=1
govern.default.rate-limit=100       # ops/s，0 表示不限流
govern.default.error-threshold=20   # 连续失败 20 次熔断
govern.default.open-duration=5s     # 熔断持续 5s 后半开试探
```

配完这份文件（加上 §1 的 `govern.source.file.path`），项目里的 redis、gorm、mongo、http-client……全部自动套用这套超时/重试/限流/熔断，且**热重载**——file source 盯着这个文件，改完不用重启（远程 source 同样的效果）。

> 规则文件里的键就是 `govern.*` 命名空间，与过去的 `${govern}` 属性一字不差。`govern.default.*` 下可用字段是 `resilience.Config` 的全部旋钮：`timeout` / `max-retries` / `rate-limit` / `burst` / `error-threshold` / `open-duration` / `breaker-strategy`(consecutive|error-rate) / `error-rate-threshold` / `min-requests` / `breaker-window`，以及端点选择类的 `balancer` / `outlier-threshold` / `outlier-suspend-for`（见 §3.1）。字段含义见 [cloud/governance/resilience/config.go](resilience/config.go)。

---

## 3. 多 starter 项目：给不同资源配不同策略

真实项目里 redis 和 gorm 的容忍度不一样。用 `govern.rules[N]` 给特定资源单独配——**资源 label 写在 `resources` 值里**（不是 key），所以冒号随便写、properties 不转义、YAML 不加引号：

```properties
# conf/govern.properties
govern.enabled=true
govern.driver=default

# 默认策略：兜底，大部分资源用这个
govern.default.enabled=true
govern.default.timeout=1s
govern.default.max-retries=2

# redis 单独收紧：缓存快失败，少重试
govern.rules[0].resources=redigo:cache
govern.rules[0].enabled=true
govern.rules[0].timeout=200ms
govern.rules[0].max-retries=0

# mysql 放宽：数据库慢查询多，超时给宽
govern.rules[1].resources=gorm:mysql:orders-db
govern.rules[1].enabled=true
govern.rules[1].timeout=3s
govern.rules[1].max-retries=1

# http 下游服务按服务名
govern.rules[2].resources=http:user-svc
govern.rules[2].enabled=true
govern.rules[2].timeout=800ms
```

YAML 规则文件里同样干净（冒号在值里，不是 key）：

```yaml
# conf/govern.yaml
govern:
  enabled: true
  driver: default
  default:
    enabled: true
    timeout: 1s
    max-retries: 2
  rules:
    - resources: redigo:cache
      enabled: true
      timeout: 200ms
      max-retries: 0
    - resources: gorm:mysql:orders-db
      enabled: true
      timeout: 3s
    - resources: http:user-svc
      enabled: true
      timeout: 800ms
```

### 为什么是 `rules[N]` 而不是 `override.<label>`

早期版本曾用 `govern.override.<label>.<field>`，把资源 label 当 map key。但 label 用冒号分段（`gorm:mysql:orders-db`），冒号进到 YAML key 里会让映射解析错乱（`gorm:mysql:orders-db:` 被当成嵌套映射），每个 key 都得加引号、漏一个就静默解析错。所以改成列表形式：**label 退到 `resources` 值的位置**，key 永远是 dot-safe / colon-safe 的数字索引，两种配置格式都自然。

### 3.1 端点选择（负载均衡策略 + 剔除）

除了"调用怎么被保护"，同一个资源的"调用发给谁"也在这份文档里：`balancer` 选策略，
`outlier-threshold` / `outlier-suspend-for` 决定一个反复失败的实例多久被摘出候选集。

```properties
# 发现模式下路由到 user-svc 的 client：改用最少连接，且连续失败 5 次摘除 10s
govern.rules[3].resources=http:user-svc
govern.rules[3].enabled=true
govern.rules[3].balancer=least_conn
govern.rules[3].outlier-threshold=5
govern.rules[3].outlier-suspend-for=10s
```

- **`balancer`**：`round_robin`(默认) / `least_conn` / `consistent_hash` / `weighted` / `zone_aware`（包内还注册了 `random` / `p2c`，公司策略名如 `luohua` 同理）。留空 = 保持该 client 的默认（round_robin）。写错策略名会被**忽略并沿用当前策略**，不影响调用——治理文档没有错误通道（"你推什么，你担保什么"）。
- **`outlier-threshold`**：与 `error-threshold` 是同一套语义（连续失败 + 半开试探），区别在作用对象——`error-threshold` 熔断的是**整个资源**，`outlier-threshold` 摘的是**单个实例**。0 表示不摘除。
- 两个旋钮都**原地生效**：改完 push，下一次请求就走新策略/新阈值，不用重启，也不会重建 transport。（策略自身的状态不跨切换保留——`least_conn` 的在途计数、`consistent_hash` 的哈希环、p2c 的延迟模型都会重来。）

**覆盖到的客户端**：所有走发现模式的 `loadbalance.Pool` 消费者——`http`（starter-http-client）、
`gateway`（每条路由）、`gorm` 全部方言、`redigo`、`redis`（go-redis）、`mongodb`。它们用
**与保护策略相同的资源标签**绑定（`resilience.ResourceLabel`，见 §6 表），所以一条 Rule 同时管住
一条资源的超时/重试/熔断和它的选点策略。

**覆盖不到的客户端，三条理由，都是有意为之**：

- **直连（固定 addr / host）**：没有候选集可选，这条路整体旁路。`neo4j` `memcached`
  `elasticsearch` 等只要不配 `service-name` 就属于这一类。
- **只挑一次的客户端——`neo4j`**：启动时挑一个端点并把主机名固化进 URI，之后没有"每次挑选"，
  订阅了也影响不了任何一次决策。它的治理只到保护策略为止。
- **成熟客户端自持选择的——`elasticsearch`、`memcached`**：选择权在库内部——ES 有自己的节点选择器，
  memcached 的选择就是按 key 做一致性哈希（按 key 亲和正是它的语义，把一个可重排的池套上去会直接
  破坏它）。**这不等于没得配**：这类客户端的选点规则在它自己的配置里（如 ES 的节点选择器、
  `servers` 顺序），govern 不插手——与 MQ/broker 族（kafka/pulsar/rocketmq/nats/mqtt/rabbitmq）
  以及 `s3` 一样，它们的"发给谁"是协议层/集群层的事。判据见 DESIGN_CN 的中立层边界：**库里已经有
  成熟选择器时不自己造一个**。

> **别把"选择权归属"和"地址新鲜度"混成一件事**——它们是两条轴：
>
> | | 谁挑节点 | 地址集跟随命名服务 |
> |---|---|---|
> | `http`/`gateway`/gorm/`redigo`/`redis`/`mongodb` | `loadbalance.Pool`（可配 `balancer`） | ✅ 每次建连/每请求重读 |
> | `elasticsearch` | ES transport 的选择器 | ✅（1s 传播预算，见 starter-elasticsearch USAGE） |
> | `memcached` | 库的按 key 一致性哈希 | ✅ 每次 key 查找重读 |
> | `neo4j` | driver | ⚠️ `bolt://` 不重读；`neo4j://` 靠 `AddressResolver` 在种子主机消失后重找集群 |
>
> 所以"新实例不生效"对 ES/memcached 已经不是问题；`balancer` 对它们仍然无效，因为挑节点的不是我们。

> 一句话自查：**这条 client 每次请求/每次建连会重新挑端点吗？** 会 → 它的资源标签就能配 `balancer`；
> 不会（只挑一次、或交给库内部挑） → 别指望 `govern.rules[N].balancer` 对它生效。

**两个语义边界**（写规则前值得知道）：

- **`grpc` 客户端**的策略选择天然是 gRPC service config（`grpc.WithDefaultServiceConfig(LoadBalancingConfig(s))`）。
  治理的 `balancer` 是**叠在它上面的进程级默认**，标签固定为 `grpc:client`——所以给 `grpc:client` 配
  `balancer=` 会同时改掉**所有**内置 `gs_*` balancer 的策略（与它原本就管的摘除阈值行为一致）。不做
  进程级覆盖时，service config 的选择原样生效；`RegisterBalancer` 注册的自定义名字不受影响（它们存在
  的意义就是保留自己的策略）。
- **DB/缓存客户端的剔除粒度是"连接"而非"查询"**：gorm / redigo / go-redis / mongodb 的挑选发生在
  建连时，能喂给 `Tracker` 的成败信号只有 dial 结果本身。所以这些 client 上 `outlier-threshold` 摘的是
  **反复连不上**的实例；单条查询/命令的失败由它们各自的 resilience executor 管，不参与点数。

### 3.2 让某一条资源不上治理

客户端一律会挂 executor，**没有 per-resource 的治理开关**——开关是进程级的（`govern.enabled`）。
想让某条资源事实上不受治理（裸调用），给它配一条**所有旋钮都为 0 的 Rule** 即可：Rule 是整体替换
default，全零 Rule 算出来的就是零 Policy，executor 退化为透传。

```properties
# kafka:<brokers> 这条 client 不上治理
govern.rules[0].resources=kafka:10.0.0.1:9092
# 字段全不写 = 全零 = 透传
```

> 早期版本里 kafka / mqtt / pulsar / rabbitmq 各有一个 per-instance `${governance:=true}` 开关，
> 已删除：它表达的就是"这条资源不上治理"，而这件事 Rule 已经能表达，且可热更。

### 几条规则

- **一条 Rule 可匹配多个资源**：`govern.rules[0].resources=redigo:cache,redigo:session`（逗号分隔），这几个资源共享同一份策略。
- **Rule 是整体替换，不是字段合并**：给 `redigo:cache` 配了 Rule，它就**完全不继承** `govern.default`——漏写的字段按零值处理（零值=禁用该能力）。只想微调一个字段的话，把 default 里要保留的字段也抄进 Rule。
- **多条 Rule 命中同一 label 时，前面的赢**（first-match）。所以具体的 Rule 放前面。
- **`resources` 留空不匹配任何资源**——兜底请用 `govern.default`，不要用空 resources 的 Rule。

### 怎么知道某个资源的 label 是什么？

查 [DESIGN_CN.md §6](./DESIGN_CN.md#6-资源标签label约定) 的表。标签由 starter 用 `resilience.ResourceLabel(prefix, names...)` 拼接——取第一个非空的 name。所以 label 的取值取决于你配置里填的是 `service-name` 还是 `addr`：

- 你配了 `spring.redigo.instances.cache.service-name=cache-svc` → label 是 `redigo:cache-svc`
- 没配 service-name、只有 `spring.redigo.instances.cache.addr=10.0.0.1:6379` → label 是 `redigo:10.0.0.1:6379`

**建议**：给每个资源配一个稳定的 `service-name`，让 label 可读、不随地址漂移。

---

## 4. 入站流量的治理（gin / grpc）

gin / grpc 是**入站**侧——策略作用在“处理一个进来的请求”上，label 用监听地址：

```properties
# conf/govern.properties
govern.enabled=true
govern.driver=default

govern.default.enabled=true
govern.default.timeout=2s             # 单个请求处理超时
govern.default.rate-limit=1000        # 入站限流 1000 QPS

# gin 监听 :8080 → label = gin::8080
govern.rules[0].resources=gin::8080
govern.rules[0].enabled=true
govern.rules[0].rate-limit=500
```

入站和出站可以共用同一个 `govern.default`，也可以用 Rule 把 API 网关的限流和数据库的超时分开。

### gateway 有两个 label（注意）

gateway 的一条路由对应**两个** label，因为它们作用在两个不同粒度的对象上：

| 作用对象 | label | 说明 |
|---|---|---|
| 保护策略（timeout / retries / 熔断 / 限流） | `gateway:<resilience-policy-name>` | 即 `spring.gateway.resilience.<name>` 的名字。多条路由引用同一 policy **共享** breaker 状态，所以按 policy 而非按路由。 |
| 端点选择（balancer / outlier-*） | `gateway:<route-id>` | 即 `spring.gateway.routes.<id>` 的 id。每个路由有自己的池（各 upstream 的候选集），所以按路由。 |

```properties
# 保护：所有引用 policy "strict" 的路由
govern.rules[0].resources=gateway:strict
govern.rules[0].enabled=true
govern.rules[0].timeout=2s
govern.rules[0].max-retries=1

# 选择：只有路由 api/v1 的上游
govern.rules[1].resources=gateway:api/v1
govern.rules[1].enabled=true
govern.rules[1].balancer=least_conn
```

---

## 5. dubbo 的治理

dubbo 走的是 URL-param 模型，govern 只覆盖它的 `timeout` 和 `retries`（cluster-failover 级别），label 是：

- 应用级：`dubbo:<app-name>`
- 每个 reference：`dubbo:<interface>:<version>:<group>`

```properties
# conf/govern.properties
govern.rules[0].resources=dubbo:com.example.UserService:1.0.0
govern.rules[0].enabled=true
govern.rules[0].timeout=300ms
govern.rules[0].max-retries=2
```

dubbo 专属旋钮（loadbalance / cluster / serialization）不进 govern，留在 dubbo 自己的配置段。

---

## 6. fault（放火）——随 govern 集中化，同一个 source 驱动 ⚠️

fault 注入已**收进治理中心**，和 resilience 共用同一个 `governance.Config`——也就是同一份规则文档（参见 [DESIGN_CN.md §8](DESIGN_CN.md)）。所以放火的 key 是 `govern.fault.*`（不再是顶层 `fault.*`）：

```properties
# conf/govern.properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic
```

这一条会**同时给全进程所有 starter 放火**——redis、gorm、http-client、gin 入站……全部以 50% 概率注入错误。这是集中化的预期效果：starter 侧通过中立的 `fault.InjectorFor()` seam 拿到唯一的进程级 injector，starter 自己不再绑 fault 配置、也不 import 治理中心。

### 想只给某个资源放火

用 `govern.fault.rules[]` 做定向（catch-all 之外的细分）：

```properties
govern.fault.enabled=true

# 默认（catch-all）：不实际注入错误，只让框架进入“fault 模式”
govern.fault.rate=0

# 只给 redis 放火
govern.fault.rules[0].resources=redigo:cache
govern.fault.rules[0].rate=0.5
govern.fault.rules[0].error=timeout
```

> 注意 `govern.fault.rules[N].resources` 里的值要和该 starter 实际传给 injector 的 resource label 对上（client 侧是 `redigo:cache` 这类）。server 侧（gin/grpc/echo/hertz/trpc/dubbo）的 fault 走中间件/拦截器，per-call 解析 injector，label 规则见各 server starter 文档（grpc 的 label 是 `grpc:<FullMethod>`）。

### fault 的安全保险

放火忘了关很危险，fault 内置两个自愈上限：

```properties
govern.fault.max-duration=10m     # 放火 10 分钟后自动停（从第一次生效算）
govern.fault.max-affected=1000    # 累计影响 1000 次调用后自动停
```

**强烈建议**生产环境放火时必设其一，set fire and walk away 也不会烧到天荒地老。

> 注意：集中化后 `max-duration`/`max-affected` 是**进程级计数**（不再是 per-resource）。详见 DESIGN_CN.md §8 的取舍说明。

### fault 与真实流量

用 `govern.fault.scope` 限定只烧压测流量、不碰真实请求（依赖 cloud/governance/traffic 的压测标记）：

```properties
govern.fault.scope=loadtest   # 只给带压测标记的流量放火；真实流量不受影响
                              # 反向：real = 只烧真实流量；空 = 全烧（默认）
```

### 运行时热更

规则文档随 file/http/config-center source push 到中心，**改配置 push 即可在不重启进程的情况下开关 fault**——starter 通过 `fault.InjectorFor()` per-call 惰性解析，中心 `SetConfig` 原地热更。

---

## 7. 一份完整的多 starter 项目配置示例

一个同时用 gin（入站）+ redigo（缓存）+ gorm-mysql（DB）+ http-client（调下游）的项目。**业务配置**留 `app.properties`，**治理规则**独立一份：

```properties
# ============ conf/app.properties：业务 starter 配置 + 治理引导 ============
spring.gin.api.address=:8080
spring.redigo.instances.cache.service-name=cache
spring.redigo.instances.cache.addr=10.0.0.1:6379
spring.gorm.orders.driver=mysql
spring.gorm.orders.dsn=orders:pwd@tcp(10.0.0.2:3306)/orders
spring.http.user.service-name=user-svc
spring.http.user.addr=10.0.0.3:8081

# 治理规则不在这里，只给它指个文件
govern.source.file.path=conf/govern.properties
```

```properties
# ============ conf/govern.properties：治理规则，一处下发，处处生效 ============
govern.enabled=true
govern.driver=default

# 默认策略
govern.default.enabled=true
govern.default.timeout=1s
govern.default.max-retries=1
govern.default.rate-limit=200
govern.default.error-threshold=10
govern.default.open-duration=10s

# redis 收紧：缓存要快
govern.rules[0].resources=redigo:cache
govern.rules[0].enabled=true
govern.rules[0].timeout=100ms
govern.rules[0].max-retries=0

# DB 放宽：慢查询容忍
govern.rules[1].resources=gorm:mysql:orders
govern.rules[1].enabled=true
govern.rules[1].timeout=3s
govern.rules[1].max-retries=2

# fault：默认关，需要时翻开关
govern.fault.enabled=false
# 演练时打开：
# govern.fault.enabled=true
# govern.fault.scope=loadtest
# govern.fault.rules[0].resources=redigo:cache
# govern.fault.rules[0].rate=0.3
# govern.fault.rules[0].error=timeout
# govern.fault.max-duration=5m
```

入口：

```go
import (
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-gin"
    _ "go-spring.org/starter-redigo"
    StarterGormMysql "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-http-client"
)
```

---

## 8. 常见误区

| 误区 | 正解 |
|---|---|
| 把 `govern.enabled` / `govern.default.*` 写进 `app.properties` | 治理规则是独立文档，经 source 进入中心。`app.properties` 里只放 `govern.source.*` 引导 key。 |
| 在每个 starter 自己的配置段写 `resilience.*` | 已废弃。resilience 现在只认治理规则文档，starter 段里的 resilience 配置不生效。 |
| 在 client 自己的配置段写 `balancer` / `suspend-threshold` / `suspend-for` | 已废弃。端点选择也是按资源的治理策略，写进 `govern.rules[N]`（键为 `balancer` / `outlier-threshold` / `outlier-suspend-for`）。 |
| `govern.rules[N]` 只写一个字段想“微调” | Rule 是整体替换 default，漏写字段=禁用该能力。要保留的 default 字段得抄进 Rule。 |
| 用 `govern.override.<label>` 旧写法 | 已改为 `govern.rules[N].resources=<label>`。label 放值里，别再当 key（冒号会废掉 YAML）。 |
| 同时开着 govern 的 `max-retries` 和 client 自己的 retry 旋钮 | **重试次数是相乘的**。客户端级的重试留在客户端（它们的语义不同，见下），所以两边都开 = 双重退避。二选一。 |
| 以为 govern 的 `timeout` 能替代 client 的 `read-timeout` 之类 | 两者管的层次不同：`timeout` 是**单次调用**的整体预算（executor 层），client 的 dial/read/write timeout 是**传输层**的。client 的传输超时留在 client（构造期参数，改不了不用重启的假象）。 |
| 不知道资源 label 是什么 | 配 `service-name` 让 label 稳定可读；查 DESIGN_CN.md §6 表。 |
| 多 starter 项目写 `govern.fault.enabled=true` 以为只烧一个 | fault 是全进程共享开关，会烧所有 starter。用 `govern.fault.rules[].resources` 定向。 |
| 没 import starter-governance | 门面未生效，resilience 完全旁路，不报错但也不生效。 |
| 配了 `govern.*` 但忘了 `govern.source.file.path`（或其它 source） | 治理 disabled——没有 source 就没有规则来源。 |
| 改了规则文件没生效 | 确认 file source 在盯它（`govern.source.file.path` 指向的目录未变）；远程 source 确认 push 成功。规则文档本身热重载。 |
