# Go-Spring 实战第 9 课 —— IoC 容器：复杂 Go 应用如何统一对象装配

上一篇我们收束了 Go-Spring 的配置系统。但一个完整的应用还需要回答更多的问题，比如对象应该由谁创建，依赖关系应该怎样组织，初始化和销毁逻辑应该放在哪里，多个组件之间又应该如何协作。因此，接下来咱们详细介绍 Go-Spring 的 IoC 容器。

首先，咱们需要解释一下，为什么需要 IoC 容器？

在一个小型 Go 服务里，手写 `NewXxx()` 构造函数，然后逐层传参是最普遍的做法。也是大家认为最 Go 的做法。但是随着组件数量增加，且我们开始采用 DDD 的设计方式时，对象以及对象之间的依赖关系开始变得复杂。todo （这段和下段的衔接不好，不自然）

## 职责分离

先来看一个最小依赖关系。

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

在上面的代码中，业务对象 `UserController` 在构造时自己创建了下游依赖 `UserService`。看起来很简单，对吧。但它实际上违反了清晰编码所需遵循的基础原则之一，即“职责分离”。

我们鼓励在写代码的时候，将创建者和使用者的角色进行分离，这样可以有更好的可维护性和可测试性。当我们使用入参表达依赖关系时，我们可以在使用者之外更合理的创建这个依赖，它可能是一个真实的对象，也可能是一个 mock 对象。

看下面的代码。

```go
type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

在上面的代码中，我们将 `UserService` 的创建职责从 `UserController` 内部移动到了外部，通过入参表达这种依赖关系。那么我们就可以在测试里更方便的替换 `UserService` 的实现。

看下面的代码。

```go
func TestUserController_GetUser(t *testing.T) {
	repo := NewUserRepository(db)
	service := NewUserService(repo, cache)
	controller := NewUserController(service)
	//...
}
```

**这条编程原则是 Go-Spring 得以存在的基础**。如果我们否认这条原则，那么 Go-Spring 确实就没有存在的意义了。

## IoC 容器

在我们编码的过程中，很少有人会关注对象之间的依赖关系。传统的面向过程的编程方式也很难产出组件之间的依赖关系图。

但是当我们开始使用面向对象或者面向组件的编程方式时，我们期望能做的更好，这不仅仅是减少手工装配的工作量，也想着能自动分析和产出对象之间的依赖关系图。

看下面的例子，当应用里只有几个对象时，手工装配足够清楚。

```go
repo := NewUserRepository(db)
service := NewUserService(repo, cache)
controller := NewUserController(service)
server := NewHTTPServer(controller)
```

但是当对象数量增加时，手工装配就不再行了。

```go
todo 构造非常复杂的但是合理的手动依赖注入过程
```

当构造函数的入参越来越多，当需要共享的依赖越来越多，手工装配就很麻烦了。我们希望找到一种自动分析和装配依赖关系的手段。

这种手段就是 IoC 容器。IoC 容器不仅可以帮助我们自动分析和装配依赖关系，更重要的是，还可以帮助我们统一对象的生命周期管理，并且对项目施加更严格的项目管理规则，这一点在后面的文章中能逐渐感受到。

## 注册入口

下面这个例子把服务、控制器和 HTTP 路由放在同一条启动链路里。阅读时先看两个点：`UserController` 通过构造函数声明依赖，`gs.Provide` 把对象来源注册给 Go-Spring 容器。

```go
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

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
	gs.Provide(NewUserService)
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

这里的 `init()` 只是在注册候选定义，并没有立即创建完整对象图。真正启动时，`gs.Run()` 会驱动 Go-Spring 应用启动，容器再解析注册信息，并按依赖关系创建 `UserService`、`UserController` 和 `gs.HttpServeMux`。

这个例子也说明了 Go-Spring 的容器定位。业务对象仍然是普通 Go 类型，依赖仍然写在普通 Go 函数签名里。Go-Spring 负责在启动期把这些入口连接成可运行的应用结构。

## Bean

Go 里的包是代码组织单位，但包不等于运行期对象。一个包里可以有多个服务、多个客户端、多个 Runner，也可以只提供一组构造函数。

Bean 是 Go-Spring 用来描述运行期对象的单位。它可以是服务、控制器、路由器、客户端、配置对象，也可以是实现了 `Runner` 或 `Server` 的运行单元。

有了 Bean 这个层次以后，Go-Spring 就能在包边界之外表达对象关系。依赖注入、生命周期回调、条件启用和测试替换，都落在 Bean 的粒度上，而不是被迫混在包和函数的组织方式里。

这也是 `gs.Provide` 只注册候选定义，而不是直接执行所有创建动作的原因。Go-Spring 需要先拿到一组 Bean 定义，再结合配置、条件和 root 入口，判断哪些对象应该参与本次启动。

## 启动期装配

IoC 容器最重要的时间点是启动期。Go-Spring 会在应用启动时集中解析 Bean 定义、配置值和条件，尽量把对象图错误暴露在服务真正对外提供能力之前。

这类错误包括依赖缺失、同类型多候选无法选择、配置绑定失败、初始化回调返回错误等。它们如果拖到请求处理或后台任务运行时才暴露，排查成本会高很多。

启动期装配还有一个好处：生命周期顺序可以由同一张对象图推导。一个 Bean 依赖另一个 Bean，Go-Spring 就可以先准备被依赖对象，再创建依赖方；退出时则按相反方向释放资源。对于没有直接注入关系但需要顺序约束的对象，也可以通过 Bean 元信息补充。

也就是说，Go-Spring 的 IoC 容器不是运行期随处查找对象的入口，而是启动期建立应用结构的入口。业务代码不需要在处理请求时向容器要对象，业务代码应该通过构造函数参数、字段标签或生命周期回调拿到自己需要的依赖。

## 容器边界

IoC 容器能统一对象装配，但它不应该替代模块设计。一个对象依赖什么能力，仍然应该通过类型、接口和函数签名表达清楚。如果所有选择都推给配置，或者所有对象都依赖一个全局容器，代码反而会失去 Go 里最有价值的显式边界。

比较稳妥的做法是：核心链路依赖优先用构造函数或字段标签表达；多实现和环境差异通过名称、条件和配置选择表达；资源初始化和关闭进入生命周期；真正属于业务流程的分支，仍然留在业务代码里。

这样使用时，Go-Spring 不会把依赖藏起来，而是把依赖关系集中到启动期解析。开发者仍然能从类型签名、注册语句和 Bean 元信息里看见对象图怎样形成。

## IoC 容器定位

配置系统解决的是外部参数如何进入应用，IoC 容器解决的是应用内部对象如何组成一张可运行、可检查、可关闭的对象图。

Go-Spring 把 Bean 注册、依赖注入、条件启用和生命周期管理放到同一套启动期装配模型里。显式注册让对象来源可见，启动期解析让错误尽早暴露，生命周期管理让初始化和退出有统一顺序。

理解这个定位以后，Go-Spring 的 IoC 就不是“自动 new 对象”的工具，而是复杂 Go 应用里维护对象关系和运行边界的基础设施。
