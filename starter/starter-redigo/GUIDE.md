# starter-redigo 使用指南

`starter-redigo` 把 [redigo](https://github.com/gomodule/redigo)（`gomodule/redigo`）的连接池接入 Go-Spring：配置文件里每个 `${spring.redigo.instances.<name>}` 条目就是一个连接池 bean，注入即用。它在原生 `*redis.Pool` 之上叠了两层横切能力——**可观测**（trace/metric/log）和**弹性**（限流/熔断/重试/超时），并对每条命令自动生效。

> 本文面向实际使用：配置怎么写、类型怎么注入、扩展点在哪。内部实现细节见源码注释。

---

## 目录

- [一、核心概念：三个自定义类型](#一核心概念三个自定义类型)
- [二、快速开始](#二快速开始)
- [三、配置项参考](#三配置项参考)
- [四、多实例](#四多实例)
- [五、Resilience（限流 / 熔断 / 重试 / 超时）](#五resilience限流--熔断--重试--超时)
- [六、Observability（trace / metric / 访问日志）](#六observabilitytrace--metric--访问日志)
- [七、扩展：自定义 Driver](#七扩展自定义-driver)
- [八、扩展：per-command 钩子（CommandInterceptor）](#八扩展per-command-钩子commandinterceptor)
- [九、关闭内置功能](#九关闭内置功能)
- [十、服务发现](#十服务发现)
- [十一、作为缓存使用（cache.Cache bean）](#十一作为缓存使用cachecache-bean)
- [十二、健康检查与连接池监控](#十二健康检查与连接池监控)
- [十三、TLS](#十三tls)
- [十四、与 starter-go-redis 的关系](#十四与-starter-go-redis-的关系)

---

## 一、核心概念：三个自定义类型

理解这三个类型，就理解了整个 starter：

| 类型 | 是什么 | 你需要知道什么 |
|---|---|---|
| **`Pool`** | `*redis.Pool` 的包装 bean | 你注入的就是它。它**嵌入**了 `*redis.Pool`，所以 `Get()`/`Stats()`/`Close()` 等 redigo 原生方法原样可用；额外承载 resilience / observability 的配置字段。每个 `${spring.redigo.instances.<name>}` 条目产出一个 `*Pool` bean，名字是 `<name>`。 |
| **`Conn`** | `redis.Conn` 的插桩包装 | `pool.Get()` 拿到的连接其实是 `*Conn`。它对每条命令（`Do` / `DoContext` / `DoWithTimeout`）自动套上 trace span、metric、访问日志，并在开启 resilience 时套上执行器。**对调用方完全透明**——你照常用 redigo 的 `conn.Do(...)`，插桩在背后发生。 |
| **`Driver`** | 创建连接池的扩展接口 | 默认实现 `DefaultDriver` 满足绝大多数场景。需要自定义拨号逻辑（公司内部寻址、特殊鉴权、代理…）时实现它并作为可选容器 bean 提供，详见[第七节](#七扩展自定义-driver)。 |

---

## 二、快速开始

### 1. 引入包（副作用导入）

```go
import _ "go-spring.org/starter-redigo"
```

`init()` 会在导入时完成所有注册。[自定义 Driver](#七扩展自定义-driver) 走可选容器 bean，不需要非空导入。

### 2. 配置一个实例

`example/conf/app.properties`：

```properties
spring.redigo.instances.cache.addr=127.0.0.1:6379
```

`cache` 是你给这个实例起的名字（任意），下面所有 `spring.redigo.instances.cache.*` 都属于它。

### 3. 注入并使用

```go
import (
    "github.com/gomodule/redigo/redis"
    StarterRedigo "go-spring.org/starter-redigo"
)

type Service struct {
    Redis *StarterRedigo.Pool `autowire:"cache"` // 按名字注入第 2 步的实例
}
```

```go
// 用法和原生 redigo 完全一致：借连接 → 执行命令 → 还连接
conn := s.Redis.Get()
defer func() { _ = conn.Close() }()

v, err := redis.String(conn.Do("GET", "key"))   // GET
_, err = conn.Do("SET", "key", "value")         // SET
n, err := redis.Int(conn.Do("INCR", "counter")) // INCR
```

> ⚠️ 注意：redigo 没有 `redis.Client` 这种东西（那是 go-redis 的 API）。这里始终是「池里借连接 → `conn.Do(...)` → 还连接」。旧版 README 把 API 抄成了 go-redis，以本文为准。

### 4. 运行

完整可运行示例见 [example/example.go](example/example.go)（覆盖 SET/GET/INCR/EXPIRE/TTL、服务发现、健康检查、连接池监控），配套 `docker-compose.yml` 起一个 Redis。

---

## 三、配置项参考

所有键挂在 `spring.redigo.instances.<name>.*` 下（`<name>` 是实例名）。

### 连接

| 键 | 默认值 | 说明 |
|---|---|---|
| `addr` | _空_ | Redis 地址，如 `127.0.0.1:6379`。`addr` 与 `service-name` 至少填一个。 |
| `password` | _空_ | 密码。 |
| `username` | _空_ | ACL 用户名（Redis 6+）。 |
| `db` | `0` | 数据库编号。 |
| `tls.*` | — | TLS 配置，见[第十三节](#十三tls)。 |

### 连接池

| 键 | 默认值 | 说明 |
|---|---|---|
| `pool-size` | `10` | 池最大活跃连接数（`MaxActive`）。 |
| `max-idle` | `5` | 最大空闲连接数（`MaxIdle`）。 |
| `conn-max-lifetime` | `2m` | 单连接最长复用时间。**开服务发现时建议保持较短**，让池内连接平滑切到新地址而无需重建池。 |
| `dial-timeout` | `5s` | 拨号超时。 |
| `read-timeout` | `3s` | 读超时。 |
| `write-timeout` | `3s` | 写超时。 |

### 发现 / 驱动 / 启动

| 键 | 默认值 | 说明 |
|---|---|---|
| `service-name` | _空_ | 填了就走[服务发现](#十服务发现)，`addr` 被忽略。 |
| `scheme` | _空_ | 发现时按传输 scheme 过滤（如 `tls`）。仅 `service-name` 非空时有效。 |
| `discovery` | — | 选哪个已注册的 discovery 后端。仅 `service-name` 非空时有效；未配置即无后端。 |
| `driver` | `DefaultDriver` | 选哪个[ Driver](#七扩展自定义-driver)。 |
| `startup-ping` | `false` | 启动期拨一条连接 `PING`，地址错/不可达时启动即失败（fail-fast），而非等到首次请求。redigo 池是惰性拨号的，建议生产打开。 |

> **resilience 是全局键**（`resilience.*`，见[第五节](#五resilience限流--熔断--重试--超时)）。observe 层内置无条件（未装 starter-otel 时空操作）；health 可按实例关闭，见[第九节](#九关闭内置功能)（`health.enabled`）。

---

## 四、多实例

`spring.redigo` 是个 map，每个条目一个独立池，按名字注入：

```properties
spring.redigo.instances.cache.addr=127.0.0.1:6379
spring.redigo.instances.session.addr=10.0.0.2:6379
spring.redigo.instances.session.db=1
```

```go
type Service struct {
    Cache   *StarterRedigo.Pool `autowire:"cache"`
    Session *StarterRedigo.Pool `autowire:"session"`
}
```

每个实例还会自动得到一个[健康指标](#十二健康检查与连接池监控)，名字是 `redigo:<name>`。

---

## 五、Resilience（限流 / 熔断 / 重试 / 超时）

> ⭐ **关键**：resilience 配置是**全局顶层**键 `resilience.*`（没有 `spring.` 前缀，也没有实例名前缀）。同一套 `resilience.*` 驱动**所有** client starter（redigo、go-redis、gorm、bigcache……），不是每个 Redis 实例单独配。

### 开启与基本项

```properties
resilience.enabled=true       # 总开关，默认 false（关闭则命令直连，零开销）
resilience.driver=default     # 后端驱动，default 或 sentinel
```

### 各能力开关（均为可选，0/空 = 关闭该项）

| 键 | 说明 |
|---|---|
| `resilience.rate-limit` | 限流：每秒最大请求数（0=不限）。超限返回 `ErrRateLimited`。 |
| `resilience.burst` | 瞬时允许超过 rate-limit 的额度（0=驱动默认）。 |
| `resilience.error-threshold` | 熔断：连续失败几次跳闸（consecutive 策略）。 |
| `resilience.open-duration` | 熔断打开持续多久后放一次试探。 |
| `resilience.breaker-strategy` | 熔断计数策略：`consecutive`（默认）或 `error-rate`。 |
| `resilience.error-rate-threshold` | error-rate 策略下的失败率阈值 (0,1]。 |
| `resilience.min-requests` | error-rate 策略下窗口内最少样本数。 |
| `resilience.breaker-window` | error-rate 策略的滚动窗口（默认 1s）。 |
| `resilience.max-concurrent` | 舱隔离：单资源最大并发，超了返回 `ErrBulkheadFull`。 |
| `resilience.max-retries` | 重试：首次失败后额外重试次数。 |
| `resilience.initial-interval` | 退避：首次重试前等待（0=不退避）。 |
| `resilience.multiplier` | 退避增长因子（0/1=固定间隔）。 |
| `resilience.max-interval` | 退避上限。 |
| `resilience.randomization-factor` | 抖动比例 [0,1)。 |
| `resilience.attempt-timeout` | 单次尝试超时。 |
| `resilience.max-duration` | 整次调用（含所有重试）的总超时。 |

### 几个要记住的语义

- **热更新**：`resilience.*` 是 `gs.Dync` 绑定，配置文件改了**无需重启**即生效（试一次：把 `rate-limit` 调小，立刻就能看到请求被拒）。
- **`redis.ErrNil`（缓存未命中）算成功**，不会触发熔断/重试——和 gorm 的 `ErrRecordNotFound` 同理。
- **被拒（限流/熔断打开/舱满）原样上抛**：你会拿到 `ErrRateLimited` / `ErrCircuitOpen` / `ErrBulkheadFull`。
- **资源粒度**：限流器/熔断器按 Redis 实例隔离（不是按命令），key 优先取 `service-name`，回退到 `addr`。
- **attempt-timeout 只对 `DoContext` 生效**：redigo 的 `Do`/`DoWithTimeout` 不接受 context，执行器的 attempt-timeout 打不断它们。需要超时保护时优先用 `redis.DoContext(conn, ctx, ...)`。

  ```go
  reply, err := redis.DoContext(conn, ctx, "GET", "key")
  ```

---

## 六、Observability（trace / metric / 访问日志）

观测层由 starter **内置且无条件生效**：未导入 starter-otel 时 trace/metric 自动是空操作，导入后自动搭上 starter-otel 装的全局
TracerProvider/MeterProvider（`spring.observability.*` 管理 exporter、采样率、服务名）。

- 每条命令一个 client span（`db.system`/`db.operation`/`db.statement` 属性）+ `db.client.operation.duration` 直方图。
- 访问日志走项目 `log` 包（tag `_app_redigo_access`）：错误 → Warn；带参数的成功 → Debug（惰性求值）；其余成功 → Info。
- 日志只记命令名 + 第一个参数（通常是 key），**不记 value**（可能敏感或很大），且截断到 512 字节。
- 健康探测的 `PING` 整体静默（span+metric+log 一起跳过），避免刷屏。

可运行示例参考 `starter-bigcache/example-cloudnative`。

---

## 七、扩展：自定义 Driver

当默认拨号逻辑不够用（公司内部寻址、特殊鉴权、代理、连接级 hack……），实现 `Driver` 接口：

```go
type Driver interface {
    CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*redis.Pool, io.Closer, error)
}
```

### 提供与选用

```go
import StarterRedigo "go-spring.org/starter-redigo"

func init() {
    // 可选容器 bean：构造函数返回 Driver 接口即可
    gs.Provide(func() StarterRedigo.Driver { return MyDriver{} })
}
```

容器里没有 Driver bean 时，装配内部回退到内置 `DefaultDriver`；存在多个 Driver bean 时，
实例可按名指定：`spring.redigo.instances.<name>.driver = <bean 名>`（留空 = 按类型注入唯一
Driver bean；指定的 bean 不存在则启动失败）。

### teardown（关闭语义）

`DefaultDriver` 经 `NewPool` 返回的 `*Pool` 自带 `Close()`，是随池一起释放的 teardown：

- discovery 不再为 teardown 贡献任何东西：loader 无资源、无后台 watch、无 `Stop`，
  新鲜度在后端内部（`pool.go:150-152`）。
- 你的 driver 没有额外资源时，`Pool.Close` 只需关底层连接池即可，无需（也无法）再停什么
  discovery 后台任务。

`(*Pool).Destroy`（gs 销毁时自动调用，`starter.go:155`）会先关执行器、再关底层 redis 池。

### 最小示例

```go
type MyDriver struct{}

func (MyDriver) CreateClient(ctx context.Context, c StarterRedigo.Config, backend discovery.Discovery) (*redis.Pool, io.Closer, error) {
    pool := &redis.Pool{
        MaxActive: c.PoolSize,
        MaxIdle:   c.MaxIdle,
        Dial: func() (redis.Conn, error) {
            return redis.Dial("tcp", c.Addr,
                redis.DialPassword(c.Password),
                redis.DialConnectTimeout(c.DialTimeout),
            )
        },
    }
    return pool, discovery.NopCloser(), nil
}
```

> 完整可运行示例见 [example/example.go](example/example.go) 里的 `AnotherRedisDriver`。

---

## 八、扩展：per-command 钩子（CommandInterceptor）

想对**单条命令**做自定义处理——本地缓存命中就短路、命令 deny-list、改写参数、按命令埋点——给池注册一个 `CommandInterceptor`：

```go
type CommandHandler func(ctx context.Context, cmd string, args []interface{}) (reply interface{}, err error)
type CommandInterceptor func(next CommandHandler) CommandHandler
```

它是 **洋葱型**（与 grpc Interceptor 同形）：`next` 是内层路径（其余用户拦截器 → observe span → resilience executor → 真正的 Redis 调用）。调一次 `next` 命令才到 Redis；**不调就短路**。

### 注册（按池，在 bean Init 里）

```go
gs.Provide(func(s *Service) gs.Init {
    return func(ctx context.Context) {
        s.Main.UseCommandInterceptor(func(next StarterRedigo.CommandHandler) StarterRedigo.CommandHandler {
            return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
                // 例 1：本地缓存短路 GET，不打扰 Redis、不消耗熔断配额、不发 span
                if cmd == "GET" {
                    if v, ok := localCache.Get(args[0]); ok {
                        return v, nil // 不调 next
                    }
                }
                // 例 2：其余命令照走，顺带记一笔
                return next(ctx, cmd, args)
            }
        })
    }
})
```

### 要记住的语义

- **作用域是单池**：`UseCommandInterceptor` 注册到**这一个**池，可注册多条（先注册的在外层），不同池各自独立。请在流量到来前的 bean Init 里注册——折叠发生在每次拨号时，之后注册的只影响之后拨出的连接。
- **钩子在最外层**：在 observe span 和 resilience executor **之前**。所以短路时**不发 span、不消耗限流/熔断配额**——本地命中不该被记成一次 Redis 调用。想让命令计入熔断的观察型钩子，必须调 `next`。
- **`next` 返回的错误**：`redis.ErrNil`（未命中）、`ErrRateLimited`/`ErrCircuitOpen`/`ErrBulkheadFull` 都会原样回到钩子，你可以翻译或吞掉。
- **不注册零开销**：没注册时 `Conn` 直接走内置路径，无额外间接调用。

---

## 九、关闭内置功能

内置功能各自带开关，按实例（配置）控制。下表 `spring.redigo.instances.<name>.*` 下的键：

| 内置功能 | 开关键 | 默认 | 关掉的效果 |
|---|---|---|---|
| **health 指标** | `health.enabled` | `true` | 不注册 `redigo:<name>` 健康指标，不进聚合健康。 |
| **resilience** | `resilience.enabled`（全局） | `false` | 本就 opt-in；不开启则命令直连。见[第五节](#五resilience限流--熔断--重试--超时)。 |
| **startup-ping** | `startup-ping` | `false` | 本就 opt-in；不开启则惰性拨号。 |

```properties
# 某个非关键缓存池：不进聚合健康
spring.redigo.instances.softcache.addr=10.0.0.5:6379
spring.redigo.instances.softcache.health.enabled=false
```

> 缓存抽象 bean（名为 `redigo:<实例名>` 的 `*cache.Cache`）是惰性的：无人注入就不实例化，无配置开关。

---

## 十、服务发现

填 `service-name` 即走服务发现，`addr` 被忽略：

```properties
spring.redigo.instances.cache.service-name=redis-cluster
spring.redigo.instances.cache.discovery=nacos.default   # 可选，选已注册的 discovery 后端；未配置即无后端
spring.redigo.instances.cache.scheme=tls          # 可选，按 scheme 过滤端点
spring.redigo.instances.cache.conn-max-lifetime=30s  # 建议调短，平滑切址
```

工作方式：

- starter 用注入的后端 bean 解析 `redis-cluster` 的存活端点；新鲜度在后端内部，无后台 watch。
- 连接池**每次新建连接**时 `Pick()` 一个存活实例拨过去；配合较短的 `conn-max-lifetime`，池内连接会逐步换到更新后的地址，**无需重建池**。
- loader 无资源、无 `Stop`——`Pool.Close` 只需关 resilience executor + 底层 redis 池，无需停任何 watch（`pool.go:150-152`）。
- **mesh 模式**（`GS_MESH=on`）下，发现被整个跳过——sidecar 接管发现+LB，池直接拨配置的 `addr`（服务的稳定 mesh 地址）。

注册一个 discovery 后端的示例见 [example/discovery.go](example/discovery.go)。

---

## 十一、作为缓存使用（cache.Cache bean）

每个 redigo 实例（连接池）都会另提供一个 `cache.Cache` bean（基于 `bytecache` 子包，GET/SET/DEL 走 `redis.DoContext`），bean 名为 `redigo:<实例名>`：

```go
import "go-spring.org/cloud/cache"

type Service struct {
    // 名为 cache 的 redigo 实例上的带类型缓存门面
    KV *cache.Cache `autowire:"redigo:cache"`
}
```

无人注入该 bean 则不实例化，因此没有配置开关。

---

## 十二、健康检查与连接池监控

### 自动健康指标

每个实例自动注册一个 `health.Indicator`，名字 `redigo:<name>`：借一条连接 `PING`，成功即 UP。配合 actuator 聚合：

```properties
spring.actuator.enabled=true
spring.actuator.addr=:9370
# /healthz 聚合所有指标；/readyz 含 readiness
```

### 连接池运行时计数

`Pool` 嵌入了 `*redis.Pool`，`Stats()` 直接可用：

```go
stats := s.Redis.Stats()
fmt.Println("active:", stats.ActiveCount, "idle:", stats.IdleCount)
```

---

## 十三、TLS

字段布局与 `starter-go-redis` 完全一致：

```properties
spring.redigo.instances.cache.tls.enabled=true
spring.redigo.instances.cache.tls.ca-file=/path/to/ca.pem
# 双向 TLS 再加：
spring.redigo.instances.cache.tls.cert-file=/path/to/client.pem
spring.redigo.instances.cache.tls.key-file=/path/to/client.key
spring.redigo.instances.cache.tls.server-name=redis.example.com
# 仅自签测试用：
spring.redigo.instances.cache.tls.insecure-skip-verify=true
```

`tls.enabled=false`（默认）时走明文。

---

## 十四、与 starter-go-redis 的关系

两者配置字段布局刻意保持一致（`addr`/`pool-size`/`tls.*`/`service-name`/`discovery`/`scheme`/resilience 全部对齐），切换通常**只改 import**：

| | starter-redigo | starter-go-redis |
|---|---|---|
| 底层库 | `gomodule/redigo` | `redis/go-redis` |
| API 风格 | 借连接 `conn.Do(...)` | `client.Get(ctx,...)` |
| 插桩缝 | 每连接包装（`Conn`） | go-redis Hook |

**怎么选**：

- 想**接近原生 redis-py 风格、强类型命令、内置 cluster 编排** → go-redis。
- 想要**轻量、显式管理连接、与旧 redigo 代码兼容** → redigo。
- 两者都内置 observe + resilience，能力对等。
