# 应用启动与关闭

Go-Spring 提供了完整的应用生命周期管理，定义了清晰的启动和关闭流程，将所有组件的生命周期统一管理。本文详细介绍应用启动原理、使用方式以及如何自定义 Runner 和 Server。

## 设计原则与约束

Go-Spring 的启停流程遵循以下原则：

1. **启动流程线性不可回滚**：一旦开始启动，失败直接退出，不回滚已创建资源
2. **init 阶段仅注册元数据**：`init()` 函数只做 Bean/Module 注册，不执行业务逻辑
3. **启动失败即进程退出**：不清理已初始化资源，交给 OS 回收
4. **仅支持启动期注入**：不支持运行时动态注入，保持设计简单
5. **长期运行行为必须封装为 Server**：需要 `start/stop` 生命周期的组件都应该是 Server
6. **Runner 为一次性执行单元**：执行完成即结束，**不允许**在 Runner 中阻塞或启动后台 goroutine
7. **所有生命周期受 root ctx 约束**：整个应用的生命周期由 root context 管理
8. **日志必须在进程退出前完成 flush**：确保日志不丢失
9. **Server ready 之后发生异常进入 shutdown 流程**：优雅退出

## init 阶段（元数据注册）

`init()` 函数的唯一职责就是**注册元数据**：
- 注册全局 Bean
- 注册 Module

不建议在 `init()` 中做任何其他事情：
- 不要执行业务逻辑
- 不要触发副作用
- 不要建立网络连接

## 完整启动流程

应用启动过程是顺序执行的，完整流程如下：

### 1. 应用构建阶段
- 创建 App 实例，初始化 root context
- 打印 banner（支持自定义配置）
- 执行所有 `configure` 回调，允许用户在启动前自定义配置和注册 Bean

### 2. 配置加载阶段
按优先级合并所有来源的配置，优先级从高到低：
  1. 命令行参数 (`-Dkey=value`)
  2. 环境变量 (`GS_key=value`)
  3. 配置文件（支持 `application.properties`、`application.yaml` 等，包含 import 导入机制）
  4. 代码中通过 `app.Property()` 设置的配置
- 配置加载或解析失败 → 立即终止启动

### 3. 日志初始化阶段
- 解析 `logging` 配置节
- 初始化全局日志组件，设置日志级别、输出格式、输出位置
- 此时日志系统就绪，后续启动流程可以正常输出日志

### 4. IoC 容器初始化
- 注册 `App` 自身、`ContextProvider`、`PropertiesRefresher` 为内置 Bean
- 从 root beans 出发，递归遍历依赖图，**按需创建并自动注入**所有 Bean
- 自动收集所有实现了 `Runner` 和 `Server` 接口的 Bean
- 如果没有动态配置字段，释放配置缓存以节省内存

### 5. Runner 执行阶段
- **顺序执行**所有 Registered Runner
- Runner 是一次性执行单元，执行完即退出
- 任何一个 Runner 返回错误 → 应用启动失败，直接退出
- 原则：Runner 之间不应该有依赖关系
- **禁止**在 Runner 中启动长期运行的后台 goroutine，也不能阻塞，如有长期运行需求请封装为 Server

### 6. Server 启动阶段
- **并行启动**所有 Registered Server，每个 Server 在独立的 goroutine 中运行
- 每个 Server 通过 `ReadySignal` 通知应用自己已经准备就绪
- 等待所有 Server 都发出 ready 信号 → 应用启动成功，开始对外提供服务
- 如果任一 Server 在启动过程中发生 panic 或返回错误，立即触发优雅关闭流程

## 优雅关闭流程

当收到退出信号或手动调用 `Shutdown()` 后，应用进入优雅关闭流程：

### 1. 取消 root context
- 调用 root context 的 cancel 函数，所有监听 `Done()` 的组件都会收到退出通知
- 整个过程是幂等的，可以多次调用

### 2. 并行停止所有 Server
- 同时调用所有 Server 的 `Stop()` 方法
- 等待所有 Server 的 goroutine 退出完成
- Go-Spring **不设置强制关闭超时**，给所有正在处理的请求足够时间完成，避免强制中断导致数据不一致

### 3. 资源销毁
- 关闭 IoC 容器，调用所有实现了 `Destroy` 接口的 Bean 的销毁方法
- 销毁日志组件，等待所有异步日志 flush 完成，确保日志不丢失

**自定义销毁**：
如果你的 Bean 需要在应用关闭时释放资源（比如关闭数据库连接、关闭文件句柄等），可以实现 `Destroy` 接口：

```go
type MyConnectionPool struct {
	// ...
}

func (p *MyConnectionPool) Destroy() error {
	// 关闭连接池，释放资源
	return p.pool.Close()
}
```

应用关闭时会自动调用你的 `Destroy()` 方法。

### 4. 进程退出
- 所有资源清理完成后，进程退出

## 使用方式

