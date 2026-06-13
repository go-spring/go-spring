# Go-Spring 实战第 18 课 —— App 使用：启动、配置与运行期扩展

上一篇文章，梳理了 Go-Spring App 的运行模型。我们了解了，应用在启动时会依次完成配置加载、日志初始化、容器启动、Runner 执行和 Server 启动，退出时会停止服务并释放资源。在此基础上，本篇咱们来看看如何在项目中使用 App。

对于大多数服务而言，调用 `gs.Run()` 就可以完成应用的启动。但实际项目中往往还有更多的需求，例如关闭内置的 HTTP Server、在不依赖配置文件时设置默认配置、注册当前应用专属的 Bean，或者执行数据库迁移、接入消息消费者、自定义网络服务等。

本篇，我们围绕这些使用场景，介绍 Go-Spring App 提供的主要入口和扩展方式。

## 启动应用

对于普通的独立应用，我们可以直接在 `main()` 函数中调用 `gs.Run()`。

```go
func main() {
	gs.Run()
}
```

这种写法适合生命周期完全由 Go-Spring 管理的应用，也是最常见的启动方式。

如果项目已经有了自己的运行流程，需要将 Go-Spring 嵌入现有程序，可以使用 `gs.RunAsync()`。

```go
func main() {
	stop, err := gs.RunAsync()
	if err != nil {
		log.Fatal(err)
	}
	defer stop()

	// 继续执行当前程序自己的逻辑
}
```

`RunAsync()` 不会监听操作系统信号，调用方需要在自己的退出流程中调用 `stop()`。这个函数不仅会发送停止信号，还会等待 Server 退出和容器关闭，确保相关资源得到释放。

## 启动前配置

大多数应用直接使用 App 的默认行为即可。需要进一步定制时，可以使用 `gs.Configure()`。它接收一个配置函数，并返回当前应用的启动器。

下面看一个示例。

```go
type MyService struct {
	// ...
}

type AppEntry struct {
	Service *MyService `autowire:""`
}

func main() {
	gs.Configure(func(app gs.App) {
		app.Property("service.timeout", "30s")
		app.Provide(&MyService{})
		app.Root(&AppEntry{})
	}).Run()
}
```

在这个示例中，我们使用了 `Property`、`Provide` 和 `Root` 三个函数。

`app.Property()` 用于在代码中设置默认配置。这类配置的优先级较低，配置文件、Profile、环境变量和命令行参数都可以覆盖它。因此，`Property` 更适合用来表达没有外部配置时，应用所采用的默认值。

`app.Provide()` 用于向当前 App 的 IoC 容器注册 Bean。它与全局的 `gs.Provide()` 使用相同的 Bean 注册规则，但两者的注册范围不同。`app.Provide()` 注册的 Bean 只属于本次创建的 App，因此更适合注册应用入口专属对象，以及集成场景。

`app.Root()` 用于注册当前应用的根 Bean。如果应用关闭了内置 HTTP Server，并且没有其他 Runner 或 Server，可以通过 `Root` 让指定的 Bean 随应用一起启动。

### 关闭内置 HTTP Server

Go-Spring 默认启动内置的 HTTP Server。如果当前程序只需要使用配置、IoC 容器或其他类型的 Server，可以通过 `gs.Web(false)` 将其关闭。

```go
func main() {
	gs.Web(false).Run()
}
```

`gs.Web(false)` 实际上是下面这项配置的便捷写法。

```go
app.Property("spring.http.server.enabled", "false")
```

`gs.Web(false)` 返回的也是应用启动器，因此可以与 `Configure()` 组合使用。

```go
func main() {
	gs.Web(false).Configure(func(app gs.App) {
		app.Root(&AppEntry{})
	}).Run()
}
```

### 自定义 Banner

Go-Spring 支持自定义 Banner。可以通过 `gs.Banner()` 修改应用启动时打印的内容。

```go
func init() {
	gs.Banner(`
  My Application
  Powered by Go-Spring
`)
}
```

如果不需要打印 Banner，将它设置为空字符串即可。

```go
func init() {
	gs.Banner("")
}
```

## Runner

