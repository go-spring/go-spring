# loadtest 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`loadtest` 是各 client starter 的 example-load 程序背后的共享 harness。
它存在的目的:让"压一轮、裁定保护栈是否扛住"成为一个可复用的程序,而不
是每个 starter 各复制一份 main。

## 1. 职责与边界

- **做:** 在可插拔 `Driver` 下驱动 `Op`,记录时延 + 分类错误桶 + GC 摘
  要,跑注册的 `Assert`,打印可 diff 的报告。
- **不做:**
  - 不承诺生产级。example 级:单 mutex recorder、朴素 `time.Sleep` 步
    调、不做分布拟合。需要统计严谨性时请用专业压测工具。
  - 对操作本身零主张。`Op` 是 `func(ctx) error`;starter 特定的
    example main 提供具体 client 调用。

## 2. 四个 seam

- **`Driver` 管"何时",runner 管"做什么"。** `Runner.Run` 把 op 包成
  已自带计时与记录的 `invoke`,driver 因此只剩纯调度 —— 节奏、并发,
  别无其他。契约只有两条:每个调度的 op 调一次 invoke;所有已启动的
  invoke 完成前不得返回(不丢尾部时延)。单一缝隙同时覆盖闭环、开环、
  爬坡、阶梯与任意 `Schedule` 形态,runner 无需认识其中任何一个。
- **`Schedule` 组合出 `Driver`。** 开环家族(`OpenLoop`、`Ramp`、
  `Staircase`、`Scheduled`)是同一个 `scheduledDriver` 按步调函数
  `func(elapsed) float64` 参数化的结果;deadline 驱动派发(每次迭代重
  算间隔,而非固定 ticker)让变化的 schedule 能加速减速。
  `MaxConcurrent` 是有界信号量 —— 否则开环在持续超额到达下会无限制地
  起 goroutine。
- **`Assert` 把一次运行变成裁定。** 断言在压测结束后针对成品 `Result`
  运行,拿到的是**原始** ctx(不是带时长限制的那个),后续查询不会被取
  消。`Result.Passed` 在全部通过时为真 —— 没注册断言的运行无可失败。
- **`Classify` 把错误映射到桶。** 默认是 resilience + fault 分类法;
  `""` 与未知标签归并到 `other`,自定义 classifier 不会丢错误。

## 3. 与 traffic 压测标记的关系

每个操作的 context 在派发前经 `traffic.WithLoadTest(runCtx,
"loadtest.Run")` 打标,整条下游链路 —— http client、gateway、cache ——
都能通过 `traffic.IsLoadTest` 与传播 helper 识别合成流量。
`cloud/governance/traffic` 里的影子表路由、fault 注入范围、指标标注都
以该标记为键;本 harness 是它的主要生产者。不打标,压测流量除了在
driver 处以外与生产流量无法区分。

## 4. 单轨错误桶

内置标签与自定义标签共用一张扁平 `map[string]int64` —— 不设单独的
"custom" 区。`Result.Print` 固定顺序:五个内置常量(`circuit`、
`rate-limited`、`bulkhead`、`fault-injected`、`other`)按声明序,随后
自定义标签按字典序。无论 map 迭代顺序如何,报告都可 diff —— 这很重要,
因为 example-load 的输出要跨运行、跨 starter 地目检(偶尔 diff)。

## 5. 权衡与放弃的方案

- **example 级 recorder,不做无锁。** 一把 mutex 罩住时延切片和桶
  map,在这个操作量级是更简单的取舍;per-label atomic 买不到可见收益。
- **`time.Sleep` 步调,不做时间轮。** 取消在下一次循环迭代被感知;
  对 harness 足够,driver 保持可读。
- **断言优先于统计。** 通过/失败阈值(`AssertMinQPS` 等)回答的正是
  example-load 关心的运维问题;分位置信区间不在范围内。
- **保留传统 `Run` 作为闭环 `Runner` 的薄包装**而不删除:各 starter
  的 example main 已在用,一行式正好是一轮冒烟压测该有的尺寸。
