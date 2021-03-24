# spring-gin

封装 github.com/gin-gonic/gin 实现的 Web 框架。

- [创建 Web 容器](#创建-web-容器)
    - [NewContainer](#newcontainer)
- [适配 gin 框架](#适配-gin-框架)
    - [Handler](#handler)
    - [Filter](#filter)
    - [GinContext](#gincontext)
    - [WebContext](#webcontext)

### 创建 Web 容器

#### NewContainer

创建 gin 实现的 WebContainer。

    func NewContainer(config SpringWeb.ContainerConfig) *Container {}

### 适配 gin 框架

#### Handler

适配 gin 形式的处理函数。

    func Handler(fn gin.HandlerFunc) SpringWeb.Handler {}

#### Filter

适配 gin 形式的中间件函数。

    func Filter(fn gin.HandlerFunc) SpringWeb.Filter {}

#### GinContext

将 SpringWeb.WebContext 转换为 *gin.Context。

    func GinContext(webCtx SpringWeb.WebContext) *gin.Context {}

#### WebContext

将 *gin.Context 转换为 SpringWeb.WebContext。

    func WebContext(ginCtx *gin.Context) SpringWeb.WebContext {}