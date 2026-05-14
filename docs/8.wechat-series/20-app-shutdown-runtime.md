# Go-Spring 实战第 20 课：应用关闭与运行期管理：优雅退出、Runner、Server 和动态配置

一个服务真正可靠，不只看它能不能启动，也要看它如何退出、如何管理运行中的任务。

在线服务需要处理退出信号、优雅关闭、长期运行任务、根 Context 和动态配置刷新。如果这些能力都散落在各个业务模块里，生命周期边界会很难统一。

所以，Go-Spring 把这些能力纳入应用运行时，而不是让每个业务模块各自处理。上一篇我们看启动，这一篇把生命周期的另一半补完整。

## 退出信号

`gs.Run()` 启动成功后会监听常见退出信号：

- `SIGINT`：通常由 Ctrl+C 触发。
- `SIGTERM`：Docker、Kubernetes 等环境停止容器时发送。

收到信号后，Go-Spring 就会进入 `ShutDown()` 流程。

`gs.RunAsync()` 不会自动接管调用方的进程生命周期。因此使用它时，调用方应在自己的退出流程中调用 `stop()`。

## 优雅关闭流程

关闭过程可以理解为：

```text
触发 ShutDown()
  -> 取消 root context
  -> 调用所有 Server 的 Stop()
  -> 等待 Server goroutine 退出
  -> 关闭 IoC 容器
  -> 调用 Bean 的 Destroy 回调
  -> flush 日志并退出
```

Go-Spring 不设置全局强制关闭超时。这里是一个设计取舍：不同业务对关闭等待时间要求不同，框架不替业务决定超时时间。

如果某个 Server 可能长期阻塞，就应在自己的 `Stop()` 或业务逻辑中设计超时控制。

## 实现 Runner

Runner 适合一次性初始化任务：

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

Runner 在 Server 启动前执行，因此适合处理服务可用前必须完成的准备工作。我们不应该把长期循环任务塞进 Runner，否则启动流程会被阻塞。

## 实现 Server

Server 适合长期运行服务：

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

一个简化 HTTP Server：

```go
type MyServer struct {
	Addr string `value:"${server.addr:=:8080}"`
	srv  *http.Server
}

func (s *MyServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	s.srv = &http.Server{Addr: s.Addr}

	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

	<-sig.TriggerAndWait()

	err = s.srv.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *MyServer) Stop() error {
	return s.srv.Shutdown(context.Background())
}
```

Ready 信号应在监听成功后触发，而不是在真正具备服务能力前提前触发。这样 Go-Spring 等待 Ready 才有实际意义。

## 注入根 Context

业务代码应优先从应用 root context 派生上下文。这样应用关闭时，root context 会被取消，所有监听 `ctx.Done()` 的逻辑就能收到通知。

可以注入 `ContextProvider`：

```go
type MyService struct {
	CtxProvider *gs.ContextProvider `autowire:""`
}

func (s *MyService) DoWork() {
	ctx := s.CtxProvider.Context

	select {
	case <-ctx.Done():
		return
	default:
		// 正常处理
	}
}
```

这比在业务中直接使用 `context.Background()` 更适合应用生命周期管理。否则后台逻辑可能感知不到应用正在退出。

## 刷新动态配置

动态配置字段使用 `gs.Dync[T]`：

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

运行期可以通过 `PropertiesRefresher` 刷新：

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

刷新只影响 `gs.Dync[T]` 字段。普通配置字段仍然保持启动时绑定的值，所以动态配置和启动配置的边界是清楚的。

## 运行时把生命周期补完整

启动、退出、长期任务、根 Context 和动态配置刷新合在一起，才是 Go-Spring 在线应用完整的运行时。Go-Spring 把这些能力收进统一生命周期，业务模块就不用各自发明一套关闭和刷新协议。

接下来进入日志系统，看看 Go-Spring 如何组织结构化日志、标签路由和输出管线。
