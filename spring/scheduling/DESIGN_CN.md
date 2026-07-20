# scheduling 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`scheduling` 是 stdlib 层零依赖抽象,承载周期任务与 cron 定时任务。
`starter-scheduler` 会把它接入 IoC 容器,以 `gs.Server` 形态参与优雅停机。

## 1. 职责与边界

- 从 `Trigger` 计算下次触发时间,在 `ConcurrencyPolicy` 与可选的每次运行超时 /
  分布式锁约束下运行 `Job`。
- 不是队列,不是分布式调度器,不是工作流引擎。每个进程的 scheduler 独立;多副
  本协调通过 `WithLock` 附加 `Locker` 覆盖。
- Cron 解析器有意放在本包内。引入第三方 cron 库会破坏 stdlib 零依赖契约;内置
  parser 处理标准 5 段表达式与 day-of-month / day-of-week 的 OR 规则。

## 2. 关键抽象与缝隙

- `Trigger.Next(TriggerContext) time.Time`。返回零值代表"永不再触发"。
  `FixedRate` 锚定 `LastScheduled` 避免漂移累积;`FixedDelay` 锚定
  `LastCompletion`;`Cron` 走已解析 spec 遍历。
- **`serialTrigger` marker**:`fixedDelay` 实现未导出 `serial()` 方法。
  scheduler 通过类型断言识别并**在 loop 内同步跑**,让下一次触发从上一次完成
  时刻计算;其他 trigger 则脱离 loop 由 concurrency 策略调度。
- `Locker` / `Lock` **在本包内声明极简形态**("TryAcquire 返回本地 `Lock`"),
  保零依赖。`spring/lock.Locker` 不直接满足——集成层
  (`starter-scheduler`)桥接,并把 TTL / 续期选项烤进适配器。
- `Observer` 在每次运行和每次跳过后触发;`Skipped=true` 时
  `Reason="policy"` 或 `"lock"`——两种被吞掉的路径。
- 注册入口:`Schedule(name, trigger, job, opts...)` 返回停 loop + 移除任务的
  `cancel`。`Start` 前后调用皆可。

## 3. 约束(禁止破坏)

- **错配 fail-fast**。`Schedule` 拒绝 nil trigger、nil job、重名;
  `FixedRate` / `FixedDelay` 在非正 duration 时 panic;`Cron` 在表达式非法时
  panic(`ParseCron` 返 error)。
- **锁跳过不是错误**。`Locker.TryAcquire` 返 `ok=false` 上报
  `Skipped:"lock"`、`Err=nil`;真正的后端故障上报 `Skipped:"lock"` +
  `Err=err`——两条路径故意区分。
- **`safeRun` 把 panic 转 error**。panicking job 不能干掉 loop;转成 error
  由 `Observer` 上报。
- **`Stop` 等待在飞行的运行**。先 cancel scheduler ctx,再等所有 task loop
  返回,再等所有在飞行运行的 `runWg`;然后才认为 drain 完成。若调用方 ctx 先
  到期,`Stop` 返回 `ctx.Err()`——运行继续在后台完成。
- **`Stop` 之后 scheduler 不能重启**。新建一个。

## 4. 权衡 / 未做的方案

- **scheduler 是 `gs.Server`,不是 `gs.Runner`**。Runner 必须尽快返回;
  scheduler 得活到进程结束,并在 SIGTERM 时 drain。Server 语义契合——这是对
  "Runner"措辞的有意偏离。
- **`ConcurrencyPolicy` 只作用于非 serial trigger**。fixed-delay 天然不重
  叠;在其上加策略是雷。
- **无 cron 秒字段**。标准 5 段最少意外;分钟以下用 `FixedRate` /
  `FixedDelay` 更自然。
- **锁走适配器,不直接 import**。避免把 `spring/lock` 拉进本包,保层次独立
  性;调用方可以提供任意极简 `Locker`(内存、测试替身...),不用引入 lock 抽
  象。
