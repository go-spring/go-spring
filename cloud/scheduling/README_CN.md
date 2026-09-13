# scheduling

[English](README.md) | [中文](README_CN.md)

`scheduling` 跑周期与 cron 调度的后台任务——Spring `@Scheduled` /
`TaskScheduler` 的 Go 惯用法等价物。一个 `Job` 就是绑定到 `Trigger` 的普通
函数，由带优雅停机排空的 `Scheduler` 驱动。`starter-scheduler` 把同一套 API
接进 IoC 容器。

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

这写法能用，但会慢慢跑偏。两轮之间真正的间隔是 10 秒**加上** `doWork()` 花掉的时间，所以每一轮都再往后滑一点。「每天 03:00 跑一次」则根本写不出来——sleep 表达的是一段时长，不是一天里的某个时刻。而且睡在 `time.Sleep` 里的任务叫不醒，关停就只能等它睡完。

`scheduling` 把这件事拆成两半。`Trigger` 只回答一个问题：**下一次该在什么时候触发？** 给它当前时间和此前的触发情况，它返回下一个时刻；它自己不等待、也不存状态。等待交给 `Scheduler` 里每个任务一个的 loop，每一轮做四件事：

1. 用**当前的** `Now` 和上一次的 `LastScheduled` 问 trigger 要下一次时刻；
2. 给那一刻上一个定时器，同时等定时器和关停信号；
3. 定时器到点就触发，按并发策略派发这一次运行；
4. 把**刚刚计划的时刻**记为 `LastScheduled`，回到第 1 步。

漂移消在第 4 步：回流给 trigger 的是上一轮的**计划时刻**，不是这一轮实际开始或结束的时刻。于是 `FixedRate(d)` 的下一发永远是「上次计划时刻 + d」，第 k 发落在「首发起始 + k·d」，误差不累积；把基准换成「跑完的此刻 + d」，就成了每轮重新锚定一次——那才是漂移。落后的判断靠第 1 步那个**当前的** `Now`：被甩下一整个周期时，trigger 不退回 `Now + d`，而是跳到**原来那根时间轴**上的下一个槽位，错过的槽位整批丢弃、不补发，相位不变。（`FixedDelay` 是刻意的例外：它锚在完成时刻，用于绝不允许重叠的任务。）

拆开之后，其余的好处是顺带的：

- **cron 才写得出来。** 「每天 03:00」需要一天里的某个时刻，只有绝对时刻能表达。
- **关停立刻生效。** 等待中的任务会被关停信号唤醒，不管离下次触发还有多久。
- **重叠由你决定。** 上一次还在跑时，这一发是跳过、排队还是顶掉它，是显式的 `ConcurrencyPolicy`，而不是循环碰巧怎么做。

### 等待的机制与精度

等待不是轮询，也不是时间轮——它说到底就是一次 sleep，只是「睡到某个绝对时刻」而非「睡一段时长」：每一轮只对**这一发**调一次 `time.NewTimer(time.Until(next))`，一次性、到点即弃、下轮重建。所以没有 tick 频率可调，也没有「最小睡眠粒度」，要等 3 小时就是一次性等 3 小时。

开销按**任务数**算，不按触发次数算：一个任务同时只有一个在途 timer，外加一个阻塞在 `select` 上的 loop goroutine；timer 触发后即被回收，下一轮重建。这些 timer 在运行时里共用同一个定时器堆，由调度器自己检查，不是每个 timer 一个 OS 定时器或一条线程。任务数是 job 量级（几十到几百）时，这点开销可以忽略。

精度由 Go 运行时的定时器决定：它**不早于**计划时刻触发，随后在运行时唤醒它的延迟内触发——毫秒量级，受 `GOMAXPROCS` 与当时负载影响。这点延迟不是漂移，因为下一发的基准是计划时刻；`Event` 里 `Scheduled` 是计划时刻、`Start` 是真实开始时刻，两者之差就是这一发的实际唤醒延迟。

cron 的精度另有上限：表达式没有秒字段，`Next` 会把秒和纳秒清零，所以 cron 任务最早也只能按分钟对齐。