Go-Spring 提供了两种启动方式，满足不同场景需求。

### `Run()` - 标准启动方式

`Run()` 是最常用的启动方式，一个函数完成完整的启停流程，自动监听操作系统的退出信号（SIGINT/Ctrl+C、SIGTERM），收到信号后自动触发优雅关闭。

```go
package main

import (
	"github.com/go-spring/spring-core/gs"
	_ "github.com/go-spring/stdlib/httpsvr"
)

func init() {
	// 在这里注册你的 Bean
	gs.Provide(&MyService{})
}

func main() {
	gs.Run()
}
```

配合 `Configure` 回调使用：

```go
func main() {
	gs.Configure(func(app gs.App) {
		// 自定义配置
		app.Property("server.port", "8080")
		// 注册额外的 Bean
		app.Provide(&MyCustomService{})
	}).Run()
}
```

**信号处理**：
`Run()` 会自动监听退出信号：
- `os.Interrupt` (Ctrl+C)
- `syscall.SIGTERM` (Docker/Kubernetes 停止信号)

收到信号后自动触发优雅关闭，不需要你手动处理。

**适用场景**：从头开始的全新 Go-Spring 项目。

### `RunAsync()` - 异步启动方式

`RunAsync()` 以异步方式启动应用，返回一个 `stop` 函数供手动调用关闭。适用于将 Go-Spring 作为库集成到现有项目的场景。

```go
func main() {
	// 初始化并启动应用
	stop, err := gs.Configure(func(app gs.App) {
		app.Property("spring.http.server.enabled", "false")
		// 配置你的 Bean...
	}).RunAsync()
	
	if err != nil {
		log.Fatalf("start app failed: %v", err)
	}
	defer stop() // 确保程序退出前关闭应用
	
	// 现有项目的其他逻辑...
	// 可以继续运行主循环，处理其他业务
}
```

**适用场景**：
- 改造遗留项目，逐步迁移到 Go-Spring
- 将 Go-Spring 作为依赖库使用，不接管整个进程生命周期

### `Configure` - 配置回调

`Configure()` 方法允许你在应用启动前进行自定义配置，包括设置配置属性和注册 Bean。多个 `Configure` 调用会按顺序累积执行。

```go
func main() {
	// 第一个配置回调 - 设置基础配置
	app := gs.Configure(func(app gs.App) {
		app.Property("env", "production")
		app.Provide(&Database{})
	})
	
	// 根据条件追加配置
	if enableCache {
		app = app.Configure(func(app gs.App) {
			app.Provide(&RedisCache{})
		})
	}
	
	// 最后启动
	app.Run()
}
```

在 `Configure` 回调中，你可以：
- 使用 `app.Property(key, value)` 设置应用配置
- 使用 `app.Provide(objOrCtor)` 注册 Bean
- 使用 `app.Root(obj)` 标记根 Bean

### `Provide` vs `Root` 的区别

- `app.Provide(objOrCtor)` - 仅注册 Bean 定义，**只有被其他 Bean 依赖时才会创建**。适合被其他组件依赖的服务。
- `app.Root(obj)` - 注册 Bean 并标记为根 Bean，启动时一定会创建，并且会从这个 Bean 开始递归注入所有依赖。适合作为应用入口点。

什么时候用 `Root`：
- 当你的 Bean 不被其他 Bean 依赖，但又希望它在启动时被创建
- 当你的 Bean 需要作为入口点触发整个依赖链的创建

### 自定义 Banner

你可以在 `init()` 中调用 `gs.Banner()` 设置自定义的启动 Banner：

```go
func init() {
	gs.Banner(`
  ____  _      ____                 _
 / ___|| |__  / ___|_   _ ___  ___ | |_
 \___ \| '_ \| |  _ | | | / __|/ _ \| __|
  ___) | | | | |_| || |_| \__ |  __/| |_
 |____/|_| |_|\____|\__ | ___|\___| \__|
                    |___/
My Application v1.0.0
`)
}
```

如果不需要 Banner，可以设置为空字符串：
```go
gs.Banner("")
```

## 自定义组件

Go-Spring 提供了 `Runner` 和 `Server` 两种扩展接口，分别处理不同类型的业务组件。

### 自定义 Runner

`Runner` 接口用于在应用启动完成后、Server 启动前执行一些一次性初始化任务。例如：
- 数据库 schema 初始化
- 预热缓存
- 执行数据迁移
- 执行一次性的业务初始化

**Runner 接口定义**：
```go
type Runner interface {
	Run(ctx context.Context) error
}
```

**完整示例**：
```go
// DatabaseMigrator 数据迁移
type DatabaseMigrator struct {
	DB *sql.DB `autowire:""`
}

func (m *DatabaseMigrator) Run(ctx context.Context) error {
	// 创建表
	_, err := m.DB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate failed: %w", err)
	}
	fmt.Println("database migration complete")
	return nil
}

func init() {
	// 注册为 Bean，Go-Spring 会自动收集并执行
	gs.Provide(&DatabaseMigrator{})
}
```

