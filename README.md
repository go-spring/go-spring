<div>
 <img src="https://raw.githubusercontent.com/go-spring/go-spring/master/logo@h.png" width="140" height="*" alt="logo"/>
 <br/>
 <img src="https://img.shields.io/github/license/go-spring/go-spring" alt="license"/>
 <img src="https://img.shields.io/github/go-mod/go-version/go-spring/spring-core" alt="go-version"/>
 <img src="https://img.shields.io/github/v/release/go-spring/go-spring?include_prereleases" alt="release"/>
</div>

> 如果你想参与 Go-Spring 项目的研发和管理，欢迎加入项目团队，你可以是擅长编码的技术极客，可以是擅长项目管理的沟通达人，
> 可以是擅长撰写文档的文字高手，Go-Spring 项目团队都热切欢迎你的加入！
> 
> 如果你觉得 Go-Spring 项目很棒，但是没有时间亲身加入，你也可以通过捐赠的方式助力和守护 Go-Spring 项目的成长，所有
> 捐赠的资金都将透明地用于 Go-Spring 项目团队的人员激励和项目推广。
> 
> 最后，欢迎 🌟 本项目，你的关注是我和团队继续前进的动力！破釜沉舟，百二秦关终属楚；卧薪尝胆，三千越甲可吞吴！

Go-Spring 的愿景是让 Go 程序员也能用上如 Java Spring 那般威力强大的编程框架，立志为用户提供简单、安全、可信赖的编程体验。

其特性如下：

1. 提供了完善的 IoC 容器，支持依赖注入、属性绑定；
2. 提供了强大的启动器框架，支持自动装配、开箱即用；
3. 提供了常见组件的抽象层，支持灵活地替换底层实现；  
   3.1 抽象 web 框架，echo 和 gin 可以灵活替换。  
   3.2 抽象 redis 框架，redigo 和 go-redis 可以灵活替换。
4. 遵循最小依赖原则，部分组件零依赖，避免依赖地狱；  
   4.1 提供 assert 包，满足日常测试断言的需求。  
   4.2 提供 cast 包，满足日常数据转换的需求。  
   4.3 提供 atomic 包，方便并发安全的存取数据。
5. 首创基于框架的流量录制和回放技术，让测试更简单；
6. 实现 Log4J 的日志架构，让日志管理更简单、更强大；

Go-Spring 当前使用 Go1.14 进行开发，使用 Go Modules 进行依赖管理。

## 项目列表

- [spring-base](spring/spring-base/README.md) - Go 准标准库 (like cpp boost to go)。
  - [apcu](spring/spring-base/apcu/README.md) - 提供了进程内缓存组件。
  - [assert](spring/spring-base/assert/README.md) - 提供了一些常用的断言函数。
  - [atomic](spring/spring-base/atomic/README.md) - 封装标准库 atomic 包的操作函数。
- [spring-core](spring/spring-core/README.md) - Go-Spring 核心库，IoC、Web 框架、Redis 封装、MySQL 封装 ...
  - [gs](spring/spring-core/gs/README.md) - 实现了 go-spring 的核心骨架。
  - [web](spring/spring-core/web/README.md) - 为社区优秀的 Web 服务器提供一个抽象层，使得底层可以灵活切换。
  - [redis](spring/spring-core/redis/README.md) - Redis 封装。
- 模块列表
  - [spring-echo](spring/spring-echo/README.md) - echo 封装。
  - [spring-gin](spring/spring-gin/README.md) - gin 封装。
  - [spring-swag](spring/spring-swag/README.md) - swagger 封装。
  - [spring-go-redis](spring/spring-go-redis/README.md) - go-redis 封装。
  - [spring-redigo](spring/spring-redigo/README.md) - redigo 封装。
  - [spring-rabbit](spring/spring-rabbit/README.md) - rabbitmq 封装。
- 启动器列表
  - [starter-echo](starter/starter-echo/README.md) - echo 启动器。
  - [starter-gin](starter/starter-gin/README.md) - gin 启动器。
  - [starter-go-redis](starter/starter-go-redis/README.md) - go-redis 启动器。
  - [starter-redigo](starter/starter-redigo/README.md) - redigo 启动器。
  - [starter-gorm](starter/starter-gorm/README.md) - gorm 启动器。
  - [starter-go-mongo](starter/starter-go-mongo/README.md) - go-mongo 启动器。
  - [starter-grpc](starter/starter-grpc/README.md) - grpc 启动器。
  - [starter-k8s](starter/starter-k8s/README.md) - k8s 启动器。
  - [starter-rabbit](starter/starter-rabbit/README.md) - rabbitmq 启动器。

### 路线图

