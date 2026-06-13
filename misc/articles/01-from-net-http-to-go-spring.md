# 从 net/http 到 Go-Spring：先接管应用启动

BookMan Pro 的第一步只做一件事：提供一个 `/echo` 接口，确认服务能启动、能响应请求。

这一版代码在：

```text
misc/course/01-hello-http
```

Handler 仍然用标准库写：

```go
http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("BookMan Pro is running"))
})
```

这里没有 Controller，也没有 Service。第一篇先不急着拆层，重点只看启动方式从 `http.ListenAndServe` 换成 `gs.Run()` 以后，应用模型有什么变化。

## 标准库版本的问题边界

如果只用标准库，代码大概是这样：

```go
package main

import "net/http"

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BookMan Pro is running"))
	})

	http.ListenAndServe(":9090", nil)
}
```

这段代码足够小，也能正常工作。对一个临时 demo 来说，没有必要再加框架。

但服务稍微往前走一步，就会碰到一些和 Handler 无关的问题：端口从哪里来，配置什么时候加载，对象在哪里创建，应用退出时 HTTP Server 怎么关闭，后台任务和其他 Server 又放在哪里。

`net/http` 负责 HTTP 协议本身，这些应用层的启动和关闭流程需要额外处理。

## 换成 gs.Run

当前示例的完整代码是：

```go
package main

import (
	"net/http"

	"go-spring.org/spring/gs"
)

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BookMan Pro is running"))
	})

	gs.Run()
}
```

这个变化很小，但边界很明确：业务处理还是 `net/http` 的 Handler，应用启动交给 Go-Spring。

`gs.Run()` 会进入 Go-Spring 的应用启动流程。后面的文章会逐步用到配置加载、Bean 装配、Runner、Server 和关闭信号。第一篇先保留最小代码，让读者看到：Go-Spring 不要求一上来就改掉标准库 Handler。

## 运行这个最小服务

进入示例目录：

```bash
cd misc/course/01-hello-http
go run .
```

另开终端请求：

```bash
curl http://127.0.0.1:9090/echo
```

预期输出：

```text
BookMan Pro is running
```

如果访问 `/`，返回 404 是正常的；这一版只注册了 `/echo`。

端口 `9090` 现在还没有出现在代码里，因为 Go-Spring 内置 HTTP Server 会使用默认配置。下一篇会把端口、返回内容和功能开关放进配置文件，这样就不用为了换环境去改 Handler。

## 这一篇留下的边界

当前阶段只做两件事：

- Handler 仍然按 Go 标准库方式编写。
- 应用启动、信号处理和后续生命周期能力交给 Go-Spring。

这个边界很重要。后面继续引入配置、IoC、Runner 和自定义 Server 时，业务代码不需要一次性改成另一套写法。
