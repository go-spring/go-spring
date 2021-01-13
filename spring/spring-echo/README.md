# spring-echo

封装 github.com/labstack/echo 实现的 Web 框架。

- [创建 Web 容器](#创建-web-容器)
    - [NewContainer](#newcontainer)
- [适配 echo 框架](#适配-echo-框架)
    - [Handler](#handler)
    - [Filter](#filter)
    - [EchoContext](#echocontext)
    - [WebContext](#webcontext)

### 创建 Web 容器

#### NewContainer

创建 echo 实现的 Web 容器。

    func NewContainer(config SpringWeb.ContainerConfig) *Container {}

### 适配 echo 框架

#### Handler

适配 echo 形式的处理函数。

    func Handler(fn echo.HandlerFunc) SpringWeb.Handler {}

#### Filter

适配 echo 形式的中间件函数。

    func Filter(fn echo.MiddlewareFunc) SpringWeb.Filter {}

#### EchoContext

将 SpringWeb.Context 转换为 echo.Context。

    func EchoContext(ctx SpringWeb.Context) echo.Context {}

#### WebContext

将 echo.Context 转换为 SpringWeb.Context。

    func WebContext(echoCtx echo.Context) SpringWeb.Context {}