- [x] 完善 Base 基础库的能力。
  - [x] 实现 assert 包常用的断言能力。
  - [x] 实现 atomic 包常用原子操作的封装。
  - [x] 实现 cast 包常用的类型转换。
  - [x] 实现上下文缓存组件包 knife。
  - [x] 实现进程内缓存组件包 cache。
  - [ ] 实现 jsonpath 操作 json 数据的能力。
- [x] 实现基本完善的 IoC 和 Boot 框架。
  - [x] 实现读取应用程序配置的组件。
  - [x] 实现 IoC 依赖注入框架。
  - [x] 实现 Boot 自动装配框架。
- [x] 实现 Log4J 风格的日志框架。
- [x] 实现 Web 框架以及对 Echo 和 Gin 的适配。
  - [x] 实现 Web 服务器以及中间件能力。
  - [x] 实现 Echo 适配以及开箱即用的能力。
  - [x] 实现 Gin 适配以及开箱即用的能力。
- [x] 实现 Redis 框架以及对 Redigo 和 Go-Redis 的适配。
  - [x] 实现 Redis 客户端以及中间件能力。
  - [x] 实现 Redigo 适配以及开箱即用的能力。
  - [x] 实现 Go-Redis 适配以及开箱即用的能力。
- [ ] 实现 MyBatis 风格的 SQL 框架。
- [ ] 实现 MQ 框架以及对 RabbitMQ 和 RocketMQ 的适配。
  - [ ] 实现 MQ 服务器和客户端能力。
  - [ ] 实现 RabbitMQ 适配以及开箱即用的能力。
  - [ ] 实现 RocketMQ 适配以及开箱即用的能力。
- [x] 实现 go-mongo 开箱即用的能力。
- [x] 实现 gorm 开箱即用的能力。
- [ ] 实现基本完善的流量录制和回放框架。
- [ ] 实现 Web 组件的流量录制和回放。
- [ ] 实现 Redis 组件的流量录制和回放。
- [ ] 实现 Gorm 组件的流量录制和回放。
- [ ] 实现 MQ 组件的流量录制和回放。
- [ ] 实现前后端分离的应用程序组织标准。

### 优秀教程

