# starter-ratelimit-redis 使用说明 — 参考手册

[English](USAGE.md) | [中文](USAGE_CN.md)

详尽使用参考。概览见 [README_CN.md](README_CN.md)。下文所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`driver.go`）、`starter-go-redis/experimental/ratelimit.go` 的
令牌桶实现、[resilience 限流器注册表](../../../cloud/governance/resilience) 与自校验的
[example/](example)（`example/check.sh`）核实。限流语义（令牌桶）是标准概念——本文只写
go-spring 的增量。

**激活方式**：任一 `spring.ratelimit.redis.<name>.*` 配置即为每个 `<name>` 注册一个
`resilience.LimiterDriver` 实例；每个实例复用其 `client` 字段指名的 `*redis.Client`
bean（由 starter-go-redis 在 `spring.go-redis.<client>` 下提供）。消费方按名选择
driver——starter-gateway 的 `rateLimit(driver=...)` 过滤器、`resilience.GetLimiter(name)`、
或直接注入。

---

## 1. 完整工程示例

两个 HTTP"副本"共享 Redis 中的一个全局令牌预算——跨副本限流的演练场。文件树：

```
demo/
├── go.mod
├── main.go
├── web.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/redis/go-redis/v9        latest
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-go-redis      latest
    go-spring.org/starter-ratelimit-redis latest
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-ratelimit-redis"
)

func main() { gs.Run() }
```

**web.go** —— 应用的全部限流面：

```go
package main

import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
    "go-spring.org/spring/gs"
)

func init() {
    // 按实例名注入 Driver bean；LimitPolicy 是每个调用点的事，
    // 不是 starter 配置。
    gs.Provide(func(d resilience.LimiterDriver) *gs.HttpServeMux {
        mk := func() resilience.RateLimiter {
            lim, err := d.NewRateLimiter(resilience.LimitPolicy{
                Rate:  2,  // 每秒令牌数
                Burst: 5,  // 桶容量；<=0 时默认 max(1, Rate)
            })
            if err != nil {
                panic(err) // "no redis client bound" —— 装配 bug
            }
            return lim
        }
        limA, limB := mk(), mk() // 两个"副本"——Redis 里一份共享预算

        mux := http.NewServeMux()
        serve := func(l resilience.RateLimiter) http.HandlerFunc {
            return func(w http.ResponseWriter, r *http.Request) {
                ok, err := l.Allow(r.Context(), "api") // key "api"，挂在 "ratelimit:" 下
                if err != nil {
                    http.Error(w, "limiter backend error: "+err.Error(), http.StatusInternalServerError)
                    return
                }
                if !ok {
                    http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
                    return
                }
                _, _ = w.Write([]byte("ok"))
            }
        }
        mux.Handle("/a/", serve(limA))
        mux.Handle("/b/", serve(limB))
        return &gs.HttpServeMux{Handler: mux}
    }, gs.TagArg("gateway"))
}
```

**conf/app.properties** —— 上述代码用到的完整注释配置面：

```properties
# --- redis client（starter-go-redis 持有；driver 按名复用） --------------------
spring.go-redis.cache.addr=127.0.0.1:6379

# --- limiter driver --------------------------------------------------------------
spring.ratelimit.redis.gateway.client=cache
spring.ratelimit.redis.gateway.driver=redis    # 注册进 limiter 注册表的名字
                                               # （不设时默认用实例名）
```

不用代码、纯配置消费（配 starter-gateway）：

```properties
spring.gateway.route.demo.filters=rateLimit(rate=100,driver=redis)
```

从"每副本各自限流"切到"跨副本共享预算"只是换 driver 名；breaker/retry/timeout 仍走默认
driver——limiter 注册表与 executor 注册表相互独立。

**验证**（本地 Redis，可用 `example/docker-compose.yml`）：

```bash
go run . &
for i in $(seq 1 10); do curl -s -o/dev/null -w '%{http_code}\n' localhost:9090/a/; done
# 前 5 个（burst）→ 200，其后 → 429；/b/ 消耗的是同一份预算
```

