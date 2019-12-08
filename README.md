# Go-Spring

Go-Spring 的愿景是让 GoLang 程序员也能用上如 Java Spring 那般威力强大的编程框架。🎉 wiki 已更新 🎉。

特性：

1. 提供完善的 IoC 容器，支持依赖注入、属性绑定；
2. 提供强大的启动器框架，支持自动装配、开箱即用；
3. 提供常见组件的抽象层，支持灵活地替换底层实现；

Go-Spring 当前使用 Go1.12 进行开发，使用 Go Modules 进行依赖管理。

### IoC 容器

Go-Spring 实现了如 Java Spring 那般功能强大的 IoC 容器，不仅支持通过对象注册 Bean，还支持通过 Bean 的构造函数注册 Bean，可以非常方便地在项目中引入 Go-Spring 框架。

Go-Spring 还扩充了 Bean 的概念，在 Golang 中，对象(即指针)、数组、Map、函数指针，这些都是 Bean，都可以放到 IoC 容器里。

`@Value`

Go-Spring 不仅支持基础类型的属性绑定，也支持自定义值类型的属性绑定，以及结构体属性的嵌套绑定。

`@Autowired` `@Qualifier` `@Required`

Go-Spring 通过 BeanId 的概念将上面的三个功能合并到了一个 Tag 属性里面，通过 BeanId 可以指定要绑定的 Bean 的名称、类型，以及绑定是否可以为空。

`@Configurable`

Go-Spring 可以对容器外部的对象进行属性和变量绑定。

`@Profile`

Go-Spring 可以设定 IoC 容器的运行环境，满足特定环境下 Bean 的注册问题。

`@Primary`

Go-Spring 可以在 Bean 注册时设定优先级，满足单元测试时注入 Mocked Bean 的需求。

`@DependsOn`

Go-Spring 支持对 Bean 的非直接依赖首先进行初始化。

`@Lookup`

Go-Spring 可以通过注册原型 Bean 工厂的方式在程序中方便的获取原型 Bean。

`ConstructorBinding`

Go-Spring 可以通过注册构造函数的方式进行属性绑定。

`@ComponentScan` `@Indexed`

Go-Spring 使用包匿名引用机制实现对注册 Bean 的扫描，并且无需借助其他手段如索引就能实现高效率的扫描。

`@Conditional`

Go-Spring 支持对 Bean 的注册设定各种 Matches 条件。
 
`@ConditionalOnExpression`

暂未支持。
  
`@ConditionalOnProperty` 

Go-Spring 支持通过匹配属性值的方式决定是否注册 Bean。

`@ConditionalOnBean` `@ConditionalOnMissingBean`

Go-Spring 支持通过判断指定 Bean 存在的方式决定是否注册 Bean。

`@ConditionalOnClass` `@ConditionalOnMissingClass`

GoLang 不会出现类型不存在的情况。

### 属性绑定

Go-Spring 不仅支持对普通数据类型进行属性绑定，也支持对自定义的值类型进行属性绑定，而且还支持对结构体属性的嵌套绑定。

```
type DB struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
	DB       string `value:"${db}"`
}

type DbConfig struct {
	DB []DB `value:"${db}"`
}
```

比如上面的这段代码就可以通过下面的配置进行绑定：

```
db:
  -
    username: root
    password: 123456
    url: 1.1.1.1
    port: 3306
    db: db1
  -
    username: root
    password: 123456
    url: 1.1.1.1
    port: 3306
    db: db2
```

而且更多属性绑定的能力还在等待你去发现。

### Boot 框架

Go-Spring 提供了一个简单但功能强大的启动器框架，不仅开箱即用，还支持开发自己的模块来利用开箱即用的能力，使得代码不仅仅是库层面的代码复用。

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
	SpringBoot.RegisterBean(new(Controller))
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

#### go-spring-boot-starter

提供常见组件的启动器，实现开箱即用。

### 项目成员

#### 发起者/负责人

[lvan100 (LiangHuan)](https://github.com/lvan100)

如何成为贡献者？ 提交有意义的PR，并被采纳。

### QQ 交流群

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="140" height="*" /> <img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(2).jpeg" width="140" height="*" />

### 微信公众号

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="140" height="*" />

### License

The Go-Spring is released under version 2.0 of the Apache License.
