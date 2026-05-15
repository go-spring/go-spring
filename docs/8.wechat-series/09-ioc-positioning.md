# Go-Spring 实战第 9 课：IoC 容器定位：复杂 Go 应用如何统一对象装配

配置系统解决的是参数从哪里来、怎样合并、怎样绑定。参数进入应用以后，紧接着就会落到另一个工程问题上——对象由谁创建，依赖由谁连接，启动和退出顺序又由谁保证。

小型 Go 服务里，手写 `NewXxx()` 再逐层传参通常很直接。但组件数量增加以后，问题会从“能不能 new 出对象”变成“能不能稳定维护一张对象关系图”。同一个服务里可能同时有路由、仓储、客户端、后台任务、可选插件和测试替身，它们还会受配置和环境影响。

Go-Spring 做 IoC 不是把 Java Spring 的模型搬进 Go。Go 语言鼓励显式依赖和简单组合，这个前提没有变。Go-Spring 的 IoC 容器补上的，是大型应用里对象装配、条件启用和生命周期管理的统一入口。

## 依赖注入解决的是创建职责归属

先看一个最小依赖关系。如果业务对象在构造时自己创建下游依赖，代码一开始会很短。

```go
type UserController struct {
	service *UserService
}

func NewUserController() *UserController {
	return &UserController{
		service: NewUserService(),
	}
}
```

这段代码的问题不在于写法复杂，而在于 `UserController` 同时承担了控制器职责和 `UserService` 的创建职责。一旦测试里要换成 mock，或者某个环境要切换实现，修改点就会落回控制器内部。

依赖注入把这条边界往外挪。业务对象只声明自己需要什么，创建和选择由外部完成。

```go
type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

这样处理以后，`UserController` 的代码只关心 `UserService` 的能力，不再关心具体实现从哪里来。Go-Spring 的容器能力也是从这个边界出发的，只是它把“外部完成装配”进一步做成了启动期模型。

## 对象数量增加后，手工装配会丢掉统一语义

当应用里只有几个对象时，手工装配足够清楚。

```go
service := NewUserService()
controller := NewUserController(service)
```

但真实服务不会只停留在这两行。对象数量增长以后，手工装配还要同时处理条件启用、初始化回调、退出清理、测试替换和多环境实现选择。也就是说，代码要维护的不只是创建顺序，还有一整套运行语义。

Go-Spring 的 IoC 容器接管的是这些启动期工作。

- 收集应用注册的 Bean。
- 分析 Bean 之间的依赖关系。
- 按正确顺序创建对象。
- 将依赖注入到目标位置。
- 管理初始化和销毁生命周期。

因此，Go-Spring 的重点不是让业务代码看起来更“框架化”。它要统一的是对象装配、生命周期和条件启用这些原本分散在各处的规则。

## 最小示例把注册、构造依赖和运行入口连起来

下面的例子把控制器、服务和 HTTP 路由放在同一条启动链路里。阅读时先看两个点，`UserController` 通过构造函数声明依赖，`gs.Provide` 把创建入口注册给 Go-Spring 容器。

```go
type UserService struct{}

func (s *UserService) GetUser() string {
	return "Alice"
}

type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", c.service.GetUser())
}

func init() {
	gs.Provide(new(UserService))
	gs.Provide(NewUserController)

	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
```

这里的 `init()` 只是在注册候选 Bean，并没有立即创建完整对象图。真正启动时，`gs.Run()` 会驱动 Go-Spring 容器解析注册信息，再按依赖关系创建 `UserService`、`UserController` 和 `gs.HttpServeMux`。

这个例子也说明了容器定位。业务对象仍然通过普通 Go 类型和函数表达，Go-Spring 负责在启动期把这些入口连接成可运行的应用结构。

## Bean 让运行期对象脱离包边界表达

Go 里的包是代码组织单位，但包不等于运行期对象。一个包里可以有多个服务、多个客户端、多个 Runner，也可以只提供一组构造函数。

Bean 则是 Go-Spring 用来描述运行期对象的单位。它可以是服务、控制器、路由器、客户端、配置类，也可以是实现了 `Runner` 或 `Server` 的运行单元。

有了 Bean 这个层次以后，Go-Spring 就能在包边界之外表达对象关系。依赖注入、生命周期回调、条件启用和测试替换，都落在 Bean 的粒度上，而不是被迫混在包和函数的组织方式里。

## IoC 容器统一的是启动期装配模型

Go-Spring IoC 容器的价值不在于隐藏依赖，而在于把应用对象、配置、生命周期和环境差异放到同一套启动期装配模型里。显式注册让依赖入口可见，启动期解析让错误尽早暴露，条件和生命周期则让对象图能随环境稳定变化。

理解容器定位以后，下一个问题就落到代码写法上。既然依赖要交给容器连接，那么依赖声明应该放在结构体字段上，还是放进构造函数参数里。
