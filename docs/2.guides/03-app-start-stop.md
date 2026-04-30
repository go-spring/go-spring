# 应用启停与运行机制

Go-Spring 的启动流程由一组清晰的生命周期阶段组成：加载配置、初始化日志、启动 IoC 容器、执行初始化任务、启动长期服务，并在退出时按相反方向释放资源。理解这些阶段后，可以更准确地选择启动方式，也能在启动失败或关闭卡住时快速定位问题。

---

## 一、启动方式

Go-Spring 提供三种常用启动方式：

- `Run()`：标准阻塞启动，适合独立运行的服务，也是新项目的推荐入口。
- `RunAsync()`：异步启动，适合集成到已有程序，由调用方自行控制退出时机。
- `RunTest()`：测试启动，用于需要 IoC 容器参与的单元测试或集成测试。

### Run() - 标准启动

`Run()` 是最常用的启动方式。调用后，它会完成应用启动，并阻塞当前 goroutine，直到收到退出信号。

```go
package main

import (
    "github.com/go-spring/spring-core/gs"
)

func main() {
    // Run 会按约定完成以下工作：
    //   1. 加载 application.yaml / application.properties 等配置文件
    //   2. 初始化日志系统
    //   3. 启动 IoC 容器，完成 Bean 创建和依赖注入
    //   4. 启动内置 HTTP Server（默认端口 9090）
    //   5. 监听 Ctrl+C 和 SIGTERM，触发优雅关闭
    gs.Run()
}
```

`Run()` 的完整流程如下：

```
打印 Banner
  -> 加载配置
  -> 初始化日志系统
  -> 启动 IoC 容器
  -> 执行所有 Runner
  -> 启动所有 Server
  -> 监听退出信号并等待关闭
```

这种方式的优势是启动代码少、默认行为完整，并且内置了信号监听和优雅关闭逻辑。对于绝大多数服务端应用，应优先使用 `Run()`。

---

### RunAsync() - 异步启动

当 Go-Spring 需要集成到已有系统中时，阻塞式的 `Run()` 可能并不合适。此时可以使用 `RunAsync()` 非阻塞启动应用。

`RunAsync()` 启动成功后会返回一个 `stop` 函数，调用该函数即可触发应用关闭。

```go
package main

import (
    "log"

    "github.com/go-spring/spring-core/gs"
)

func main() {
    // 异步启动应用，不阻塞当前 goroutine。
    stop, err := gs.RunAsync()
    if err != nil {
        log.Fatal("启动失败:", err)
    }
    defer stop()

    // 这里可以继续接入已有系统逻辑，例如：
    //   - 启动其他 HTTP 服务
    //   - 接入已有进程的生命周期管理
    //   - 运行定时任务或后台调度逻辑
    select {}
}
```

需要注意的是，`RunAsync()` 不会自动监听操作系统信号。程序退出前必须调用 `stop()`，否则 Server 和容器资源无法按 Go-Spring 生命周期正常释放。

---

### RunTest() - 测试启动

`RunTest()` 用于在测试中启动一个轻量级 Go-Spring 应用。它会创建 IoC 容器、完成依赖注入，并在测试函数返回后自动关闭应用。

对于可以脱离容器验证的逻辑，优先编写普通单元测试；当测试需要验证配置绑定、自动装配、Bean 协作等容器能力时，再使用 `RunTest()`。

```go
func TestMyService(t *testing.T) {
    gs.RunTest(t, func(ts *struct {
        MyService *MyService `autowire:""`
        DB        *Database  `autowire:""`
    }) {
        result := ts.MyService.DoSomething()
        assert.NotNil(t, result)
    })
}
```

`RunTest()` 的执行过程：

1. 启动轻量级应用，执行配置加载、日志初始化和 IoC 容器启动。
2. 将测试函数入参中的注入接收结构体注册为 Root Bean。
3. 为带有 `autowire` 标签的字段注入对应 Bean；依赖缺失时测试失败。
4. 执行测试函数，此时所有依赖均已准备完成。
5. 测试函数返回后自动关闭应用，并调用相关 Bean 的 Destroy 方法。

