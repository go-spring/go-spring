# 我以为 http.ListenAndServe 就够了，直到我开始写一个真正的服务

刚开始学 Go 写后端时，我最喜欢标准库 HTTP。

它太直接了。注册一个路由，监听一个端口，浏览器或者 curl 一请求，就能看到结果：

```go
http.HandleFunc("/echo", handler)
http.ListenAndServe(":9090", nil)
```

这让我有一种错觉：写服务好像也没那么复杂。

直到我想认真做一个小项目。这个项目叫 BookMan Pro，先从一个图书管理服务开始。第一步当然不用搞太复杂，我只想让它有一个 `/echo` 接口，证明服务能跑起来。

配套代码在：

```text
course/01-hello-http
```

## 我最开始会这样写

如果只靠 Go 标准库，我大概会写出这样的代码：

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

这段代码当然能跑。

但我很快开始想后面的事情：端口以后是不是要从配置文件读？不同环境是不是端口不一样？服务退出时，是不是要等正在处理的请求结束？如果后面有 Controller、Service、DAO，这些对象应该在哪里创建？

我突然发现，`http.ListenAndServe` 只帮我解决了“监听端口”这件事。至于“一个应用应该怎么启动、怎么装配、怎么关闭”，它没有管。

这不是标准库不好，而是标准库本来就只负责 HTTP。

## 我第一次看到 gs.Run 的感觉

Go-Spring 的入门示例里，把最后一行换成了：

```go
gs.Run()
```

完整代码是这样：

```go
package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BookMan Pro is running"))
	})

	gs.Run()
}
```

我一开始有点疑惑：这不就是把 `ListenAndServe` 换了个名字吗？

后来才理解，不完全是。

`http.HandleFunc` 还是标准库的写法，Handler 没有被框架绑架。真正变的是启动方式。`gs.Run()` 启动的不只是一个 HTTP Server，而是一个 Go-Spring 应用。

它会顺手做这些事：

- 加载配置。
- 初始化容器。
- 创建和装配 Bean。
- 启动内置 HTTP Server。
- 监听退出信号，做优雅关闭。

现在这个例子还很小，所以看不出太多差别。但后面当我开始加配置、加 Service、加后台任务时，这个差别就会越来越明显。

## 先跑起来

进入示例目录：

```bash
cd course/01-hello-http
go run .
```

另开一个终端请求：

```bash
curl http://127.0.0.1:9090/echo
```

期望输出：

```text
BookMan Pro is running
```

如果访问 `/` 返回 404，不用紧张。现在只注册了 `/echo`。

如果端口 `9090` 被占用，应用会启动失败。下一篇我会把端口放进配置里，而不是写死在代码或者依赖默认值。

## 我这一步真正理解了什么

这一篇写完，我对 Go-Spring 的第一印象不是“它让我少写了一行代码”。

如果只是少写一行，那没必要引入框架。

我真正理解的是：Go-Spring 想接管的是应用启动这件事。HTTP Handler 仍然可以按 Go 原生方式写，但配置加载、依赖装配、生命周期和优雅关闭这些工程问题，可以交给框架统一处理。

所以这个 `/echo` 不是为了展示一个复杂功能，而是给后面所有改造找一个很小的起点。

## 给自己留个小练习

我给自己加了一个小任务：新增 `/healthz` 接口，返回 `ok`。

验证命令：

```bash
curl http://127.0.0.1:9090/healthz
```

要求只有一个：继续用标准库 `http.HandleFunc` 写 Handler，启动方式仍然用 `gs.Run()`。

如果能做到这一点，说明我已经抓住了第一篇的重点：业务处理还是 Go 的写法，应用启动开始交给 Go-Spring。
