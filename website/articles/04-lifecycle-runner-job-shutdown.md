# Runner 处理启动动作，后台任务监听 Context

`course/04-lifecycle` 在图书查询服务上加了两类启动逻辑：

- 启动自检：确认种子图书数据存在。
- 后台统计任务：定时打印图书数量。

这两类代码很容易被塞进 `main`，或者直接写成一个裸 goroutine：

```go
go runJob()
```

短期能跑，长期会留下几个问题：启动失败怎么算，HTTP Server 什么时候开始对外服务，应用退出时 goroutine 怎么停，测试结束后后台任务会不会继续跑。

这篇用 Runner 和 Context 把这些边界拆开。

## Runner 适合启动阶段的一次性动作

启动自检应该在应用对外服务前完成。示例里检查内存 DAO 里至少有一本初始图书：

```go
type StartupCheckRunner struct {
	Service  *BookService `autowire:""`
	Required bool         `value:"${bookman.seed.required:=true}"`
}

func (r *StartupCheckRunner) Run(ctx context.Context) error {
	if r.Required && len(r.Service.ListBooks()) == 0 {
		return errors.New("book seed data is empty")
	}
	log.Printf("startup check passed, books=%d", len(r.Service.ListBooks()))
	return nil
}
```

注册成 `gs.Runner`：

```go
gs.Provide(&StartupCheckRunner{}).Export(gs.As[gs.Runner]())
```

`bookman.seed.required` 给了一个小开关：默认要求种子数据存在，如果某个环境允许空数据启动，可以改成：

```bash
go run . -Dbookman.seed.required=false
```

这种检查适合放在 Runner 里。失败时直接返回 error，让应用启动失败；成功后 Runner 结束，启动流程继续。

## 后台任务不能阻塞 Runner

图书统计任务是长期运行逻辑：

```go
type BookStatsJob struct {
	Service *BookService `autowire:""`
}

func (j *BookStatsJob) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Print("book stats job stopped")
			return
		case <-ticker.C:
			log.Printf("book count=%d", len(j.Service.ListBooks()))
		}
	}
}
```

重点是 `ctx.Done()`。应用关闭时，Go-Spring 会取消根 Context。后台循环收到信号后退出，`ticker` 也会释放。

如果没有这个退出分支，goroutine 只知道启动，不知道停。测试、热重启和优雅退出都会变得不可控。

## 用 Runner 拉起后台任务

`BookStatsJob` 本身不是 Go-Spring 的特殊接口。示例用另一个 Runner 启动它：

```go
type JobRunner struct {
	Job *BookStatsJob `autowire:""`
}

func (r *JobRunner) Run(ctx context.Context) error {
	go r.Job.Run(ctx)
	return nil
}
```

注册：

```go
gs.Provide(&BookStatsJob{})
gs.Provide(&JobRunner{}).Export(gs.As[gs.Runner]())
```

这里有一个约束：Runner 不能长期阻塞。长期任务要放到 goroutine 里，并且必须监听传入的 Context。

## 启动动作和长期服务分开

这一篇只用了 Runner 和普通后台 goroutine。第 11 篇会用 `gs.Server` 管理自定义 TCP Server。

可以先按这个规则判断：

```text
启动前检查一次 -> Runner
运行期间循环执行 -> Job + Context
长期对外监听端口 -> Server
```

Runner 适合做启动自检、预热、注册检查这类短任务。后台 Job 适合应用内部的周期性逻辑。Server 适合 HTTP、gRPC、TCP 这类需要 ready 和 stop 语义的组件。

## 运行验证

启动：

```bash
go run .
```

观察日志里是否出现自检成功，以及后续周期性的图书数量日志。

按 `Ctrl+C` 退出时，后台任务应该打印停止日志并结束。这个验证比接口返回更重要，因为它确认了任务没有脱离应用生命周期。
