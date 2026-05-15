# Go-Spring 实战第 20 课：应用关闭与运行期管理：ShutDown 如何收束 Server、root context 和动态配置

服务能启动只是第一步。线上服务更难处理的是退出时机——容器平台发送停止信号后，HTTP 请求还没处理完，后台任务还在循环，日志缓冲区里还有数据，配置刷新也可能正在发生。

如果这些动作分散在业务模块里，每个模块都会发明自己的关闭协议。结果往往是某些 goroutine 感知不到退出，某些 Server 已经停止但容器还没销毁，或者配置刷新和停止流程互相踩到。

Go-Spring 把退出信号、Server 停止、root context 取消、Bean 销毁和动态配置刷新放进同一个运行时模型。启动流程解决服务怎样进入可用状态，关闭与运行期管理则补上生命周期的另一半。

## Run 接住退出信号后进入统一关闭链路

`gs.Run()` 启动成功后不会直接返回，而是继续监听常见退出信号。

- `SIGINT` 通常由 Ctrl+C 触发。
- `SIGTERM` 通常由 Docker、Kubernetes 等运行环境停止容器时发送。

收到信号后，Go-Spring 会进入 `ShutDown()` 流程。这个设计让独立服务不需要在 `main` 函数里再写一套 signal 监听和关闭编排。

`gs.RunAsync()` 的边界不同。它只完成非阻塞启动，不会自动接管调用方的进程生命周期。因此使用 `RunAsync()` 时，调用方要在自己的退出流程中调用启动时返回的 `stop()`，把 Go-Spring 应用交还给同一条关闭链路。

## 关闭流程先停止 Server，再销毁容器

退出不是简单取消一个 context。Go-Spring 需要先让对外服务停止接收或处理新工作，再释放容器里的对象。关闭过程可以按下面这条链路理解。

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

Go-Spring 不设置全局强制关闭超时。这是一个有意保留给业务的设计点，因为不同系统对关闭等待时间的要求差异很大。如果某个 Server 可能长期阻塞，就要在自己的 `Stop()` 或业务逻辑中设计超时控制。

## Runner 只放启动前必须完成的准备动作

`Runner` 属于启动阶段，而不是运行期任务容器。下面这个 Runner 把数据库迁移放在服务启动前执行，适合“完成之后才能对外提供服务”的准备动作。

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

这个例子的关键点是返回时机。迁移成功后 `Run` 返回，Go-Spring 才会继续启动 Server；迁移失败时返回 error，应用直接启动失败。

因此，长期循环任务不适合放进 Runner。一旦 Runner 不返回，启动流程就会一直卡在 Server 之前，应用既不会 Ready，也不会进入正常运行期。

## Server 同时描述长期运行和停止方式

真正需要持续运行的能力应该实现 `Server`。Go-Spring 通过同一个接口同时拿到运行入口和停止入口，这样运行期调度和关闭期收束可以对应起来。

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

下面这个简化 HTTP Server 展示了关键顺序。`Run` 先绑定监听端口，监听成功后触发 Ready，随后阻塞在 `Serve`；关闭时，`Stop` 负责调用 `Shutdown`。

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

Ready 信号通常在监听成功后触发，而不是在 goroutine 刚进入 `Run` 时触发。这样 Go-Spring 等待 Ready 才有实际意义，外部健康检查看到应用就绪时，Server 至少已经完成关键启动动作。

## 业务 goroutine 要从 root context 派生

Server 之外，业务代码里也可能启动后台 goroutine。它们如果直接使用 `context.Background()`，应用退出时就很难收到统一取消信号。

Go-Spring 在容器里提供 `ContextProvider`。业务对象可以注入它，再从应用级 root context 派生自己的上下文。

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

这样一来，应用进入关闭流程时，root context 取消会传递给监听 `ctx.Done()` 的业务逻辑。后台任务是否立即退出仍由业务代码决定，但退出信号来自同一个生命周期入口。

## Dync 字段只承接明确声明的运行期刷新

启动配置默认是稳定值。只有需要在运行期读取新值的字段，才用 `gs.Dync[T]` 包装，业务代码每次通过 `Value()` 获取当前值。

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

当外部配置发生变化后，可以注入 `PropertiesRefresher` 触发一次重新加载。下面的例子用环境变量改写配置，再调用刷新入口。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

刷新只影响 `gs.Dync[T]` 字段。普通配置字段仍然保持启动时绑定的值。也就是说，Go-Spring 把启动配置和动态配置分成两类，只有显式声明为动态字段的值才进入运行期刷新语义。

## 运行时边界让启动之外的行为也可治理

启动、退出、长期任务、root context 和动态配置刷新合在一起，才构成 Go-Spring 在线应用完整的运行时。Go-Spring 统一这些边界后，业务模块就不用各自发明关闭和刷新协议。

生命周期统一以后，应用还需要被持续观察。运行期发生的请求、任务、依赖调用和错误，都要继续通过日志系统进入可检索、可治理的观测链路。
