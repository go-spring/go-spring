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
[`spring/scheduling`](../../spring/scheduling) 包;本 starter 只是把配置与 IoC 容器
接入其上的薄集成层。

## 安装

```bash
go get go-spring.org/starter-scheduler
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-scheduler"
```

### 2. 为每个工作单元注册 Job

`scheduler.Provide` 会以任务名命名 bean 并将其导出为 `Job`,这样调度器就能收集它并
按名字匹配到对应的配置项。

```go
import scheduler "go-spring.org/starter-scheduler"

func main() {
    scheduler.Provide("cleanup", func(ctx context.Context) error {
        return svc.Cleanup(ctx)
    })
    gs.Run()
}
```

### 3. 声明各任务的调度

```properties
# 每 5 分钟(标准 5 段 cron)
spring.scheduler.jobs.cleanup.cron=*/5 * * * *

# 每 30 秒,以每次计划触发时刻为基准
spring.scheduler.jobs.heartbeat.fixed-rate=30s

# 上一次运行结束后再过 10 秒(永不重叠)
spring.scheduler.jobs.reindex.fixed-delay=10s
```

配置了却没有同名 `Job` bean,或 `Job` bean 的触发方式缺失 / 有歧义,都是**快速失败
的启动错误**——拼写错误会在启动时暴露,而不是变成一个悄无声息永不触发的任务。

## 触发方式

每个任务必须**恰好设置一个**:

| 键            | 含义                                                       |
|---------------|-----------------------------------------------------------|
| `cron`        | 标准 5 段 cron 表达式(`分 时 日 月 周`)。                 |
| `fixed-rate`  | 每隔固定间隔触发,以每次计划触发时刻为基准。               |
| `fixed-delay` | 上一次运行**结束后**再过该间隔触发;永不重叠。             |

## 每任务选项

| 键            | 默认    | 含义                                                            |
|---------------|---------|-----------------------------------------------------------------|
| `timeout`     | `0`     | 为正时,运行超过该时长后其 context 被取消。                      |
| `concurrency` | `skip`  | `fixed-rate`/`cron` 的重叠策略:`skip`、`queue` 或 `replace`。   |
| `lock`        | —       | 一个 `lock.Locker` bean 的名字;每次触发只有持锁者运行。         |
| `lock-key`    | 任务名  | 在 locker 上获取的键。                                          |
| `lock-ttl`    | `30s`   | 租约时长;持锁期间自动续租。                                     |

`concurrency` 对 `fixed-delay` 任务无效——后者天生串行。

## 多副本去重

要让某任务在同一时刻只在一个副本上运行,把它的 `lock` 指向由
`starter-lock-{redis,etcd,consul}` 贡献的 `lock.Locker` bean。每次触发都会尝试获取
锁,未抢到的副本跳过本次。

```properties
spring.scheduler.jobs.nightly.cron=0 2 * * *
spring.scheduler.jobs.nightly.lock=jobs      # 名为 "jobs" 的 lock.Locker bean
spring.scheduler.jobs.nightly.lock-ttl=5m
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
`spring.scheduler.drain-timeout`(默认 `30s`)约束——它是框架级
`app.shutdown.timeout` 之上的一层保险。

## 配置项参考

| 键                               | 默认    | 说明                                       |
|----------------------------------|---------|--------------------------------------------|
| `spring.scheduler.enabled`       | `true`  | 启用调度器(注册 ≥1 个 Job 后才真正生效)。 |
| `spring.scheduler.drain-timeout` | `30s`   | `Stop` 等待在途运行的最长时间。            |
| `spring.scheduler.jobs.<name>.*` | —       | 每任务的触发方式与选项(见上)。           |

## 示例

见 [`example/`](example) 中可运行的演示,覆盖 `fixed-rate`、`fixed-delay`、`cron`
以及一个带锁任务(由进程内 `MemoryLocker` 支撑,故无需 docker):

```bash
cd example && ./check.sh
```
