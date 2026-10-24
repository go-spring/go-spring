# circuitbreaker

[English](README.md) | [中文](README_CN.md)

`circuitbreaker` 是**方向中立的熔断器**：open / half-open / closed 状态机本身，不含任何一方的策略。

目前由出站容错引擎（[outbound/resilience](../../outbound/resilience/)）消费；状态机留在 kernel，是因为它是机制而不是某个方向的策略。调用方从自己的策略拼装出 [`Spec`](#spec)，本包不知道被保护的是一个调用还是一个请求。

本包是叶子包：只依赖标准库。

## 工作原理

一个正在恶化的依赖，会因为调用方持续猛打而更糟：每个新调用都占用一条调用方线程、一个连接和一个对方根本服务不过来的超时，于是“一个慢组件”演变成“一场级联故障”。熔断器通过**快速失败**斩断这个循环：一旦策略判定单元不健康，就不再放行调用，下游获得喘息恢复的时间，调用方拿到一个即刻、廉价的错误，而不是昂贵的超时。

它是一个三态状态机：

```
Closed ──达到阈值──▶ Open ──冷却结束──▶ HalfOpen
   ▲                   │                    │
   └──全部试探成功──────┘────第一个试探失败────┘
                     (重新打开，冷却重新计时)
```

- **Closed（闭合）**——正常状态：调用放行，每个结果都喂给计数策略（`StrategyConsecutive`、`StrategyErrorRate`、`StrategySlowCall`、`StrategyErrorCount`）。
- **Open（打开）**——熔断状态：调用立即拒绝，持续 `Spec.OpenDuration`。熔断前放行的调用产生的迟到结果会被忽略——它描述的是一个已不存在的状态。
- **HalfOpen（半开）**——试探状态：冷却结束后放行有界的试探调用（`Spec.HalfOpenTrials` 个）探测下游是否恢复。全部成功才闭合；第一个失败立即重新打开、冷却重新计时。半开轮次按 trial 的结果决定，不按时间闭合：低流量下保持半开，每个新请求都作为下一次试探（与 resilience4j / gobreaker 一致）。需要防止试探丢失时，可配置 `Spec.HalfOpenTimeout`。

## Spec

`Spec` 是熔断器自己的词汇——构建一个熔断器所需的参数。消费方从自己的策略拼装它，状态机本身永远看不到那套策略词汇。

字段未设置时的结构性默认值：`MinSamples` 1、`Window` 1s、`OpenDuration` 5s、`HalfOpenTrials` 1、`HalfOpenTimeout` 无限等待。策略层在调用 `New` 之前可能套用自己的主流默认值。

### 策略

每个熔断器**只跑一种**计数策略（`Strategy` 是单值，不是列表）。唯一事实上的组合是：`StrategyErrorRate` 配上慢调用对（`SlowThreshold` + `SlowRate`）时，按**错误率或慢比率**触发——见下面 `StrategyErrorRate`。

这与主流一致：Resilience4j 同样是单熔断器单策略、错误或慢的组合内置在比率策略里，gobreaker 更是只有连续失败一种。确实组合多种策略的框架（Sentinel、Envoy）是在同一被保护单元上叠加**多个独立熔断器实例**，而不是在单个熔断器内叠加策略——那是 policy 层的选项，无需改动本包。

#### `StrategyConsecutive`——连续 N 次失败

从上次成功起累计失败；连续 `FailureThreshold` 次失败即触发，任意一次成功就把计数清零。

- 参数：`FailureThreshold`。
- 适用：依赖平时健康、出问题就突然死掉（进程崩溃、连接拒绝）——最简单也最常用。
- 注意：一次侥幸成功会掩盖持续故障；`失败,成功,失败,成功` 交错进行时，哪怕失败率 50% 也永不触发。

```go
circuitbreaker.Spec{Strategy: circuitbreaker.StrategyConsecutive, FailureThreshold: 5}
```

#### `StrategyErrorRate`——窗口内 bad/total 比率

维护 `Window` 内 bad/total 的滚动估计；窗口的估计样本数达到至少 `MinSamples` 且比率达到 `FailureRate` 时触发。配置慢调用对后增加一条独立臂：耗时**严格大于** `SlowThreshold` 的调用（成功或失败）计入慢率，慢率达到 `SlowRate` 时同样触发。这防住的是“能用但每次都挂住”的依赖。

- 参数：`FailureRate`、`Window`、`MinSamples`，可选 `SlowThreshold` + `SlowRate`。
- 适用：流量平稳、关心失败*比例*；或慢而成功的调用与失败同样有害（面向用户的时延预算）。
- 注意：低流量时窗口内样本很少，`MinSamples` 很关键——kernel 默认 1（第一个失败即触发），策略层默认 5。

```go
circuitbreaker.Spec{
    Strategy:      circuitbreaker.StrategyErrorRate,
    FailureRate:   0.5, MinSamples: 20, Window: time.Minute,
    SlowThreshold: 2 * time.Second, SlowRate: 0.8, // 错误或慢任一触发
}
```

#### `StrategySlowCall`——窗口内慢比率

与 `StrategyErrorRate` 类似，但**只有慢**是触发信号：`Window` 内慢调用（耗时**严格大于** `SlowThreshold`）占比达到 `SlowRate` 即触发。超过阈值的调用无论成功失败都计入慢率分子；快速失败只进分母，会稀释比率但永远不会触发。

- 参数：`SlowThreshold` + `SlowRate`、`Window`、`MinSamples`。
- 适用：防的是劣化而不是报错——依赖能成功但要花好几秒。
- 注意：依赖开始**快速**失败（1ms 内连接拒绝）时慢调用熔断器永不触发；两种失败模式都可能发生时，与基于错误的熔断器搭配使用。

```go
circuitbreaker.Spec{
    Strategy:      circuitbreaker.StrategySlowCall,
    SlowThreshold: time.Second, SlowRate: 0.5, Window: time.Minute, MinSamples: 10,
}
```

#### `StrategyErrorCount`——窗口内失败数

`Window` 内失败的**数量**（不是比率）达到 `FailureThreshold` 即触发，中间夹多少成功都无所谓。

- 参数：`FailureThreshold`、`Window`。
- 适用：受伤的是失败的绝对数量（每次失败泄漏资源、触发告警或花钱），关心绝对数而非比例。
- 注意：高流量下健康的 0.1% 错误率也能攒够 `FailureThreshold` 个原始失败——通常比率策略是更好的默认。

```go
circuitbreaker.Spec{
    Strategy: circuitbreaker.StrategyErrorCount, FailureThreshold: 100, Window: time.Minute,
}
```

### 怎么选

`Strategy` 必须设置；kernel 不会根据阈值反向推断策略，留空会被拒绝。策略层在未配置时默认填 `StrategyConsecutive`，并可能在调用 `New` 之前用自己的主流默认值补齐缺失的跳闸条件。单独一组慢调用参数不会自动切换到 `StrategyErrorRate`，需要显式指定策略。

| 要防的症状 | 策略 |
| --- | --- |
| 依赖突然死掉 | `StrategyConsecutive` |
| 失败比例升高，或既报错又挂住 | `StrategyErrorRate`（+ 慢调用对） |
| 时延劣化，报错无所谓 | `StrategySlowCall` |
| 失败的绝对数量才要紧 | `StrategyErrorCount` |

## 用法

每个被保护单元（一个服务或一个实例）构建一个 `Breaker`，可并发使用：

```go
spec := circuitbreaker.Spec{
    Strategy:     circuitbreaker.StrategyErrorRate,
    FailureRate:  0.5,
    MinSamples:   20,
    Window:       time.Minute,
    OpenDuration: 10 * time.Second, // 熔断后的冷却时间
}
b, err := circuitbreaker.New(spec)
if err != nil {
    return err
}
```

包住每个被保护的调用：

```go
permit, ok := b.Allow()
if !ok {
    // Open（或 half-open 且试探名额用尽）：在这里快速失败——
    // 返回降级结果、缓存副本或"稍后再试"。不要忙等重试：
    // 打开熔断的目的就是让依赖安静一会儿。
    return ErrCircuitOpen
}
start := time.Now()
err := doWork()
b.Record(permit, err == nil, time.Since(start)) // 时长喂给慢调用逻辑
```

`Allow` 返回的 `Permit` 把这次结果绑定到放行它的那个状态。把它原样交还给
`Record`，且恰好一次。若放行后熔断器已离开该状态——熔断前放行的调用，或已作废
轮次里的试探——对应的 `Permit` 会被忽略：既不能闭合、重新打开，也不能延长冷却期
或污染旧窗口。

没有裁决的结果——调用方在拿到单元结果之前取消——交给 `RecordIgnored`：
它不喂任何计数器；在 HalfOpen 下归还试探许可，让本轮继续等待真正的裁决。

触发之后会发生什么：

- **Open 期间**所有 `Allow` 被拒绝，持续 `Spec.OpenDuration`。
- **HalfOpen 期间**最多放行 `HalfOpenTrials` 个试探。全部成功才闭合；第一个失败立即重新打开、冷却重新计时。不存在按时间闭合：成功结果按流量到达的节奏累计；`HalfOpenTimeout`（默认无限等待）只负责把始终不落定的试探轮重新打开。
- 关闭的熔断器就是 **holder 手里的 nil `*Breaker`**——与 `*Bulkhead` 相同的 OFF 约定：holder 判空后自己放行，不构造实例，关闭该层在运行期零开销。**存在**的熔断器即开启状态，必须完整：选中了策略却缺少必要阈值属于构造错误，不会被静默当成关闭。熔断器必须经 `New` 构建，零值不可用。

## 状态读取

熔断器自己不发任何事件。`Stats()` 在熔断器自己的锁下返回只读快照：当前状态以及进入 Open/HalfOpen/Closed 的单调计数。持有它的治理器官在自己的采集路径上读取并**在自己的包里**发布——本包不 import 任何 observability SDK，保持内核叶子属性。
