# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` 收纳 `go-spring.org/cloud` 各模块共享的限流计数算法 —— 弹性
stage（[cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience/)）和任何进程内
limiter（如网关的内存计数器）都构建在它之上。计数代码零依赖。规则里写的是
要用的**算法**；[Factory] 为一个 scope 创建计数器；[Limiter] 就是那个计数器。
所有计数器都并发安全。

## 算法

### TokenBucket —— 默认选择

`token_bucket.go`，移植自 Guava 的 `SmoothRateLimiter`。以 `rate-limit`/秒
连续平滑放行：每次扣减在时间轴队列的队头放行、成本记到下一个请求头上，
实际速率收敛于配置值，没有窗口边界。

- **`rate-limit`** —— 持续预算，单位/秒。唯一必填项；0 表示关闭本 stage。
- **`burst`** —— 节奏之上的免费存量，排队前先花它。从空开始，只在空闲时
  回补。默认约为速率一秒的量。想无等待吸收多大的瞬时尖峰，就配多大。
- **`rate-limit-max-wait`** —— 一次扣减最多排队多久。0 拒绝不等待（纯拒绝）；
  正值把突发变成有界等待（流量整形）。配成调用方能接受的真实延迟预算。
- **`rate-limit-warm-up-duration`** —— 冷启动保护（Guava
  `SmoothWarmingUp`）：桶满冷启动，存量高于暖阈值期间令牌最贵卖到
  `warm-up-cold-factor`（默认 3）倍稳态价，随流量抽干存量逐步降价。进程
  刚重启或闲置变冷后不希望以全量配额冲击冷下游（连接池、数据库、对端服务）
  时开启。容量由预热时长推导；开启期间 `burst` 被忽略。

**什么时候选它**：几乎总是。三个算法里只有它做平滑而非封顶，也只有它有
突发容忍和预热。

### SlidingWindow —— 滚动窗口硬上限

`sliding_window.go`。每个滚动 `window` 最多 `rate-limit × window` 个单位，
用上一个窗口按剩余重叠比例加权来估算。不为每个事件存时间戳，只有两个计数。

- **`rate-limit`** 配 **`window`**（默认 1s）—— 上限是
  `rate-limit × window`；如 100/s 配 10s 窗口 = 每滚动 10 秒 1000 个。
- 估算假设事件在窗口内均匀分布，边界刚过时计数可能略偏高或偏低。

**什么时候选它**：要的是"每窗口不超过 N"的硬预算（配额核算），而不是
平滑速率。

### FixedWindow —— 最便宜的封顶

`fixed_window.go`。每个**对齐**窗口最多 `rate-limit × window`，窗口边界落在
时钟整点倍数上，多实例预算无需协调即可对齐。只有一个计数。

- 参数同 SlidingWindow。弱点是边界突刺：客户端可以在一个窗口末尾花光
  全部配额、又在下一个窗口开头再花一份 —— 近乎瞬间最多 2 倍上限。

**什么时候选它**：要按窗口对齐封顶、能接受边界突刺、且状态量必须最小的
场景。

## 用法

```go
// factory：容器里的 Factory —— 进程内 provider + 各 starter 贡献的 store
lim, err := factory.NewLimiter("token-bucket", "checkout") // 或 "sliding-window"、"fixed-window"、"redis:token-bucket"
ok, err := lim.Allow(ctx, ratelimit.Policy{RateLimit: 100, Burst: 200}, 1)
```

非法配置在边界被拒绝：`Policy` 字段自带 `expr` 约束，`conf.Bind` 会拒绝
负 rate/burst/window；规则点名一个没有任何 provider 导出的算法，在文档
apply 时被拒绝。

设计说明保留在代码注释里 —— provider 缝在 `factory.go`，队列模型在
`token_bucket.go`。
