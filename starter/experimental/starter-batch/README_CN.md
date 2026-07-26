# starter-batch

[English](README.md) | [中文](README_CN.md)

`starter-batch` 以 Go-Spring 应用生命周期的一部分运行
[`spring/batch`](../../spring/batch) 定义的批处理任务——分块(chunk)长批与一次性
Cloud Task 都覆盖。空导入后,为每个批任务注册一个 `JobDefinition`,然后要么在配置
里打开 `run-on-startup=true`(Cloud Task),要么在另一个 server(比如
`starter-scheduler`)里通过导出的 `*Launcher` bean 触发它。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):不开监听端口,而是导出一个 `gs.Server`,
让 runner 加入 server 生命周期——应用就绪后启动型任务才开始触发;收到 `SIGTERM`
时,进程退出前会先排空在途运行。

引擎、`Reader`/`Processor`/`Writer` 接口、`JobRepository` 缝隙以及进程内 memory
repository 全部来自零依赖的 [`spring/batch`](../../spring/batch) 包;本 starter
只是把配置与 IoC 容器接入其上的薄集成层。持久化后端(Redis、SQL 等)是独立的
starter,各自贡献自己的 `batch.JobRepository` bean。

## 安装

```bash
go get go-spring.org/starter-batch
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-batch"
```

### 2. 为每个批任务注册 `JobDefinition`

`batch.Provide` 会以任务名命名 bean 并将其导出为 `JobDefinition`,这样 runner
就能收集它并按名字匹配到对应的配置项。

```go
import (
    starter "go-spring.org/starter-batch"
    "go-spring.org/spring/cloud/batch"
)

var reportStep = batch.Func("generate", func(ctx context.Context) error {
    return svc.GenerateReport(ctx)
})

func main() {
    // Cloud Task:一次性函数包装为单步 job。
    starter.Provide("report", reportStep)

    // 分块任务:一个或多个有类型的 step 组合成 job。
    starter.Provide("reconcile", &batch.ChunkStep[Row, Row]{
        Name:      "load",
        Reader:    &csvReader{path: "/data/txn.csv"},
        Writer:    &sqlWriter{db: db},
        ChunkSize: 500,
    })

    gs.Run()
}
```

### 3. 声明哪些任务在启动时运行

```properties
# 应用就绪后触发一次 "report"(Cloud Task)。
spring.batch.jobs.report.run-on-startup=true
spring.batch.jobs.report.params.date=2026-07-19

# "reconcile" 没有配置项:由其他 bean(scheduler.Job、HTTP handler 等)
# 通过注入 *Launcher 按需触发。
```

配置了却没有同名 `JobDefinition` bean 是**快速失败的启动错误**——拼写错误会
在启动时暴露,而不是变成一个悄无声息永不触发的任务。

## 两种形态:Cloud Task 与定时批处理

starter 支持两种触发方式,对应 Spring Batch 的两种常见用法:

| 形态       | 触发方式                                                     | 配置                                                     |
|------------|--------------------------------------------------------------|----------------------------------------------------------|
| Cloud Task | 应用就绪时触发一次。                                          | `spring.batch.jobs.<name>.run-on-startup=true`           |
| 定时批处理 | 每次 scheduler 触发(cron / fixed-rate / fixed-delay)时触发。 | `spring.scheduler.jobs.<name>.cron=…` + 注入 `*Launcher` |

对于定时形态,注册一个 `scheduler.Job` 注入 `*Launcher` 并调用 `Launch`:

```go
import (
    scheduler "go-spring.org/starter-scheduler"
    starter   "go-spring.org/starter-batch"
    "go-spring.org/spring/cloud/batch"
)

type NightlyReconcile struct {
    Launcher *starter.Launcher `autowire:""`
}

func (n *NightlyReconcile) JobName() string { return "nightly-reconcile" }

func (n *NightlyReconcile) Run(ctx context.Context) error {
    _, err := n.Launcher.Launch(ctx, "reconcile", batch.Params{
        "date": time.Now().Format("2006-01-02"),
    })
    return err
}

func main() {
    gs.Provide(&NightlyReconcile{}).
        Name("nightly-reconcile").
        Export(gs.As[scheduler.Job]())
    gs.Run()
}
```

```properties
spring.scheduler.jobs.nightly-reconcile.cron=0 2 * * *
```

启动触发与手工触发共用同一个 repository,所以无论如何触发,重启语义是一致的。

## 进度存储(JobRepository)

引擎通过 `batch.JobRepository` bean 记录进度。runner 分三步选择:

1. 若设置了 `spring.batch.repository`,必须存在同名 repo bean。找不到即
   **启动快速失败**。
2. 否则,如果只存在一个 repo bean,就用它(常见场景:应用导入了一个持久化后端
   starter,例如 `starter-batch-redis`)。
3. 否则,runner 回退到 `batch.NewMemoryRepository()`。这是**进程内**的实现,
   进程崩溃会丢失全部进度,仅适合测试与 demo。

多个 repo bean 但未显式设置 `spring.batch.repository` 是**启动快速失败**——
静默选一个只会让操作者迷惑。

## 优雅停机

收到 `SIGTERM` 后,runner 取消所有在途的启动型运行并等它们结束,受
`spring.batch.drain-timeout`(默认 `30s`)约束——它是框架级
`app.shutdown.timeout` 之上的一层保险。遵守 context 的 step 会及时返回,并把
step 留在 `stopped` 状态——下次启动可以从最后一次提交的 checkpoint **续跑**。

## 配置项参考

| 键                                             | 默认    | 说明                                                                                          |
|------------------------------------------------|---------|-----------------------------------------------------------------------------------------------|
| `spring.batch.enabled`                         | `true`  | 启用 runner(注册 ≥1 个 `JobDefinition` 后才真正生效)。                                       |
| `spring.batch.repository`                      | —       | 用作进度存储的 `batch.JobRepository` bean 的名字。                                             |
| `spring.batch.drain-timeout`                   | `30s`   | `Stop` 等待在途启动型运行的最长时间。                                                          |
| `spring.batch.jobs.<name>.run-on-startup`      | `false` | 为 `true` 时,应用就绪后触发一次该任务。                                                       |
| `spring.batch.jobs.<name>.params.<k>`          | —       | 启动触发时的参数。与任务名一起构成 repository 中的 job 实例标识。                              |

## 示例

见 [`example/`](example) 中可运行的演示。
