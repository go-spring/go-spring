# Go-Spring 实战第 19 课：应用启动链路：gs.Run 如何决定配置、日志、容器和 Server 的先后顺序

服务启动失败时，最后一条错误日志经常只告诉我们“某个 Server 没起来”或者“某个 Bean 创建失败”。但真正的问题可能更早发生在配置加载、日志初始化、条件判断或 root bean 选择阶段。

如果这些阶段由业务代码分散调用，启动问题就很难定位。配置什么时候可用、日志什么时候能输出、Runner 为什么早于 Server 执行、没有 Server 时谁来触发依赖图创建，这些判断都需要一条明确的生命周期链路。

Go-Spring 把这条链路收进 `gs.Run()` 和 `gs.RunAsync()`。从配置加载、日志初始化，到 IoC 容器启动、Runner 执行和 Server 启动，Go-Spring 都按固定顺序推进。这样排查启动问题时，定位依据就不再是“入口函数里写了什么”，而是“应用已经走到哪个阶段”。

理解这条链路以后，再决定哪些能力放在启动前配置，哪些逻辑放进 Runner，哪些服务实现 Server，边界会更清楚。

## Run 和 RunAsync 对应两种生命周期接管方式

启动入口首先要回答一个问题，即当前进程的生命周期由 Go-Spring 接管，还是由外部宿主接管。Go-Spring 提供了两种入口，分别对应这两种场景。

普通在线服务通常把进程生命周期交给 Go-Spring。代码里只需要调用 `gs.Run()`，后续的启动、阻塞等待和退出信号监听都会进入 Go-Spring 的应用生命周期。

```go
func main() {
	gs.Run()
}
```

`gs.Run()` 完成应用启动后，会阻塞当前 goroutine 并监听退出信号。因此独立服务优先使用这种方式，业务代码不需要再额外写一套主循环。

如果 Go-Spring 应用只是嵌入已有进程，调用方还要继续执行自己的生命周期管理，就不能让启动入口阻塞当前 goroutine。这时使用 `gs.RunAsync()`，并保存返回的 `stop` 函数。

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

`RunAsync()` 仍然会执行完整启动链路，只是不再接管整个进程。也就是说，Go-Spring 负责把应用启动起来，调用方负责在自己的退出流程里调用 `stop()`，否则 Server、容器和日志资源就没有统一关闭入口。

## 启动前配置决定 Server 开关和依赖图入口

在启动链路真正开始前，业务代码还有一次调整应用行为的机会。这个阶段适合设置内置默认值、关闭内置 Server、注册只属于当前启动的 Bean，或者指定依赖图入口。

有些进程只使用 Go-Spring 的配置和 IoC 能力，并不需要内置 HTTP Server。这个判断要在启动前表达，因为 Server 是否存在会影响后续生命周期调度。

```go
gs.Web(false).Run()
```

这和下面这个配置项表达的是同一件事，都是在启动链路进入 Server 阶段前关闭内置 HTTP Server。

```properties
spring.http.server.enabled=false
```

如果要给应用提供代码内置默认值，可以在启动前写入当前 App。这里的重点不是覆盖环境配置，而是给没有外部配置的场景提供兜底。

```go
gs.Configure(func(app gs.App) {
	app.Property("spring.http.server.addr", ":8080")
	app.Property("env", "production")
})
```

`Property()` 写入的配置优先级较低，适合作为代码内置默认值。环境差异仍然应该放到配置文件、环境变量或命令行参数中，这样同一份代码才能在不同部署环境里得到不同配置。

如果某个 Bean 只属于当前这次启动，也可以通过当前 App 注册。这样做不会改变包级 `init` 中的通用注册，只影响当前应用实例。

```go
gs.Configure(func(app gs.App) {
	app.Provide(&MyService{})
})
```

关闭内置 HTTP Server 后，还要继续考虑依赖图入口。如果当前应用没有 Runner 或 Server，可以显式标记 root bean，让 Go-Spring 从这个对象开始创建依赖图。

```go
gs.Configure(func(app gs.App) {
	app.Root(&AppEntry{})
})
```

`app.Root()` 的语义是指定依赖图入口，而不是把所有 Bean 都强制创建出来。只有从 root bean 可达的依赖，才会沿着依赖关系进入创建过程。

启动横幅也属于启动前行为。需要替换默认 Banner 时，可以在应用启动前设置文案。

```go
func init() {
	gs.Banner("My Application v1.0")
}
```

不需要 Banner 时可以设置为空字符串。这个配置只影响启动输出，不改变后续生命周期阶段。

## 启动链路按配置、日志、容器和服务顺序推进

入口选定以后，Go-Spring 会按固定顺序推进启动阶段。下面这条链路适合用来定位“失败发生在哪一步”。

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

配置必须先于日志和容器完成加载，因为日志系统、条件判断和 Bean 属性绑定都会读取配置。Go-Spring 的配置加载遵循下面这个优先级。

1. 命令行参数。
2. 环境变量。
3. Profile 配置文件。
4. 基础配置文件。
5. `Property()` 设置的配置项。

配置完成后，Go-Spring 日志系统会先于 IoC 容器初始化。原因很直接，容器启动过程本身也需要输出日志。接着，IoC 容器会注册内置 Bean，例如 `ContextProvider` 和 `PropertiesRefresher`，再从 root beans 出发创建依赖图。

这样一来，启动链路里的每一步都依赖前一个阶段准备好的能力。日志初始化失败不会伪装成 Bean 创建失败，容器启动失败也不会继续拖到 Server 阶段才暴露。

## Runner 只适合完成后才能继续启动的一次性任务

容器创建完成后，应用还没有真正对外提供服务。这个空档适合放置“必须完成之后才能启动 Server”的动作，例如数据库迁移、缓存预热、基础数据检查。Go-Spring 用 `Runner` 表达这类启动任务。

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

Runner 在容器初始化完成之后、Server 启动之前执行。所有 Runner 顺序执行，任意一个返回错误都会导致应用启动失败。这个语义让启动前置条件可以在服务对外暴露前失败，而不是让请求进来后才发现基础数据或依赖状态不对。

反过来说，Runner 必须快速返回。长期循环任务如果放进 Runner，启动链路会一直停在 Runner 阶段，Server 也就没有机会进入 Ready 流程。

## Server 承载真正长期运行的服务

长期运行的能力要实现 `Server`。HTTP、gRPC、MQ 消费者或任务调度器都属于这一类，因为它们启动后需要持续运行，并在应用退出时有明确的停止动作。

所有 Server 会在独立 goroutine 中并行启动。每个 Server 在完成监听绑定或具备服务能力后触发 Ready 信号，Go-Spring 会等待所有 Server Ready 后才认为应用启动完成。

Ready 信号的价值在于把“goroutine 已经启动”和“服务已经具备服务能力”区分开。否则健康检查可能已经开放，流量也开始进入，但某个关键 Server 还没有完成监听绑定。

## 排查启动问题先看失败阶段

排查启动问题时，最后一条错误日志往往只是结果。更有效的方式是先把错误放回启动链路里，判断它发生在配置加载、日志初始化、容器启动、Runner 执行，还是 Server 启动阶段。

Go-Spring 把启动阶段串成一条有顺序的链路，核心价值是让配置、日志、容器和服务不再各自启动。启动完成以后，应用生命周期还没有结束，退出信号、优雅关闭、长期任务、根 Context 和动态配置刷新会继续决定运行期边界。
