# 内置 HTTP Server

Go-Spring 提供了基础的 HTTP Server 集成，基于 Go-Spring 的 starter 机制自动初始化，生命周期纳入应用启停管理，支持优雅关闭。

## 定位与设计原则

Go-Spring 内置 HTTP Server 的核心设计理念：

### 基于 Starter 机制自动配置
Go-Spring 的 HTTP Server 是通过 starter 机制自动集成的，只需引入依赖即可自动创建并启动 HTTP Server，无需手动编写启动代码。你只需要专注于业务路由的开发，Go-Spring 会帮你处理好服务器的生命周期管理。

### 完全兼容 Go 标准库
这是 Go-Spring HTTP Server 最重要的特性之一：**100% 兼容 Go 标准库 `net/http`**。这意味着：

- 你可以使用任何你熟悉的标准库写法，学习成本为零
- 现有的基于标准库的代码可以无缝接入
- 所有兼容标准库的第三方中间件、工具都可以直接使用
- 不需要改变你的编程习惯

### 不重复造轮子，聚焦集成
Go-Spring 内置 HTTP Server **不提供高阶 Web 能力**，保持极简：
- 不提供 Web 框架级上下文对象
- 不提供参数绑定、返回值自动序列化
- 不提供路由分组、路由优先级控制
- 不提供模板渲染

这样设计的好处是：Go-Spring 只做它该做的事情——提供统一的生命周期管理，把具体的路由能力交给专业的工具去做。如果你需要高阶功能，可以无缝集成社区中优秀的第三方路由。

### 统一生命周期管理
HTTP Server 的启动和关闭完全纳入 Go-Spring 的应用启停模型，和整个应用生命周期保持一致，支持优雅关闭（graceful shutdown）。

## 快速开始

### 1. 引入依赖

在 `go.mod` 中引入 HTTP starter：

```go
require (
    github.com/go-spring/spring-boot/v2
    github.com/go-spring/spring-starter/v2/http
)
```

或者使用 import 引入：

```go
import _ "github.com/go-spring/spring-starter/http"
```

### 2. 配置端口

在配置文件中添加监听地址（默认是 `:8080`）：

```properties
# 监听端口
http.addr=:8080
# 读写超时
http.read-timeout=10s
http.write-timeout=30s
# 是否启用（默认为 true）
http.enable=true
```

### 3. 注册路由

你可以完全使用标准库的方式注册路由：

```go
package main

import (
	"net/http"
	"github.com/go-spring/spring-boot/v2"
)

func init() {
	// 直接使用标准库的 http.HandleFunc 注册路由
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Go-Spring!"))
	})
}

func main() {
	springboot.Run()
}
```

就这么简单！运行你的应用，访问 `http://localhost:8080/hello` 就能看到结果了。

## HTTP Server 核心特性

### 路由注册抽象

Go-Spring 对路由注册进行了抽象，让你可以方便地注册路由，同时支持中间件机制。你可以通过 `router` 实例来注册路由：

```go
package main

import (
	"net/http"
	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-starter/http/router"
)

func init() {
	// 注入 router 实例
	springboot.Provide(func(r *router.Router) {
		r.HandleFunc("/hello", helloHandler)
	})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, Go-Spring!"))
}

func main() {
	springboot.Run()
}
```

### 中间件支持

Go-Spring 抽象了路由注册，天然支持中间件机制，让你可以方便地对请求进行切面处理。中间件使用标准的 `func(http.Handler) http.Handler` 签名：

```go
// 标准中间件签名
type Middleware func(http.Handler) http.Handler
```

下面是一个完整的日志记录中间件示例：

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-starter/http/router"
)

func init() {
	springboot.Provide(func(r *router.Router) {
		// 注册全局中间件
		r.Use(loggingMiddleware)
		
		// 注册路由
		r.HandleFunc("/hello", helloHandler)
		
		// 也可以给单个路由添加中间件
		r.Handle("/private", authMiddleware(http.HandlerFunc(privateHandler)))
	})
}

// 日志记录中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Started %s %s", r.Method, r.RequestURI)
		
		next.ServeHTTP(w, r)
		
		log.Printf("Completed in %v", time.Since(start))
	})
}

// 认证中间件示例
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
			return
		}
		// 验证 token ...
		next.ServeHTTP(w, r)
	})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, Go-Spring!"))
}

func privateHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("This is a private area"))
}

func main() {
	springboot.Run()
}
```

多个中间件会按照注册顺序执行，形成一个调用链。你可以组合多个中间件来实现不同的功能，比如：日志记录、访问日志、权限认证、CORS 处理、请求限流等。

所有兼容标准库 `net/http` 的第三方中间件都可以直接使用，比如常用的：

- `github.com/rs/cors` - CORS 跨域处理
- `github.com/gorilla/handlers` - 各种有用的处理句柄
- `github.com/didip/tollbooth` - 请求限流

## 集成第三方路由

Go-Spring 内置 HTTP Server 只提供了基础的路由能力，如果你需要更高级的路由功能（比如动态路由、参数路由、路由分组等），可以很方便地集成社区中优秀的第三方路由库。**只要第三方路由实现了标准的 `http.Handler` 接口，就可以无缝接入**。

下面是几个常见路由库的集成示例：

### 集成 Gin 框架

[Gin](https://github.com/gin-gonic/gin) 是目前非常流行的高性能 Go Web 框架，你可以很方便地和 Go-Spring 整合：

```go
package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-starter/http/router"
)

