# Go-Spring 实战第 27 课 —— HTTP Server：把 net/http 纳入启动、就绪和优雅关闭

日志系统解决的是应用运行时怎样被观察。再往前走一步，一个服务端应用总要把能力暴露出去，最常见的入口就是 HTTP。

很多 Go 项目一开始会直接使用标准库 `net/http`。这没有问题，`http.Handler`、`http.ServeMux` 和 `http.Server` 已经足够稳定。真正容易变复杂的地方不在路由本身，而在服务入口和应用生命周期的关系：配置什么时候加载，端口什么时候监听，应用什么时候算 Ready，退出时怎样停止接收新请求，又怎样等待正在处理的请求结束。

Go-Spring 内置 HTTP Server 要解决的正是这个问题。它不替代 `net/http` 的路由模型，也不要求业务迁移到某个特定 Web 框架；Go-Spring 只把最终的 `http.Handler` 接入应用的配置、启动、Ready 信号和优雅关闭流程。

## 默认 ServeMux

先看最小接入方式。这个例子要证明的是：如果应用已经使用标准库默认路由，Go-Spring 不要求额外创建路由器，只要把进程入口交给 `gs.Run()`。

```go
func init() {
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Go-Spring!"))
	})
}

func main() {
	gs.Run()
}
```

这里的路由仍然注册在 `http.DefaultServeMux` 上。Go-Spring 在内置 HTTP Server 启用时，会在没有自定义 `*gs.HttpServeMux` 的情况下提供一个默认 `HttpServeMux`，它包装的就是 `http.DefaultServeMux`。

所以，这段代码的语义不是“Go-Spring 提供了新的路由 DSL”，而是“标准库默认路由可以直接成为 Go-Spring 的 HTTP 入口”。应用可以先保持最小改造，把监听、Ready 和关闭交给 Go-Spring 管理。

## spring.http.server

HTTP 入口进入生命周期之后，第一类差异通常来自部署环境：本地、测试和线上可能使用不同端口，也可能需要不同超时。Go-Spring 把内置 HTTP Server 的配置收敛在 `spring.http.server` 前缀下。

| 配置项 | 说明 | 默认值 |
| --- | --- | --- |
| `spring.http.server.addr` | 监听地址 | `:9090` |
| `spring.http.server.readTimeout` | 读取请求超时 | `5s` |
| `spring.http.server.headerTimeout` | 请求头读取超时 | `1s` |
| `spring.http.server.writeTimeout` | 写响应超时 | `5s` |
| `spring.http.server.idleTimeout` | 空闲连接超时 | `60s` |
| `spring.http.server.enabled` | 是否启用内置 HTTP Server | `true` |

下面的配置只证明一件事：监听地址是配置输入，不应该写死在启动代码里。

```properties
spring.http.server.addr=:8080
```

`spring.http.server.addr` 会绑定到 `gs.SimpleHttpServerConfig.Address`。同一组超时配置也会绑定到 `time.Duration` 字段，所以 `5s`、`60s` 这类写法仍然走 Go-Spring 的配置绑定和类型转换规则。

如果当前进程已经被别的宿主托管，或者应用只想使用配置、IoC、Runner 等能力，可以关闭内置 HTTP Server。

```properties
spring.http.server.enabled=false
```

代码里也可以显式表达同样的启动模式。

```go
func main() {
	gs.Web(false).Run()
}
```

这里的边界要分清：关闭的是 Go-Spring 内置 HTTP Server，不是关闭 Go-Spring 应用本身。配置加载、Bean 装配、Runner 执行和容器关闭仍然可以继续使用。

## HttpServeMux

默认 `http.DefaultServeMux` 适合简单应用。随着路由变多，路由组装往往也会依赖配置、控制器或中间件。Go-Spring 用 `gs.HttpServeMux` 表达“容器里最终提供给 HTTP Server 的 handler”。

```go
type HttpServeMux struct {
	http.Handler
}
```

