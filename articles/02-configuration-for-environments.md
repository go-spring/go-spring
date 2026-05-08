# 端口和开关都写死以后，我第一次认真整理 Go 服务配置

上一篇我把 `/echo` 跑起来了。

刚开始我还挺满意：代码短，服务也能访问。但很快我就遇到一个很现实的问题：端口、返回文案、功能开关都写在代码里。

本地跑一个 demo 没关系。可一旦我想模拟不同环境，比如开发环境用一个端口，测试环境用另一个端口，或者临时把 `/echo` 关掉，我就不能每次都去改代码。

这时我才意识到：配置不是“高级功能”，而是一个服务稍微认真一点就必须面对的问题。

## 我原来会怎么做

如果不用框架，我可能会自己写一个 `config.json`，启动时读一下：

```go
type Config struct {
	Addr    string
	Message string
}
```

这能解决一部分问题，但马上又会冒出更多问题：

环境变量怎么覆盖？命令行临时参数怎么覆盖？不同环境的配置文件怎么合并？如果配置缺了，是用默认值还是直接报错？

我以前会觉得这些都可以以后再说。后来发现，配置越晚整理，越容易散成一堆特殊逻辑。

## 先写一个 app.properties

Go-Spring 默认会加载 `conf/app.properties`。我先把几个配置放进去：

```properties
spring.http.server.addr=:9090

bookman.app.name=BookMan Pro
bookman.feature.echo=true
bookman.echo.message=BookMan Pro is running
```

这里我第一次注意到，配置最好有自己的命名空间。

`spring.http.server.addr` 是框架内置 HTTP Server 的配置。

`bookman.*` 是我的业务配置。以后图书、缓存、价格服务都可以继续放到这个命名空间下面，不会和框架配置混在一起。

## 我不想在 Handler 里读文件

一开始我很容易写成这样：Handler 里面读配置，或者全局变量保存配置。

但这样会让业务代码知道太多细节。配置来自文件、环境变量还是命令行，Handler 其实不应该关心。

Go-Spring 的写法是声明自己需要什么配置：

```go
type EchoConfig struct {
	AppName string `value:"${bookman.app.name:=BookMan Pro}"`
	Enabled bool   `value:"${bookman.feature.echo:=true}"`
	Message string `value:"${bookman.echo.message:=BookMan Pro is running}"`
}
```

`value:"${key:=default}"` 这段我刚开始看有点陌生。后来理解成一句话就够了：从配置里找 `key`，找不到就用默认值。

如果不写默认值，配置缺失时启动会失败。这个特性挺有用，因为有些配置确实不能缺，比如数据库 DSN。

## Handler 只使用已经绑定好的配置

Handler 变成这样：

```go
type EchoHandler struct {
	Config EchoConfig `value:"${}"`
}

func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.Config.Enabled {
		http.Error(w, "echo disabled", http.StatusNotFound)
		return
	}
	_, _ = w.Write([]byte(h.Config.Message))
}
```

我喜欢这段代码的一点是：它没有打开文件，也没有读环境变量。它只是使用配置结果。

配置的加载、合并、类型转换，都是启动阶段完成的。

注册 Handler 和路由：

```go
func init() {
	gs.Provide(&EchoHandler{})

	gs.Provide(func(h *EchoHandler) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/echo", h.ServeHTTP)
		return &gs.HttpServeMux{Handler: mux}
	})
}
```

这里我也顺便开始接触 Bean，不过先不用急着完全理解。下一篇会专门讲对象装配。

## 试试不同方式覆盖配置

默认启动：

```bash
go run .
curl http://127.0.0.1:9090/echo
```

临时换端口和返回内容：

```bash
go run . -Dspring.http.server.addr=:9091 -Dbookman.echo.message=hello-dev
curl http://127.0.0.1:9091/echo
```

用环境变量覆盖：

```bash
GS_BOOKMAN_ECHO_MESSAGE=hello-env go run .
curl http://127.0.0.1:9090/echo
```

我第一次看到 `GS_BOOKMAN_ECHO_MESSAGE` 映射到 `bookman.echo.message` 时，才感觉环境变量和配置文件之间终于有了一套固定规则。

## Profile 解决环境差异

如果只是临时覆盖，用命令行就行。如果一套环境有很多配置，就适合放到 Profile 文件里。

新增 `conf/app-dev.properties`：

```properties
bookman.echo.message=hello-dev-profile
```

启动 dev Profile：

```bash
go run . -Dspring.profiles.active=dev
```

这时基础配置仍然加载，但 dev 配置里的同名 key 会覆盖它。

我给自己记了一个优先级：

```text
命令行参数 > 环境变量 > Profile 配置 > 基础配置 > 代码默认值
```

这个顺序很重要。以后如果配置结果和预期不一样，我至少知道该从哪里查起。

## 我踩到的几个小坑

配置 key 是大小写敏感的。`bookman.echo.message` 和 `bookman.echo.Message` 不是一个东西。

端口改了，请求地址也要改。我有一次用 `:9091` 启动，却还在 curl `9090`，白白排查了半天。

默认值不要乱给。像开关、展示文案可以有默认值；像生产数据库地址这种关键配置，缺了就应该启动失败。

## 给自己留个小练习

我准备加一个 `bookman.echo.prefix` 配置，让 `/echo` 返回：

```text
{prefix}: {message}
```

然后用命令行覆盖：

```bash
go run . -Dbookman.echo.prefix=dev
```

做到这里，我对配置的理解就从“读一个文件”变成了“让服务行为可以被外部环境控制”。下一篇我会继续处理另一个让我很头疼的问题：对象到底该谁来 new。
