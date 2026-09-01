# scheduling

[English](README.md) | [中文](README_CN.md)

`scheduling` 跑周期与 cron 调度的后台任务——Spring `@Scheduled` /
`TaskScheduler` 的 Go 惯用法等价物。一个 `Job` 就是绑定到 `Trigger` 的普通
函数,由带优雅停机排空的 `Scheduler` 驱动。`starter-scheduler` 把同一套 API
接进 IoC 容器。

## API

| API | 作用 |
| --- | --- |
| `NewScheduler(opts...)` | 创建调度器;`Start` 之前什么都不跑。`WithObserver(fn)` 钩住每次触发(运行或跳过)做指标/日志。 |
| `Schedule(name, trigger, job, opts...)` | 注册任务(`Start` 前后都行);返回的 cancel 会停止并移除该任务。拒绝 nil trigger/job 或重名。 |
| `FixedRate(d)` | 每隔 d 触发,锚定上次**计划**时间,不累积漂移。 |
| `FixedDelay(d)` | 上次运行**完成**后再隔 d 触发;天然不重叠,并发策略对它无效。 |
| `Cron(expr)` / `ParseCron(expr)` | 标准 5 段 cron(`Cron` 遇坏表达式 panic,`ParseCron` 返回错误)。支持 `*`、范围 `a-b`、步进 `*/n`、列表;dom/dow 经典 OR 规则。 |
| `WithConcurrencyPolicy(p)` | `Skip`(默认,K8s "Forbid")、`Queue`(最多排一个)、`Replace`(取消在跑的,K8s "Replace")——管 fixed-rate/cron 的重叠触发。 |
| `WithTimeout(d)` | 每次运行超时后取消 job 的 context。 |
| `WithLock(locker, key)` | 多副本去重:拿到锁才运行。`Locker` 是本包定义的最小接口;`starter-scheduler` 负责适配 `cloud/lock.Locker`(TTL/续期烧进适配器)。 |
| `Start(ctx)` / `Stop(ctx)` | 生命周期。`Stop` 先取消,再等所有 loop 与在跑任务结束;调用方 ctx 先结束则返回 `ctx.Err()`,任务自行跑完。停掉的调度器不能重启——新建一个。 |

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

- `FixedRate`——节奏恒定,不管单次跑多久(可能重叠,见并发策略)。
- `FixedDelay`——间隔从完成时起算;天然串行。
- `Cron("*/5 * * * *")`——墙上时钟调度,按参考时间的时区求值。没有秒字段
  ——亚分钟粒度正是 `FixedRate`/`FixedDelay` 的职责。

### 3. 重叠触发

fixed-rate/cron 任务到了触发点而上一次还在跑:

- `Skip`(默认):丢弃本次触发。
- `Queue`:允许一次等待;再多的丢弃。
- `Replace`:取消在跑任务的 context,启动新的。

### 4. 观察发生了什么

`Event` 报告每次触发:运行的 `Name`、`Scheduled`、`Start`/`Duration`/`Err`;
被吞掉的触发 `Skipped=true` 且 `Reason` 为 `"policy"` 或 `"lock"`。别的副本
持有锁不算错误(`Err=nil`);后端真实错误才带 `Err`——两条路径刻意区分。
observer 不得阻塞。

## 包保证的规则

- panic 的 job 杀不死 loop:panic 转成错误经 observer 上报。
- `Stop` 确定性排空:loop 返回、在跑任务完成,之后调度器才算停。
- 配置错误 fail-fast:`Schedule` 拒绝 nil trigger/job 与重名;
  `FixedRate`/`FixedDelay` 遇非正时长 panic。
- 每个进程的调度器独立——这不是分布式调度器;副本协同由 `WithLock` 层叠加。
