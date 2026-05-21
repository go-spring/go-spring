# Go-Spring 实战第 20 课 —— 应用关闭与运行期管理：ShutDown 如何收束服务和动态配置

上一课讲了 `gs.Run()` 怎样把应用从配置输入推进到服务 Ready。服务能启动只是第一步，线上服务更难处理的是退出时机：容器平台发送停止信号后，HTTP 请求可能还没处理完，后台任务还在循环，日志缓冲区里还有数据，配置刷新也可能正在发生。

如果这些动作分散在业务模块里，每个模块都会发明自己的关闭协议。结果往往是某些 goroutine 感知不到退出，某些 Server 已经停止但容器还没销毁，或者配置刷新和停止流程互相影响。

Go-Spring 把退出信号、Server 停止、root context 取消、Bean 销毁和动态配置刷新放进同一个运行时模型。启动链路解决服务怎样进入可用状态，关闭与运行期管理则补上生命周期的另一半。

## 退出入口

`gs.Run()` 启动成功后不会直接返回，而是继续监听常见退出信号。

| 信号 | 常见来源 |
|------|----------|
| `SIGINT` | Ctrl+C |
| `SIGTERM` | Docker、Kubernetes 等运行环境停止容器 |

独立服务通常只需要把入口交给 `gs.Run()`。

```go
func main() {
	gs.Run()
}
```

收到退出信号后，Go-Spring 会进入 `ShutDown()` 流程。这样应用启动和应用关闭都落在同一个生命周期模型里。

`gs.RunAsync()` 的边界不同。它只完成非阻塞启动，不自动接管调用方的进程生命周期。因此使用 `RunAsync()` 时，调用方要在自己的退出流程中调用启动时返回的 `stop()`。

```go
func main() {
	stop, err := gs.RunAsync()
	if err != nil {
		log.Fatal(err)
	}
	defer stop()

	// 调用方继续管理自己的生命周期
}
```

`stop()` 的意义不是简单退出一个 goroutine，而是把 Go-Spring 应用交还给同一条关闭链路。

## ShutDown 顺序

退出不是简单取消一个 context。Go-Spring 需要先让对外服务停止接收或处理新工作，再释放容器里的对象。下面这条链路说明的是：关闭顺序和启动顺序一样需要编排。

```text
触发 ShutDown()
  -> 取消 root context
  -> 调用所有 Server 的 Stop()
  -> 等待 Server goroutine 退出
  -> 关闭 IoC 容器
  -> 调用 Bean 的 Destroy 回调
  -> flush 日志并退出
```

root context 先被取消，是为了让业务 goroutine 尽早收到退出通知。接着 Go-Spring 调用所有 Server 的 `Stop()`，并等待 Server 的运行 goroutine 结束。只有长期运行服务收束以后，IoC 容器才会进入关闭阶段，Bean 的 Destroy 回调也才有稳定的资源边界。

Go-Spring 不设置全局强制关闭超时。这是留给业务的设计点，因为不同系统对关闭等待时间的要求差异很大。如果某个 Server 可能长期阻塞，就要在自己的 `Stop()` 或业务逻辑中设计超时控制。

## Runner 与 Server 边界

关闭问题经常来自启动阶段的职责放错位置。`Runner` 属于启动阶段，而不是运行期任务容器。下面的迁移任务完成一次性准备动作后就返回。

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

迁移成功后 `Run` 返回，Go-Spring 才会继续启动 Server；迁移失败时返回 error，应用直接启动失败。长期循环任务如果放进 Runner，启动流程会卡在 Server 之前，应用既不会 Ready，也不会进入正常运行期。

真正需要持续运行的能力应该实现 `Server`。下面这个简化 HTTP Server 把同一项长期服务的启动和收束放在一起。

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

Ready 信号通常在监听成功后触发，而不是在 goroutine 刚进入 `Run` 时触发。关闭时，`Stop` 负责让 `Serve` 退出。这样 Go-Spring 才能先停止长期服务，再进入容器销毁阶段。

## root context

Server 之外，业务代码里也可能启动后台 goroutine。它们如果直接使用 `context.Background()`，应用退出时就很难收到统一取消信号。

Go-Spring 在容器里提供 `ContextProvider`。业务对象可以从应用级 root context 派生自己的上下文。

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

应用进入关闭流程时，root context 取消会传递给监听 `ctx.Done()` 的业务逻辑。后台任务是否立即退出仍由业务代码决定，但退出信号来自同一个生命周期入口。

## Dync 动态字段

启动配置默认是稳定值。只有需要在运行期读取新值的字段，才用 `gs.Dync[T]` 包装。普通字段和动态字段在 Go-Spring 里是两类配置。

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

业务代码每次通过 `Value()` 获取当前值。`gs.Dync[T]` 的读取是并发安全的，适合在请求处理、后台任务或多个 goroutine 中读取。

外部配置发生变化后，可以注入 `PropertiesRefresher` 触发一次重新加载。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

刷新只影响 `gs.Dync[T]` 字段。普通配置字段仍然保持启动时绑定的值。也就是说，Go-Spring 把启动稳定值和运行期动态值分开，只有显式声明为动态字段的值才进入刷新语义。

## 应用关闭与运行期管理

运行期管理不是把所有行为都交给容器。Go-Spring 负责提供统一的退出入口、Server 停止顺序、root context 和动态配置刷新；业务代码负责让自己的长期任务响应 context、让 `Stop()` 有明确退出路径，并谨慎选择哪些配置允许动态刷新。

这条边界让启动之外的行为也可以治理。服务启动、退出、长期任务和动态配置刷新都进入同一个应用生命周期后，运行期代码就不需要各自发明关闭协议，也不会把动态配置误当成普通字段的自动更新。