关于 IoC 测试的更多用法，请参考 [07-testing.md](07-testing.md)。

---

## 二、启动配置

除了默认启动行为，Go-Spring 也提供了若干启动定制能力，例如关闭内置 HTTP Server、设置默认配置、注册测试 Bean、指定 Root Bean 或自定义 Banner。

### Web() - 开关 HTTP 服务

`Web()` 用于控制是否启动内置 HTTP Server。

```go
// 启用 Web 服务（默认已开启，可省略）
gs.Web(true).Run()

// 关闭 Web 服务，仅运行 Runner、Server 或其他后台组件
gs.Web(false).Run()
```

`Web(false)` 等价于设置以下配置项：

```go
app.Property("spring.http.server.enabled", "false")
```

使用 `Web(false)` 可以避免在代码中记忆具体配置键名。

---

### 设置配置项

可以通过 `Property()` 为应用设置默认配置：

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":8080")
    app.Property("env", "production")
})
```

这类配置优先级最低，通常用于提供兜底默认值。实际环境配置仍建议放在配置文件、环境变量或启动参数中。

---

### 注册 Bean

`Provide()` 可以在启动配置阶段向容器注册 Bean：

```go
gs.Configure(func(app gs.App) {
    app.Provide(&MyService{})
})
```

这种写法常用于测试或少量启动期定制。普通业务 Bean 更推荐使用包级 `gs.Provide()` 注册，使模块自身完成声明。

---

### 标记 Root Bean

`Root()` 可以将一个 Bean 标记为 Root Bean：

```go
gs.Configure(func(app gs.App) {
    app.Root(&AppEntry{})
})
```

Go-Spring 的 IoC 容器会从 Root Bean 开始递归创建依赖图。与旧项目集成时，如果关闭了内置 HTTP Server，并且没有提供任何 `Runner` 或 `Server`，可能没有 Bean 会被主动创建。此时可以通过 `Root()` 指定入口 Bean，确保相关依赖被初始化。

---

### Banner() - 自定义启动画面

`Banner()` 用于自定义启动时打印的 Banner。

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

如果不需要 Banner，可以设置为空字符串：

```go
gs.Banner("")
```

---

## 三、启动流程

Go-Spring 的完整启动流程如下：

```
调用 Run() / RunAsync()
      |
      v
  打印 Banner
      |
      v
  加载配置
  命令行 > 环境变量 > 配置文件 > Configure
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
  任意 Runner 返回 error 都会终止启动
      |
      v
  并行启动所有 Server
  等待所有 Server 发出 Ready 信号
      |
      v
  启动完成
  Run() 监听信号 / RunAsync() 交还控制权
```

下面依次说明各阶段的关键行为。

---

### 配置加载

Go-Spring 支持多层配置源。按优先级从高到低依次为：

1. 命令行参数
2. 环境变量
3. 配置文件
4. `Configure()` 中的代码配置

同名配置项由高优先级覆盖低优先级；不同配置项会自动合并。

#### 命令行参数

启动时可以通过 `-Dkey=value` 临时覆盖配置：

```bash
go run main.go -Dspring.http.server.addr=:8080
go run main.go -Denv=production -Dlogging.level=error
```

#### 环境变量

环境变量使用 `GS_KEY=value` 格式，适合容器化部署：

```bash
export GS_SPRING_HTTP_SERVER_ADDR=:8080
export GS_ENV=production

docker run -e GS_ENV=production my-app
```

环境变量名会移除 `GS_` 前缀，并将下划线 `_` 转为点号 `.`。因此 `GS_SPRING_HTTP_SERVER_ADDR` 对应配置键 `spring.http.server.addr`。

#### 配置文件

项目根目录下的 `application.yaml` 或 `application.properties` 是最常用的配置方式：

```yaml
spring:
  http:
    server:
      addr: :9090
env: development
logging:
  level: info
