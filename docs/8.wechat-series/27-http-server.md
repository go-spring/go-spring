# 内置 HTTP Server

## 本篇要解决的问题

Go-Spring 内置 HTTP Server，用于在应用中直接暴露 HTTP 接口。它基于标准库 `net/http`，默认随应用启动，并纳入统一生命周期管理。

本篇只讨论内置 HTTP Server：配置、路由接入、第三方路由集成和生命周期。

## 快速开始

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

默认情况下，Go-Spring 会使用 `http.DefaultServeMux` 作为路由入口。

## 配置项

内置 HTTP Server 支持：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `spring.http.server.addr` | 监听地址 | `:9090` |
| `spring.http.server.readTimeout` | 读取请求超时 | `5s` |
| `spring.http.server.headerTimeout` | 请求头读取超时 | `1s` |
| `spring.http.server.writeTimeout` | 写响应超时 | `5s` |
| `spring.http.server.idleTimeout` | 空闲连接超时 | `60s` |
| `spring.http.server.enabled` | 是否启用 | `true` |

修改监听端口：

```properties
spring.http.server.addr=:8080
```

关闭内置 Server：

```properties
spring.http.server.enabled=false
```

或在代码中：

```go
func main() {
	gs.Web(false).Run()
}
```

## 路由机制

Go-Spring 使用 `gs.HttpServeMux` 包装标准库 `http.Handler`：

```go
type HttpServeMux struct {
	http.Handler
}
```

如果容器中存在自定义 `*gs.HttpServeMux`，内置 HTTP Server 会使用它替换默认路由器。

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

路由定义本身可以使用依赖注入，因此控制器、服务和配置都能进入路由组装过程。

## 第三方路由集成

Gin、gorilla/mux、chi 等框架都实现了 `http.Handler`，因此可以直接作为 `gs.HttpServeMux` 的 Handler。

Gin：

```go
gs.Provide(func() *gs.HttpServeMux {
	g := gin.Default()
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})
	return &gs.HttpServeMux{Handler: g}
})
```

gorilla/mux：

```go
gs.Provide(func() *gs.HttpServeMux {
	m := mux.NewRouter()
	m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: m}
})
```

chi：

```go
gs.Provide(func() *gs.HttpServeMux {
	c := chi.NewRouter()
	c.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	return &gs.HttpServeMux{Handler: c}
})
```

第三方框架负责自己的路由、中间件和参数解析；Go-Spring 只负责把最终 `http.Handler` 纳入应用生命周期。

## 生命周期

内置 HTTP Server 由 `gs.SimpleHttpServer` 实现，并实现 `gs.Server` 接口。

启动时，它先监听端口，尽早发现端口占用等错误；监听成功后触发 Ready 信号，并等待其他 Server 也就绪；最后开始接受请求。

停止时调用标准库 `http.Server.Shutdown`，停止接受新连接，等待进行中的请求完成，关闭空闲连接。

这让 HTTP Server 的启动、就绪和关闭都和 Go-Spring 应用生命周期一致。

## 边界

本篇只围绕内置 HTTP Server。组件封装和 Starter 机制会在下一篇讨论，测试体系放到之后单独展开。

