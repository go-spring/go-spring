# Go-Spring 实战第 27 课：HTTP Server 接入：把 net/http 纳入启动、就绪和优雅关闭

Go-Spring 日志系统解决了应用运行中的观测问题。接下来我们回到服务入口：一个 Go-Spring 应用如果要直接暴露 HTTP 接口，HTTP Server 该怎样接入这套生命周期。

很多 Go 项目会从标准库 HTTP Server 起步。Go-Spring 没有绕开这套生态，而是在 `net/http` 之上提供默认接入和生命周期管理。

也就是说，内置 HTTP Server 用于在应用中直接暴露 HTTP 接口，默认随应用启动，也会纳入统一启动、就绪和关闭流程。下面从配置、路由接入、第三方路由集成和生命周期几块看它的定位。

## 最小 HTTP 服务直接使用默认路由

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

应用启动后访问 `http://localhost:9090/hello` 即可看到响应。

默认情况下，Go-Spring 会使用 `http.DefaultServeMux` 作为路由入口。这样一来，最简单的 HTTP 服务不需要额外注册路由器。

## HTTP Server 配置监听地址和超时

Go-Spring 内置 HTTP Server 支持：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `spring.http.server.addr` | 监听地址 | `:9090` |
| `spring.http.server.readTimeout` | 读取请求超时 | `5s` |
| `spring.http.server.headerTimeout` | 请求头读取超时 | `1s` |
| `spring.http.server.writeTimeout` | 写响应超时 | `5s` |
| `spring.http.server.idleTimeout` | 空闲连接超时 | `60s` |
| `spring.http.server.enabled` | 是否启用 | `true` |

如果只是调整监听端口，改 `spring.http.server.addr` 就够了：

```properties
spring.http.server.addr=:8080
```

如果应用已经有外部 HTTP 宿主，可以从配置中关闭内置 Server：

```properties
spring.http.server.enabled=false
```

也可以在启动代码里关闭，适合嵌入到已有进程时显式表达启动模式：

```go
func main() {
	gs.Web(false).Run()
}
```

如果应用已经被其他框架或宿主进程托管，我们可以关闭内置 Server，只保留 Go-Spring 的配置、容器和运行时能力。

## HttpServeMux 把路由器交给容器

Go-Spring 使用 `gs.HttpServeMux` 包装标准库 `http.Handler`：

```go
type HttpServeMux struct {
	http.Handler
}
```

如果 Go-Spring 容器中存在自定义 `*gs.HttpServeMux`，内置 HTTP Server 会使用它替换默认路由器。

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

这样路由定义本身也可以使用依赖注入，控制器、服务和配置都能进入路由组装过程。

## 第三方路由器作为 http.Handler 接入

Gin、gorilla/mux、chi 等框架都实现了 `http.Handler`，因此可以直接作为 `gs.HttpServeMux` 的 Handler。接入方式其实就是把最终路由器交给 Go-Spring 管。

Gin 本身已经是 `http.Handler`，因此只要把最终的 engine 放进 `gs.HttpServeMux`：

```go
gs.Provide(func() *gs.HttpServeMux {
	g := gin.Default()
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})
	return &gs.HttpServeMux{Handler: g}
})
```

gorilla/mux 也是同样思路：路由器负责匹配规则，Go-Spring 负责生命周期：

```go
gs.Provide(func() *gs.HttpServeMux {
	m := mux.NewRouter()
	m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: m}
})
```

chi 的接入方式也没有额外包装层，最终仍然交回一个 `http.Handler`：

```go
gs.Provide(func() *gs.HttpServeMux {
	c := chi.NewRouter()
	c.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: c}
})
```

第三方框架负责自己的路由、中间件和参数解析；Go-Spring 只负责把最终 `http.Handler` 纳入应用生命周期。这样两边职责就不会混在一起。

## 启动和停止都纳入 Server 生命周期

Go-Spring 内置 HTTP Server 由 `gs.SimpleHttpServer` 实现，并实现 `gs.Server` 接口。

启动时，它先监听端口，尽早发现端口占用等错误；监听成功后触发 Ready 信号，并等待其他 Server 也就绪；最后开始接受请求。

停止时调用标准库 `http.Server.Shutdown`，停止接受新连接，等待进行中的请求完成，关闭空闲连接。

这样 HTTP Server 的启动、就绪和关闭都和 Go-Spring 应用生命周期一致。

## HTTP Server 的职责是生命周期接入

Go-Spring 的内置 HTTP Server 负责把标准库 `http.Handler` 接入统一生命周期。路由、中间件和参数解析可以继续由标准库或第三方框架处理，Go-Spring 关心的是配置、启动、就绪和优雅关闭。

HTTP Server 是内置组件的例子。更多基础设施组件如果也要复用，就需要把注册、配置和生命周期打包成 Starter。
