# scheduling

[English](README.md) | [中文](README_CN.md)

`scheduling` 跑周期与 cron 调度的后台任务——Spring `@Scheduled` /
`TaskScheduler` 的 Go 惯用法等价物。一个 `Job`（由 `NewJob` 构造）把运行
函数、`Trigger` 与执行选项打成一个注册包，由带优雅停机排空的 `Scheduler`
驱动。`starter-scheduler` 把同一套 API 接进 IoC 容器。

调度器是单进程的。多副本部署时 `WithLock` 做触发去重（只有持锁者执行）——
它不提供分片、故障接管或任务编排；需要这些请用外部任务平台（如
`starter-xxl-job`）。任务属于进程内工作（刷缓存、心跳、清理）、不值得为它
运维一套调度中心时，用 `scheduling`。

## 运行原理

给一个任务做定时的最直接写法是循环：

```go
for {
    doWork()
    time.Sleep(10 * time.Second)
}
```

这写法能用，但会慢慢跑偏：两轮真正的间隔是 10 秒**加上** `doWork()` 的耗时，每轮都往后滑；「每天 03:00」根本写不出来（sleep 是一段时长，不是一天里的时刻）；睡在 `time.Sleep` 里的任务也叫不醒，关停只能等它睡完。

`scheduling` 把这件事拆成两半。`Trigger` 只回答一个问题：**下一次该在什么时候触发？**——它不等待、不存状态。等待交给 `Scheduler` 里每个任务一个的 loop，每一轮做四件事：

1. 用当前的 `Now` 和上一次的 `LastScheduled` 问 trigger 要下一次时刻；
2. 给那一刻上一个定时器，同时等定时器和关停信号；
3. 定时器到点就派发这一次运行（fixed-delay 则内联执行）；
4. 把**刚刚计划的时刻**记为 `LastScheduled`，回到第 1 步。

漂移消在第 4 步：回流给 trigger 的是**计划时刻**，所以 `FixedRate(d)` 的下一发永远是「上次计划时刻 + d」，第 k 发落在「首发起始 + k·d」，误差不累积。被慢运行甩下一整个周期时，trigger 跳到**原来那根时间轴**上的下一个槽位，错过的整批丢弃、不补发，相位不变。（`FixedDelay` 是刻意的例外：它锚在完成时刻，用于绝不允许重叠的任务。）

拆开之后其余好处是顺带的：cron 才写得出来（一天里的时刻只有绝对时刻能表达）、关停立刻生效（等待中的任务会被信号唤醒）、重叠是显式的 `ConcurrencyPolicy` 而不是循环碰巧怎么做。

### 等待的机制与精度

等待就是每一轮一个一次性的 `time.NewTimer(time.Until(next))`——**睡到某个绝对时刻**而非睡一段时长——没有轮询、没有时间轮、没有 tick 频率。开销按**任务数**算而非触发次数：每任务一个在途 timer 加一个停在 `select` 上的 loop goroutine，共用运行时的定时器堆。job 量级（几十到几百个）时可以忽略。

精度由 Go 运行时决定：**不早于**计划时刻，随后在毫秒量级的唤醒延迟内（受 `GOMAXPROCS` 与负载影响）。这点延迟不是漂移——下一发仍以计划时刻为基准——并作为 `scheduling.lag` 直方图导出。cron 另有上限：表达式没有秒字段，最早只能按分钟对齐。

### 运行模型

`Start` 之后，每个任务一个 **loop** goroutine：问触发器、等待、派发。触发要么在 loop 内**内联**执行（fixed-delay——串行正来源于此），要么派发到 **run** goroutine（fixed-rate / cron），并发上界由策略决定：

| 策略 | 在途运行 | 运行中又来一炮 |
| --- | --- | --- |
| `Skip`（默认） | ≤ 1 | 丢弃（`skipped_policy`） |
| `Queue` | ≤ 1 在跑 + 1 排队 | 第一个排队，之后丢弃 |
| `Replace` | ≤ 1（最新者） | 取消在跑的，启动新的 |

context 是一棵以 `Start` ctx 为根的树：每任务一个 loop ctx（cancel 函数取消），每次触发一个 run ctx（再叠 `WithTimeout` / `Replace` 的取消）——取消根，全树皆停。两把锁从不嵌套：调度器的管注册表与生命周期，任务自己的管计时状态，管理任务与运行任务互不争锁。

生命周期：`Schedule` 在 `Start` 前后都能调；其 cancel 函数只移除该任务。`Stop` 是终态——取消根、以调用方 ctx 为界排空所有 loop 与运行，超时报 deadline 错误（剩余运行自行跑完）。

## API

