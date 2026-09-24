# starter-scheduler 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-scheduler` 属于 **global / infrastructure** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4),驱动周期与 cron 定时后台任务,
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
- **调度在构造处声明。**`scheduling.NewJob(name, run, trigger, opts...)` 收的全是
  cloud 包自己的类型——工作是 `scheduling.Job`,触发器是
  `scheduling.Trigger`(FixedRate / FixedDelay / ParseCron),选项是
  `scheduling.Option`——job 声明的东西与 cloud/scheduling 的文档逐字对应,
  用户代码依赖的是 cloud 类型;starter 只补 bean 本身和按名解析锁的桥。此前把
  `${spring.scheduler.jobs.<name>.*}` 按名字对到 bean 上,且校验不对称——
  配置→bean 报错、bean→配置仅告警,于是 bean 侧拼错就是"永不触发的任务"。
  配置里只剩进程级旋钮:`spring.scheduler.enabled`。
- **统一形态:具体 bean。**job 就是 `*scheduling.Job`,由 `NewJob` 从工作
  (通常是作者自有 struct 的绑定方法)建成,经 `gs.Provide(...).Name(name)`
  注册——无需 Export,因为没有接口要建索引。注册走容器,构造函数的依赖
  ——其他 bean、或绑定到 value tag struct 的配置——与任何 bean 一样被解析;
  先于容器运行的形态永远够不到这两样。空名字、nil 运行函数或 nil 触发器,
  被 `NewJob` 以错误拒绝——装配期,而不是留到后面再查一遍。
- **锁直通。**
  job 在构造时经 `WithLock(lk, key, ttl)` 选项挂锁——`lock.Locker` bean
  注入 job 构造函数,引用不可能悬空。WithLock 直接收 cloud/lock 类型;
  调用方与锁之间没有适配器、没有中间接口。
  (更早的设计按 bean 名索引 locker、启动时解析,因为
  任务用 `WithLock` 指名,名字在调度器启动时才解析(注册时那个 bean 还
  不存在)。`cloud/scheduling` 自定义了极简
  `Locker` / `Lock` 接口(保零依赖),故 starter 内 `lockerAdapter` 桥接
  `lock.Locker` 并把 TTL / 续租 option 烤进适配器。
- **插桩内置在 `cloud/scheduling`。**按全仓「插桩并入领域包」的方向
  (observe 套件已拆除),cloud 包自己上报每次触发——
  `scheduling.runs{job,status}`、`scheduling.run.duration`、`scheduling.lag`,
  一条同键日志,以及每次运行在全局 OTel 管线上的一根 span。本 starter
  不含任何可观测代码,只负责把 job 接进生命周期。panic 包着
  `ErrJobPanicked`,于是 panic 成了一个可计数的 outcome(`status=panic`),
  而不是要去匹配的错误字符串。
- **停机边界是 shutdown ctx,不是旋钮。**`Stop` 在框架的 shutdown ctx
  内排空——真正的期限是编排层的击杀时限(K8s terminationGracePeriod、
  systemd TimeoutStopSec),过了就强杀;进程内再造一个超时旋钮只会在没有
  编排层的环境里生效。

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
