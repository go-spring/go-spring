# 快速开始

本文档帮助你快速上手 Go-Spring，创建第一个应用。

## 环境要求

- Go 1.26+（始终推荐使用最新 Go 版本）
- Go modules 启用

## 创建方式

创建 Go-Spring 项目有两种方式：

1. **手动从零创建**
    * 适合想要理解框架工作原理的初学者。

2. **使用 `gs init` 创建模板项目**
    * 一键生成包含目录结构、配置文件、依赖管理的完整项目骨架，适合快速开始开发。

## 手动从零创建

### 安装

安装 Go-Spring 核心框架：

```bash
go get github.com/go-spring/spring-core@latest
```

### 初始化项目

```bash
mkdir hello-go-spring
cd hello-go-spring
go mod init hello
```

### 第一个示例：兼容标准 `net/http`

Go-Spring 可以完美兼容 Go 标准库，你不需要改变现有的编码习惯。

创建 `main.go`:

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func main() {
	// 使用标准库注册 HTTP 处理器
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello Go-Spring!"))
	})

	// 启动 Go-Spring 应用
	// 对比 `http.ListenAndServe(":8080", nil)`，`gs.Run()` 额外提供：
	//   ✅ 自动加载配置文件（YAML/properties/ENV）
	//   ✅ 开箱即用的依赖注入容器
	//   ✅ 完整的 Bean 生命周期管理
	//   ✅ 优雅关闭支持
	gs.Run()
}
```

Go-Spring 默认会启动 HTTP 服务器监听 `9090` 端口，你可以通过配置文件修改这个端口。

运行应用：

```bash
go run main.go
```

测试接口：

```bash
curl http://127.0.0.1:9090/echo
# Output: Hello Go-Spring!
```

你看，只需要一句 `gs.Run()`，你就得到了一个功能完整的 Go-Spring 应用。

### 第二个示例：使用依赖注入

这是 Go-Spring 的核心特性，让我们看看如何使用依赖注入组织代码。

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 注册一个 HelloService 对象到 IoC 容器
	gs.Provide(&HelloService{})

	// 注册一个 HelloHandler 对象到 IoC 容器
	gs.Provide(&HelloHandler{})

	// 注册一个 gs.HttpServeMux 对象到 IoC 容器，同时也接收一个 HelloHandler 对象
	gs.Provide(func(h *HelloHandler) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", h.ServeHTTP)
		// 通过 gs.HttpServeMux 封装 http.Handler 可以实现中间件机制
		return &gs.HttpServeMux{Handler: mux}
	})
}

type HelloService struct{}

func (s *HelloService) SayHello(name string) string {
	return "Hello, " + name + "!"
}

type HelloHandler struct {
	HelloService *HelloService `autowire:""` // 通过 autowire 注入 HelloService 对象
}

func (h *HelloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	msg := h.HelloService.SayHello("Go-Spring")
	_, _ = w.Write([]byte(msg))
}

func main() {
	gs.Run()
}
```

运行后访问：

```bash
curl http://127.0.0.1:9090/hello
# Output: Hello, Go-Spring!
```

## 使用 `gs init` 创建项目

<!-- 待补充 -->

## 下一步

现在你已经创建了第一个 Go-Spring 应用，接下来可以学习：

- [配置](../3.guides/01-configuration.md) - 学习如何使用配置系统加载外部配置
- [IoC 容器](../3.guides/02-ioc-container.md) - 深入学习依赖注入容器的使用
- [应用启停](../3.guides/03-startup-shutdown.md) - 了解完整的应用生命周期管理