可运行的 [example/](example) 断言共享预算与持续补充；`example/check.sh` 用 docker compose
包裹执行。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-go-redis + starter-ratelimit-redis
  └─ gs.Module(gs.OnProperty("spring.ratelimit.redis"))
        └─ conf.BindEach("${spring.ratelimit.redis}") 逐条目 <name>：
             ├─ client == "" 时 fail fast（启动报错并点名实例）
             ├─ driver 名 = c.Driver，未设时用实例 <name>
             └─ Provide func(client) *Driver → bean "<name>"
                  （gs.TagArg(c.Client)；Export resilience.LimiterDriver——Export
                   让 bean 根可达，注册副作用必然执行）

gs.Run()
  ├─ 配置绑定：${spring.ratelimit.redis.<name>} → Config（value tag）
  ├─ ctor：driverFor(driver, client)
  │    - 进程级 sync.Map "drivers"：每个 driver 名一个 Driver
  │    - 首次使用：resilience.RegisterLimiter(name, d)——注册表对重名
  │      PANIC；后续装配（如同一测试二进制里的 gs.RunTest）改为重绑
  │      client 而不再 panic
  ├─ bean 装配：消费方 autowire:"<name>" 解析
  └─ SIGTERM：无需释放——注册表条目是进程级的，redis client 的
                 Close 属于 starter-go-redis。
```

Driver bean 本身是薄适配器：`NewRateLimiter(p)` 在调用时捕获已绑定的 client（RWMutex
保护），委托给 starter-go-redis 的 `experimental.NewRateLimiter`——Lua 令牌桶真正住在那里。

### 2.2 一次 Allow 的逐层走读

`LimitPolicy{Rate: 2, Burst: 5}` 下 `lim.Allow(ctx, "api")`：

1. `Rate` 为 0 时短路为无限放行且**不发生 Redis 往返**（配置丢失 = 不限流，属危险面）。
2. 原子 Lua 脚本完全在 Redis 内对 key `ratelimit:api`（`tokens` + `ts` 的 hash）执行：
   按 `流逝秒数 × rate` 补充并封顶 `burst`，够则扣一个令牌，`HSET` 新状态，并把 key
   `EXPIRE` 为 `ceil(burst/rate)+1` 秒（闲置 key 自删——废弃预算不泄漏）。
3. 返回 `1`/放行或 `0`/限流。状态在 Redis 里，因此同一 key 上的所有副本共享一份预算——
   这正是本 starter 的意义。
4. `Burst <= 0` 在构造 limiter 时默认为 `max(1, int(Rate))`。
5. Redis 故障时 `Allow` 返回错误——fail-open 还是 fail-closed 由消费方决定（example 答
   500；gateway 按它自己的策略）。

### 2.3 driver 不做的事

- `LimitPolicy.Algorithm` 现在**响亮报错**：策略要求的算法不是令牌桶（`""` 或
  `token-bucket`）时 `NewRateLimiter` 直接返回错误——此前 `SlidingWindow` 会被静默降级成
  突发特性完全不同的令牌桶。`Window` 仍被忽略（对令牌桶无意义，策略契约如此）。
- 副本时钟偏差影响补充公平性：`now` 由调用方传入脚本。
- 自身无指标——限流可观测（若有）在消费侧（gateway/resilience executor）呈现。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.ratelimit.redis.<name>` 之下（精确匹配，无宽松形态）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `client` | string | — | **必填**。`spring.go-redis.<client>` 下的 `*redis.Client` bean 名，注册前检查。`TagArg(c.Client)` 即 driver 与 redis 实例的接线 seam。 | 空 → 启动失败并点名实例；拼错 → 启动期装配失败。 |
| `driver` | string | 实例名 | 注册进 resilience limiter 注册表的名字——`resilience.GetLimiter` 与 gateway `rateLimit(driver=...)` 用的字符串。⚠ 不设时默认用实例名：`spring.ratelimit.redis.web.*` 会静默注册名为 `web` 的 driver，可能与无关名字冲突。重名现在启动期快速失败：本 starter 两个实例撞名 → 启动错误点名双方；名字已被其他模块注册（如内置 `default`）→ ctor 明确报错而非注册表 panic。 | 同名实例 → 启动报错点名双方；意外的默认名 → 消费方解析到计划外的 limiter。 |

