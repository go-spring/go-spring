# 配置不应该散在 Handler 里

`misc/course/02-configuration` 还是只有一个 `/echo`，但这一版开始把端口、开关和返回内容移出代码。

最小服务里写死字符串没有问题。BookMan Pro 如果要模拟不同环境，就不能每次都改 Handler：本地用一个返回文案，测试环境用另一个文案，某些环境还可能临时关闭 `/echo`。

这篇先处理配置边界，暂时不展开 IoC。代码里只有一个 `EchoHandler`，足够说明问题。

## 配置文件先放在 conf 目录

Go-Spring 会加载 `conf/app.properties`。示例配置是：

```properties
spring.http.server.addr=:9090

bookman.app.name=BookMan Pro
bookman.feature.echo=true
bookman.echo.message=BookMan Pro is running
bookman.echo.prefix=dev
```

这里有两个命名空间。

`spring.http.server.addr` 是框架 HTTP Server 的配置，控制监听地址。

`bookman.*` 是 BookMan Pro 自己的配置。业务配置放在自己的前缀下，后面继续加图书、缓存、价格服务相关配置时，不会和框架配置混在一起。

## Handler 只拿绑定后的值

代码里没有手动打开配置文件，也没有在请求进来时读环境变量：

```go
type EchoHandler struct {
	AppName string `value:"${bookman.app.name:=BookMan Pro}"`
	Enabled bool   `value:"${bookman.feature.echo:=true}"`
	Message string `value:"${bookman.echo.message:=BookMan Pro is running}"`
	Prefix  string `value:"${bookman.echo.prefix:=local}"`
}
```

`value:"${key:=default}"` 表示从配置里绑定 `key`，没有配置时使用默认值。

这类默认值适合演示文案、功能开关、超时时间等低风险配置。数据库地址、密钥、必须存在的外部服务地址，通常不应该给一个看起来能跑的假默认值；缺配置时让应用启动失败，排查成本更低。

Handler 里只使用已经绑定好的字段：

```go
func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled {
		http.Error(w, "echo disabled", http.StatusNotFound)
		return
	}
	_, _ = fmt.Fprintf(w, "%s: %s", h.Prefix, h.Message)
}
```

配置来源可以是文件、环境变量、命令行参数或 Profile。Handler 不需要知道这些来源。

## 路由仍然是标准库 ServeMux

这一版开始把 Handler 注册成 Bean：

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

这里已经出现了对象装配的味道：`NewServeMux` 这段函数需要 `*EchoHandler`，Go-Spring 会在启动时把它传进来。

不用急着把所有东西都容器化。当前目标只是让 `/echo` 使用外部配置，下一篇再系统看 Controller、Service、DAO 的依赖关系。

## 覆盖配置

默认运行：

```bash
go run .
curl http://127.0.0.1:9090/echo
```

临时改端口和返回内容：

```bash
go run . -Dspring.http.server.addr=:9091 -Dbookman.echo.message=hello-dev
curl http://127.0.0.1:9091/echo
```

用环境变量覆盖：

```bash
GS_BOOKMAN_ECHO_MESSAGE=hello-env go run .
curl http://127.0.0.1:9090/echo
```

`GS_BOOKMAN_ECHO_MESSAGE` 映射到 `bookman.echo.message`。这个规则让配置文件和容器环境之间有固定对应关系，不需要业务代码再写一层转换。

## Profile 放成套环境差异

示例里有一个 `conf/app-test.properties`：

```properties
bookman.echo.message=BookMan Pro from test profile
bookman.echo.prefix=test
```

启动 test Profile：

```bash
go run . -Dspring.profiles.active=test
```

基础配置仍然加载，Profile 文件里的同名 key 覆盖基础配置。适合放一组环境差异，比如测试环境的端口、文案、开关和外部依赖地址。

排查配置结果时，优先看命令行参数和环境变量，再看 Profile 文件，最后看基础配置和代码默认值。这个顺序能减少很多“明明改了配置却没生效”的时间浪费。

## 这篇的取舍

这一版没有引入复杂配置对象，也没有把 Handler 拆成多层。原因很简单：当前只有 `/echo`，还不到设计大型配置模型的时候。

先把配置来源和业务代码隔开，已经能解决最直接的问题：同一份二进制在不同环境下运行，不需要重新改代码。
