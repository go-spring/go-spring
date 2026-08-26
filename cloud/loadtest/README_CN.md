# loadtest
[English](README.md) | [中文](README_CN.md)

`loadtest` 是各 client starter 的 example-load 程序共用的压测 harness。
它在可插拔的调度 `Driver` 下、限定的时长内驱动一个 `Op`,记录时延与分类
后的错误分布,再跑健康 `Assert` 断言,让一次压测能给出裁定 —— "保护栈扛
住了、信号没丢" —— 而不只是产出一堆数字。它是 example 级工具,不是生产
代码。

## 快速开始

传统一行式(闭环、N 个 worker):

```go
res := loadtest.Run(ctx, loadtest.Config{Concurrency: 50, Duration: 30 * time.Second}, op)
res.Print(os.Stdout)
```

完整形态是 `Runner` builder:

```go
res := loadtest.New().
    Driver(loadtest.Ramp(100, 1000, time.Minute, 500)). // 开环线性爬坡
    Duration(2 * time.Minute).
    CaptureGC(true).
    Assert("qps-floor", loadtest.AssertMinQPS(500)).
    Assert("error-ceiling", loadtest.AssertErrorRateBelow(0.01)).
    Assert("p99", loadtest.AssertP99Below(200*time.Millisecond)).
    Run(ctx, op)
if !res.Passed() { /* 裁定 FAILED */ }
res.Print(os.Stdout)
```

## 四个 seam

- **`Driver` 决定操作何时发。** `Drive` 收到的 `invoke` 已自带计时与记
  录,driver 只管节奏与并发,且必须等它启动的所有 invoke 完成后才能返
  回。内置:
  - `ClosedLoop{Concurrency: N}` —— N 个 worker 尽快循环;吞吐随时延
    自限("N 个并发用户"模型)。
  - `OpenLoop(rps, maxConcurrent)` —— 固定到达率、与时延无关;暴露闭
    环自节流藏住的尾延迟行为。
  - `Ramp(from, to, over, max)` —— RPS 线性爬坡后保持。
  - `Staircase(max, steps...)` —— `Step{RPS, Duration}` 阶梯。
  - `Scheduled(schedule, max)` —— 任意 `Schedule`
    (`func(elapsed) float64`),可建模突发/昼夜/回放形态;返回 `<= 0`
    暂停派发。
  - 自定义 `Driver` 可建模任意流量形状;报告里默认用 `%T` 类型名。
- **`Schedule`** 是开环家族背后的步调函数;见 `ConstantSchedule`、
  `LinearRamp`、`StepSchedule`。
- **`Assert` 定义"扛住"的标准。** `func(ctx, *Result) error`,nil 即通
  过。内置:`AssertMinQPS`、`AssertErrorRateBelow`、`AssertP99Below`、
  `AssertGCPauseAvgBelow`(需 `Runner.CaptureGC(true)`)。`Result` 或
  runtime 能读到的都可以自己断 —— "熔断开过"、"metric registry 拿到
  了预期样本数"、"无 goroutine 泄漏"。
- **`Classify` 决定错误如何入桶。** `func(error) string`;默认
  (`DefaultClassify`)用 resilience + fault 分类法。返回 `""` 计入
  `other`。

## 错误桶

`Result.Buckets` 把每个错误标签映射到计数。内置标签(常量,
  `Result.Print` 固定按此顺序输出,保证报告可 diff):

| 常量 | 标签 | 含义 |
|---|---|---|
| `BucketCircuit` | `circuit` | `resilience.ErrCircuitOpen` |
| `BucketRateLimited` | `rate-limited` | `resilience.ErrRateLimited` |
| `BucketBulkhead` | `bulkhead` | `resilience.ErrBulkheadFull` |
| `BucketInjected` | `fault-injected` | `fault.IsInjected` |
| `BucketOther` | `other` | 其余全部 |

自定义 `Classify` 的标签进同一张 map;`Print` 先按上列顺序输出五个内置
桶,再按字典序输出自定义标签。

## 压测流量标记

每个操作的 context 在派发前经 `traffic.WithLoadTest` 打标,全链路下游
client 都能识别合成流量(`traffic.IsLoadTest`) —— 影子表路由、fault
注入范围、指标标注都以此为准。

## 安装

```
go get go-spring.org/cloud
```
