# 应用启停与运行机制

Go-Spring 应用启动时会依次完成配置加载、日志初始化、IoC 容器启动、执行 Runner 和启动 Server。
应用关闭时，会取消根上下文、停止服务并释放容器资源。

理解这些阶段，有助于我们选择合适的启动方式，也便于我们在启动失败、服务未就绪或关闭阻塞时定位问题。

## 启动方式

Go-Spring 提供了两种常用的启动方式：

- `gs.Run()`：标准阻塞启动，适用于独立运行的服务，也是新项目的推荐入口。
- `gs.RunAsync()`：异步启动，适用于集成到已有程序，由调用方控制退出时机。

### 阻塞启动

`gs.Run()` 是最常用的启动方式。
调用后，框架会完成应用启动，并阻塞当前 goroutine，直到收到退出信号。

```go
package main

import (
    "github.com/go-spring/spring-core/gs"
)

func main() {
    // gs.Run() 完成以下工作：
    //   1. 加载 ./conf/app.*、./conf/app-{profile}.* 等配置文件
    //   2. 初始化日志系统
    //   3. 启动 IoC 容器，完成 Bean 创建和依赖注入
    //   4. 启动内置 HTTP Server（默认端口 9090）
    //   5. 监听 Ctrl+C 和 SIGTERM，触发优雅关闭
    gs.Run()
}
```

`gs.Run()` 的完整流程如下：

```
打印 Banner
  -> 加载配置
  -> 初始化日志系统
  -> 启动 IoC 容器
  -> 执行所有 Runner
  -> 启动所有 Server
  -> 监听退出信号并等待关闭
```

这种方式启动代码少，默认行为完整，并且包含信号监听和优雅关闭。
对于大多数服务端应用，建议优先使用 `gs.Run()`。

### 非阻塞启动

当 Go-Spring 需要集成到已有系统中时，阻塞式的 `gs.Run()` 可能不适用。
此时可以使用 `gs.RunAsync()` 以非阻塞方式启动应用。

`gs.RunAsync()` 启动成功后返回 `stop` 函数，调用该函数可以触发应用关闭。

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/go-spring/spring-core/gs"
)

func main() {
    // 异步启动应用，不阻塞当前 goroutine。
    stop, err := gs.RunAsync()
    if err != nil {
        log.Fatal("启动失败:", err)
    }
    defer stop()

    // 这里可以继续接入已有系统逻辑，例如启动其他服务或等待外部生命周期事件。
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
}
```

`gs.RunAsync()` 不会自动监听操作系统信号。
程序退出前应调用 `stop()`，确保 Server 和容器资源按照 Go-Spring 生命周期正常释放。

## 启动配置

Go-Spring 提供了若干启动定制的能力，
例如关闭内置 HTTP Server、设置默认配置、注册 Root Bean 或者自定义 Banner。
这些配置需要在应用启动前完成。

### 禁用内置 HTTP Server

`gs.Web()` 用于控制内置 HTTP Server 是否启动。

```go
// 关闭内置 HTTP Server
gs.Web(false).Run()
```

实际上 `gs.Web(false)` 等价于设置以下配置项：

```go
app.Property("spring.http.server.enabled", "false")
```

使用 `gs.Web(false)` 可以避免直接书写具体配置键名。

### 设置默认配置

我们可以通过 `gs.Configure()` 提供的 `app.Property()` 方法设置应用的默认配置：

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":8080")
    app.Property("env", "production")
})
```

这类配置的优先级最低，仅高于 `value` tag 中的默认值。
实际环境配置仍然建议放在配置文件、环境变量或启动参数中。

### 注册容器 Bean

我们可以通过 `gs.Configure()` 提供的 `app.Provide()` 方法向容器注册 Bean：

```go
gs.Configure(func(app gs.App) {
    app.Provide(&MyService{})
})
```

这种写法通常用于测试。普通业务 Bean 建议通过全局 `gs.Provide()` 注册，由模块自身完成声明。

