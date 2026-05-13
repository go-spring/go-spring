# 应用启动机制

`gs.Run()` 看起来只是一个入口函数，但它实际接管了应用启动的主要流程。

从配置加载、日志初始化，到 IoC 容器启动、Runner 执行和 Server 启动，Go-Spring 都把它们串在同一条启动链路上。

理解这条链路有两个直接价值：

- 能判断启动失败发生在哪个阶段。
- 能知道哪些扩展点应该在启动前配置，哪些逻辑应该放到 Runner 或 Server。

## 启动方式

Go-Spring 提供两种常用启动方式。

`gs.Run()` 是标准阻塞启动：

```go
func main() {
	gs.Run()
}
```

它完成应用启动后阻塞当前 goroutine，并监听退出信号。独立服务通常优先使用这种方式。

`gs.RunAsync()` 是非阻塞启动：

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

## 启动配置

启动前可以通过链式 API 或 `gs.Configure()` 调整应用行为。

关闭内置 HTTP Server：

```go
gs.Web(false).Run()
```

等价于设置：

```properties
spring.http.server.enabled=false
```

设置默认配置：

```go
gs.Configure(func(app gs.App) {
	app.Property("spring.http.server.addr", ":8080")
	app.Property("env", "production")
})
```

这类配置优先级较低，适合作为代码内置默认值。环境差异仍建议放到配置文件、环境变量或命令行参数中。

注册当前应用实例专属 Bean：

```go
gs.Configure(func(app gs.App) {
	app.Provide(&MyService{})
})
```

标记 Root Bean：

```go
gs.Configure(func(app gs.App) {
	app.Root(&AppEntry{})
})
```

如果关闭了内置 HTTP Server，且没有 Runner 或 Server，容器可能没有入口触发 Bean 创建。`app.Root()` 可以指定依赖图入口。

自定义 Banner：

```go
func init() {
	gs.Banner("My Application v1.0")
}
```

不需要 Banner 时可以设置为空字符串。

## 启动流程

完整流程可以概括为：

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

日志系统在 IoC 容器之前初始化，因为容器启动过程本身也需要输出日志。

IoC 容器启动时会注册内置 Bean，例如 `ContextProvider` 和 `PropertiesRefresher`，然后从 root beans 出发创建依赖图。

## Runner

`Runner` 用于启动阶段的一次性初始化任务，例如数据库迁移、缓存预热、基础数据检查。

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

Runner 在容器初始化完成之后、Server 启动之前执行。所有 Runner 顺序执行，任意一个返回错误都会导致应用启动失败。

Runner 应该快速返回。长期运行任务应该实现为 Server。

## Server

`Server` 用于承载长期运行服务，例如 HTTP、gRPC、MQ 消费者或任务调度器。

所有 Server 会在独立 goroutine 中并行启动。每个 Server 应在完成监听绑定或具备服务能力后触发 Ready 信号。框架会等待所有 Server Ready 后才认为应用启动完成。

这个机制避免健康检查已经开放、流量已经进入，但某个关键 Server 随后启动失败的问题。

## 启动流程要看阶段

启动问题不能只看最后一条错误日志。配置加载、日志初始化、容器启动、Runner 执行、Server 启动，每个阶段都有自己的失败方式和扩展点。

应用启动起来之后，还要面对退出、关闭和运行期刷新。生命周期的另一半，交给应用运行时继续处理。