```

配置文件还支持 profile 覆盖：

- `application.yaml`：基础配置，所有环境都会加载。
- `application-{profile}.yaml`：特定环境配置，覆盖基础配置中的同名 key。

也可以通过 `spring.config.import` 导入其他配置文件：

```yaml
spring:
  config:
    import:
      - database.yaml
      - redis.yaml
env: development
```

导入文件与当前文件处于同一优先级。同名 key 以后导入的配置为准。

#### Configure() 代码配置

`app.Property()` 通常用于设置默认值：

```go
gs.Configure(func(app gs.App) {
    app.Property("spring.http.server.addr", ":9090")
    app.Property("env", "development")
})
```

优先级示例：

| 配置源 | 值 | 最终生效 |
|---|---|---|
| 命令行参数 `-Denv=production` | `production` | 是 |
| 环境变量 `GS_ENV=staging` | `staging` | 否 |
| 配置文件 `env: test` | `test` | 否 |
| `Configure()` 中的 `app.Property("env", "dev")` | `dev` | 否 |

最终 `env` 的值为 `production`。

---

### 日志初始化

日志系统在 IoC 容器之前初始化。原因是容器启动过程本身就需要输出日志，因此日志不能依赖 Bean 创建完成后再就绪。

常见日志级别从低到高为：

- `trace`：最详细的跟踪信息。
- `debug`：调试信息。
- `info`：一般信息，通常作为默认级别。
- `warn`：警告信息。
- `error`：错误信息。
- `fatal`：致命错误，输出后程序退出。

日志配置示例：

```yaml
logging:
  level: info
  format: "%time [%level] %msg"
  time-format: "2006-01-02 15:04:05.000"

  console:
    enabled: true
    color: true

  file:
    enabled: false
    path: ./logs/app.log
    max-size: 100MB
    max-backups: 10
    max-age: 30d
    compress: true
```

也可以为不同包设置不同级别：

```yaml
logging:
  level:
    root: info
    github.com/go-spring: debug
    myapp.dao: debug
    myapp.service: info
```

如果没有配置 `logging`，框架会使用默认配置：`info` 级别、仅控制台输出、启用颜色，并使用内置日志格式。

---

### IoC 容器初始化

容器启动时会先注册内置 Bean：

- `ContextProvider`：用于获取应用 root context。
- `PropertiesRefresher`：用于触发动态配置刷新。

随后，容器会从 Root Bean 开始递归遍历依赖图，按需创建 Bean 并完成依赖注入。注入完成后，容器会收集所有实现了 `Runner` 和 `Server` 接口的 Bean，供后续阶段执行。

如果应用中没有任何 `gs.Dync[T]` 动态配置字段，容器会清理配置缓存以节省内存。

---

### 执行 Runner

`Runner` 用于执行一次性初始化任务，例如数据库迁移、缓存预热或基础数据初始化。

所有 `Runner` 会按顺序同步执行。任意一个 `Runner` 返回 error，应用启动都会失败。顺序执行的设计可以避免初始化任务之间出现竞态，也便于通过注册顺序表达前后依赖。

`Runner` 应快速返回，不适合承载长期运行的后台任务。需要持续运行的组件应实现为 `Server`。

---

### 启动 Server

`Server` 用于承载长期运行的服务，例如 HTTP、gRPC、MQ 消费者或任务调度器。所有 `Server` 会在独立 goroutine 中并行启动。

启动 `Server` 时最重要的是 Ready 信号机制。每个 `Server` 都应在完成监听绑定、真正具备服务能力后，再触发 Ready 信号。框架会等待所有 `Server` 都 Ready 后，才认为应用启动完成。

这个机制可以避免“应用已经宣布启动，但端口还没有监听成功”的时间差。对于 Kubernetes 等运行环境，这一点尤其重要：只有所有服务都真正准备好后，才应该对外声明就绪。

如果任意 `Server` 在运行过程中 panic 或返回 error，框架会触发优雅关闭，避免其他服务继续空转。

---

## 四、运行与关闭

### 信号处理

只有 `Run()` 会自动监听操作系统信号，`RunAsync()` 不会。

`Run()` 监听两个常见信号：

- `SIGINT`：通常由 Ctrl+C 触发。
- `SIGTERM`：Docker、Kubernetes 等环境停止容器时发送。

收到信号后，Go-Spring 会记录日志并调用 `ShutDown()` 进入优雅关闭流程。

---

### 触发关闭的方式

应用关闭可能由以下三种方式触发：

1. `Run()` 模式下收到操作系统信号。
2. `RunAsync()` 模式下调用 `stop()` 函数。
3. 某个 `Server` 运行时 panic 或返回 error。

### 关闭流程

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

Go-Spring 不设置全局强制关闭超时。框架会等待所有资源完成清理后再退出。

这是一个有意的设计取舍：不同业务对关闭等待时间的要求差异很大，框架无法给出适用于所有场景的默认值。需要超时控制时，应在业务自己的 `Stop()` 方法中实现。

---

## 五、自定义启动项

Go-Spring 提供两类自定义启动项，对应两种生命周期：

- `Runner`：一次性初始化任务，启动阶段顺序执行，执行完成即结束。
- `Server`：长期运行的服务，启动后持续运行，直到应用关闭。

### Runner：一次性任务

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
    gs.Provide(&DBMigrator{})
}
```