下面的例子要证明的是：路由器创建函数本身可以参与依赖注入。控制器先作为 Bean 注册，随后由创建 `*gs.HttpServeMux` 的函数接收并组装路由。

```go
type UserController struct{}

func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.FormValue("user"))
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func init() {
	gs.Provide(new(UserController))

	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)
		return &gs.HttpServeMux{Handler: logging(mux)}
	})
}
```

这段代码的关键不在 `http.NewServeMux()`，而在 `func(c *UserController) *gs.HttpServeMux` 这个构造函数。它让路由组装进入容器依赖图：控制器可以由容器创建，中间件可以从配置或 Bean 中获得，最后只有一个 `http.Handler` 被交给 HTTP Server。

如果容器中存在自定义 `*gs.HttpServeMux`，内置 HTTP Server 会使用它；如果不存在，Go-Spring 才会回退到默认 `http.DefaultServeMux`。这让应用可以从默认路由平滑过渡到容器管理的路由组装。

## 第三方路由器

Go-Spring 对第三方 Web 框架的接入边界也很明确：只要最终能给出 `http.Handler`，就可以放进 `gs.HttpServeMux`。下面这些例子证明的是同一件事，路由匹配和中间件仍由第三方框架负责，Go-Spring 只接管最终入口。

```go
gs.Provide(func() *gs.HttpServeMux {
	g := gin.Default()
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})
	return &gs.HttpServeMux{Handler: g}
})
```

```go
gs.Provide(func() *gs.HttpServeMux {
	m := mux.NewRouter()
	m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: m}
})
```

```go
gs.Provide(func() *gs.HttpServeMux {
	c := chi.NewRouter()
	c.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: c}
})
```

这些代码不会改变 Gin、gorilla/mux 或 chi 自己的语义。第三方框架继续处理路由、中间件、参数解析和响应写出；Go-Spring 只把它们作为 `http.Handler` 纳入统一生命周期。

因此，选择路由器时不需要围绕 Go-Spring 重新选型。已经有成熟路由框架的项目，可以保留原有路由层，只把服务启动和关闭交给 Go-Spring。

## Server 生命周期

前面的示例都在交出 `http.Handler`。真正让 HTTP Server 成为 Go-Spring 能力的一部分，是它实现了 `gs.Server` 生命周期。

```go
type Server interface {
	Run(ctx context.Context, sig ReadySignal) error
	Stop() error
}
```

内置实现是 `gs.SimpleHttpServer`。启动时，它会先调用 `net.Listen("tcp", addr)` 监听端口；监听成功后触发 Ready 信号并等待应用整体就绪；随后再调用标准库 `http.Server.Serve` 开始处理请求。

这个顺序有一个工程含义：端口占用这类错误会在启动阶段暴露，而不是等应用看起来已经启动后才失败。Ready 信号也不是单个 HTTP Server 自己说了算，而是放在 Go-Spring 的 Server 协调流程里。

停止时，`SimpleHttpServer.Stop()` 调用的是标准库 `http.Server.Shutdown(context.Background())`。它会停止接受新连接，并给正在处理的请求留出完成机会。Go-Spring 在应用退出阶段调用 Server 的 `Stop()`，再继续收束容器和资源。

## HTTP Server 生命周期接入

Go-Spring 内置 HTTP Server 的核心价值，是把 `net/http` 入口放进应用生命周期，而不是提供另一套路由体系。

如果只是一个简单 HTTP 服务，可以继续使用 `http.DefaultServeMux`。如果路由组装需要依赖注入，可以提供自定义 `*gs.HttpServeMux`。如果项目已经使用 Gin、gorilla/mux 或 chi，只要把最终 `http.Handler` 交给 `gs.HttpServeMux`。在这些路径里，Go-Spring 始终负责同一件事：配置监听参数，协调 Ready，启动 Server，并在退出时优雅关闭。

HTTP Server 是一个内置组件的例子。下一类问题是，当数据库、Redis、pprof 这类组件也需要在多个项目里复用时，注册、配置和生命周期应该怎样封装。