《Go-Spring 学习笔记》
- [Go-Spring 学习笔记一](https://blog.csdn.net/qq_33129963/article/details/121378573)
- [Go-Spring 学习笔记二](https://blog.csdn.net/qq_33129963/article/details/121387401)
- [Go-Spring 学习笔记三](https://blog.csdn.net/qq_33129963/article/details/121402740)
- [Go-Spring 学习笔记四](https://blog.csdn.net/qq_33129963/article/details/121521937)

《Go-Spring 入门篇》
- [Go-Spring 入门篇 一](https://learnku.com/articles/63101)
- [Go-Spring 入门篇 二](https://learnku.com/articles/63131)
- [Go-Spring 入门篇 三](https://learnku.com/articles/63133)
- [Go-Spring 入门篇 四](https://learnku.com/articles/63175)
- [Go-Spring 入门篇 五](https://learnku.com/articles/63332)
- [Go-Spring 入门篇 六](https://learnku.com/articles/63341)

《从零构建 Go-Spring 项目》
- [快速开始](https://github.com/dragons96/go-spring-demo)

### IoC 容器

Go-Spring 不仅实现了如 Java Spring 那般功能强大的 IoC 容器，还扩充了 Bean 的概念。在 Go 中，对象(即指针)、数组、Map、函数指针，这些都是 Bean，都可以放到 IoC 容器里。

| 常用的 Java Spring 注解				  | 对应的 Go-Spring 实现			|
| :-- 									| :-- 							|
| `@Value` 								| `value:"${}"` 				|
| `@Autowired` `@Qualifier` `@Required` | `autowire:"?"` 				|
| `@Configurable` 						| `WireBean()` 					|
| `@Profile` 							| `ConditionOnProfile()` 		|
| `@Primary` 							| `Primary()` 					|
| `@DependsOn` 							| `DependsOn()` 				|
| `@ConstructorBinding` 				| `RegisterBeanFn()` 			|
| `@ComponentScan` `@Indexed` 			| Package Import 				|
| `@Conditional` 						| `NewConditional()` 			|
| `@ConditionalOnExpression` 			| `NewExpressionCondition()` 	|
| `@ConditionalOnProperty` 				| `NewPropertyValueCondition()`	|
| `@ConditionalOnBean` 					| `NewBeanCondition()` 			|
| `@ConditionalOnMissingBean` 			| `NewMissingBeanCondition()`	|
| `@ConditionalOnClass` 				| Don't Need 					|
| `@ConditionalOnMissingClass` 			| Don't Need 					|
| `@Lookup` 							| —— 							|

### 属性绑定

Go-Spring 不仅支持对普通数据类型进行属性绑定，也支持对自定义值类型进行属性绑定，而且还支持对结构体属性的嵌套绑定。

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

上面这段代码可以通过下面的配置进行绑定：

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

### Boot 框架

Go-Spring 提供了一个功能强大的启动器框架，不仅实现了自动加载、开箱即用，而且可以非常容易的开发自己的启动器模块，使得代码不仅仅是库层面的复用。

### 快速示例

下面的示例使用 v1.1.0-rc2 版本测试通过。

```
import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func init() {
	gs.Object(new(Controller)).Init(func(c *Controller) {
		gs.GetMapping("/", c.Hello)
	})
}

type Controller struct {
	GOPATH string `value:"${GOPATH}"`
}

func (c *Controller) Hello(ctx web.Context) {
	ctx.String("%s - hello world!", c.GOPATH)
}

func main() {
	fmt.Println(gs.Run())
}

```

启动上面的程序，控制台输入 `curl http://localhost:8080/`， 可得到如下结果：

```
/Users/didi/go - hello world!
```

更多示例： https://github.com/go-spring/go-spring/tree/master/examples

### 普通路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/a/b/c", func(ctx web.Context) {
		ctx.String("OK")
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c
OK
```

### java 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/{*:d}", func(ctx web.Context) {
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("d"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a b=c *=d
a=a b=c *=d
```

### echo 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/*", func(ctx web.Context) {
		ctx.String("a=%s c=%s *=%s", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a c=c *=d
```

### gin 风格路由

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.GetMapping("/:a/b/:c/*d", func(ctx web.Context) {
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("*"))
		ctx.String("a=%s b=%s *=%s\n", ctx.PathParam("a"), ctx.PathParam("c"), ctx.PathParam("d"))
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl http://127.0.0.1:8080/a/b/c/d
a=a b=c *=d
a=a b=c *=d
```

### 文件服务器

```
package main

import (
	"fmt"
	"net/http"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

func main() {
	gs.HandleGet("/public/*", web.WrapH(http.StripPrefix("/public/", http.FileServer(http.Dir("public")))))
	fmt.Println(gs.Run())
}
```

然后在项目下创建一个 public 目录，里面创建一个内容为 hello world! 的 a.txt 文件。

```
➜ curl http://127.0.0.1:8080/public/a.txt
hello world!
```

### BIND 模式

```
package main

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
)

type HelloReq struct {
	Name string `form:"name"`
}

type HelloResp struct {
	Body string `json:"body"`
}

func main() {
	gs.GetBinding("/hello", func(ctx context.Context, req *HelloReq) *web.RpcResult {
		return web.SUCCESS.Data(&HelloResp{Body: "hello " + req.Name + "!"})
	})
	fmt.Println(gs.Run())
}
```

```
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100' 
{"code":200,"msg":"SUCCESS","data":{"body":"hello lvan100!"}}
```

### 中间件

#### Basic Auth

```
package main

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	"github.com/go-spring/spring-echo"
	_ "github.com/go-spring/starter-echo"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {

	gs.Provide(func( /* 可以通过配置将用户名密码传进来 */ ) web.Filter {
		m := middleware.BasicAuth(func(u string, p string, e echo.Context) (bool, error) {
			if u == "lvan100" && p == "123456" {
				return true, nil
			}
			return false, nil
		})
		return SpringEcho.Filter(m)
	})

	gs.GetMapping("/hello", func(ctx web.Context) {
		ctx.String("hello %s!", ctx.QueryParam("name"))
	})

	fmt.Println(gs.Run())
}
```

```
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100'
Unauthorized
➜ curl 'http://127.0.0.1:8080/hello?name=lvan100' -H'Authorization: Basic bHZhbjEwMDoxMjM0NTY='
{"code":200,"msg":"SUCCESS","data":{"body":"hello lvan100!"}}
```

### 详细文档

[https://docs.lavend.net/](https://docs.lavend.net/)

### 项目成员

#### 发起者

[@lvan100 (LiangHuan)](https://github.com/lvan100)

#### 贡献者

<a href="https://github.com/go-spring/go-spring/graphs/contributors"><img src="https://contributors-img.web.app/image?repo=go-spring/go-spring" /></a>

#### 特别鸣谢

[@shenqidebaozi](https://github.com/shenqidebaozi)

如何成为贡献者？提交有意义的 PR 或者需求，并被采纳。

### QQ 交流群

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/qq(1).jpeg" width="140" height="*" />
<br>QQ群号:721077608

### 微信公众号

<img src="https://raw.githubusercontent.com/go-spring/go-spring-website/master/go-spring-action.jpg" width="140" height="*" />

### 支持我们！

<img src="https://raw.githubusercontent.com/go-spring/go-spring/master/sponsor.png" width="140" height="*" />

为了更好地吸引和激励开发者，我们需要您的捐赠，帮助项目快速发展。

### 特别鸣谢！

感谢 JetBrains 公司的 IntelliJ IDEA 产品提供方便快捷的代码编辑和测试环境。

### License

The Go-Spring is released under version 2.0 of the Apache License.