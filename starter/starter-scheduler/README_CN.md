# starter-scheduler

[English](README.md) | [中文](README_CN.md)

`starter-scheduler` 以 Go-Spring 应用生命周期的一部分运行周期性 / cron 定时后台
任务。空导入后,为每个工作单元注册一个 `Job`,并在配置中声明各任务的触发方式——
starter 负责驱动它们、参与优雅停机,并可借助分布式锁在多副本间去重。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../../DESIGN.md) §2.4):不开监听端口,而是导出一个 `gs.Server`,
让调度器加入 server 生命周期——应用就绪后任务才开始触发;收到 `SIGTERM` 时,进程
退出前会先排空在途运行。

触发与并发原语来自零依赖的
[`cloud/scheduling`](../../../cloud/scheduling) 包;本 starter 只是把 IoC 容器接入其
上的薄集成层。任务的节奏是任务本身的一部分,所以在注册任务的地方声明——配置里只剩
进程级旋钮(`enabled`、`drain-timeout`)。

## 安装

```bash
go get go-spring.org/starter-scheduler
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. 注册 Job 并同时给出调度

`scheduler.Provide` 会以任务名命名 bean、将其导出为 `Job` 供调度器收集,并把调度作为
选项收下——节奏就写在它所描述的工作旁边。

```go
import scheduler "go-spring.org/starter-scheduler"

func main() {
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return svc.Cleanup(ctx)
    }, scheduler.Every(5*time.Minute))

    scheduler.Provide("nightly", svc.Prune, scheduler.Cron("0 3 * * *"))
    gs.Run()
}
```

没有触发方式、或给了两个,都会在注册时 panic,也就是在启动期间——错误在启动时暴露,
而不是变成一个悄无声息永不触发的任务。

## 触发方式

每个任务**恰好声明一个**:

| 选项                   | 含义                                                     |
|------------------------|----------------------------------------------------------|
| `scheduler.Cron(expr)` | 标准 5 段 cron 表达式(`分 时 日 月 周`)。                |
| `scheduler.Every(d)`   | 每隔 `d` 触发,以每次计划触发时刻为基准。                 |
| `scheduler.After(d)`   | 上一次运行**结束后**再过 `d` 触发;永不重叠。             |

## 每任务选项

与触发方式一起在注册时给出:

| 选项                           | 默认             | 含义                                                          |
|--------------------------------|------------------|---------------------------------------------------------------|
| `scheduler.WithTimeout(d)`     | 无               | 为正时,运行超过 `d` 后其 context 被取消。                     |
| `scheduler.WithConcurrency(p)` | `scheduling.Skip`| `Every`/`Cron` 的重叠策略:`Skip`、`Queue` 或 `Replace`。      |
| `scheduler.WithLock(bean)`     | —                | 一个 `lock.Locker` bean 的名字;每次触发只有持锁者运行。       |
| `scheduler.WithLockKey(k)`     | 任务名           | 在 locker 上获取的键。                                        |
| `scheduler.WithLockTTL(d)`     | locker 自身默认  | 租约时长;持锁期间自动续租。                                   |

`WithConcurrency` 对 `After` 任务无效——后者天生串行。

## 多副本去重

要让某任务在同一时刻只在一个副本上运行,把它的 `WithLock` 指向由
`starter-lock-{redis,etcd,consul}` 贡献的 `lock.Locker` bean。每次触发都会尝试获取
锁,未抢到的副本跳过本次。

```go
scheduler.Provide("nightly", svc.Prune, scheduler.Cron("0 2 * * *"),
    scheduler.WithLock("jobs"),                // 名为 "jobs" 的 lock.Locker bean
    scheduler.WithLockTTL(5*time.Minute))
```

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"   // 贡献 "jobs" locker
    _ "go-spring.org/starter-scheduler"
)
```

## 优雅停机

收到 `SIGTERM` 后,调度器停止触发并等待在途运行结束,受
`spring.scheduler.drain-timeout`(默认 `30s`)约束——这是调度器对自身优雅停机
的边界。

## 可观测性

引入 `starter-otel`（或任何 SDK provider）后，每次 job 运行会开一个 span
（`scheduler.job <name>`），每次触发——运行或跳过——都会喂给三个 metric：
`scheduling.runs{job,outcome}`、`scheduling.run.duration{job,outcome}` 和
`scheduling.lag{job}`。outcome 词表为 `ok`、`error`、`panic`、`skipped_policy`、
`skipped_lock`。

`lag` 是触发被计划的时刻与运行真正开始时刻之间的距离：它持续爬升说明运行开始得越来越
晚，这是别的 instrument 看不到的。未装 SDK provider 时以上全部为 no-op。完整的
instrument 参考见 [USAGE §1.1](USAGE_CN.md#11-可观测变体example-otel)。

## 配置项参考

| 键                               | 默认    | 说明                                       |
|----------------------------------|---------|--------------------------------------------|
| `spring.scheduler.enabled`       | `true`  | 启用调度器(注册 ≥1 个 Job 后才真正生效)。 |
| `spring.scheduler.drain-timeout` | `30s`   | `Stop` 等待在途运行的最长时间。            |

## 示例

见 [`example/`](example) 中可运行的演示,覆盖 `fixed-rate`、`fixed-delay`、`cron`
以及一个带锁任务(由进程内 `MemoryLocker` 支撑,故无需 docker):

```bash
cd example && ./check.sh
```