⚠ 限流参数（`rate`、`burst`、key）由每个调用点的 `resilience.LimitPolicy` 给出，不在此
配置里。

---

## 4. 验证与故障演练

### 4.1 跨"副本"共享预算

```bash
for i in $(seq 1 10); do
  p=/a/; [ $((i%2)) -eq 1 ] || p=/b/
  curl -s -o/dev/null -w "%{http_code} " localhost:9090$p
done; echo   # 恰好 burst 个 200 在 /a/ 与 /b/ 间交错，其余 429
```

Redis 里的状态：

```bash
redis-cli hgetall ratelimit:api          # tokens + ts
redis-cli ttl ratelimit:api              # ceil(burst/rate)+1
```

### 4.2 burst + 补充演练

`Rate: 2, Burst: 5`：打光预算（5×200）后等 ~2.2 秒，确认持续补充（curl 循环 → 至少
`rate × 秒数` 个新 200）。example 自动化了这一步。

### 4.3 fail-open/fail-closed 演练

停掉 Redis（`docker compose stop redis`）：每次 `Allow` 返回错误。你的 handler 决定——
example 答 500（fail-closed）。按端点敏感度显式选择。

### 4.4 注册表查询演练

应用代码（即 gateway 过滤器内部做的事）：

```go
d, ok := resilience.GetLimiter("redis")   // 配置的 driver 名
lim, _ := d.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 50})
ok, err := lim.Allow(ctx, "tenant-a")
```

### 4.5 冒烟测试

```bash
cd example && ./check.sh    # docker 门控：compose 起 redis，跑自校验 example
```

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 `ratelimit-redis: instance "<n>" missing required property ...client` | 实例缺 `client` | 设为既有 `spring.go-redis.<name>`。 |
| 启动失败：driver 名 `x` 被实例 `a`、`b` 同时占用 | 两个实例解析到同一 driver 名（别忘了实例名默认） | 每个实例显式、互异的 `driver`（错误会点名双方）。 |
| `NewRateLimiter` 报 `no redis client bound` | driver bean 未走 client 注入路径构造 | 只经 starter 构造（或测试中先绑定再使用）。 |
| 完全不限流、全 200 | `LimitPolicy.Rate` 为 0 → 无限放行 | 显式设置正的 Rate。 |
| `NewRateLimiter` 对 `Algorithm: SlidingWindow` 报错 | 不支持的算法现在响亮拒绝（只有令牌桶） | 用 Rate/Burst 表达上限，或选支持滑动窗口的 driver。 |
| 出现 500 而不是 429 | Redis 不可达；`Allow` 报错被 handler 转成 500 | 按端点决定 fail-open/closed；修复 Redis。 |
| 各副本独立限流 | 实例指向不同 Redis client/key，或 Allow 的 key 串不同 | 副本间同 `client` + 同 Allow key。 |
| 测试二进制第二次装配 panic | 注册表注册每进程一次 | starter 对同名会重绑 client；避免用不同实例名注册同一 driver 串。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 2 |
| 其中必填 | 1（`client`） |
| quickstart 前置外部依赖 | 1（Redis） |
| "注意/坑"条数 | 4 |

设计嫌疑清单：

- 已解决（2026-08）：driver 重名不再 panic 也不再静默共享——本 starter 内两实例撞名 →
  启动错误点名双方；跨模块撞名（如 `default`）→ ctor 明确报错。
- 进程级 `drivers` sync.Map + 注册表使同一测试二进制内的行为依赖装配顺序（重绑只缓解
  client，不覆盖已注册集合）。
- 已解决（2026-08）：`Algorithm` 取非令牌桶值时 `NewRateLimiter` 直接报错，不再静默忽略。