## API

| API | 作用 |
| --- | --- |
| `NewScheduler(opts...)` | 创建调度器；`Start` 之前什么都不跑。`WithObserver(fn)` 钩住每次触发（运行或跳过）做指标/日志。 |
| `Schedule(name, trigger, job, opts...)` | 注册任务（`Start` 前后都行）；返回的 cancel 会停止并移除该任务。拒绝 nil trigger/job、重名或已停止的调度器。 |
| `FixedRate(d)` | 每隔 d 触发，锚定上次**计划**时间，不累积漂移。 |
| `FixedDelay(d)` | 上次运行**完成**后再隔 d 触发；天然不重叠，并发策略对它无效。 |
| `ParseCron(expr)` | 标准 5 段 cron，返回 `(Trigger, error)`。支持 `*`、范围 `a-b`、步进 `*/n`、列表；day-of-month / day-of-week 经典 OR 规则。 |
| `WithConcurrencyPolicy(p)` | `Skip`（默认，K8s “Forbid”）、`Queue`（最多排一个）、`Replace`（取消在跑的，K8s “Replace”）——管 fixed-rate/cron 的重叠触发。 |
| `WithTimeout(d)` | 每次运行超时后取消 job 的 context。 |
| `WithLock(locker, key)` | 多副本去重：拿到锁才运行。`Locker` 是本包定义的最小接口；`starter-scheduler` 负责适配 `cloud/lock.Locker`（TTL/续期烧进适配器）。 |
| `Start(ctx)` / `Stop(ctx)` | 生命周期。`Stop` 先取消，再等所有 loop 与在跑任务结束；调用方 ctx 先结束则返回 `ctx.Err()`，任务自行跑完。停掉的调度器不能重启——新建一个。 |

## 用法

### 1. 调度一个任务

```go
sch := scheduling.NewScheduler(scheduling.WithObserver(func(e scheduling.Event) {
    if e.Err != nil {
        log.Printf("job %s failed: %v", e.Name, e.Err)
    }
}))

_, err := sch.Schedule("heartbeat", scheduling.FixedRate(10*time.Second),
    func(ctx context.Context) error {
        log.Println("tick")
        return nil
    },
    scheduling.WithTimeout(3*time.Second),
)
if err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithCancel(context.Background())
_ = sch.Start(ctx)
defer func() { cancel(); _ = sch.Stop(context.Background()) }()
```

### 2. 选对触发器

- `FixedRate`——节奏恒定，不管单次跑多久（可能重叠，见并发策略）。
- `FixedDelay`——间隔从完成时起算；天然串行。
- `ParseCron("*/5 * * * *")`——墙上时钟调度，按参考时间的时区求值。没有秒字段
  ——亚分钟粒度正是 `FixedRate`/`FixedDelay` 的职责。

### 3. 重叠触发

fixed-rate/cron 任务到了触发点而上一次还在跑：

- `Skip`（默认）：丢弃本次触发。
- `Queue`：允许一次等待；再多的丢弃。
- `Replace`：取消在跑任务的 context，启动新的。

### 4. 观察发生了什么

`Event` 报告每次触发：运行的 `Name`、`Scheduled`、`Start`/`Duration`/`Err`；
被吞掉的触发 `Skipped=true` 且 `Reason` 为 `"policy"` 或 `"lock"`。别的副本
持有锁不算错误（`Err=nil`）；后端真实错误才带 `Err`——两条路径刻意区分。
observer 不得阻塞。

## 包保证的规则

- panic 的 job 杀不死 loop：panic 转成错误经 observer 上报，且该错误包着 `ErrJobPanicked`，用 `errors.Is` 就能把它和「跑完返回了错误」区分开。
- `Stop` 确定性排空：loop 返回、在跑任务完成，之后调度器才算停。
- 配置错误 fail-fast：`Schedule` 拒绝 nil trigger/job 与重名；
  `FixedRate`/`FixedDelay` 遇非正时长 panic。
- 每个进程的调度器独立——这不是分布式调度器；副本协同由 `WithLock` 层叠加。