注册为 Bean 后，框架会自动发现并执行该 `Runner`。

---

### Server：长期服务

`Server` 接口如下：

```go
type Server interface {
    // Run 必须阻塞运行，直到 ctx 被取消或服务退出。
    Run(ctx context.Context, ready ReadySignal) error

    // Stop 用于停止服务并释放资源。
    Stop() error
}
```

一个自定义 HTTP Server 的简化示例如下：

```go
type MyServer struct {
    Addr string `value:"${server.addr:=:8080}"`
    srv  *http.Server
}

func (s *MyServer) Run(ctx context.Context, ready gs.ReadySignal) error {
    s.srv = &http.Server{Addr: s.Addr}

    // 1. 先完成监听绑定。
    l, err := net.Listen("tcp", s.Addr)
    if err != nil {
        return err
    }

    // 2. 再触发 Ready，并等待所有 Server 都 Ready 后统一放行。
    <-ready.TriggerAndWait()

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

Ready 信号必须在 `Listen` 成功之后触发。如果在监听端口之前就触发 Ready，应用可能提前声明就绪，但实际端口尚未打开，外部请求会失败。

---

## 六、Context 生命周期控制

整个应用生命周期都绑定在一个 root context 上。应用关闭时，该 context 会被 cancel。

业务代码应优先从 root context 派生上下文，而不是直接使用 `context.Background()`：

```go
type MyService struct {
    CtxProvider *gs.ContextProvider `autowire:""`
}

func (s *MyService) DoWork() {
    ctx := s.CtxProvider.Context()

    select {
    case <-ctx.Done():
        // 应用正在关闭，停止接收新任务。
        return
    default:
        // 正常处理。
    }
}
```

这样应用关闭时，业务逻辑可以及时感知退出信号，停止接收新任务，并给正在执行的操作留出收尾机会。

---

## 七、动态配置刷新

可以通过 `PropertiesRefresher` 在运行时触发配置刷新：

```go
type ConfigManager struct {
    Refresher *gs.PropertiesRefresher `autowire:""`
}

func (m *ConfigManager) ReloadConfig() error {
    return m.Refresher.RefreshProperties()
}
```

刷新后，使用 `gs.Dync[T]` 声明的动态配置字段会自动更新：

```go
type MyService struct {
    Timeout gs.Dync[time.Duration] `value:"${service.timeout:=30s}"`
}

func (s *MyService) Handle() {
    timeout := s.Timeout.Get()
    _ = timeout
}
```

`gs.Dync[T]` 是并发安全的，适合在运行过程中多次读取最新配置值。更多配置绑定与动态刷新细节，请参考 [01-configuration.md](01-configuration.md)。
