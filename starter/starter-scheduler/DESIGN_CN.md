# starter-scheduler 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` 属于 **global / infrastructure** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4),驱动周期与 cron 定时后台任务,
作为 Go-Spring server 生命周期的一部分。触发与并发原语来自零依赖的
`spring/scheduling`;本 starter 只是薄薄的集成层。

## 1. 职责与边界

- **在范围内:**收集 `Job` bean、匹配到 `spring.scheduler.jobs.<name>` 配置、
  交给 `spring/scheduling`、参与优雅停机。
- **不在范围内:**触发算法、并发策略、锁语义(都在 `spring/scheduling`);
  锁的后端(`starter-lock-*`)。

## 2. 关键决策

- **调度器是 `gs.Server` 而非 `gs.Runner`。**Runner 的 `Run` 必须快速返回;
  调度器要长活整个应用生命周期并在 `SIGTERM` 时排空在途运行——Server 才是
  合适形态。这是对设计文档"Runner"措辞的有意偏离。
- **`fixed-delay` 用 `serialTrigger` marker。**fixed-delay 天生串行:
  `spring/scheduling` 用同步 next-fire 实现,以 `LastCompletion` 为锚;与
  `fixed-rate` / `cron` 的异步 dispatch 区分,后者由
  `ConcurrencyPolicy`(`skip` / `queue` / `replace`)治理。
- **注册糖 `scheduler.Provide(name, fn)`。**一次做完
  `gs.Provide` + `Name(name)` + `Export(gs.As[Job]())`。裸 `NewJob` 收集
  不到,因为容器只按导出接口建索引(见 memory `gs export interface
  index`)。
- **配置项 fail-fast。**配置了却没有同名 `Job`,或 `Job` 触发方式缺失 /
  有歧义,都是启动错。拼写错误不会变成"永不触发的任务"。
- **锁按 bean 名解析,边界处适配。**
  `Lockers map[string]lock.Locker autowire:"?"` 按 bean 名收集所有 locker,
  调度器根据 job 的 `lock` 字段查名。`spring/scheduling` 自定义了极简
  `Locker` / `Lock` 接口(保零依赖),故 starter 内 `lockerAdapter` 桥接
  `lock.Locker` 并把 TTL / 续租 option 烤进适配器。
- **停机参与框架级 drain。**`spring.scheduler.drain-timeout`(默认 `30s`)
  约束 `Stop`;它是 `app.shutdown.timeout` 之上的兜底——调度器立刻停止接受
  新触发,等在途集合结束。

## 3. 约束

- **cron 是 5 段式。**5 段表达式(`分 时 日 月 周`),最小粒度 1 分钟;
  example 冒烟窗口内有意不触发 cron(冒烟只验接线)。
- **重叠策略对 `fixed-delay` 无效。**天生串行。
- **多副本去重是严格"max 并发 = 1"。**stdlib 的
  `TestWithLockDeduplicates` 用共享 `inFlight` / `maxSeen` 原子断言此
  性质(而非"一个副本永远赢"——每次触发都重抢锁)。

## 4. 取舍 / 弃选方案

- **每 job 一个 goroutine + `time.Ticker`——弃选。**cron、drain、重叠策略、
  带锁触发需要真正的调度循环;`spring/scheduling` 集中处理。
- **从函数指针自动派生 job 名——弃选。**函数指针名与编译器相关;显式
  bean 名让配置稳定。
- **`spring/scheduling` 直接依赖 `spring/lock.Locker`——弃选。**会把整个
  锁抽象拖进零依赖 scheduling 包;starter 边界处适配让两者都干净。
