# 设计：starter-ratelimit-redis

## 1. 问题

resilience 自带的 "default" limiter driver 把计数器放在进程内，N 个副本
就会放行 N 倍预算。共享配额要的是全局唯一预算，状态必须放到任何单个
进程之外——也就是 Redis。

## 2. 为什么走 limiter driver 注册，而不是切换 resilience driver

resilience 有两个彼此独立的注册表：

- `RegisterDriver`/`GetDriver`——构建 **Executor**（限流+熔断+重试+超时
  打包在一起）。整体切换会把熔断/重试一并从 `default`/`sentinel` 拖走，
  而没人想要这个：分布式限流与进程内熔断是正交的两件事。
- `RegisterLimiter`/`GetLimiter`——**LimiterDriver** 注册表，本来就是为
  "分布式限流"预留的接缝（见 ratelimit.go 注释），且已有按名消费方
  （gateway 的 `rateLimit(rate=…,driver=…)`）。

所以本 starter 按可配置的名字注册 `LimiterDriver`（缺省取实例名，多实例
天然不冲突），executor driver 原封不动。结果是 "sentinel executor +
redis limiter" 可以自由组合。

## 3. 复用，不重写

Lua 令牌桶已经存在于 `starter-go-redis/experimental`（单条 EVAL 原子
补充+扣减、hash 状态、`ratelimit:` key 前缀、TTL=ceil(burst/rate)+1s、
零速率直通）。在这里再写一份等于分叉脚本。因此本模块是薄薄的
Contributor starter：配置进，`resilience.LimiterDriver` bean + 注册表
条目出。代价是依赖 starter-go-redis 的 experimental 子包；该包若迁移，
本 starter 跟着走。

## 4. 接线

每个 `spring.ratelimit.redis.<name>`（gs.OnProperty + conf.BindEach）：

- `client`（必填，fail-fast）：starter-go-redis 的 bean 名，通过
  `gs.TagArg(client)` 按名注入——与 starter-session-redis 相同的按名
  接缝。
- `driver`（可选）：注册名，缺省等于实例名。

bean 构造函数调用 `driverFor(name, client)`：包级 `sync.Map` 里每个名字
一个 `*Driver` 单例。向 `resilience.RegisterLimiter` 的注册每进程每名字
只发生一次（注册表对重复注册会 panic）；容器重新接线——同一测试二进制
里第二次 `gs.RunTest`——只是换绑 client。bean 通过
`Export(gs.As[resilience.LimiterDriver]())` 挂到根上，构造（注册副作用）
在 prod 必然执行。

## 5. 继承自 Lua 桶的语义

- 每次 Allow/AllowN 一条 EVAL：读 hash → 按流逝毫秒补充 → 够则扣减 →
  写 hash → EXPIRE。没有客户端锁，N 个副本竞争同一个 key 也恰好只放行
  burst 个。
- `AllowN` 在脚本内全有或全无。
- key 形如 `ratelimit:<key>`；不同 key 各自独立预算。
- `Rate` 为零直接放行，不碰 Redis。
- 时钟：调用方以毫秒传 `now`；补充的正确性依赖副本间时钟大体一致
  （偏移影响公平性，不影响原子扣减的安全性）。

## 6. 失败姿态

后端错误以 `(false, err)` 从 Allow 冒出。本 starter 不决定放行还是
拒绝，由消费方决定（gateway 选择放行）。策略不进 driver，与"容器只
装配不裁决"的总纲一致——限流器是机制，响应方式由调用方选择。