### 注册 Root Bean

我们可以通过 `gs.Configure()` 提供的 `app.Root()` 方法将一个 Bean 标记为 Root Bean：

```go
gs.Configure(func(app gs.App) {
    app.Root(&AppEntry{})
})
```

Go-Spring 的 IoC 容器会从 Root Bean 开始递归创建依赖图。
在与旧项目集成时，如果关闭了内置 HTTP Server，且没有提供其他 `gs.Runner` 或者 `gs.Server`，
那么可能不会有 Bean 被主动创建。
此时我们可以通过 `app.Root()` 方法指定入口 Bean，确保相关依赖完成初始化。

### 自定义 Banner

`gs.Banner()` 用于自定义启动时打印的 Banner。

```go
func init() {
    gs.Banner(`
   _____ __  __  _____
  / ____|  \/  |/ ____|
 | |  __| \  / | |  __
 | | |_ | |\/| | | |_ |
 | |__| | |  | | |__| |
  \_____|_|  |_|\_____|
  My Application v1.0
`)
}
```

如果不需要 Banner，可以将其设置为空字符串：

```go
gs.Banner("")
```

## 启动流程

Go-Spring 的完整启动流程如下：

```
调用 Run() / RunAsync()
      |
      v
  打印 Banner
      |
      v
  加载配置
  - 命令行 > 环境变量 > Profile 配置 > 基础配置 > Property
      |
      v
  初始化日志系统
      |
      v
  启动 IoC 容器
  - 从 Root Bean 开始递归创建和注入依赖
  - 收集所有 Runner 和 Server
      |
      v
  顺序执行所有 Runner
  - 任意 Runner 返回 error 都会终止启动
      |
      v
  并行启动所有 Server
  - 等待所有 Server 发出 Ready 信号
      |
      v
  启动完成
  - Run() 监听信号 / RunAsync() 交还控制权
```

### 配置加载

Go-Spring 支持多层配置源，优先级从高到低依次为：

1. 命令行参数
2. 环境变量
3. Profile 配置文件
4. 基础配置文件
5. `Property()` 设置的配置项

同名配置项会高优先级覆盖低优先级，不同配置项会自动合并。

#### 命令行参数

我们可以在启动时通过 `-Dkey=value` 覆盖配置项。
当没有显式赋值时，该配置项会被视为 `true`。

```bash
go run main.go -Dspring.http.server.addr=:8080
go run main.go -Denv=production -Dlogging.level=error
go run main.go -Ddebug
```

#### 环境变量

我们也可以通过环境变量覆盖配置项，这种方式常用于容器化部署。
环境变量使用 `GS_KEY=value` 格式。

```bash
export GS_SPRING_HTTP_SERVER_ADDR=:8080
export GS_ENV=production

docker run -e GS_ENV=production my-app
```

带 `GS_` 前缀的环境变量会先移除前缀，再将下划线 `_` 转为点号 `.`，并转换为小写。
因此 `GS_SPRING_HTTP_SERVER_ADDR` 就对应了配置键 `spring.http.server.addr`。

不带 `GS_` 前缀的环境变量会保留原始键名，通常不建议用于 Go-Spring 应用配置。

#### 配置文件

Go-Spring 的默认配置目录为 `./conf`，我们可以通过 `spring.app.config.dir` 配置项进行修改。
Go-Spring 会从该目录加载名字为 `app.*` 和 `app-{profile}.*` 形式的配置文件。
Go-Spring 默认支持 `.properties`、`.yaml`、`.toml`、`.json` 格式的配置文件。

由于框架需要在加载配置文件前确定配置目录，因此 `spring.app.config.dir` 通常应通过命令行参数、
环境变量或 `gs.Configure()` 设置。

```yaml
# ./conf/app.yaml
spring:
  http:
    server:
      addr: :9090
env: development
logging:
  level: info
```

