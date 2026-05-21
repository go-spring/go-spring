# Go-Spring 实战第 19 课 —— 应用启动链路：gs.Run 如何串起配置、日志、容器和服务

IoC 解决的是对象装配，但一个在线服务真正启动时，问题不止是 Bean 能不能创建出来。配置要先加载，日志要能输出，容器要完成依赖图，启动任务要先于服务执行，Server 还要在真正可用后再声明 Ready。

如果这些阶段由业务代码分散调用，启动问题就很难定位。配置什么时候可用、日志什么时候初始化、Runner 为什么早于 Server 执行、没有 Server 时谁来触发依赖图创建，这些判断都需要一条明确的生命周期链路。

Go-Spring 把这条链路收进 `gs.Run()` 和 `gs.RunAsync()`。从配置加载、日志初始化，到 IoC 容器启动、Runner 执行和 Server 启动，Go-Spring 都按固定顺序推进。理解这条链路以后，排查启动失败时就可以先判断应用已经走到哪个阶段。

## Run 和 RunAsync

启动入口首先要回答一个问题：当前进程的生命周期由 Go-Spring 接管，还是由外部宿主管理。

下面这个例子要证明的是：独立在线服务通常直接把生命周期交给 Go-Spring。

```go
func main() {
	gs.Run()
}
```

`gs.Run()` 会完成应用启动，并在启动成功后阻塞当前 goroutine，监听退出信号。对于独立服务，业务代码不需要再写一套主循环、signal 监听和统一关闭编排。

如果 Go-Spring 应用只是嵌入已有进程，调用方还要继续执行自己的生命周期管理，就不能让入口阻塞当前 goroutine。这时使用 `gs.RunAsync()`。

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

`RunAsync()` 仍然会执行完整启动链路，只是不接管进程等待。调用方需要在自己的退出流程里调用 `stop()`，把 Server、容器和日志资源交回 Go-Spring 的关闭链路。

## 启动前配置

启动链路真正开始前，业务代码还有一次调整应用行为的机会。这个阶段适合设置代码内置默认值、关闭内置 Server、注册当前应用专属 Bean，或者指定依赖图入口。

下面这个例子要证明的是：有些进程只使用 Go-Spring 的配置和 IoC 能力，不需要内置 HTTP Server。

```go
func main() {
	gs.Web(false).Run()
}
```

`gs.Web(false)` 和设置 `spring.http.server.enabled=false` 表达的是同一类启动意图，都是在 Server 阶段前关闭内置 HTTP Server。

代码内置默认值也应该在启动前写入当前 App。

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Property("spring.http.server.addr", ":8080")
		app.Property("env", "production")
	}).Run()
}
```

`app.Property()` 适合作为可覆盖兜底值。环境差异仍然应该放到配置文件、环境变量或命令行参数中，这样同一份代码才能在不同部署环境里得到不同最终配置。

当前应用专属 Bean 可以通过 `app.Provide()` 注册。

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Provide(&MyService{})
	}).Run()
}
```

这类注册只影响当前应用实例，不会改变包级 `init()` 中的通用注册。

如果关闭了内置 Server，应用里又没有 Runner 或其他 Server，就可能缺少驱动依赖图展开的入口。这时可以显式标记 root bean。

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Root(&AppEntry{})
	}).Run()
}
```

`app.Root()` 的语义是指定依赖图入口，而不是把所有 Bean 都强制创建出来。只有从 root bean 可达的依赖，才会沿着依赖关系进入创建过程。

## 启动顺序

入口选定以后，Go-Spring 会按固定顺序推进启动阶段。下面这条链路要证明的是：配置、日志、容器和服务不是互相独立启动的。

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

配置必须先于日志和容器完成加载，因为日志系统、条件判断和 Bean 属性绑定都会读取配置。Go-Spring 的配置来源优先级在前文已经展开过，这里只保留启动链路里的判断：命令行参数和环境变量离本次启动最近，Profile 配置高于基础配置，`app.Property()` 适合作为代码内置默认值。

配置完成后，Go-Spring 日志系统会先于 IoC 容器初始化。原因很直接，容器启动过程本身也需要输出日志。接着，IoC 容器会注册内置 Bean，例如 `ContextProvider` 和 `PropertiesRefresher`，再从 root bean 出发创建依赖图。

这样一来，启动链路里的每一步都依赖前一个阶段准备好的能力。日志初始化失败不会伪装成 Bean 创建失败，容器启动失败也不会拖到 Server 阶段才暴露。

## Runner

容器创建完成后，应用还没有真正对外提供服务。这个空档适合放置必须完成之后才能启动 Server 的一次性任务，例如数据库迁移、缓存预热、基础数据检查。

Go-Spring 用 `Runner` 表达这类启动任务。下面这个接口要证明的是：Runner 是启动阶段动作，而不是长期任务容器。

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

Runner 在容器初始化完成之后、Server 启动之前执行。所有 Runner 会顺序执行，任意一个返回错误都会导致应用启动失败。这个语义让启动前置条件可以在服务对外暴露前失败，而不是让请求进来后才发现基础数据或依赖状态不对。

因此，Runner 必须返回。长期循环任务如果放进 Runner，启动链路会一直停在 Runner 阶段，Server 也就没有机会进入 Ready 流程。

## Server

长期运行的能力应该实现 `Server`。HTTP、gRPC、MQ 消费者或任务调度器都属于这一类，因为它们启动后需要持续运行，并在应用退出时有明确的停止动作。

下面这个接口要证明的是：Server 同时描述运行入口和停止入口。

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

所有 Server 会在独立 goroutine 中并行启动。每个 Server 在完成监听绑定或具备服务能力后触发 Ready 信号，Go-Spring 会等待所有 Server Ready 后才认为应用启动完成。

Ready 信号的价值在于区分“goroutine 已经启动”和“服务已经具备服务能力”。否则健康检查可能已经开放，流量也开始进入，但某个关键 Server 还没有完成监听绑定。

## 启动问题定位

排查启动问题时，最后一条错误日志往往只是结果。更有效的方式是把错误放回启动链路里，判断它发生在配置加载、日志初始化、容器启动、Runner 执行，还是 Server 启动阶段。

配置文件解析失败、Profile 没加载、环境变量覆盖异常，入口在配置阶段。日志输出不符合预期，要先看日志初始化读取到的配置。找不到依赖、Bean 冲突或初始化回调失败，入口在容器阶段。数据库迁移失败、缓存预热失败，通常发生在 Runner 阶段。端口占用、Ready 未触发，则要看 Server 阶段。

## 应用启动链路

Go-Spring 的应用启动链路把配置、日志、容器、Runner 和 Server 串成一个有顺序的过程。这个顺序让启动失败可以定位到具体阶段，也让业务代码知道哪些逻辑应该放在启动前配置，哪些应该放进 Runner，哪些应该实现为 Server。

启动完成以后，应用生命周期还没有结束。退出信号、优雅关闭、root context 和动态配置刷新，会继续决定运行期边界。
