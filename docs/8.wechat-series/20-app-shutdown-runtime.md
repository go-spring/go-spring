# Go-Spring 实战第 20 课：应用关闭与运行期管理：优雅退出、Runner、Server 和动态配置

一个服务真正可靠，不只看它能不能启动，也要看它如何退出、如何管理运行中的任务。

在线服务需要处理退出信号、优雅关闭、长期运行任务、根 Context 和动态配置刷新。如果这些能力都散落在各个业务模块里，生命周期边界会很难统一。

所以，Go-Spring 把这些能力纳入应用运行时，而不是让每个业务模块各自处理。上一篇我们看启动，这一篇把生命周期的另一半补完整。

## Run 会接住进程退出信号

`gs.Run()` 启动成功后会监听常见退出信号。

- `SIGINT` 通常由 Ctrl+C 触发。
- `SIGTERM` 由 Docker、Kubernetes 等环境停止容器时发送。

收到信号后，Go-Spring 就会进入 `ShutDown()` 流程。

`gs.RunAsync()` 不会自动接管调用方的进程生命周期。使用它时，调用方会在自己的退出流程中调用 `stop()`。

## 关闭流程先停服务，再销毁容器

关闭过程可以理解为下面这条链路。

```text
触发 ShutDown()
  -> 取消 root context
  -> 调用所有 Server 的 Stop()
  -> 等待 Server goroutine 退出
  -> 关闭 IoC 容器
  -> 调用 Bean 的 Destroy 回调
  -> flush 日志并退出
```

Go-Spring 不设置全局强制关闭超时。这里是一个设计取舍——不同业务对关闭等待时间要求不同，框架不替业务决定超时时间。

如果某个 Server 可能长期阻塞，就要在自己的 `Stop()` 或业务逻辑中设计超时控制。

## Runner 不是长期任务的放置点

下面这个 Runner 把数据库迁移放在服务启动前执行，适合“完成之后才能对外提供服务”的准备动作。

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

Runner 在 Server 启动前执行，所以适合处理服务可用前要先完成的准备工作。不过，长期循环任务如果放进 Runner，启动流程会一直卡在这一阶段。

## Server 同时描述运行和停止

Server 接口把“如何运行”和“如何停止”放在一起，方便运行时统一调度。

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

下面这个简化 HTTP Server 展示了关键顺序，即先监听端口，监听成功后触发 Ready，停止时调用 `Shutdown`。

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

Ready 信号通常在监听成功后触发，而不是在真正具备服务能力前提前触发。这样 Go-Spring 等待 Ready 才有实际意义。

## 业务 goroutine 要从 root context 派生

业务代码从应用 root context 派生上下文后，应用关闭时 root context 会被取消，所有监听 `ctx.Done()` 的逻辑都能收到通知。

业务对象可以注入 `ContextProvider`，从里面拿到应用级 root context。

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

相比直接使用 `context.Background()`，这种方式更容易把后台逻辑纳入应用生命周期。应用退出时，后台逻辑也能感知到取消信号。

## Dync 字段承接运行期刷新

需要运行期读取新值的字段用 `gs.Dync[T]` 包装，业务代码每次通过 `Value()` 获取当前值。

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

当外部配置发生变化后，可以注入 `PropertiesRefresher` 触发一次重新加载。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

刷新只影响 `gs.Dync[T]` 字段。普通配置字段仍然保持启动时绑定的值。也就是说，动态配置和启动配置的边界是清楚的。

## 运行时补齐启动之外的另一半

启动、退出、长期任务、根 Context 和动态配置刷新合在一起，才是 Go-Spring 在线应用完整的运行时。Go-Spring 把这些能力收进统一生命周期，业务模块就不用各自发明一套关闭和刷新协议。

生命周期统一以后，应用还需要被持续观察。Go-Spring 的日志系统会继续处理结构化日志、标签路由和输出管线，让运行状态变得可检索、可治理。
