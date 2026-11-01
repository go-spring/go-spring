# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` 收纳 `go-spring.org/cloud` 各模块共享的限流计数算法 —— 弹性
stage（[cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience/)）和任何进程内的
limiter（如网关的内存计数器）都构建在它之上。计数代码本身零依赖：除标准库
（以及负责 ctx 感知休眠的 `stdlib/timeutil`）之外不需要任何东西，调用方不会
为了计数拖进弹性语义；`starter.go` 只负责把内置工厂注册成容器 bean。

规则里写的是要用的**算法**——bean 名，例如 `token-bucket`（内置、
进程内）或 `redis:token-bucket`（共享 store）。[Factory] 为一个 scope 创建
计数器，[Limiter] 就是那个计数器。

所有计数器都并发安全。

## 算法

一个算法一个文件，各自在平滑度与状态量之间取舍：

| 算法 | 文件 | 放行 | 平滑度 | 状态 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `TokenBucket` | `token_bucket.go` | 瞬时最多 `burst`，之后 `rate`/秒 | 完全平滑（连续补充） | 4 个数 | 默认选择；`rate-limit-warm-up-duration` 从 `burst/coldFactor` 冷启动爬坡容量（sentinel 风格）；超限的扣减会先预留、最多睡一次到截止时刻；`n > burst` 立即拒绝 |
| `SlidingWindow` | `sliding_window.go` | 每个滚动 `window` 最多 `limit` | 窗口级（加权估算） | 2 个计数 | 用重叠比例加权上一个窗口；不用为每个事件存时间戳 |
| `FixedWindow` | `fixed_window.go` | 每个对齐窗口最多 `limit` | 边界处可能 2 倍突刺 | 1 个计数 | 窗口边界落在时钟整点倍数上，多实例预算无需协调即可对齐 |

选型经验：

- 默认用 `TokenBucket`（resilience 的 `token-bucket` 算法就跑在它上面，
  含 warm-up）。
- 要每窗口硬上限，而不是持续补充的预算？用 `SlidingWindow`；能接受边界突刺
  且在意状态量，用 `FixedWindow`。

## 安装

```
go get go-spring.org/cloud
```

## 用法

```go
// factory：容器里的 Factory —— 进程内 provider + 各 starter 贡献的 store
lim, err := factory.Create("token-bucket", "checkout") // 或 "sliding-window"、"fixed-window"、"redis:token-bucket"
ok, err := lim.Allow(ctx, ratelimit.Policy{RateLimit: 100, Burst: 200}, 1)
```

## 设计

- **同一个调用形状，两种状态位置。** [Provider] 为一个算法与 scope 造出
  [Limiter]，该 limiter 只提供 `Allow(ctx, policy, n)`：进程内 provider 把状态放在内存，
  共享 store 把状态放在进程外。规则随每次调用传入，所以配置变化是**就地更新**
  计数器，而不是重建它。
- **每个 provider 导出它能创建的算法。** 集合由 `Provider.Algorithms()` 给出：
  进程内 provider 返回纯算法名，store 加自己的前缀（`redis:token-bucket`）。
  `Factory` 只持有收集到的 providers；`Create` 时扫描它们找到导出该算法
  的 provider，没有额外需要同步的注册表。Factory 是两个方向的**必需依赖**。
  规则点名一个没有任何 provider 导出的算法，在文档 apply 时被拒绝。
- **一个进程内计数器只对应一种算法。** 算法由进程内 provider 的 `New` 固定；
  状态在首次调用时按 `Policy` 创建。数值变化就地更新；只有 sliding window
  的宽度变化才从新状态开始。
- **一个算法一个文件**，旁边是 `factory.go`（算法名、`Limiter`、`Provider`
  接缝与进程内 provider）和 `policy.go`；测试紧随其后 —— 与
  `cloud/` 其余包的布局约定一致。
- **非法配置在边界被拒绝。** `Policy` 字段自带 `expr` 约束，`conf.Bind` 会
  拒绝负 rate/burst/window；程序化构造的 `Policy` 由调用方自己保证。
- **时间只在测试里被伪造。** 计数器直接调用 `time.Now` / `timeutil.Sleep`；
  测试在 `testing/synctest` 的气泡里运行，时间是跳过去的，不是等出来的。
- **规则的 `limiter` 配置项**解析 `token-bucket`、`sliding-window` 与
  `fixed-window`。这就是全部进程内算法：没有消费者的算法不进这个包。
