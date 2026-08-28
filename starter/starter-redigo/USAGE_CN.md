# starter-redigo 使用说明 — 参考手册

详细使用文档。概览见 [README.md](README_CN.md)。所有行为声明均已对照源码
（`starter.go`、`config.go`、`pool.go`、`conn.go`、`driver.go`、`health/health.go`、`bytecache/`）
与可运行的 [example/](example/) 核实——下文括号内为 file:line 抽查点。
**redigo 语义（Do/借连接模型、reply 辅助函数）见 [redigo 官方仓库](https://github.com/gomodule/redigo)**
——本文只写 go-spring 的增量。字段布局刻意与 starter-go-redis single 模式对齐，两者切换只需
改 import + 前缀。

**激活条件**：任一 `spring.redigo.*` key。每个 `spring.redigo.<name>` 条目创建一个名为
`<name>` 的 `*StarterRedigo.Pool` bean，并附带名为 `redigo:<name>` 的健康指示器
（`health.enabled` 默认 true）。

---

## 1. 完整工程示例

一个包含静态池、发现池、用户命令拦截器与 cache 门面的服务。文件树：

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
    github.com/gomodule/redigo     latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-redigo   latest
    go-spring.org/starter-cache    latest   // cache 门面
    go-spring.org/starter-actuator latest   // 可选
    go-spring.org/starter-otel     latest   // 可选
    go-spring.org/starter-governance latest // 可选
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-redigo"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** —— borrow/Do 模式 + 拦截器：

```go
package service

import (
    "context"
    "time"

    "github.com/gomodule/redigo/redis"
    "go-spring.org/cloud/data/cache"
    "go-spring.org/spring/gs"
    StarterRedigo "go-spring.org/starter-redigo"
)

type Service struct {
    // 始终注入包装 *StarterRedigo.Pool；它内嵌 *redis.Pool，Get/Stats 原样提升。
    // 它交出的连接都是插桩过的 Conn。
    Main      *StarterRedigo.Pool `autowire:"main"`
    Discovery *StarterRedigo.Pool `autowire:"discovery"`

    // spring.cache.demo.driver=redigo:main —— "main" 池上的带类型门面。
    Cache *cache.Cache `autowire:"main"`
}

func init() {
    gs.Provide(func(s *Service) gs.Init {
        return func(ctx context.Context) {
            // 在 Init 里注册拦截器（趁池还没大流量发放连接）；
            // 先注册的 = 最外层。
            s.Main.UseCommandInterceptor(func(next StarterRedigo.CommandHandler) StarterRedigo.CommandHandler {
                return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
                    if cmd == "GET" && args[0] == "local:skip" {
                        return "served-locally", nil // 短路：不开 span、不占熔断配额
                    }
                    return next(ctx, cmd, args)
                }
            })
        }
    })

    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 标准的 borrow/Do 模式。
            c := s.Main.Get()
            defer func() { _ = c.Close() }()
            if _, err := redis.String(c.(*StarterRedigo.Conn).DoContext(ctx, "SET", "key", "value")); err != nil {
                panic(err)
            }
            _ = s.Cache.Set(ctx, "user:1", "demo", time.Minute)
        }
    })
}
```

**conf/app.properties**：

```properties
# --- main 池：静态地址 + fail-fast 拨号检查 -----------------------------------
spring.redigo.main.addr=127.0.0.1:6379
spring.redigo.main.startup-ping=true
spring.redigo.main.pool-size=20
spring.redigo.main.conn-max-lifetime=2m

# --- 发现池：addr 被忽略，每次拨号动态选端点 ----------------------------------
spring.redigo.discovery.service-name=redis-cluster
spring.redigo.discovery.conn-max-lifetime=30s

# --- 观测 -----------------------------------------------------------------
# 实例级硬开关（span+metric+log 一并关），默认 true。
spring.redigo.main.observe.enabled=true
# 访问日志详细度是全局 observability.* 策略（全部 client starter 共享），
# 这里不像 starter-go-redis 那样按实例配置：
observability.level=brief

# --- 健康 -------------------------------------------------------------------
# 默认 true；设 false 可让非关键缓存不卷入聚合健康
spring.redigo.main.health.enabled=true

# --- cache 门面 ------------------------------------------------------------
spring.cache.demo.driver=redigo:main

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
```

**验证**：

```bash
docker run -d -p 6379:6379 redis
go run .                          # 地址错误时 startup-ping 让启动失败
curl -s :9370/readyz              # components 含 redigo:main、redigo:discovery
redis-cli GET key                 # "value"——经洋葱模型写入
redis-cli GET user:1              # 经 cache 门面写入的 JSON
grep _app_redigo_access app.log   # 每次 Do 一条访问记录
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-redigo
  └─ 任一 spring.redigo.* key 存在时 gs.Module(OnProperty("spring.redigo")) 触发
        └─ conf.BindEach("${spring.redigo}") → 每个 <name> 一份 Config
              ├─ Provide(createPool).Name(<name>).Destroy(destroyPool)
              │    ctor 参数：ContextProvider、Config（IndexArg 1）、全局
              │    observability.* 策略（IndexArg 2，TagArg）[starter.go:60-64]
              └─ health.enabled 时 → Provide 名为 "redigo:<name>" 的 health.Indicator

gs.Run()
  ├─ 构造 createPool [starter.go:107]：RequireAny(addr|service-name) → 查 driver
  │   → d.CreateClient（= NewPool）：TLS 构建 → discovery resolver → 原始池
  │     → observer（observe.enabled 时）→ resilience executor → setupDial
  │     → startup-ping（仅 startup-ping=true 时）[starter.go:144-149]
  │   注意：没有独立 InitMethod——池在返回时即完整就绪 [pool.go:36-37]
  ├─ 你的 bean Init 可调用 UseCommandInterceptor（自此之后拨的连接生效）
  └─ SIGTERM → destroyPool → Pool.Close：exec.Close → resolver.Stop → pool.Close [pool.go:141-149]
```

`Pool.Close` 遮蔽了内嵌的 `(*redis.Pool).Close`，确保普通 Close 不会泄漏 discovery
resolver 的后台 watch [pool.go:137-140]。

### 2.2 命令洋葱 —— 在连接构造期折叠

池拨出的每条连接都在**拨号时**被 `Pool.wrapConn` 包装一次 [pool.go:212-222]。各层折叠成
单个组合好的 `CommandInterceptor`（`NewConn` 从最内层折起，因此第一层最终在最外
[conn.go:93-106]）：

```
用户拦截器（先注册的在外）
  → observe 层（span + 时延/在途指标 + 访问日志）
    → resilience executor（熔断/限流/重试/超时）
      → 内层 Do 调用 → Redis
```

理由（源码注释 [pool.go:205-211]、[conn.go:44-53]）：

- **用户拦截器最外**：一层可以**不开 span、不占熔断配额**地短路，可以改写 ctx/cmd/args，
  也可以只观察结果。想让命令计入统计的观察型层必须调 `next`。
- **span 在 executor 之外**：一次 Execute——含策略驱动的全部重试——共用一个 span 和
  一条访问日志。
- **executor 最内**：由它派生每次尝试的 context，守在真实网络调用前。

因为折叠发生在每次拨号时，`UseCommandInterceptor` 在部分连接已拨出后注册的拦截器只对
**之后**拨的连接生效；已拨出的连接保留它构建时的链 [pool.go:151-157]。请在流量到来前的
bean Init 里注册。

### 2.3 一条命令逐层走读：命中场景的 `DoContext(ctx, "GET", "key")`

1. 你的拦截器（若有）先跑；可改写或短路。
2. observe 层开名为 `get` 的 span，参数摘要是 `GET key`（只记命令+首个参数——值永不入日志；
   受 MaxArgBytes 截断 [conn.go:215-220]）。ctx 是**调用方**的 context，span 因此挂到请求
   trace 上，attempt-timeout 也能打断调用。
3. resilience 层向 executor（resource 标签 `redigo:<地址或服务名>`，按池
   [pool.go:190]）申请许可；可重试失败会重新驱动内层调用。
4. 内层 `Do` 读写 Redis；命中返回 bulk string。
5. `redis.ErrNil`（miss）经 nil-as-success 谓词判为成功 [conn.go:201-204]——
   **miss 永不触发熔断**。
6. span 结束；访问日志记录（tag `_app_redigo_access`）带时延/状态输出。

`Do` / `DoWithTimeout` 不带 context：它们的 span 是根 span，attempt-timeout 打不断——
两者任一要紧就用 `DoContext` [conn.go:74-79]。`Send`/`Flush`/`Receive`（pipeline）刻意不做
插桩。

### 2.4 一条池化连接的生命周期

```
pool.Get()（你的代码）
  ├─ 有空闲连接？→ 复用（MaxConnLifetime=conn-max-lifetime 限定复用时长）
  └─ 否则 Dial：凭据/TLS/SELECT db → 设置了 service-name 时 discovery
     resolver.Pick() 选端点 [pool.go:104-120] → wrapConn 折叠洋葱
  ├→ 你 Do/DoContext 命令（每条都走 用户 → observe → resilience → 网络）
  └→ conn.Close()：归还空闲池（redigo 语义）
停机：Pool.Close() —— executor、resolver watch，最后是池本身
```

`Wait: true` 是硬编码 [pool.go:86]：池耗尽时借连接方阻塞而非报错——请相应设置 `pool-size`。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.redigo.<name>.` 下（全局 `observability.*` 策略除外）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|----------|
| `addr` | string | — | 静态目标。⚠ `addr` / `service-name` 至少其一（RequireAny [starter.go:111]）。 | 都缺 → 启动报错；都配 → service-name 生效，addr 被忽略。 |
| `service-name` | string | — | 经发现解析地址；`addr` 只作标签。每次拨号选端点 + conn-max-lifetime 回收。 | 后端未注册 → 启动报错。 |
| `scheme` | string | — | 把发现端点收窄到单一 scheme。仅 service-name 时生效。 | — |
| `discovery` | string | `default` | 用哪个发现后端。 | — |
| `password` / `username` | string | — | 拨号认证；username 非空才附加 [pool.go:94-96]。 | 配错 → 首次拨号（或 startup-ping）失败。 |
| `db` | int | 0 | 每条新连接执行 `SELECT`（仅非 0 时）[pool.go:125-131]。 | 越界 → 拨号失败。 |
| `pool-size` | int | 10 | MaxActive。`Wait:true` → 耗尽时阻塞。 | 过小 → 时延而非报错。 |
| `max-idle` | int | 5 | MaxIdle。 | 大于 pool-size 无意义。 |
| `dial-timeout` / `read-timeout` / `write-timeout` | duration | 5s / 3s / 3s | 拨号参数。 | — |
| `conn-max-lifetime` | duration | 2m | MaxConnLifetime；较短值利于发现流量切换。 | 很大 + discovery → 老端点滞留。 |
| `tls.*` | group | off | 客户端 TLS；key 与 starter-go-redis 对齐。 | 配一半 → tls.Build 启动报错。 |
| `driver` | string | `DefaultDriver` | 选择已注册 Driver。 | 未知 → 启动报 "redigo driver not found"。 |
| `startup-ping` | bool | false | 可选启动探测：拨一条裸连接并 PING [pool.go:266-279]。⚠ 默认关——池是惰性的，坏地址要到首条命令才暴露。 | 期待 fail-fast 却没开 → 启动"成功"，首个请求失败。 |
| `observe.enabled` | bool | true | 实例级硬开关（span+metric+log）。false 时 Conn 近零开销直通（仍包装以保持 DoContext 透明）。 | false + 期待 trace → 静默。 |
| `health.enabled` | bool | true | 注册 `redigo:<name>` 指示器；false 让池不卷入聚合健康。 | false → readiness 静默漏掉该池。 |
| `observability.level` / `.maxArgBytes` / `.skipOps`（全局 key） | — | brief / 512 / — | 由 ctor 第 2 参 `${observability:=}` 绑定——全部 client starter 共享的全局策略，非实例级 [starter.go:60-64]。 | `spring.redigo.<n>.observability.level` 不存在——与 starter-go-redis 不对称。 |

**扩展点**：

- `Pool.UseCommandInterceptor(...CommandInterceptor)` —— 上述每命令洋葱；先注册在外；
  传 nil 拦截器直接 panic。
- `StarterRedigo.NewConn(raw, layers...)` —— 手工装配原语，供 REPLACE 型 driver 使用。
- `Driver` / `RegisterDriver` —— 拥有完整池装配；[driver.go:43-51] 记录两种形态：
  ADD（调 NewPool 后经公开 API 定制）或 REPLACE（完全自管）。example 的
  `AnotherRedisDriver` 演示"委托+定制"形态。

---

## 4. 验证与故障演练

### 4.1 经 actuator 验证健康

```bash
curl -s :9370/readyz            # redigo:main 借一条连接并 PING [health/health.go:32-39]
docker stop <redis>; curl -s :9370/readyz   # 503
```

### 4.2 验证访问日志与 span

```bash
redis-cli SET probe 1
grep _app_redigo_access app.log | tail -1
# op=set status ok duration=...；detailed 级别可见 "GET probe"
curl -s :9370/metrics | grep -E 'redigo|db.client'   # 时延直方图 + 在途 gauge
```

### 4.3 验证发现地址回收

用 `discovery` 实例（`service-name` + `conn-max-lifetime=30s`），迁移/扩缩后端 Redis；
`Stats()`（ActiveCount/IdleCount）显示 30s 内连接回收到新端点——无需重启、无需重建客户端。

### 4.4 验证 cache driver 接线

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)
v, _ := redis.String(s.Main.Get().(*StarterRedigo.Conn).Do("GET", "k"))  // 读到 JSON 形态
```

门面 miss 返回 `cache.ErrMiss`；裸 Do 返回 `redis.ErrNil`——边界做了映射
（starter-redigo/bytecache）。

### 4.5 拦截器短路演练

命中拦截器的 key（§1 的 `GET local:skip`）：直接返回应答，且**没有**新增
`_app_redigo_access` 行、不占熔断配额——证明用户层在两者之外。改调 `next` 则恢复完整插桩。

### 4.6 startup-ping 演练

`startup-ping=true` + 错误 `addr`：启动报 "redis: startup ping failed"。设为 false：
启动成功而首条命令失败——这正是该 key 控制的取舍。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动正常，首条命令 "connection refused" | 池惰性；`startup-ping` 关着 | 开 `startup-ping=true` 获得 fail-fast。 |
| 启动报 "redis: startup ping failed" | 地址/认证/TLS/发现不通 | 它拨一条裸连接；修连通性。 |
| 启动报 "one of addr/service-name required" | 两个 key 都没配 | 恰好配一个。 |
| 拦截器不生效 | 注册晚于连接拨出 | 流量前来 bean Init 里注册 [pool.go:151-157]。 |
| span 没挂到请求 trace | 用了 `Do`/`DoWithTimeout`（根 span） | 改 `DoContext` [conn.go:74-79]。 |
| 完全没有 span/指标/日志 | `observe.enabled=false` | 重新打开，或检查 starter-otel。 |
| 时延尖刺但无报错 | 池耗尽 + `Wait:true` | 调大 `pool-size`；盯 Stats()。 |
| 自定义 driver 的池没有插桩 | driver 返回了裸池 | 用 NewPool/NewConn 装配返回包装 Pool；createPool 只回填 cfg [starter.go:95-99]。 |
| 怀疑 miss 触发熔断 | 不会——ErrNil 判为成功 | 找真实失败。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 实例 18 个 + tls 组 + 全局 observability 2 个 |
| 其中必填 | 1（`addr` 或 `service-name`） |
| quickstart 前置外部依赖 | 1（Redis） |
| "注意/坑" 条数 | 6 |

设计嫌疑清单：`startup-ping` 此处可选而 starter-go-redis 无条件执行（家族不对称，
config.go:85-91 有记载）；访问日志详细度只有全局 `observability.*` 而 go-redis 按实例绑定；
借出的连接要拿到 `DoContext` 需要类型断言（`pool.Get()` 返回 `redis.Conn`）。