配置文件支持 Profile 覆盖。当我们通过 `spring.profiles.active` 指定激活的 Profile 后，
框架会加载对应的 `app-{profile}.*` 配置文件。

```bash
go run main.go -Dspring.profiles.active=prod
```

我们可以在 `spring.profiles.active` 中指定多个 Profile，Profile 之间用逗号分隔，
例如 `-Dspring.profiles.active=dev,local`。
框架会按照声明顺序加载 Profile 对应的配置文件，后加载的 Profile 配置可以覆盖先加载的同名配置项。

配置文件支持使用 `spring.app.imports` 导入其他配置文件：

```yaml
spring:
  app:
    imports:
      - ./conf/database.yaml
      - ./conf/redis.yaml
env: development
```

通过 `spring.app.imports` 导入的配置文件与声明它的配置文件处于同一优先级。
当导入的配置文件中包含同名 key 时，以后加载的配置为准。
此外，导入目前仅支持一层，即被导入的文件不能再声明新的导入，即使声明了也不会生效。

除本地文件外，我们也可以通过 `Provider` 导入其他形式的配置。
详情请参考 [配置来源](01-configuration.md#支持的配置来源) 。

#### 代码配置

我们还可以通过 `gs.Configure` 提供的 `app.Property()` 方法来设置配置项。

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":9090")
    app.Property("env", "development")
})
```

更多配置相关内容请参考 [配置管理](01-configuration.md) 。

### 初始化日志

日志系统在 IoC 容器之前初始化。因为容器启动过程本身也需要输出日志，
所以日志不能依赖 Bean 创建完成后再就绪。

Go-Spring 的日志系统是单独设计的，
详细内容参考 [日志](04-logging.md)。

日志系统采用 k-v 格式进行配置，因此可以直接从配置体系中读取配置，
而无需单独的配置文件。

### 初始化 IoC 容器

容器启动前，Go-Spring 会先注册一些内置 Bean：

- `ContextProvider`：用于获取应用的 root context。
- `PropertiesRefresher`：用于触发动态配置刷新。

随后，容器从 Root Bean 开始递归遍历依赖图，按需创建 Bean 并完成依赖注入。
注入完成后，容器会收集所有实现了 `Runner` 和 `Server` 接口的 Bean，供后续阶段执行。

如果应用中没有启用任何 `gs.Dync[T]` 动态配置字段，容器会清理配置缓存以节省内存。

### 执行 Runner

`Runner` 用于执行一次性初始化任务，例如数据库迁移、缓存预热或者基础数据初始化。
它在容器初始化之后、Server 启动之前执行，因此可以安全地使用已经注入完成的 Bean。

所有 `Runner` 会按照收集顺序同步执行。任意 `Runner` 返回 error 都会导致应用启动失败。
顺序执行的设计可以避免初始化任务之间出现竞态，也便于通过收集顺序表达前后依赖。

`Runner` 执行应当快速返回，不适合承载长期运行的后台任务。
如果需要持续运行应当实现为 `Server`，否则应用会一直停留在启动阶段。

### 启动 Server

`Server` 用于承载长期运行的服务，例如 HTTP 服务、gRPC 服务、MQ 消费者或者任务调度器。
所有 `Server` 都在独立的 goroutine 中并行启动。

启动 `Server` 时需要关注 Ready 信号机制。
每个 `Server` 都应当在完成监听绑定或者具备服务能力后，再触发 Ready 信号。
框架会等待所有 `Server` 都发出 Ready 信号后，再继续启动完成流程。

该机制可以避免健康检查接口已经可用、流量已经进入，但其他 Server 随后启动失败，
最终导致请求处理失败的情况。

如果任意 `Server` 在运行过程中 panic 或者返回 error，框架会触发优雅关闭流程。

## 监听退出信号

`gs.Run()` 在启动成功后监听两个常见信号：

- `SIGINT`：通常由 Ctrl+C 触发。
- `SIGTERM`：Docker、Kubernetes 等环境停止容器时发送。

收到信号后，Go-Spring 会记录日志，并调用 `ShutDown()` 进入优雅关闭流程。

## 优雅关闭

```
                触发 ShutDown()
                     |
                     v
              取消 root context
      所有监听 ctx.Done() 的逻辑都会收到通知
                     |
    -----------------+-----------------
    |                |                |
    v                v                v
