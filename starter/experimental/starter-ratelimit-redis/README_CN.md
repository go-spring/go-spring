# starter-ratelimit-redis

Go-Spring 的 Redis 分布式限流：向 resilience 注册表贡献
[`resilience.LimiterDriver`](../../cloud/governance/resilience/ratelimit.go)
实例，令牌桶状态放在 Redis 里，服务多副本共享同一个全局预算，而不是
每个进程各自限流。

## 功能

- 每个配置项 `spring.ratelimit.redis.<name>` 注册一个 limiter driver，
  复用 starter-go-redis 在 `spring.go-redis.<client>` 下发布的
  `*redis.Client` bean。
- 令牌桶算法（单条 Lua `EVAL` 原子完成补充+扣减、桶状态存 hash、按时间
  连续补充、key 带 TTL 防冷键堆积）就是 `starter-go-redis/experimental`
  里的实现；本 starter 只负责按 driver 名把它接进 resilience 限流器注册表。
- 按名注册，不是整体 driver 切换：resilience executor（熔断/重试/超时）
  仍用 `default`/`sentinel` driver，只有限流迁到 Redis。理由见
  [DESIGN_CN.md](DESIGN_CN.md)。

## 配置方式

```properties
# redis 客户端（starter-go-redis）
spring.go-redis.cache.addr=127.0.0.1:6379

# 每个配置项一个 driver；`client` 必填（缺省直接启动失败）。
spring.ratelimit.redis.gateway.client=cache
# 消费方按这个名字选用 driver；缺省等于实例名。
spring.ratelimit.redis.gateway.driver=redis
```

## 使用方式

注册表查找（starter-gateway 的 `rateLimit` filter 内部就是这么做的）：

```go
d, _ := resilience.GetLimiter("redis")
lim, _ := d.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 50})
ok, err := lim.Allow(ctx, "tenant-a")
```

或注入导出的 bean：

```go
gs.Provide(func(d resilience.LimiterDriver) *gs.HttpServeMux { ... },
    gs.TagArg("gateway"))
```

配合 starter-gateway 则纯配置切换：

```properties
spring.gateway.route.api.filters=rateLimit(rate=100,driver=redis)
```

## 边界

- 只有令牌桶：`LimitPolicy.Algorithm`/`Window` 被忽略——Lua 脚本建模的是
  令牌桶，没有别的。
- Redis 故障时每次 `Allow` 都返回错误——放行还是拒绝由消费方决定
  （gateway 选择放行）。
- 时钟：补充计算在 Redis 内部用调用方传入的 `now`（毫秒精度），副本间
  时钟偏移会直接影响公平性。

## 测试

`go test ./...` 用 miniredis 覆盖共享预算语义、key 隔离、`AllowN`
全有或全无、连续补充、并发放行数和桶 TTL。[example](example) 提供
docker 冒烟脚本（`example/check.sh`），两个"副本"共享一个预算打真实
Redis。
