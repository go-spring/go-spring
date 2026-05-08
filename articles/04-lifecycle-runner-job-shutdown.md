# 一个后台 goroutine 让我重新理解了应用生命周期

我以前写后台任务，第一反应就是：

```go
go runJob()
```

简单，直接，好像没什么问题。

但 BookMan Pro 写到这里，我开始不放心了。如果启动时要检查图书种子数据，这个检查应该在 HTTP Server 启动前还是启动后？如果后台任务已经跑起来，但应用启动失败了怎么办？按 `Ctrl+C` 退出时，那个 goroutine 怎么停？

这些问题让我意识到：服务不是“启动一个端口”就完事了，它有完整的生命周期。

## 我先把三件事分开

Go-Spring 里有几个概念，我一开始很容易混：

```text
Runner：启动阶段执行一次
Job：应用运行期间的后台任务
Server：长期对外提供服务
```

Runner 适合做启动自检。它不应该一直阻塞。

Job 可以长期跑，但必须监听应用的 Context。

Server 更正式，适合 HTTP、gRPC、TCP 这类需要 ready 和 stop 的服务。第 11 篇再看它。

这三个边界分清以后，我就知道自己的代码该放哪里了。

## 启动自检放 Runner

我希望应用启动前检查一下：内存 DAO 里至少有一本初始图书。

可以写一个 Runner：

```go
type StartupCheckRunner struct {
	Service *BookService `autowire:""`
}

func (r *StartupCheckRunner) Run(ctx context.Context) error {
	if len(r.Service.ListBooks()) == 0 {
		return errors.New("book seed data is empty")
	}
	return nil
}
```

注册：

```go
gs.Provide(&StartupCheckRunner{}).Export(gs.As[gs.Runner]())
```

这样如果检查失败，应用会直接启动失败，不会继续把 HTTP 端口暴露出去。

我以前不太重视这种“启动前失败”。后来发现它很重要。比起服务启动成功但第一个请求就出错，启动阶段失败更容易排查。

## 后台任务必须能停

然后我写一个简单的统计 Job，每隔一段时间看一下图书数量：

```go
type BookStatsJob struct {
	Service *BookService `autowire:""`
}

func (j *BookStatsJob) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("book count=%d", len(j.Service.ListBooks()))
		}
	}
}
```

这里我最想记住的是 `<-ctx.Done()`。

没有它，这个 goroutine 就只知道开始，不知道什么时候结束。测试时尤其明显：测试结束了，后台任务还在跑，就很麻烦。

## 用 Runner 把 Job 拉起来

Job 不是 Go-Spring 的特殊接口，所以我用一个 Runner 来启动它：

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

应用收到 `Ctrl+C` 或 `SIGTERM` 时，Go-Spring 会取消根 Context。Job 收到这个信号后退出。

这比每个 goroutine 自己监听系统信号清楚多了。

## 启动顺序这次终于说得通了

我把 Go-Spring 的启动过程理解成：

```text
加载配置 -> 初始化日志 -> 启动容器 -> 执行 Runner -> 启动 Server -> 等待退出信号
```

这说明 Runner 会在 HTTP Server 对外服务前执行。

所以启动自检放 Runner 是合理的。如果我把自检放到某个后台 goroutine，可能 HTTP 已经 ready 了，自检才发现失败。这种状态很尴尬。

## 验证一下

启动：

```bash
go run .
```

看日志，确认自检执行成功后 HTTP Server 才 ready。

然后按 `Ctrl+C`。后台 Job 应该因为 Context 取消而退出。

## 我踩到的坑

不要在 Runner 里写死循环。Runner 会阻塞启动，长期任务要用 goroutine，并监听 Context。

不要让 Job 没有退出条件。每一个后台循环都应该问自己：应用关闭时我怎么停？

不要把所有启动逻辑塞进 `main`。一旦依赖注入进来，启动逻辑最好也变成容器管理的对象。

## 给自己留个小练习

加一个配置：

```properties
bookman.seed.required=true
```

当它为 `false` 时，即使没有初始图书也允许启动；为 `true` 时必须启动失败。

写到这里，我对“应用生命周期”终于有了一个朴素理解：能启动不够，还要知道什么时候准备好、什么时候该停、停的时候谁负责收尾。