| API | 作用 |
| --- | --- |
| `NewScheduler()` | 创建调度器；`Start` 之前什么都不跑。每次触发（运行或跳过）都由包内置的指标与日志上报（见下）。 |
| `NewJob(name, trigger, run, opts...)` | 构造 [Job]：名字、运行函数、触发器、执行选项（及经 `.WithLock` 挂上的分布式锁）的注册包。空名字/nil 运行/nil 触发器返回错误。 |
| `Schedule(job)` | 注册一个 [Job]（`Start` 前后都行）；返回的 cancel 停止并移除该任务。拒绝重名或已停止的调度器。 |
| `FixedRate(d, opts...)` | 每隔 d 触发，锚定上次**计划**时间，不累积漂移。 |
| `FixedDelay(d, opts...)` | 上次运行**完成**后再隔 d 触发；天然不重叠，并发策略对它无效。 |
| `After(d)` | 只触发一次：调度启动 d 后跑一发，随后任务结束——延迟执行的一次性工作。 |
| `WithInitialDelay(d)` | `FixedRate`/`FixedDelay` 的触发器选项：首发延后 d，后续按正常节奏。 |
| `WithJitter(d)` | `FixedRate`/`FixedDelay` 的触发器选项：每发随机延后 [0, d)，避免同节奏任务齐步压垮下游。延后会进入下一发的锚点，长期平均间隔为 d + jitter/2。 |
| `DailyWindow(start, end, tr)` | 只在每日窗口 [start, end) 内按 tr 的节奏触发；落在窗口外的发次顺延到下一个窗口起点。 |
| `ParseCron(expr)` | 标准 5 段 cron，返回 `(Trigger, error)`。支持 `*`、范围 `a-b`、步进 `*/n`、列表；day-of-month / day-of-week 经典 OR 规则。 |
| `WithConcurrencyPolicy(p)` | `Skip`（默认，K8s “Forbid”）、`Queue`（最多排一个）、`Replace`（取消在跑的，K8s “Replace”）——管 fixed-rate/cron 的重叠触发。 |
| `WithTimeout(d)` | 每次运行超时后取消 job 的 context。 |
| `WithLock(l, key, ttl)` | 多副本去重：拿到锁才运行。直接收 `cloud/lock.Locker`——就是各锁后端与 starter-lock-* bean 本来的类型。 |
| `Start(ctx)` / `Stop(ctx)` | 生命周期。`Stop` 先取消，再等所有 loop 与在跑任务结束；调用方 ctx 先结束则返回 `ctx.Err()`，任务自行跑完。停掉的调度器不能重启——新建一个。 |

## 用法

### 1. 调度一个任务

```go
sch := scheduling.NewScheduler()

job := scheduling.NewJob("heartbeat", scheduling.FixedRate(10*time.Second),
    func(ctx context.Context) error {
        log.Println("tick")
        return nil
    },
    scheduling.WithTimeout(3*time.Second),
)
if _, err := sch.Schedule(job); err != nil {
    log.Fatal(err)
}
```

### 2. 选对触发器

- `FixedRate`——节奏恒定，不管单次跑多久（可能重叠，见并发策略）。
- `FixedDelay`——间隔从完成时起算；天然串行。
- `After`——延迟一发的单次触发（「稍后做一次这件事」）。
- `DailyWindow`——包住上面任意一个，只在比如 9:00–18:00 内触发。
- `ParseCron("*/5 * * * *")`——墙上时钟调度，按参考时间的时区求值。没有秒字段
  ——亚分钟粒度正是 `FixedRate`/`FixedDelay` 的职责。

两个固定间隔触发器都可以加 `WithInitialDelay(d)`：首发先等预热完成，
而不是照一个间隔起跳；也可以加 `WithJitter(d)`：大量任务共享同一节奏、
不能齐步触发时打散它们。

### 3. 重叠触发

fixed-rate/cron 任务到了触发点而上一次还在跑：

- `Skip`（默认）：丢弃本次触发。
- `Queue`：允许一次等待；再多的丢弃。
- `Replace`：取消在跑任务的 context，启动新的。

### 4. 观察发生了什么

可观测性是内置的：每次触发上报 `scheduling.runs{job,status}`，运行再上报
`scheduling.run.duration{job,status}` 与 `scheduling.lag{job}`，外加一条携带
相同 `job`/`status` 键的日志。status 互斥——
`ok | error | panic | skipped_policy | skipped_lock`——按维求和即触发次数。
别的副本持有锁是 `skipped_lock` 且无错误；锁后端真实故障也是
`skipped_lock`，但其日志行带错误。运行还会在全局 otel 管线上开一个 span
（`scheduler.job <name>`）；被吞掉的触发没有运行，无 span。

## 包保证的规则

- panic 的 job 杀不死 loop：panic 转成错误按 `panic` 状态上报，且该错误包着 `ErrJobPanicked`，用 `errors.Is` 就能把它和「跑完返回了错误」区分开。
- `Stop` 确定性排空：loop 返回、在跑任务完成，之后调度器才算停。
- 配置错误 fail-fast：`NewJob` 对空名字/nil 运行/nil 触发器返回错误；
  `Schedule` 拒绝重名；`FixedRate`/`FixedDelay`/`After`/`WithInitialDelay`/
  `WithJitter` 遇非正时长 panic；`DailyWindow` 遇 nil 触发器或跨出一天的窗口
  panic。
- 每个进程的调度器独立——这不是分布式调度器；副本协同由 `WithLock` 层叠加。