有些任务必须在应用对外提供服务之前完成，例如检查数据库状态、执行数据迁移或者预热缓存等。这类任务的特点是完成之后即可退出，不需要在应用运行期间持续阻塞，因此适合实现为 `Runner`。

`Runner` 的定义如下：

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

下面的示例展示了如何在应用启动时创建数据库表。

```go
type DBMigrator struct {
	DB *sql.DB `autowire:""`
}

func (m *DBMigrator) Run(ctx context.Context) error {
	_, err := m.DB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		);
	`)
	return err
}

func init() {
	gs.Provide(&DBMigrator{}).Export(gs.As[gs.Runner]())
}
```

注册 `DBMigrator` Bean 时，需要使用 `Export(gs.As[gs.Runner]())` 将它导出为 `gs.Runner`，这样 App 才能发现并执行它。

如果存在多个 Runner，它们会按顺序执行。只有前一个 Runner 执行完成后，下一个 Runner 才会开始执行。任意一个 Runner 返回错误，应用都会终止启动。

此外，`Runner.Run()` 接收应用的根 Context。如果数据库操作、HTTP 客户端请求等支持 Context，我们可以继续向下传递这个参数，使启动任务在应用取消时及时结束。

## Server

后端应用通常需要运行 HTTP Server、gRPC Server 或者消息消费者等长期服务。这些组件的特点是需要在应用运行期间持续工作，直到应用退出时才停止，因此适合实现为 `Server`。

`Server` 的定义如下：

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

下面是一个简化的 HTTP Server 示例。

```go
type MyServer struct {
	Addr string `value:"${server.addr:=:8080}"`
	srv  *http.Server
}

func (s *MyServer) Run(_ context.Context, sig gs.ReadySignal) error {
	srv := &http.Server{Addr: s.Addr}

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.srv = srv

	<-sig.TriggerAndWait()

	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *MyServer) Stop() error {
	if s.srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

func init() {
	gs.Provide(&MyServer{}).Export(gs.As[gs.Server]())
}
```

与 Runner 一样，Server 也需要导出为对应的接口，这样 App 才能发现它。

`Server.Run()` 用于启动并持续运行服务。在上面的示例中，Server 先调用 `net.Listen()` 完成端口绑定，然后通过 `TriggerAndWait()` 通知 App 当前 Server 已经准备就绪。所有 Server 都准备完成后，Ready 信号才会统一放行，各个服务随即开始正式运行。

`Server.Stop()` 用于停止服务。服务关闭时可能需要等待正在处理的请求或者连接结束，而 Go-Spring 无法替具体服务确定合适的等待时间。因此，示例使用了带有 5 秒超时的 Context，为优雅关闭设置明确的时间上限，避免应用一直阻塞在退出流程中。

## 使用应用 Context

Runner 和 Server 的 `Run()` 方法都会直接接收应用的根 Context，因此可以通过 `ctx.Done()` 监听应用的退出信号。如果普通 Bean 也需要感知应用退出，可以注入 `ContextProvider` Bean，通过它来获取应用的根 Context。

下面看一个示例。

```go
type MyService struct {
	CtxProvider *gs.ContextProvider `autowire:""`
}

func (s *MyService) StartTask() {
	ctx := s.CtxProvider.Context

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 执行后台任务
			}
		}
	}()
}
```

我们不建议在应用中直接使用 `context.Background()`，因为这样创建的 Context 无法延续应用的取消信号和上下文信息。而从应用的根 Context 派生子 Context，可以确保任务的跟踪信息和生命周期保持连续。

## 刷新动态配置

App 正式运行之后，普通配置字段不会再次绑定。如果某个开关、阈值或者超时时间需要在运行期间更新，可以将对应字段声明为 `gs.Dync[T]` 类型。这样，当应用触发动态配置刷新时，这些字段的值就会被安全更新。为了触发动态配置刷新，我们需要注入 `PropertiesRefresher` Bean。

下面是一个示例。

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}

type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) Reload() error {
	return m.Refresher.RefreshProperties()
}
```

当外部配置发生变化后，我们可以调用 `RefreshProperties()` 重新加载配置。该方法会先执行与启动阶段相同的配置加载逻辑，然后更新所有动态字段。由于前面的文章已经详细介绍过动态配置的绑定、校验和原子更新规则，这里就不再重复展开了。
