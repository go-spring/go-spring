# bulkhead

[English](README.md) | [中文](README_CN.md)

`bulkhead` 是一个并发隔离层：限制同时持有槽位的调用者数量。只依赖标准库，入站和出站
都能用。

## 原理

它管的是"慢"，不是"错"。熔断决定要不要调，bulkhead 决定最多多少人同时调；限流管每秒
多少，bulkhead 管同时多少。依赖变慢时，在途请求会线性增长，限流拦不住，只有并发上限
能挡住。

一个 `Bulkhead` 保护一个单元，并与该单元同寿命。单元不是代码模块，而是**会一起坏的
那组调用**：沿故障传播路径切，不沿代码结构切。两个下游服务共用一个舱，等于没有隔离。
不要按请求创建——每请求一个不隔离任何东西，只是在数并发。

`Acquire` 占一个槽位，业务结束后由调用方恰好释放一次，槽位代表一个在途调用。舱满时默认
立即拒绝；传 `WithQueueTimeout` 可以给它一个等待窗口。排队会把过载转成延迟，所以默认不
排队。

## 最佳实践

- **粒度**：默认每个下游服务一个舱。按方法切是 Hystrix 时代的经典错误——几十个舱、
  每舱十个槽，总并发明明够用却到处被拒。一个服务几个舱是常态，超过二十个左右就该合并。
- **时机**：装配期建一次，跟着配置代际走。admission 引擎每个 server 实例一个，策略
  代际变化才重建。
- **容量**：`maxConcurrent` 是调用方的预算，不是下游的容量。核心链路拿大头，边缘依赖
  给小舱；各舱之和可以略超总容量，但核心舱必须独享。
- **排队**：粗粒度的自保舱配 10~100ms 短暂排队削峰；细粒度的隔离舱不排队，让失败
  尽快到达熔断器。
- **scope key**：bulkhead、熔断器、限流器必须用同一把 key 分区，否则会出现"熔断按
  service、隔离按 method"的缝。
- **位置**：尽早 Acquire，尽晚 release；槽位要覆盖 handler、下游调用和所有重试。内层
  阶段拒绝时必须释放外层槽位。

admission 引擎的顺序就是按这两条排的：bulkhead 最外，然后限流、熔断门、放火，后续
每个拒绝都释放槽位，最终由 `Done` 归还：

```go
release, ok := bulk.Acquire(ctx)          // 1. 最外层：界定并发
if !ok {
    return refused(ErrBulkheadFull, bulk.QueueTimeout())
}
if !limiter.Allow(ctx) {                  // 2. 为请求记账
    release()
    return refused(ErrRateLimited, 0)
}
permit, ok := breaker.Allow()             // 3. 熔断门
if !ok {
    release()
    return refused(ErrCircuitOpen, 0)
}
done := func(outcome error) {             // 4. 工作结算时释放
    breaker.Record(permit, outcome == nil, elapsed)
    release()
}
```

客户端侧包一个下游依赖时形状相同：第一次尝试前 Acquire，横跨所有重试，结算时释放。

## 用法

```go
b := bulkhead.New(maxConcurrent) // 默认满即拒
// 另一种写法：给舱满的调用者一小段等待槽位的时间。
// b := bulkhead.New(maxConcurrent, bulkhead.WithQueueTimeout(queueTimeout))

release, ok := b.Acquire(ctx)
if !ok {
    return ErrBulkheadFull // 被拒绝；QueueTimeout() 即 retry-after 提示
}
defer release()
err := doWork()
```

- `New(maxConcurrent, opts...)` 在 `maxConcurrent <= 0` 时 panic："不隔离"是持有方
  持有的 nil `*Bulkhead`，不是关闭的实例；未配置的策略就保持 nil。
- 默认 fail fast；传 `WithQueueTimeout(queueTimeout)` 后最多等待 `queueTimeout` 获取
  槽位。
- `Acquire` 返回释放函数；被拒绝时不消耗槽位，返回的释放函数是 no-op。一次成功的
  Acquire 与它的 release 是调用方持有的一对，release 必须**恰好执行一次**。多调一次
  会返还没有持有的槽位，容量永久 -1；不调用则槽位被永久占住。这两者在 bulkhead 内部
  都无法检测，所以精确成对是调用契约，不是可选礼仪。
- 排队期间同时受 `ctx` 和排队时间约束，任何一方到期都返回 `ok == false`。调用
  `Acquire` 时已经结束的 `ctx` 会在碰槽位之前被拒绝。
- `QueueTimeout()` 返回排队时间，适配层可用作 retry-after。
- 等待者不严格 FIFO，但并发上界绝不会被突破。
