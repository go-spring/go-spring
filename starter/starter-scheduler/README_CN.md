# starter-scheduler

[English](README.md) | [中文](README_CN.md)

`starter-scheduler` 以 Go-Spring 应用生命周期的一部分运行周期性 / cron 定时后台
任务。空导入后,为每个工作单元注册一个 `Job`,并在配置中声明各任务的触发方式——
starter 负责驱动它们、参与优雅停机,并可借助分布式锁在多副本间去重。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):不开监听端口,而是导出一个 `gs.Server`,
让调度器加入 server 生命周期——应用就绪后任务才开始触发;收到 `SIGTERM` 时,进程
退出前会先排空在途运行。

触发与并发原语来自零依赖的
[`cloud/scheduling`](../../cloud/scheduling) 包;本 starter 只是把 IoC 容器接入其
上的薄集成层。任务的节奏是任务本身的一部分,所以在注册任务的地方声明——配置里只剩
进程级旋钮(`enabled`);停机排空由编排层的击杀期限约束,不是配置值。

## 安装

```bash
go get go-spring.org/starter-scheduler
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. 定义 Job bean

job 就是 cloud 类型 `scheduling.Job`(没有接口——不会有类型意外实现它,
代码跳转直达 struct 本体),由工作自己的构造函数调 `scheduling.NewJob`
建成,通常传入你自有 struct 的绑定方法,让任务的依赖待在那个 struct 里、
由容器解析。**不需要 `Export`**:starter 直接收集这个类型的 bean;job
构造里不会用到任何 starter 符号。

```go
import scheduling "go-spring.org/cloud/scheduling"

type CleanupService struct {
    db *gormcore.DB // 容器认识的任意 bean
}

func (s *CleanupService) Cleanup(ctx context.Context) error { ... }

func NewCleanupJob(db *gormcore.DB) *scheduling.Job {
    return scheduling.NewJob("cleanup", scheduling.FixedRate(5 * time.Minute), svc.Cleanup)
}

func main() {
    gs.Provide(NewCleanupJob)
    gs.Run()
}
```

空名字、nil 运行函数或 nil 触发器都会被 `NewJob` 拒绝(返回错误),也就是在装配期间
——错误在启动时暴露,而不是变成一个悄无声息永不触发的任务。触发器一侧的错误
(非正时长、解析不了的 cron)在 cloud 包里失败——同样发生在构造函数里,同样是
启动期。

## 触发器与选项

触发器和每任务选项都是 cloud 包自己的类型——完整契约见
[cloud/scheduling](../../../cloud/scheduling/README_CN.md)。三个触发器:
`scheduling.FixedRate(d)`(每隔 `d`,以每次计划触发时刻为基准)、
`scheduling.FixedDelay(d)`(上次运行**结束后**再过 `d`;永不重叠)、
`scheduling.ParseCron(expr)`(标准 5 段 cron)。选项:
`scheduling.WithTimeout`、`scheduling.WithConcurrencyPolicy`(对
FixedDelay 无效——后者天生串行)。

锁也是 cloud 概念:`WithLock(l, key, ttl)` 选项直接收 `cloud/lock.Locker`
——就是 starter-lock-{redis,etcd,consul} 贡献的 bean 本来的类型——中间没有
适配器。

## 多副本去重

要让某任务在同一时刻只在一个副本上运行,给 Job 挂锁:把
`lock.Locker` bean（由 `starter-lock-{redis,etcd,consul}` 贡献）注进 job
构造函数直接传入 `WithLock`。每次触发都会尝试获取锁,未抢到的副本跳过
本次。

```go
func NewNightlyJob(svc *Service, lk lock.Locker) (*scheduling.Job, error) {
    tr, err := scheduling.ParseCron("0 2 * * *")
    if err != nil {
        return nil, err
    }
    return scheduling.NewJob("nightly", tr, svc.Prune,
        scheduling.WithLock(lk, "nightly", 5*time.Minute))
}
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
框架的 shutdown ctx 约束——生产上真正的期限是编排层(K8s terminationGracePeriod、systemd TimeoutStopSec),过了就强杀,所以 starter 不设第二个旋钮。这是调度器对自身优雅停机
的边界。

## 可观测性

引入 `starter-otel`（或任何 SDK provider）后，每次 job 运行会开一个 span
（`scheduler.job <name>`，带 `status` 属性），每次触发——运行或跳过——都会喂给三个 metric：
`scheduling.runs{job,status}`、`scheduling.run.duration{job,status}` 和
`scheduling.lag{job}`。status 词表为 `ok`、`error`、`panic`、`skipped_policy`、`skipped_lock`。

每次触发还会写一行日志，走自己的 tag `_app_scheduler_access`
（`log.RegisterAppTag("scheduler", "access")`）而不是默认 app tag；生命周期行
（starting / started / drain）仍留在默认 tag。该行携带与 metric 相同的 `job` 与 `status`，
外加 `reason`（跳过时）、`duration_ms` 与失败时的 `error`——因此可以从序列 join 到解释它
的那一行。

`lag` 是触发被计划的时刻与运行真正开始时刻之间的距离：它持续爬升说明运行开始得越来越
晚，这是别的 instrument 看不到的。未装 SDK provider 时以上全部为 no-op。完整的
instrument 参考见 [USAGE §1.1](USAGE_CN.md#11-可观测变体example-otel)。

## 配置项参考

| 键                               | 默认    | 说明                                       |
|----------------------------------|---------|--------------------------------------------|
| `spring.scheduler.enabled`       | `true`  | 启用调度器(注册 ≥1 个 Job 后才真正生效)。 |

## 示例

见 [`example/`](example) 中可运行的演示,覆盖 `fixed-rate`、`fixed-delay`、`cron`
以及一个带锁任务(由进程内 `MemoryLocker` 支撑,故无需 docker):

```bash
cd example && ./check.sh
```
