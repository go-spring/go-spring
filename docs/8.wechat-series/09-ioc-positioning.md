# IoC 容器的工程定位

讲 IoC 容器之前，先把一个误会放到桌面上：Go-Spring 做 IoC，不是为了把 Java Spring 的那套东西搬进 Go。

Go 语言鼓励显式依赖和简单组合，这一点没有变。Go-Spring 需要容器，是因为大型 Go 应用里会遇到另一类问题：对象越来越多、依赖关系越来越复杂、启动和销毁顺序需要统一管理、测试环境需要替换依赖、不同环境还要启用不同实现。

IoC 容器提供的不是语法糖，而是应用级对象组织能力。

## 依赖注入解决什么

不使用依赖注入时，一个组件通常会自己创建依赖：

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

这会让 `UserController` 和 `UserService` 的具体创建方式耦合在一起。测试时要替换 `UserService`，或者运行时要切换实现，都会变得困难。

使用依赖注入后，组件只声明自己需要什么：

```go
type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

具体实现由外部传入，组件边界更清楚，也更容易测试。

## 为什么需要容器

当应用只有几个对象时，手动组装足够：

```go
service := NewUserService()
controller := NewUserController(service)
```

但当应用有几十上百个组件时，手动维护依赖顺序会迅速变得繁琐。IoC 容器接管这些工作：

- 收集应用注册的 Bean。
- 分析 Bean 之间的依赖关系。
- 按正确顺序创建对象。
- 将依赖注入到目标位置。
- 管理初始化和销毁生命周期。

Go-Spring 的重点不是让代码变得“更框架化”，而是让对象装配、生命周期和条件启用有一个统一模型。

## 快速示例

下面的例子展示了 Go-Spring 的基本使用方式：

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

`init()` 中注册 Bean，构造函数声明依赖，`gs.Run()` 启动应用。容器会完成对象创建、依赖注入和 HTTP Server 启动。

## Bean 是什么

Bean 是由容器管理的组件。它可以是服务、控制器、路由器、客户端、配置类，也可以是实现了 `Runner` 或 `Server` 的运行单元。

在 Go 中，包是代码组织单位，但包的粒度偏粗，一个包里可能包含多个运行期对象。Bean 则是运行期组织单位，用于精确描述依赖关系和生命周期。

这也是 Go-Spring 引入 Bean 的原因：它补充的是 Go 语言运行期对象管理能力，而不是替代包和函数。

## 容器不是魔法

IoC 容器的价值不在于隐藏依赖，而在于把应用对象、配置、生命周期和环境差异放到同一套装配模型里。显式注册、启动期解析和可测试性，是这套模型能落到 Go 工程里的关键。

下一步看两种最基础的注入方式：结构体字段注入和构造函数参数注入。
