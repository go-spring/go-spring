# Go-Spring 实战第 19 课：应用启动流程：gs.Run 如何串起配置、日志、容器和 Server

Go-Spring 的 IoC 容器解释了对象怎样被装配，但一个应用真正启动时，容器只是其中一段流程。`gs.Run()` 看起来只是一个入口函数，其实接管了应用启动的主要链路。

从配置加载、日志初始化，到 IoC 容器启动、Runner 执行和 Server 启动，Go-Spring 都把它们串在同一条启动链路上。这样我们排查启动问题时，就能沿着链路一步步定位。

把这条链路理清楚以后，有两个直接价值：

- 能判断启动失败发生在哪个阶段。
- 能知道哪些扩展点放在启动前配置，哪些逻辑放到 Runner 或 Server。

## Run 和 RunAsync 对应两种启动方式

Go-Spring 提供了两种常用启动方式。

`gs.Run()` 是标准阻塞启动，适合普通服务进程把生命周期交给 Go-Spring 管：

```go
func main() {
	gs.Run()
}
```

它完成应用启动后阻塞当前 goroutine，并监听退出信号。如果是独立服务，通常优先使用这种方式。

`gs.RunAsync()` 是非阻塞启动，适合嵌入已有进程或测试宿主，由调用方决定什么时候停止：

```go
func main() {
	stop, err := gs.RunAsync()
	if err != nil {
		log.Fatal(err)
	}
	defer stop()

	// 继续接入已有系统的生命周期管理
}
```

`RunAsync()` 适合把 Go-Spring 集成到已有程序中。调用方需要在合适时机调用 `stop()`，确保资源正常释放。

## 启动前可以配置应用行为

启动前可以通过链式 API 或 `gs.Configure()` 调整应用行为。

如果当前应用不需要 Go-Spring 内置 HTTP Server，可以通过链式 API 关闭：

```go
gs.Web(false).Run()
```

这和下面这个配置项表达的是同一件事：

```properties
spring.http.server.enabled=false
```

如果要给应用提供代码内置默认值，可以在启动前写入当前 App：

```go
gs.Configure(func(app gs.App) {
	app.Property("spring.http.server.addr", ":8080")
	app.Property("env", "production")
})
```

这类配置优先级较低，适合作为代码内置默认值。环境差异通常放到配置文件、环境变量或命令行参数中。

如果某个 Bean 只属于当前这次启动，也可以通过当前 App 注册：

```go
gs.Configure(func(app gs.App) {
	app.Provide(&MyService{})
})
```

如果没有 Runner 或 Server 作为入口，可以显式标记 root bean，驱动依赖图创建：

```go
gs.Configure(func(app gs.App) {
	app.Root(&AppEntry{})
})
```

如果关闭了内置 HTTP Server，且没有 Runner 或 Server，容器可能没有入口触发 Bean 创建。这时候就可以用 `app.Root()` 指定依赖图入口。

如果需要替换启动横幅，可以在启动前设置 Banner 文案：

```go
func init() {
	gs.Banner("My Application v1.0")
}
```

不需要 Banner 时可以设置为空字符串。

## 启动链路按阶段串行推进

完整流程可以这样看：

```text
调用 Run() / RunAsync()
  -> 打印 Banner
  -> 加载配置
  -> 初始化日志系统
  -> 启动 IoC 容器
  -> 执行所有 Runner
  -> 并行启动所有 Server
  -> 等待 Ready
  -> 启动完成
```

配置加载遵循优先级：

1. 命令行参数。
2. 环境变量。
3. Profile 配置文件。
4. 基础配置文件。
5. `Property()` 设置的配置项。

Go-Spring 日志系统在 IoC 容器之前初始化，因为容器启动过程本身也需要输出日志。接着，IoC 容器启动时会注册内置 Bean，例如 `ContextProvider` 和 `PropertiesRefresher`，然后从 root beans 出发创建依赖图。

## Runner 适合一次性启动任务

`Runner` 用来做启动阶段的一次性初始化任务，例如数据库迁移、缓存预热、基础数据检查。

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

Runner 在容器初始化完成之后、Server 启动之前执行。所有 Runner 顺序执行，任意一个返回错误都会导致应用启动失败。

所以，Runner 更适合快速返回。长期运行任务放到 Server 里更顺。

## Server 承载长期运行服务

`Server` 用来承载长期运行服务，例如 HTTP、gRPC、MQ 消费者或任务调度器。

所有 Server 会在独立 goroutine 中并行启动。每个 Server 在完成监听绑定或具备服务能力后触发 Ready 信号。Go-Spring 会等待所有 Server Ready 后才认为应用启动完成。

这个机制可以避免健康检查已经开放、流量已经进入，但某个关键 Server 随后启动失败的问题。

## 排查启动问题要先看阶段

排查启动问题时，最后一条错误日志往往只是结果。配置加载、日志初始化、容器启动、Runner 执行、Server 启动，每个阶段都有自己的失败方式和扩展点。

启动只是生命周期的一半。应用进入运行期以后，还要继续处理退出、优雅关闭、长期任务、根 Context 和动态配置刷新。
