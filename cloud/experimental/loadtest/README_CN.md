# loadtest
[English](README.md) | [中文](README_CN.md)

`loadtest` 是各 client starter 的 example-load 程序共用的压测 harness。
它在可插拔的调度 `Driver` 下、限定的时长内驱动一个 `Op`,记录时延与分类
后的错误分布,再跑健康 `Assert` 断言,让一次压测能给出裁定 —— "保护栈扛
住了、信号没丢" —— 而不只是产出一堆数字。它是 example 级工具,不是生产
代码:需要统计严谨性的场合请用真正的压测工具。

可运行、自校验的独立演示在 [example](example/example.go)
(`./example/check.sh` 退出码 0)。

## 快速开始

一行式(闭环、N 个 worker):

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

`Op` 是 `func(ctx) error`——成功返回 nil;由各 starter 的 main 提供具体客
户端调用。每个 op 的 context 都打了压测流量标记(`traffic.WithLoadTest`),
下游客户端据此识别合成流量(`cloud/governance/traffic` 的影子表路由、故障
注入范围、指标打标都依赖它)。

## 选 driver

`Driver` 决定 op **何时**触发;计时与记录由 runner 负责,driver 只做调度。

| Driver | 形状 |
|---|---|
| `ClosedLoop{Concurrency: N}` | N 个 worker 循环、op 返回即再发;吞吐自限("N 个并发用户") |
| `OpenLoop(rps, maxConcurrent)` | 固定到达速率、与延迟无关;暴露闭环自限流掩盖的尾部队列行为 |
| `Ramp(from, to, over, max)` | RPS 线性爬坡后保持 |
| `Staircase(max, steps...)` | `Step{RPS, Duration}` 阶梯 |
| `Scheduled(schedule, max)` | 任意 `Schedule`(`func(elapsed) float64`),突发/昼夜/回放形状;返回 `<= 0` 暂停发送 |

`Schedule` 积木:`ConstantSchedule`、`LinearRamp`、`StepSchedule`。自定义
`Driver` 可建模任意形状;契约只有两条——每个调度的 op 调一次 invoke,且所
有已启动的 invoke 结束后才返回(不丢尾部时延)。

## 给出裁定

`Assert` 是 `func(ctx, *Result) error`,nil 即通过。内建:
`AssertMinQPS`、`AssertErrorRateBelow`、`AssertP99Below`、
`AssertGCPauseAvgBelow`(需 `Runner.CaptureGC(true)`)。自定义断言可读
`Result` 与 runtime 的一切——"熔断器开过"、"指标注册表有了样本"、"无
goroutine 泄漏"。

不注册断言的 run 无从失败(`Passed()` 恒 true)。

## 错误分桶

`Result.Buckets` 把每个错误标签映射到计数。内建标签(`Result.Print` 固定
按此顺序输出,报告可 diff):

| 常量 | 标签 | 含义 |
|---|---|---|
| `BucketCircuit` | `circuit` | `resilience.ErrCircuitOpen` |
| `BucketRateLimited` | `rate-limited` | `resilience.ErrRateLimited` |
| `BucketBulkhead` | `bulkhead` | `resilience.ErrBulkheadFull` |
| `BucketInjected` | `fault-injected` | `fault.IsInjected` |
| `BucketOther` | `other` | 其余全部 |

`Classify`(`Runner.Classify`)替换默认分类器(`DefaultClassify`)以增加自
定义桶——比如把 "other" 拆成按状态码的分布;自定义标签按字母序排在内建
之后。返回 `""` 计入 `other`,自定义分类器不会丢错误。

## 边界

单锁 recorder、`time.Sleep`  pacing、阈值判定而非置信区间——example 规模
下的合理取舍,不是基准测试工具。

## 安装

```
go get go-spring.org/cloud
```
