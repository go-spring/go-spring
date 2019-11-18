# Go-Spring

Go-Spring 的目标是让 GoLang 程序员也能用上如 Java Spring 那般威力强大的编程框架。感谢 Java Spring。

特性：

1. 提供完善的 IoC 容器，支持依赖注入、属性绑定；
2. 提供强大的启动器框架，支持自动装配、开箱即用；
3. 提供常见组件的抽象层，支持灵活地替换底层实现；

### 入门指南

Go-Spring 当前使用 Go1.12 进行开发，使用 Go Modules 进行依赖管理。

```
package main

import (
	_ "github.com/go-spring/go-spring-boot-starter/starter-gin"
	_ "github.com/go-spring/go-spring-boot-starter/starter-web"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/spring-core"
	"net/http"
)

func init() {
	SpringBoot.RegisterModule(func(ctx SpringCore.SpringContext) {
		ctx.RegisterBean(new(Controller))
	})
}

type Controller struct{}

func (c *Controller) InitWebBean(wc SpringWeb.WebContainer) {
	wc.GET("/", c.Home)
}

func (c *Controller) Home(ctx SpringWeb.WebContext) {
	ctx.String(http.StatusOK, "OK!")
}

func main() {
	SpringBoot.RunApplication("config/")
}
```

更多示例： https://github.com/go-spring/go-spring-boot-demo

### 项目列表

#### go-spring-parent

所有 Go-Spring 项目的根模块，包含日志，常量等。

#### go-spring

实现了 IoC 容器和 Boot 框架。

#### go-spring-web

不依赖 Go-Spring 框架的独立的 Web 模块。

#### go-spring-redis

不依赖 Go-Spring 框架的独立的 Redis 模块。

#### go-spring-rpc

不依赖 Go-Spring 框架的独立的 RPC 模块。

#### go-spring-orm

不依赖 Go-Spring 框架的独立的 ORM 模块。

#### go-spring-message

不依赖 Go-Spring 框架的独立的 MQ 模块。

#### go-spring-singlet

使用全局 SpringContext 变量改进后的 SpringBoot 框架。

#### go-spring-boot-starter

提供常见组件的启动器，实现开箱即用。

### 项目成员

#### 发起者/负责人

[lvan100 (LiangHuan)](https://github.com/lvan100)

如何成为贡献者？ 提交有意义的PR，并被采纳。

### QQ 交流群

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq.jpeg" width="140" height="*" />

### License

The Go-Spring is released under version 2.0 of the Apache License.
