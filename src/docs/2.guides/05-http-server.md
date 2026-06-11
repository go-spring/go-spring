# 内置 HTTP Server

Go-Spring 内置 HTTP Server，适用于在应用中直接暴露 HTTP 接口的场景。
它基于标准库 `net/http` 构建，默认随应用一同启动，并统一纳入 Go-Spring 的生命周期管理。

- **默认启用**：基于 Starter 机制自动注册和初始化，无需额外配置即可使用。
- **标准兼容**：完全兼容 `net/http`，可以沿用标准库的路由和处理器写法。
- **灵活扩展**：可以接入任何实现了 `http.Handler` 接口的路由框架。
- **优雅关闭**：启动和停止流程由应用生命周期统一管理，适合生产环境使用。

## 快速开始

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 使用 HTTP 标准库的路由和处理器写法，Go-Spring 会自动接管并生效。
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Go-Spring!"))
	})
}

func main() {
	gs.Run()
}
```

应用启动后，使用浏览器访问 `http://localhost:9090/hello` 即可看到响应。

## 配置项

内置 HTTP Server 支持以下配置项：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `spring.http.server.addr` | 监听地址 | `:9090` |
| `spring.http.server.readTimeout` | 读取请求的超时时间 | `5s` |
| `spring.http.server.headerTimeout` | 读取请求头的超时时间 | `1s` |
| `spring.http.server.writeTimeout` | 写入响应的超时时间 | `5s` |
| `spring.http.server.idleTimeout` | 空闲连接的超时时间 | `60s` |
| `spring.http.server.enabled` | 是否启用 HTTP Server | `true` |

如果需要修改监听端口，可以在配置文件中设置：

```properties
spring.http.server.addr=:8080
```

如果需要关闭内置 HTTP Server，可以设置：

```properties
spring.http.server.enabled=false
```

我们也可以通过 `gs.Web(false)` 在代码中禁用内置 HTTP Server：

```go
func main() {
	gs.Web(false).Run()
}
```

## 路由机制

Go-Spring 使用 `gs.HttpServeMux` 包装标准库的 `http.Handler`，
因而可以将 HTTP 路由接入 IoC 容器以及应用生命周期管理。
同时还可以复用标准库处理器模型，保留 HTTP 中间件的组合方式。

```go
type HttpServeMux struct {
	http.Handler
}
```

默认情况下，Go-Spring 会将 `http.DefaultServeMux` 包装为 Bean 注册到容器中。
因此我们可以直接通过全局的 `http.HandleFunc`、`http.Handle` 注册路由。

如果我们需要替换默认的路由器，可以提供一个自定义的 `*gs.HttpServeMux`。
并且在创建 `*gs.HttpServeMux` 的时候，还可以通过依赖注入使用其他 Bean。

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

type UserController struct{}

func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.FormValue("user"))
}

// logging 是标准 HTTP 中间件，用于记录请求日志。
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func init() {
	// UserController 作为普通 Bean 注册，可注入到路由定义中。
	gs.Provide(new(UserController))

	// 重定义 HTTP Server 的路由入口。
	// Go-Spring 默认使用 http.DefaultServeMux；
	// 但当容器中存在自定义的 *gs.HttpServeMux 时，
	// 内置 HTTP Server 会使用这里返回的 *gs.HttpServeMux 替换默认路由器。
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)

		// 自定义路由器也可以包装中间件，再交给 HttpServeMux。
		return &gs.HttpServeMux{Handler: logging(mux)}
	})
}

func main() {
	gs.Run()
}
```

## 路由集成

Gin、chi、gorilla/mux 等常见 HTTP 路由框架都实现了 `http.Handler` 接口，
因此都可以通过提供自定义 `*gs.HttpServeMux` 的方式接入。

这种接入方式不会改变第三方框架自身的用法。路由、中间件、参数解析等能力仍由对应框架负责，
Go-Spring 只负责将最终的 `http.Handler` 作为 HTTP Server 的路由入口。

### 集成 Gin

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-core/gs"
)

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// 创建 Gin 引擎，可以使用 Gin 原生 API 定义路由。
		g := gin.Default()

		// 注册 Gin 中间件。
		g.Use(func(c *gin.Context) {
			log.Printf("%s %s", c.Request.Method, c.Request.URL.Path)
			c.Next()
		})

		// 注册 Gin 路由。
		g.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "pong"})
		})

		// Gin Engine 实现了 http.Handler，因此可作为 Go-Spring 的路由入口。
		return &gs.HttpServeMux{Handler: g}
	})

	gs.Run()
}
```

### 集成 gorilla/mux

```go
package main

import (
	"log"
	"net/http"

	"github.com/go-spring/spring-core/gs"
	"github.com/gorilla/mux"
)

// logging 是标准 HTTP 中间件，用于记录请求日志。
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// 创建 gorilla/mux 路由器，可以使用 gorilla/mux 原生 API 定义路由。
		m := mux.NewRouter()

		// 注册 gorilla/mux 中间件。
		m.Use(logging)

		// 注册 gorilla/mux 路由。
		m.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})

		// gorilla/mux Router 实现了 http.Handler，因此可作为 Go-Spring 的路由入口。
		return &gs.HttpServeMux{Handler: m}
	})

	gs.Run()
}
```

### 集成 chi

```go
package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-spring/spring-core/gs"
)

// logging 是标准 HTTP 中间件，用于记录请求日志。
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	gs.Provide(func() *gs.HttpServeMux {
		// 创建 chi 路由器，可以使用 chi 原生 API 定义路由和中间件。
		c := chi.NewRouter()

		// 注册 chi 中间件。
		c.Use(logging)

		// 注册 chi 路由。
		c.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})

		// chi Router 实现了 http.Handler，因此可作为 Go-Spring 的路由入口。
		return &gs.HttpServeMux{Handler: c}
	})

	gs.Run()
}
```

## 生命周期

Go-Spring 内置的 HTTP Server 由 `gs.SimpleHttpServer` 实现。
它实现了 `gs.Server` 接口，通过 `Run` 和 `Stop` 接入应用的启动和停止流程。

```go
func (s *SimpleHttpServer) Run(ctx context.Context, sig ReadySignal) error {
	// 先监听端口，尽早发现端口占用等启动错误。
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.svr.Addr)
	}

	// 等待 ReadySignal 触发，确保其他 Server 已经就绪。
	<-sig.TriggerAndWait()

	// 应用就绪后再开始接受 HTTP 请求。
	err = s.svr.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}
```

启动阶段的一个关键点是：在端口监听成功后，HTTP Server 会等待其他 Server 完成准备工作，
然后才开始接受请求。这样可以避免健康检查接口已经可用、流量已经进入，但其他 Server 随后启动失败，
最终导致请求处理失败的情况。

```go
// Stop 优雅停止 HTTP Server，允许正在处理的请求完成。
func (s *SimpleHttpServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
```

当应用收到停止信号（如 `SIGINT`、`SIGTERM`）时，Go-Spring 会自动调用 `Stop` 方法，
执行标准的 HTTP Server 优雅关闭流程：停止接受新连接、等待进行中的请求完成、关闭空闲连接并退出。
通过这种机制，服务停止时不会主动中断正在处理的业务请求，适用于在线服务的发布、重启和下线场景。
