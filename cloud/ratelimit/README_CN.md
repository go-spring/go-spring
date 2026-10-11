# ratelimit

[English](README.md) | [中文](README_CN.md)

`ratelimit` 收纳 `go-spring.org/cloud` 各模块共享的限流计数算法 —— 弹性
stage（[cloud/resilience](../resilience/)）和任何进程内的
`resilience.ClusterLimiter` 存储（如网关的内存计数器）都构建在它之上。它是一个
叶子包：除标准库（以及负责 ctx 感知休眠的 `stdlib/timeutil`）之外零依赖，
调用方不会为了计数拖进任何弹性语义。

所有计数器都并发安全。

## 算法

一个算法一个文件，各自在平滑度与状态量之间取舍：

| 算法 | 文件 | 放行 | 平滑度 | 状态 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `TokenBucket` | `token_bucket.go` | 瞬时最多 `burst`，之后 `rate`/秒 | 完全平滑（连续补充） | 4 个数 | 默认选择；`WithWarmUp` 从 `burst/coldFactor` 冷启动爬坡容量（sentinel 风格）；`WaitN` 可排队等待到截止时刻 |
| `SlidingWindow` | `sliding_window.go` | 每个滚动 `window` 最多 `limit` | 窗口级（加权估算） | 2 个计数 | 用重叠比例加权上一个窗口；不用为每个事件存时间戳 |
| `FixedWindow` | `fixed_window.go` | 每个对齐窗口最多 `limit` | 边界处可能 2 倍突刺 | 1 个计数 | 窗口边界落在时钟整点倍数上，多实例预算无需协调即可对齐 |
| `LeakyBucket` | `leaky_bucket.go` | 恒定漏出速率下装得下 `capacity` 即放行 | 出流平滑 | 2 个数 | 令牌桶的对偶：按漏出限流，溢出即拒绝、不排队 |
| `GCRA` | `gcra.go` | `rate`/秒，容忍 `burst` | 完全平滑（虚调度） | 1 个时间戳 | 令牌桶化为单个"理论到达时刻"——迁移到原子存储脚本（Redis Lua）时每个 key 状态最少 |

选型经验：

- 默认用 `TokenBucket`（resilience 的 `token-bucket` 算法就跑在它上面，
  含 warm-up）。
- 要每窗口硬上限？`SlidingWindow`；能接受边界突刺且在意状态量，
  用 `FixedWindow`。
- 要严格的恒定出流，或存储脚本里每个 key 只想存一个数？分别选
  `LeakyBucket` / `GCRA`。

## 安装

```
go get go-spring.org/cloud
```

## 用法

```go
import "go-spring.org/cloud/ratelimit"

// 每秒 100、突发 200、30 秒 warm-up 的预算
b := ratelimit.NewTokenBucket(100, 200).WithWarmUp(30*time.Second, 3)
if !b.AllowN(1) {
    // 超预算 —— 立即拒绝
}

// 排队形式：最多等 50ms 拿不到令牌再拒绝
ok := b.WaitN(ctx, 1, 50*time.Millisecond)
```

每秒硬上限：

```go
w := ratelimit.NewSlidingWindow(100, time.Second)
if !w.AllowN(1) { /* 超上限 */ }
```

## 设计

- **叶子包、不设接口。** 各计数器的方法集刻意不同
  （`TokenBucket.AllowN(float64)` 外加 `WaitN`，两个窗口是
  `AllowN(int)`）；如今的消费方（resilience 的 `newRateState`、网关的
  `scopeState`）都在构造时一次性选定具体类型，公共接口只会把这个差异
  削平。真出现按算法多态的调用点时，由它在自己的包里声明所需的单方法
  接口即可。
- **一个算法一个文件**（外加 `doc.go`），测试紧随其后 —— 与 `cloud/`
  其余包的布局约定一致。
- **resilience 的 `Algorithm` 配置项**目前只接入 `token-bucket` 和
  `sliding-window`；`FixedWindow`、`LeakyBucket`、`GCRA` 是包的储备
  能力，尚未成为执行器 stage。
