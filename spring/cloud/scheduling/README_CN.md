# scheduling
[English](README.md) | [中文](README_CN.md)

`scheduling` 是与框架无关、零依赖的周期任务与定时任务抽象——Spring
`@Scheduled` / `TaskScheduler` 的 Go 惯用等价物。job 是一个普通函数,绑定到
`Trigger`,由参与应用生命周期的 `Scheduler` 驱动执行。

## 特性

- 零第三方依赖;cron parser 有意放在本包内,保 stdlib 自足。
- 三种内置 trigger:`FixedRate`、`FixedDelay`、`Cron`(5 段表达式由
  `ParseCron` 解析)。
- `ConcurrencyPolicy`(`Skip`、`Queue`、`Replace`)管理 fixed-rate / cron 的
  并发运行;fixed-delay 天然串行。
- 每次运行的 `WithTimeout` 到期后 cancel job 的 ctx。
- `WithLock(locker, key)`——通过极简本地 `Locker` 接口在多副本间去重。
  `spring/lock.Locker` 由集成层(`starter-scheduler`)桥接为该本地接口,让本
  包保持零依赖。
- 运行 panic-guard,`Stop` 时确定性 drain,可选 `Observer` 钩子上报指标/日志。

## 快速开始

Import 路径: `go-spring.org/spring/scheduling`。

```go
package main

import (
    "context"
    "log"
    "time"

    "go-spring.org/spring/cloud/scheduling"
)

func main() {
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
        scheduling.WithConcurrencyPolicy(scheduling.Skip),
        scheduling.WithTimeout(3*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    _ = sch.Start(ctx)
    time.Sleep(35 * time.Second)
    cancel()
    _ = sch.Stop(context.Background())
}
```

cron 表达式用 `scheduling.Cron("*/5 * * * *")`(表达式非法直接 panic),或
`scheduling.ParseCron` 自行处理错误。多副本去重使用 `starter-scheduler`——它
会把 `spring/lock.Locker` 桥接到本包的本地 `Locker` 接口并烤入 TTL / 续期选
项。
