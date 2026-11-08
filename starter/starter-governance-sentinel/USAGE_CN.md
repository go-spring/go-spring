# 使用说明 — starter-governance-sentinel

本模块把 sentinel-golang 变成出站 resilience 引擎的一个**限流算法**，不是后端选择，
也不是第二套 executor。

## 1. 导入

```go
import _ "go-spring.org/starter-governance-sentinel"
```

导入时：

- 调用 `sentinel.InitDefault()` —— 环境异常会在启动时 panic；
- 注册一个 `ratelimit.Provider`，导出算法名 `sentinel`。

## 2. 选中算法

治理文档里（可热更新）：

```properties
spring.governance.rate-limit.services[0].service=orders
spring.governance.rate-limit.services[0].action=-
spring.governance.rate-limit.services[0].limiter=sentinel
spring.governance.rate-limit.services[0].rate-limit=100
spring.governance.rate-limit.services[0].window=1s
```

`limiter` 就是算法名。进程里任何 provider 都可以导出它；本模块增加的是 `sentinel`。
选中它只改变 flow 决策——timeout、retry、熔断、匹配仍由引擎负责。

程序化使用：

```go
fact, _ := ratelimit.NewFactory([]ratelimit.Provider{
	ratelimit.NewLocalProvider(),
	StarterGovernanceSentinel.Provider{},
})
mgr := resilience.NewManager(fact)
mgr.ApplyRateLimit(resilience.RateLimitSettings{Default: ratelimit.Policy{
	Limiter:   StarterGovernanceSentinel.LimiterAlgorithm,
	RateLimit: 100,
}})
exec := mgr.ExecutorFor("orders")
```

## 3. 映射关系

| 中立旋钮 | Sentinel 规则字段 |
| --- | --- |
| `RateLimit` | `Threshold`（每秒，归一到 `StatIntervalInMs`） |
| `Window` | `StatIntervalInMs` |
| `MaxWait` | `ControlBehavior = Throttling`、`MaxQueueingTimeMs` |
| `WarmUp` / `ColdFactor` | `TokenCalculateStrategy = WarmUp`（+ 周期 / 冷启动系数） |

预热优先于排队：sentinel 只允许 WarmUp 搭配 Reject。

不映射（也不伪造）：`Burst`（sentinel 没有突发额度）、本地算法 kind（本 provider
就是该算法），以及所有非 flow 阶段（timeout / retry / 熔断 / 匹配归引擎）。

## 4. 排障

**`ratelimit: unknown algorithm "sentinel"`** —— 模块没被链接。provider 由 sentinel
starter 的 `init` 注册；需要空导入（或导入一个链接了它的包）。

**`panic: starter-governance-sentinel: sentinel init failed`** —— sentinel 自身环境
有问题（通常是错误的 `SENTINEL_*` 配置）。修环境；框架拒绝带着半初始化的 SDK 启动。

**两条规则看起来共用了一个限额** —— 引擎按规则身份分桶，不是按 service。给每条规则
不同的 `action`（或者使用默认规则，它的 scope 就是 service）。

**`Burst` 没效果** —— 预期行为：sentinel 的 flow 规则没有突发项。用
`RateLimit` + `Window`（需要排队时加 `MaxWait`）。

## 5. 测试

- `limiter_test.go` 端到端穿过 resilience 引擎：策略写 `sentinel` 时先放行预算，
  超额返回引擎的中立 `ErrRateLimited`。
- 容器测试证明导入期注册确实进入了进程的 limiter factory。