**自动收集机制**：
Go-Spring 在容器刷新完成后，会**自动从 IoC 容器中收集**所有实现了 `Runner` 接口的 Bean，不需要你手动告诉 App。你只需要正常注册 Bean 就可以了。
```

然后启动应用即可，Go-Spring 会在启动阶段自动执行你的 Runner。

**重要约束**：
- `Run()` 方法必须快速返回，不能阻塞，也不能启动长期运行的后台 goroutine
- 如果需要长期运行，请实现 `Server` 接口
- 如果返回错误，应用会立即终止启动

### 自定义 Server

`Server` 接口用于封装需要长期运行的服务，例如 HTTP 服务、gRPC 服务、TCP/UDP 服务等。Server 需要支持启动和优雅关闭。

**Server 接口定义**：
```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

- `Run(ctx, sig)` - 启动服务，必须阻塞运行直到 context 取消，然后返回
- `Stop()` - 强制停止服务，释放资源
- `ReadySignal` - 用于在 Server 准备就绪后发送就绪信号

**完整示例**：自定义一个简单的 HTTP Server：

```go
// MyHttpServer 自定义 HTTP Server
type MyHttpServer struct {
	Addr string `value:"${server.addr:8080}"`
	srv *http.Server
}

// Run 启动 Server
func (s *MyHttpServer) Run(ctx context.Context, sig gs_app.ReadySignal) error {
	s.srv = &http.Server{
		Addr: s.Addr,
		Handler: s.setupRoutes(),
	}
	
	// 先完成 listen，然后发送 ready 信号
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	
	// 发送 ready 信号，告诉应用已经准备好接收请求
	sig.TriggerAndWait() <- struct{}{}
	
	// 等待请求处理或 context 取消
	go func() {
		<-ctx.Done()
		// context 取消，触发关闭
		_ = s.srv.Shutdown(context.Background())
	}()
	
	return s.srv.Serve(l)
}

// Stop 停止 Server
func (s *MyHttpServer) Stop() error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(context.Background())
}

// setupRoutes 配置路由
func (s *MyHttpServer) setupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	return mux
}

func init() {
	gs.Provide(&MyHttpServer{})
}
```

然后在 `main.go` 中正常启动即可：

```go
func main() {
	gs.Run()
}
```

**自动收集机制**：
和 Runner 一样，Go-Spring 会自动从 IoC 容器中收集所有实现了 `Server` 接口的 Bean，你只需要正常注册 Bean 就可以了。

**工作原理**：
1. 应用启动时，会为每个 Server 启动一个独立的 goroutine 调用 `Run()`
2. `Run()` 方法完成初始化后，通过 `ReadySignal` 通知应用就绪
3. 所有 Server 都就绪后，应用正式启动完成
4. 当需要关闭时，root context 被取消，`Run()` 方法应该检测到 ctx.Done() 并正常返回
5. `Stop()` 方法会被调用来确保资源被释放

**使用 ReadySignal**：
- `sig.TriggerAndWait()` 返回一个 channel，准备好接收请求后发送一个空结构体即可
- 应用会等待所有 Server 都发送就绪信号后才正式启动完成
- 这样可以保证在所有 Server 都真正就绪后再接受外部流量，避免启动过程中收到请求导致拒绝

## Context 注入

整个应用的生命周期由 root context 管理，当应用关闭时 root context 会被取消。你的 Bean 可以通过两种方式获取这个 root context：

### 方式一：使用 ContextProvider（推荐）

应用提供了 `gs_app.ContextProvider` 可以直接注入，随时获取当前的 root context：

```go
type MyService struct {
	CtxProvider *gs_app.ContextProvider `autowire:""`
}

func (s *MyService) DoSomething() {
	ctx := s.CtxProvider.Context
	// 使用 ctx 执行操作，当应用关闭时 ctx 会被取消
}
```

### 方式二：实现 ContextAware 接口

实现 `ContextAware` 接口可以在 Bean 创建时被注入 root context：

```go
type MyService struct {
	ctx context.Context
}

func (s *MyService) SetContext(ctx context.Context) {
	s.ctx = ctx
}
```

**设计目的**：禁止业务代码使用 `context.Background()` 或 `context.TODO()`，所有操作都在 root ctx 生命周期之下，确保应用关闭时所有操作都能正确取消。

## 属性刷新支持

你可以注入 `PropertiesRefresher` 来动态刷新配置：

```go
type MyService struct {
	Refresher *gs_app.PropertiesRefresher `autowire:""`
}

func (s *MyService) refreshConfig() error {
	return s.Refresher.RefreshProperties()
}
```

调用 `RefreshProperties()` 后：
1. 重新从所有配置源加载配置
2. 合并配置，解决优先级冲突
3. 将变化自动更新到所有 `gs.Dync[T]` 动态字段