Server 1 Stop()  Server 2 Stop()  ...
    |                |                |
    -----------------+-----------------
                     |
                     v
          等待所有 Server goroutine 退出
                     |
                     v
              关闭 IoC 容器
          调用相关 Bean 的 Destroy 方法
                     |
                     v
            flush 日志并结束进程
```

需要注意的是，Go-Spring 没有设置全局强制关闭超时。框架会等待所有资源完成清理后再退出。

这是一个明确的设计取舍：
不同业务对关闭等待时间的要求差异较大，框架无法给出适用于所有场景的默认值。

## 实现 Runner

`Runner` 适合一次性初始化任务，在启动阶段顺序执行，执行完成即结束。

`Runner` 接口如下：

```go
type Runner interface {
	Run(ctx context.Context) error
}
```

示例：启动时自动建表。

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

上面的代码将 `DBMigrator` 实例注册为 `Runner`。
在启动阶段，框架会自动发现并执行该 `Runner`。

## 实现 Server

`Server` 适合长期运行的服务，在 Runner 执行完成后持续运行，直到应用关闭。

`Server` 接口如下：

```go
type Server interface {
    // Run 必须阻塞运行，直到 ctx 被取消或服务退出。
	Run(ctx context.Context, sig ReadySignal) error

    // Stop 用于停止服务并释放资源。
	Stop() error
}
```

下面是一个自定义 HTTP Server 的简化示例：

```go
type MyServer struct {
	Addr string `value:"${server.addr:=:8080}"`
	srv  *http.Server
}

func (s *MyServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	s.srv = &http.Server{Addr: s.Addr}

	// 1. 先完成监听绑定。
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

	// 2. 再触发 Ready，并等待所有 Server 都 Ready 后统一放行。
	<-sig.TriggerAndWait()

	// 3. 开始提供服务。Serve 会阻塞，直到服务退出。
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

需要注意 Ready 信号应在 `Listen` 成功之后触发。
如果在监听端口之前触发 Ready，应用可能提前声明就绪，但实际端口尚未打开，外部请求会失败。

## 注入根 Context

我们强烈建议将整个应用的生命周期绑定在一个 root context 上。
业务代码应当优先从 root context 派生上下文，而不是直接使用 `context.Background()`。
当应用关闭时，该 context 会被取消，并触发所有监听 `ctx.Done()` 的逻辑。

`ContextProvider` 对象可以通过结构体字段或构造函数注入。

```go
type MyService struct {
    CtxProvider *gs.ContextProvider `autowire:""`
}

func (s *MyService) DoWork() {
    ctx := s.CtxProvider.Context

    select {
    case <-ctx.Done():
        // 应用正在关闭，停止接收新任务。
        return
    default:
        // 正常处理。
    }
}
```

## 刷新动态配置

我们可以在结构体字段上使用 `gs.Dync[T]` 声明动态配置字段。

```go
type MyService struct {
	Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
	timeout := s.Timeout.Value()
	_ = timeout
}
```

随后可以通过 `PropertiesRefresher` 对象在运行时触发配置刷新。
动态刷新仅适用于使用 `gs.Dync[T]` 声明的配置字段。

```go
type ConfigManager struct {
	Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
	os.Setenv("GS_SERVICE_TIMEOUT", "10s")
	return m.Refresher.RefreshProperties()
}
```

`gs.Dync[T]` 是并发安全的，适合在运行过程中读取最新配置值。
配置绑定与动态刷新细节请参考 [配置管理](01-configuration.md)。
