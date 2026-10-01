# starter-ratelimit-redis

Go-Spring 限流阶段的 Redis 计数器后端：贡献一份
[`resilience.Counters`](../../../cloud/resilience/ratelimit.go) 计数器存储，
其令牌桶状态放在 Redis 里，于是服务多副本（replica）共享每个 scope 的同一个预算，
而不是每个副本各自计数。

## 功能

- 只贡献一个 `resilience.Counters` 类型的 bean，建立在
  [`spring.ratelimit.redis.client`](#配置方式) 所指名的 `*goredis.Client` bean 之上。
- 不需要让谁让位：容器里没有 `resilience.Counters` bean 时，每个 executor 用自己持有的一份
  预算计数（"一份预算覆盖该 label 的所有调用方"依然成立——manager 按 label 建一份 executor）。
  贡献本存储就是全部的开关：resilience driver 会注入它，它构建的每个 executor 从此花同一个
  共享预算——限流由计数器构成，而不由 executor 构成。改变的只是预算能扩多宽，覆盖范围不变。
- 计数本身（单条 Lua `EVAL` 原子完成补充+扣减、桶状态存 hash、key 带 TTL 防冷键堆积）就是
  `starter-go-redis/experimental` 里的实现；本 starter 只决定计数器放在哪里。
- executor/熔断/重试 driver 原封不动：超限请求是拒绝还是排队仍由 executor 决定。所以
  跨副本限流就是"跑起本 starter"，既没有按路由的开关，也没有按客户端的开关。

## 配置方式

```properties
# redis 客户端（starter-go-redis）
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# 唯一一个 key：哪个 *goredis.Client bean 提供共享计数器。必填——
# 值为空即装配失败。
spring.ratelimit.redis.client=cache
```

配置块存在即打开本 starter；只 import 未配置则什么都不贡献，每个 executor 保持自己的私有预算。限流参数
（`rate-limit`、`burst`、`algorithm`、`window`、`rate-limit-max-wait`）是治理规则文档上的
[`resilience.ClientPolicy`](../../../cloud/resilience/policy.go) 字段，不是 starter 配置。

## 使用方式

调用点无需任何接线：已经在限流的东西——各客户端 starter 的 executor、gateway 的 `rateLimit`
过滤器——在 starter 配置好后自动花这份存储。

```properties
# 过滤器只写预算，不写 driver：本 starter 跑起来后，预算是
# 所有 gateway 副本共享的。
spring.gateway.route.api.filters=rateLimit(rate=100)
```

自己构造 policy 的应用代码走同一个权威：

```go
exec := mgr.ClientExecutorFor("ratelimit-redis", "ratelimit-redis:api")
err := exec.Execute(ctx, func(context.Context) error { return nil })
// errors.Is(err, resilience.ErrRateLimited) → 429
```

## 边界

- sliding-window 的 scope 按令牌桶计数：共享预算的正确性来自那条原子脚本，而窗口无法廉价地
  套进一条脚本。计数器共享时请用令牌桶。
- 不提供排队：`resilience.ClientPolicy.RateLimitMaxWait` 被忽略，超限请求立即以 `ErrRateLimited`
  拒绝。等一个令牌意味着轮询 Redis；排队留给内存存储。
- Redis 故障不会拦住调用：executor 记录计数器存储失败后放行。
- 时钟：补充计算在 Redis 内部用调用方传入的 `now`（毫秒精度），副本间时钟偏移会直接影响
  公平性。

## 测试

`go test ./...` 用 miniredis 覆盖共享预算语义（两个存储对同一 Redis）、scope 隔离、批量
全有或全无、零速率直通、并发放行数和 key TTL，以及装配契约：配置后贡献 `resilience.Counters`
bean、未配置则不贡献，client 名为空时启动失败。[example](example) 提供 docker 冒烟脚本（`example/check.sh`），
两个"副本"共享一个预算打真实 Redis。
