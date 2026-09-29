# 设计：starter-ratelimit-redis

## 1. 问题

进程内计数把 scope 的预算放在进程里，N 个副本就各自放行 N 倍预算。共享配额要的是状态
放在任何单个进程之外——也就是 Redis。

## 2. 为什么走计数器存储，而不是整体切换 driver

resilience 有两条接缝，回答的是两个不同的问题：

- `Driver` 回答 *用哪个引擎* 跑一条 policy——它把 **Executor**（限流+熔断+重试+超时）作为
  一个整体构建。从这条接缝去接 Redis，会把熔断和重试一并从 `default`/`sentinel` 拖走，而
  没人想要这个：分布式限流与进程内熔断是正交的两件事。
- `Counters` 回答 *计数器放在哪里*——限流阶段自己的接缝（`Allow(ctx, scope, policy, n)`）。
  它换掉限流所依赖的状态，却不碰由哪个引擎执行。

存储是可选的，由容器挑选：容器里没有 `resilience.Counters` bean 时，driver 让每个 executor
用自己持有的一份预算计数；贡献了存储的后端 starter——也就是本模块——则让 driver 构建的每个
executor 都花那一份，自带的 "default" 与任何别的后端一样，因为接线把存储注入到 driver bean
上。结果是 "sentinel executor + redis counters" 可以自由组合，也不存在会被忘掉的按路由或
按客户端开关。

## 3. 复用，不重写

Lua 令牌桶已经存在于 `starter-go-redis/experimental`（单条 EVAL 原子补充+扣减、hash 状态、
`ratelimit:` key 前缀、TTL=ceil(burst/rate)+1s、零速率直通）。在这里再写一份等于分叉脚本。
因此本模块是薄薄的 Contributor starter：配置进，一个 `resilience.Counters` bean 出。代价是
依赖 starter-go-redis 的 experimental 子包；该包若迁移，本 starter 跟着走。

## 4. 接线

一个配置块 `${spring.ratelimit.redis}`，由
`gs.Module(gs.OnProperty("spring.ratelimit.redis"), setup)` 武装，因此只 import 未配置的
starter 什么都不贡献：

- `client`（必填，fail-fast）：starter-go-redis 的 bean 名，通过 `gs.TagArg(client)` 按名
  注入——与其他消费 redis 的 starter 相同的按名接缝。

bean 由下面这段贡献：

```go
r.Provide(func(client *goredis.Client) (resilience.Counters, error) {
    return experimental.NewCounters(client.UniversalClient)
}, gs.TagArg(c.Client)).Caller(1)
```

返回接口类型正是让 bean 索引在 `resilience.Counters` 下的原因：gs 取构造函数第一个返回类型
作为 bean 类型，因此不需要 `Export(gs.As[...])`——而这恰好也是 driver 注入参数所匹配的类型。
必须用 gs.Module（而不是普通 bean 构造函数），因为配置块存在与否才是开关：
`gs.Module(gs.OnProperty("spring.ratelimit.redis"), setup)` 门控这次贡献，`setup` 先绑定
该配置块再贡献 bean。

## 5. 继承自 Lua 桶的语义

- 每次 Allow 一条 EVAL：读 hash → 按流逝毫秒补充 → 够则扣减 → 写 hash → EXPIRE
  ceil(burst/rate)+1s。没有客户端锁，N 个副本竞争同一个 key 也恰好只放行 burst 个。
- `n` > 1 在脚本内全有或全无。
- key 形如 `ratelimit:<scope>`；不同 scope 各自独立预算。
- `RateLimit` 为零直接放行，不碰 Redis。
- 时钟：调用方以毫秒传 `now`；补充的正确性依赖副本间时钟大体一致（偏移影响公平性，不影响
  原子扣减的安全性）。

## 6. 明示的边界

- sliding-window 的 scope 按令牌桶计数。共享预算的正确性来自那条原子脚本，而窗口无法廉价地
  套进一条脚本——存储宁可明说不支持，也不悄悄换一套突发特性去作答。
- 不提供排队：`Policy.RateLimitMaxWait` 被忽略，超限的单位立即被拒绝。等一个令牌意味着轮询
  Redis；排队留给内存存储。

## 7. 失败姿态

Redis 不可达时存储返回自己的错误，executor 的限流阶段记录该错误后放行——计数器后端坏掉不该
变成一次故障。拒绝始终由 executor 决定（`ErrRateLimited` → 调用方 429）。放行/拒绝的选择不
放进存储，与"容器只装配不裁决"的总纲一致：存储是机制，响应方式属于调用方。
