# 治理中心配置指南

本文讲**怎么写配置文件**。设计原理（为什么是单 Dync、per-label Register 怎么分发）见 [DESIGN_CN.md](./DESIGN_CN.md)；资源标签（label）格式表见 [DESIGN_CN.md §6](./DESIGN_CN.md#6-资源标签label约定)。

适用于一个项目里**同时用多个 starter**（redis + gorm + http-client + gin 入站……）的场景。

---

## 1. 前置：引入 cloud/govern

govern 不是自动生效的——必须在程序入口 blank import cloud/governance，它才会在启动时（作为一个 `gs.Runner`）把治理中心接到 resilience 的中立 seam 上：

```go
import (
    _ "go-spring.org/cloud/governance"   // 启动时注册 resilience executor provider
    _ "go-spring.org/starter-redigo"   // 你的业务 starter
    // ... 其他 starter
)
```

> **client starter 不需要 import（也不注入）govern。** 每个 client（redis/gorm/http/…）只调中立函数 `resilience.ExecutorFor(资源label)` 拿到它的 executor——不知道 govern 的存在。cloud/governance 内置了这段接线（`starter.go`），在进程启动时把治理中心注册成那个 executor 的来源（`gs.Runner` 保证执行，跟 actuator 同机制）。这是 2026-08-14 的重构：之前每个 client 注入治理中心，现在零 govern 耦合。详见 [DESIGN_CN.md](./DESIGN_CN.md)。

**不 import cloud/governance 时**：没有 provider 注册，`ExecutorFor` 返回透传的 noop executor，resilience 完全旁路（直连后端），不会报错。所以"没配治理"和"不能用 starter"是两回事。

**例外：starter-dubbo**。dubbo 走自己的 URL-param 治理模型（timeout/retries 是 dubbo 参数，不走 resilience executor），所以它直接走门面 `governance.PolicyFor` 读策略字段（center 类型本身不导出，谁也注入不了）。这是唯一的直接门面消费 client。

---

## 2. 最小可用：一份默认策略管全部

最常见用法——全进程所有资源共享同一份韧性策略：

```properties
# === 治理总开关 ===
govern.enabled=true
govern.driver=default          # 或 sentinel；全进程统一后端，一处切换处处生效

# === 默认策略（所有没有专门 Rule 的资源都用这份）===
govern.default.enabled=true
govern.default.timeout=500ms
govern.default.max-retries=1
govern.default.rate-limit=100  # ops/s，0 表示不限流
govern.default.error-threshold=20   # 连续失败 20 次熔断
govern.default.open-duration=5s     # 熔断持续 5s 后半开试探
```

配完这 7 行，项目里的 redis、gorm、mongo、http-client……全部自动套用这套超时/重试/限流/熔断，且**热重载**——改完文件不用重启。

> `govern.default.*` 下的可用字段就是 `resilience.Config` 的全部旋钮：`timeout` / `max-retries` / `rate-limit` / `burst` / `error-threshold` / `open-duration` / `breaker-strategy`(consecutive|error-rate) / `error-rate-threshold` / `min-requests` / `breaker-window`。字段含义见 [cloud/governance/resilience/config.go](resilience/config.go)。

---

## 3. 多 starter 项目：给不同资源配不同策略

真实项目里 redis 和 gorm 的容忍度不一样。用 `govern.rules[N]` 给特定资源单独配——**资源 label 写在 `resources` 值里**（不是 key），所以冒号随便写、properties 不转义、YAML 不加引号：

```properties
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

YAML 里同样干净（冒号在值里，不是 key）：

```yaml
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

### 几条规则

- **一条 Rule 可匹配多个资源**：`govern.rules[0].resources=redigo:cache,redigo:session`（逗号分隔），这几个资源共享同一份策略。
- **Rule 是整体替换，不是字段合并**：给 `redigo:cache` 配了 Rule，它就**完全不继承** `govern.default`——漏写的字段按零值处理（零值=禁用该能力）。只想微调一个字段的话，把 default 里要保留的字段也抄进 Rule。
- **多条 Rule 命中同一 label 时，前面的赢**（first-match）。所以具体的 Rule 放前面。
- **`resources` 留空不匹配任何资源**——兜底请用 `govern.default`，不要用空 resources 的 Rule。

### 怎么知道某个资源的 label 是什么？

查 [DESIGN_CN.md §6](./DESIGN_CN.md#6-资源标签label约定) 的表。标签由 starter 用 `resilience.ResourceLabel(prefix, names...)` 拼接——取第一个非空的 name。所以 label 的取值取决于你配置里填的是 `service-name` 还是 `addr`：

- 你配了 `spring.redigo.cache.service-name=cache-svc` → label 是 `redigo:cache-svc`
- 没配 service-name、只有 `spring.redigo.cache.addr=10.0.0.1:6379` → label 是 `redigo:10.0.0.1:6379`

**建议**：给每个资源配一个稳定的 `service-name`，让 label 可读、不随地址漂移。

---

## 4. 入站流量的治理（gin / grpc）

gin / grpc 是**入站**侧——策略作用在"处理一个进来的请求"上，label 用监听地址：

```properties
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

---

## 5. dubbo 的治理

dubbo 走的是 URL-param 模型，govern 只覆盖它的 `timeout` 和 `retries`（cluster-failover 级别），label 是：

- 应用级：`dubbo:<app-name>`
- 每个 reference：`dubbo:<interface>:<version>:<group>`

```properties
govern.rules[0].resources=dubbo:com.example.UserService:1.0.0
govern.rules[0].enabled=true
govern.rules[0].timeout=300ms
govern.rules[0].max-retries=2
```

dubbo 专属旋钮（loadbalance / cluster / serialization）不进 govern，留在 dubbo 自己的配置段。

---

## 6. fault（放火）——随 govern 集中化,一个开关烧全进程 ⚠️

fault 注入已**收进治理中心**,和 resilience 共用同一个 `${govern}` Dync(参见 [DESIGN_CN.md §8](DESIGN_CN.md))。所以放火的 key 是 `govern.fault.*`(不再是顶层 `fault.*`):

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic
```

这一条会**同时给全进程所有 starter 放火**——redis、gorm、http-client、gin 入站……全部以 50% 概率注入错误。这是集中化的预期效果:starter 侧通过中立的 `fault.InjectorFor()` seam 拿到唯一的进程级 injector,starter 自己不再绑 fault 配置、也不 import cloud/governance。

### 想只给某个资源放火

用 `govern.fault.rules[]` 做定向(catch-all 之外的细分):

```properties
govern.fault.enabled=true

# 默认(catch-all):不实际注入错误,只让框架进入"fault 模式"
govern.fault.rate=0

# 只给 redis 放火
govern.fault.rules[0].resources=redigo:cache
govern.fault.rules[0].rate=0.5
govern.fault.rules[0].error=timeout
```

> 注意 `govern.fault.rules[N].resources` 里的值要和该 starter 实际传给 injector 的 resource label 对上(client 侧是 `redigo:cache` 这类)。server 侧(gin/grpc/echo/hertz/trpc/dubbo)的 fault 走中间件/拦截器,per-call 解析 injector,label 规则见各 server starter 文档(grpc 的 label 是 `grpc:<FullMethod>`)。

### fault 的安全保险

放火忘了关很危险,fault 内置两个自愈上限:

```properties
govern.fault.max-duration=10m     # 放火 10 分钟后自动停(从第一次生效算)
govern.fault.max-affected=1000    # 累计影响 1000 次调用后自动停
```

**强烈建议**生产环境放火时必设其一,set fire and walk away 也不会烧到天荒地老。

> 注意:集中化后 `max-duration`/`max-affected` 是**进程级计数**(不再是 per-resource)。详见 DESIGN_CN.md §8 的取舍说明。

### fault 与真实流量

用 `govern.fault.scope` 限定只烧压测流量、不碰真实请求(依赖 cloud/governance/traffic 的压测标记):

```properties
govern.fault.scope=loadtest   # 只给带压测标记的流量放火;真实流量不受影响
                            # 反向:real = 只烧真实流量;空 = 全烧(默认)
```

### 运行时热更

因为 fault 现在随 `${govern}` 走 centerHolder 的单一 Dync,**改配置 push 即可在不重启进程的情况下开关 fault**——这是集中化相比旧版"启动时必须开"的重要改进(starter 通过 `fault.InjectorFor()` per-call 惰性解析,中心 `SetConfig` 原地热更)。

---

## 7. 一份完整的多 starter 项目配置示例

一个同时用 gin（入站）+ redigo（缓存）+ gorm-mysql（DB）+ http-client（调下游）的项目：

```properties
# ============ 业务 starter 配置（各自的 key，互不干扰）============
spring.gin.api.address=:8080
spring.redigo.cache.service-name=cache
spring.redigo.cache.addr=10.0.0.1:6379
spring.gorm.orders.driver=mysql
spring.gorm.orders.dsn=orders:pwd@tcp(10.0.0.2:3306)/orders
spring.http.user.service-name=user-svc
spring.http.user.addr=10.0.0.3:8081

# ============ 治理：一处下发，处处生效 ============
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

# ============ fault：默认关，需要时翻开关 ============
fault.enabled=false
# 演练时打开：
# fault.enabled=true
# fault.scope=loadtest
# fault.rules[0].resources=redigo:cache
# fault.rules[0].rate=0.3
# fault.rules[0].error=timeout
# fault.max-duration=5m
```

入口：

```go
import (
    _ "go-spring.org/cloud/governance"
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
| 在每个 starter 自己的配置段写 `resilience.*` | 已废弃。resilience 现在只认 `${govern}`，starter 段里的 resilience 配置不生效。 |
| `govern.rules[N]` 只写一个字段想"微调" | Rule 是整体替换 default，漏写字段=禁用该能力。要保留的 default 字段得抄进 Rule。 |
| 用 `govern.override.<label>` 旧写法 | 已改为 `govern.rules[N].resources=<label>`。label 放值里，别再当 key（冒号会废掉 YAML）。 |
| 不知道资源 label 是什么 | 配 `service-name` 让 label 稳定可读；查 DESIGN_CN.md §6 表。 |
| 多 starter 项目写 `fault.enabled=true` 以为只烧一个 | fault 当前是全进程共享开关，会烧所有 starter。用 `fault.rules[].resources` 定向。 |
| 没 import cloud/governance | 门面未生效，resilience 完全旁路，不报错但也不生效。 |
| 改了配置没生效 | 确认走了热重载（file-watch / 配置中心）；govern 的单 Dync 本身支持热重载，但要看配置源是否推送了变更。 |