func init() {
	springboot.Provide(func(r *router.Router) {
		// 创建 Gin 引擎
		g := gin.Default()
		
		// 注册 Gin 路由
		g.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
		
		g.GET("/user/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.JSON(200, gin.H{
				"id": id,
			})
		})
		
		// 将 Gin 引擎作为 Handler 注册到 Go-Spring
		// 利用 Gin 处理所有请求
		r.Handle("/gin/*any", g)
	})
}

func main() {
	springboot.Run()
}
```

这样，Go-Spring 负责服务器的生命周期管理，Gin 负责路由和处理，各司其职，完美结合！

### 集成 gorilla/mux

[gorilla/mux](https://github.com/gorilla/mux) 是一个老牌的强大路由库：

```go
package main

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-starter/http/router"
)

func init() {
	springboot.Provide(func(r *router.Router) {
		// 创建 gorilla/mux 路由器
		m := mux.NewRouter()
		
		// 注册路由，支持路径参数
		m.HandleFunc("/products/{id}", func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			id := vars["id"]
			w.Write([]byte("Product ID: " + id))
		})
		
		// 注册到 Go-Spring
		r.Handle("/", m)
	})
}

func main() {
	springboot.Run()
}
```

### 集成 chi

[chi](https://github.com/go-chi/chi) 是一个轻量级、优雅的路由库：

```go
package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-starter/http/router"
)

func init() {
	springboot.Provide(func(r *router.Router) {
		r := chi.NewRouter()
		
		// 使用 chi 的中间件
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		
		// 注册路由
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("welcome"))
		})
		
		r.Route("/articles", func(r chi.Router) {
			r.Get("/", listArticles)
			r.Get("/{id}", getArticle)
		})
		
		// 注册到 Go-Spring
		r.Handle("/", r)
	})
}

func listArticles(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("list articles"))
}

func getArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Write([]byte("article id: " + id))
}

func main() {
	springboot.Run()
}
```

## 配置说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `http.addr` | 监听地址 | `:8080` |
| `http.read-timeout` | 读超时 | `10s` |
| `http.write-timeout` | 写超时 | `30s` |
| `http.idle-timeout` | 空闲连接超时 |  |
| `http.enable` | 是否启用 HTTP Server | `true` |

## 进阶用法

### 优雅关闭

Go-Spring 内置 HTTP Server 默认支持优雅关闭。当应用收到停止信号时，会：

1. 停止接受新的连接
2. 等待正在处理的请求完成
3. 关闭服务器并退出

等待超时由应用的整体关闭超时控制，无需额外配置。

### 监听多个端口

如果你需要启动多个 HTTP Server 监听不同的端口，可以自定义注册额外的 HTTP Server：

```go
package main

import (
	"net/http"
	"context"
	"github.com/go-spring/spring-boot/v2"
	"github.com/go-spring/spring-core/v2"
)

func init() {
	// 注册第二个 HTTP Server 的生命周期
	springboot.Provide(func(lc springcore.Lifecycle) {
		srv := &http.Server{
			Addr: ":9090",
			Handler: yourHandler,
		}
		
		// 将启动和停止注册到生命周期
		lc.OnStart(func(ctx context.Context) error {
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					// 处理错误
				}
			}()
			return nil
		})
		
		lc.OnStop(func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		})
	})
}
```

这样主 HTTP Server 还是走默认配置，你额外注册的 HTTP Server 会被 Go-Spring 统一管理生命周期。

### 获取底层 *http.Server 实例

如果你需要对 HTTP Server 进行更高级的自定义配置，可以注入底层的 `*http.Server` 实例：

```go
package main

import (
	"net/http"
	"github.com/go-spring/spring-boot/v2"
)

func init() {
	springboot.Provide(func(srv *http.Server) {
		// 自定义配置
		srv.MaxHeaderBytes = 1 << 20
		srv.ReadHeaderTimeout = 5 * time.Second
		// ... 其他自定义配置
	})
}
```

## 禁用内置 HTTP Server

如果你不需要内置 HTTP Server，比如你想完全自己管理 HTTP Server，或者你只需要使用 Go-Spring 的 IoC 容器，可以通过配置禁用：

```properties
http.enable=false
```

## 总结

Go-Spring 内置 HTTP Server 的设计哲学是：

1. **基于 starter 机制**：引入依赖即可使用，零配置启动
2. **完全兼容标准库**：学习成本低，生态复用性好
3. **核心职责单一**：只负责生命周期管理和启停，不抢第三方路由的活儿
4. **开放性好**：可以无缝集成任何实现了 `http.Handler` 的第三方路由

这种设计既保留了灵活性，又提供了便利性，你可以根据自己的需求选择合适的路由方案。
