# starter-scheduler 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` 属于 **global / infrastructure** 形态(见
[starter/DESIGN.md](../../DESIGN.md) §2.4),驱动周期与 cron 定时后台任务,
作为 Go-Spring server 生命周期的一部分。触发与并发原语来自零依赖的
`cloud/scheduling`;本 starter 只是薄薄的集成层。

## 1. 职责与边界

- **在范围内:**收集 `Job` bean、从 bean 上读它自己的调度、交给
  `cloud/scheduling`、参与优雅停机。
- **不在范围内:**触发算法、并发策略、锁语义(都在 `cloud/scheduling`);
  锁的后端(`starter-lock-*`)。

## 2. 关键决策

- **调度器是 `gs.Server` 而非 `gs.Runner`。**Runner 的 `Run` 必须快速返回;
  调度器要长活整个应用生命周期并在 `SIGTERM` 时排空在途运行——Server 才是
  合适形态。这是对设计文档"Runner"措辞的有意偏离。
- **`fixed-delay` 用 `serialTrigger` marker。**fixed-delay 天生串行:
  `cloud/scheduling` 用同步 next-fire 实现,以 `LastCompletion` 为锚;与
  `fixed-rate` / `cron` 的异步 dispatch 区分,后者由
  `ConcurrencyPolicy`(`skip` / `queue` / `replace`)治理。
- **调度在注册处声明。**`scheduler.Provide(name, fn, opts...)` 把触发方式
  (`Every` / `After` / `Cron`)与执行选项都收成 `JobOption`:任务在一个地方
  读完,合法性在启动时定,而不是靠按名查配置。此前把
  `${spring.scheduler.jobs.<name>.*}` 按名字对到 bean 上,且校验不对称——
  配置→bean 报错、bean→配置仅告警,于是 bean 侧拼错就是"永不触发的任务"。
  配置里只剩进程级旋钮:`spring.scheduler.enabled` 与 `drain-timeout`。
- **注册糖 `scheduler.Provide(name, fn, opts...)`。**一次做完
  `gs.Provide` + `Name(name)` + `Export(gs.As[Job]())`。裸 `NewJob` 收集
  不到,因为容器只按导出接口建索引(见 memory `gs export interface
  index`)。触发方式缺失或给了两个,就在这里 panic——注册即启动期,
  而不是留到后面再查一遍。
- **锁按 bean 名解析,边界处适配。**
  `Lockers map[string]lock.Locker autowire:"?"` 按 bean 名收集所有 locker,
  任务用 `WithLock` 指名,名字在调度器启动时才解析(注册时那个 bean 还
  不存在)。`cloud/scheduling` 自定义了极简
  `Locker` / `Lock` 接口(保零依赖),故 starter 内 `lockerAdapter` 桥接
  `lock.Locker` 并把 TTL / 续租 option 烤进适配器。
- **插桩留在 starter,不进 `cloud/scheduling`。**让 `Locker` 保持最小的
  零依赖理由同样适用于遥测:cloud 包只暴露 `Observer` / `Event` 这条缝,
  由本 starter 把每个 `Event` 变成一根 span、三个 metric 和一条日志
  (`observe.go`)。把 OTel 桥放进 cloud 包(像 `lock` 的 `WrapLocker` 那样)
  会逼它 import OTel,而零依赖恰恰是它明确声明的。有一处核心改动无法避免:
  panic 现在包着 `ErrJobPanicked`,于是 panic 成了一个可计数的 outcome,
  而不需要观察者去匹配错误字符串。
- **停机由调度器自定边界。**`spring.scheduler.drain-timeout`(默认 `30s`)
  约束 `Stop`——调度器立刻停止接受新触发,等在途集合结束。

## 3. 约束

- **cron 是 5 段式。**5 段表达式(`分 时 日 月 周`),最小粒度 1 分钟;
  example 冒烟窗口内有意不触发 cron(冒烟只验接线)。
- **重叠策略对 `fixed-delay` 无效。**天生串行。
- **多副本去重是严格"max 并发 = 1"。**stdlib 的
  `TestWithLockDeduplicates` 用共享 `inFlight` / `maxSeen` 原子断言此
  性质(而非"一个副本永远赢"——每次触发都重抢锁)。

## 4. 取舍 / 弃选方案

- **每 job 一个 goroutine + `time.Ticker`——弃选。**cron、drain、重叠策略、
  带锁触发需要真正的调度循环;`cloud/scheduling` 集中处理。
- **调度写进配置——弃选。**可运维调节的节奏是运维旋钮,而运维管理的调度正是
  任务平台(`starter-xxl-job`)的领域。对本 starter 面向的进程内工作,节奏是任务
  本身的一部分,写在代码里也就免掉了两个来源之间按名字查表这一步。
- **从函数指针自动派生 job 名——弃选。**函数指针名与编译器相关;显式
  bean 名让配置稳定。
- **`cloud/scheduling` 直接依赖 `cloud/lock.Locker`——弃选。**会把整个
  锁抽象拖进零依赖 scheduling 包;starter 边界处适配让两者都干净。
