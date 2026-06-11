# IoC 容器

> 控制反转（Inversion of Control，IoC）和依赖注入（Dependency Injection，DI）是 Java Spring 框架的核心基础。
> Go-Spring 继承了 Java Spring 的设计理念，同时保持 Go 语言的原生风格，为 Go 开发者提供了一个简洁高效的依赖注入框架。

## 什么是依赖注入？

> 如果你熟悉 Java Spring，可以直接跳过这一节。

**依赖注入**是一种设计模式，它能帮助我们写出更优雅、更易维护的代码：

1. **解耦组件依赖**：组件不需要自己创建依赖对象，而是由容器统一提供
2. **集中生命周期管理**：对象的创建、初始化、销毁都由容器统一管理
3. **更方便单元测试**：测试时可以轻松将依赖替换为 Mock 对象
4. **更好的可维护性**：依赖关系清晰可见，集中管理而非分散在代码各处

我们通过代码直观感受一下其中的区别。不使用依赖注入时，我们通常在构造函数中自己创建依赖：

```go
type UserController struct {
	service *UserService
}

// ❌ 不使用 DI：UserController 必须自己创建 UserService，这就是紧耦合
func NewUserController() *UserController {
	return &UserController{
		// UserService 被硬编码写死在这里，无法在创建时灵活替换
		service: NewUserService(),
	}
}
```

使用依赖注入后，我们只需要在构造函数中**声明**需要什么依赖，不需要自己创建：

```go
type UserController struct {
	service UserService
}

// ✅ 使用 DI：UserService 由外部传入，其实现可以灵活替换，这就是松耦合
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}
```

### 为什么需要 IoC 容器？

上面的方式已经实现了解耦，但还有一个问题需要解决：**谁来负责创建这些依赖并传入呢？**

当应用只有两三个对象时，我们可以手动组装：

```go
service := NewUserService()
controller := NewUserController(service)
```

但是当应用有几十上百个组件，并且它们之间存在复杂的依赖关系时，继续逐一手动组装会变得非常繁琐。
**这时候 IoC 容器就派上用场了** —— 它可以帮助我们自动完成以下事情：

1. 我们将所有组件注册到容器中（这一步叫做**注册 Bean**）
2. 容器自动分析组件之间的依赖关系
3. 容器按照正确的顺序创建所有对象
4. 容器自动把依赖注入到需要的地方
5. 容器全程管理组件从创建到销毁的整个生命周期

一句话概括：所有组件的组装和管理工作，都可以交给 IoC 容器自动完成，而让我们更专注于编写业务逻辑。
这，**就是 Go-Spring 的使命**！

## 快速开始

让我们通过一个简洁的示例快速了解 Go-Spring 的用法：

```go
package main

import (
	"fmt"
	"net/http"

	// 引入 Go-Spring 核心包
	"github.com/go-spring/spring-core/gs"
)

// UserService 业务服务，整个应用只需要创建一个实例
type UserService struct{}

// GetUser 获取用户名
func (s *UserService) GetUser() string {
	return "Alice"
}

// UserController HTTP 控制器，依赖 UserService
type UserController struct {
	service *UserService
}

// NewUserController 构造函数，参数即依赖声明
// 可以由容器自动分析参数类型并注入匹配的 Bean
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

// Hello 处理 /hello 请求
func (c *UserController) Hello(w http.ResponseWriter, r *http.Request) {
	user := c.service.GetUser()
	fmt.Fprintf(w, "Hello, %s!", user)
	fmt.Println("Hello,", user)
}

// init 在程序启动前执行，向容器注册所有的 Bean
func init() {
	// 注册 UserService，可被其他 Bean 依赖注入
	gs.Provide(new(UserService))

	// 注册 UserController，容器自动解析构造函数参数并注入依赖
	gs.Provide(NewUserController)

	// 注册 HTTP 路由配置，支持函数式 Bean 定义
	// 返回的 HttpServeMux 会被容器识别并自动启动 HTTP 服务
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/hello", c.Hello)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	// 启动 Go-Spring 应用
	// 自动完成所有 Bean 的创建和依赖注入，启动 HTTP 服务
	gs.Run()
}
```

使用 `go run main.go` 运行应用，然后访问 `http://localhost:9090/hello`，你会看到：

```
Hello, Alice!
```

同时控制台也会输出：

```
Hello, Alice
```

可以看到整个过程非常简洁：只需要在 `init()` 中注册服务、控制器和路由，通过构造函数声明依赖，
然后调用 `gs.Run()` 启动应用，依赖注入、对象管理和 HTTP 服务启动就都由 Go-Spring 自动完成。

## Bean 定义

**Bean** 是一种由容器统一管理的组件，其创建、初始化及销毁等生命周期过程均由容器负责。
这一概念源自 Java Spring，但在 Go-Spring 中被重新诠释，以更贴合 Go 的设计特点。

从依赖注入的角度看，Bean 可以理解为**一个可被容器管理和组合的最小功能单元**。
这一理解并非简单迁移，而是对 Go 代码组织方式的自然延伸。

在 Go 中，包主要用于代码组织与命名空间划分，粒度相对较粗，
一个包内往往包含多个在运行时具有不同依赖关系的功能单元，
而语言本身并未提供对这些单元的统一管理机制。

因此，Go-Spring 将 Bean 引入为运行时的组织单元，对包形成补充：
它将更细粒度的功能抽象为可管理的对象，用于更方便、准确地描述依赖关系与生命周期。

## Bean 注入

Go-Spring 的依赖注入体系可以从两个维度来理解：
一是注入方式，即通过不同的语法形式来声明依赖关系；
二是注入目标，即依赖最终以何种形态被接收和使用。

### 注入方式

Go-Spring 支持**两种注入方式**：结构体字段注入和构造函数参数注入。

#### 结构体字段注入

结构体字段注入是最简单、最直观的注入方式：
只需要在结构体字段上通过 `autowire`（或者 `inject`）标签声明依赖，
容器就会自动把匹配到的 Bean 注入到该字段中。

示例：

```go
// UserController 依赖 UserService
type UserController struct {
	Service UserService `autowire:""`
}
```

这种方式简洁明了，不需要手动编写构造函数，适用于依赖关系较简单的业务场景。

#### 构造函数参数注入

构造函数参数注入是指依赖通过构造函数的参数传入，
容器在创建 Bean 时自动解析这些参数并提供对应的依赖。

**示例：**

```go
// UserController 依赖 UserService
type UserController struct {
	service UserService
}

// 依赖通过构造函数参数传入
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// 注册构造函数，容器自动分析参数并注入对应的依赖
	gs.Provide(NewUserController)
}
```

Go-Spring 使用标准的 Go 构造函数进行依赖注入，
无需额外的框架级约定或者特殊的语法标记。

#### 选择哪种注入方式

从解耦的角度看，大多数场景下更推荐使用构造函数注入，
因为它能够在对象创建阶段就明确地声明依赖关系，使组件的边界更加清晰，
也更有利于保证依赖的不可变性与可测试性。

对于依赖关系比较简单或者以便捷性为优先的场景，也可以使用结构体字段注入，
在表达清晰性与开发效率之间取得平衡，而不必过度限制其使用。

### 注入目标

无论是结构体字段注入还是构造函数参数注入，Go-Spring 都支持将 Bean 注入到多种类型的目标中。  
最常见的方式是注入单个 Bean，即根据类型或名称匹配到唯一的 Bean 实例进行注入。  
同时，也支持注入多个 Bean，这种情况下，容器会自动将所有符合条件的 Bean 收集并注入到切片或 Map 中。

#### 注入单个 Bean

这是最基础、也是最常见的使用方式，即注入**唯一**符合条件的 Bean。
绝大多数业务依赖都属于这种场景。

**按类型注入**

我们可以在结构体字段上添加 `autowire` 或者 `inject` 标签，
容器就会自动把按照类型匹配到的 Bean 注入到该字段中。

```go
type Service struct {
	// 按类型自动匹配，注入唯一的 UserRepository 接口
	Repo UserRepository `autowire:""`
}
```

我们也可以在注册构造函数 Bean 时，通过 `TagArg` 来显式指定注入所使用的 Bean。
容器会根据 `TagArg` 的内容进行匹配，并将对应的 Bean 注入到构造函数参数中。

```go
// UserController 需要注入一个 UserService
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// 按类型自动匹配，此处可以省略 TagArg 的内容
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

**按名称注入**

当候选 Bean 只有一个时，我们可以直接按照类型进行注入，无需指定名称。
但当候选 Bean 有多个时，就需要通过名称进行区分，以便明确指定具体的注入目标。

```go
func init() {
	// 注册 master 和 slave 两个命名 Bean
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	// 在结构体字段中注入名为 "slave" 的 Bean
	ds DataSource `autowire:"slave"`
}
```

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	// 在构造函数参数中注入名称为 "slave" 的 Bean
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

**可空注入**

默认情况下，容器在找不到匹配的 Bean 时，会报错并终止启动。
如果希望在没有找到 Bean 时不抛出错误，而是注入对应的零值，我们可以使用 `?` 标记，表示将该依赖声明为可空。

```go
type Service struct {
	// 可空注入，找不到匹配 Bean 时不报错，保持零值
	OptionalDep Dep `autowire:"?"`

	// 同时指定名称 + 可空选项
	NamedOptional Dep `autowire:"my-name?"`
}
```

```go
// UserController 需要注入一个 UserService
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// 构造函数参数使用可空注入，当 Bean 不存在时注入零值
	gs.Provide(NewUserController, gs.TagArg("?"))

	// 同时指定名称 + 可空选项
	gs.Provide(NewUserController, gs.TagArg("my-name?"))
}
```

#### 注入多个 Bean

当需要获取**多个符合条件的 Bean** 时，
我们可以将依赖声明为 `[]T`（切片）或者 `map[string]T`（Map）。
这时，容器会自动将所有符合条件的同类型 Bean 收集并注入到对应的集合中。

##### 切片收集 `[]T`

我们可以在结构体字段中使用切片来收集多个 Bean，
也可以在构造函数参数中通过切片来收集多个 Bean。

```go
type Application struct {
	// 收集所有的 Plugin 实现到切片
	plugins []Plugin `autowire:""`
}

func init() {
	// 将多个实现注册为 Plugin 类型的 Bean
	gs.Provide(NewPluginA).Export(gs.As[Plugin]())
	gs.Provide(NewPluginB).Export(gs.As[Plugin]())
	gs.Provide(NewPluginC).Export(gs.As[Plugin]())
}
```

```go
func NewApplication(plugins []Plugin) *Application {
	return &Application{plugins: plugins}
}

func init() {
	// 注入所有 Plugin 到切片，这里省略了 TagArg 参数
	gs.Provide(NewApplication)
}
```

在未指定 tag 内容时，容器会按照 **Bean 名称的字典序** 对切片中的元素进行排序，
确保每次收集到的结果都是确定的，从而保证行为也是一致的。

如果需要**精确控制顺序**，我们可以在 tag 中通过 Bean 名称列表来显式指定排序规则。

```go
type Chain struct {
	// 顺序: auth(可空) -> tracing -> recovery
	Filters []Filter `autowire:"auth?,tracing,recovery"`
}
```

```go
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}

func init() {
	// 顺序: auth(可空) -> tracing -> recovery
	gs.Provide(NewChain, gs.TagArg("auth?,tracing,recovery"))
}
```

我们可以对列表中的每个 Bean 使用可空语法 `name?`，表示当对应 Bean 不存在时自动跳过。

此外，我们还可以使用通配符 `*` 来表示包含**所有未显式列出**的剩余 Bean。
但需要注意，通配符 `*` 在同一个表达式中只能出现一次。

当使用通配符 `*` 时，Bean 的收集顺序如下：

1. **`*` 之前的显式 Bean** 按 tag 中声明的顺序排列；
2. **`*` 匹配的剩余 Bean** 按名称字典序排列；
3. **`*` 之后的显式 Bean** 按 tag 中声明的顺序排列。

例如，`autowire:"a,*,c?,b"` 表示：
先收集 `a`，然后收集除 `c`、`b` 之外的其他所有 Bean（按名称排序），
接着是 `c`（若存在），最后是 `b`。其中 `c` 可以为空，不存在时会自动跳过。

##### Map 收集 `map[string]T`

除了使用切片收集多个 Bean，我们也可以使用 `map[string]T` 来进行收集。
两者的 tag 语法基本一致，但 `map[string]T` 的结果不保证顺序（因为 map 本身是无序的）。

在这种形式下，`map[string]T` 的 key 表示 Bean 的名称，value 则是对应的 Bean 实例。

```go
type Router struct {
	// name -> Handler 映射，key 就是 Bean 名称
	Handlers map[string]Handler `autowire:""`
}
```

```go
// 注入所有 Handler 到 Map
func NewRouter(handlers map[string]Handler) *Router {
	return &Router{handlers: handlers}
}

func init() {
	gs.Provide(NewRouter)
}
```

对于 Map 收集我们也可以使用可空语法 `name?`，表示当对应的 Bean 不存在时自动跳过。
不过，通配符 `*` 虽然语法上可用，但在 Map 场景下意义相对有限，因为 Map 本身不保证顺序，
即使使用了 `*` 也无法体现收集顺序上的控制。

```go
type Service struct {
	// 只包含指定名称的处理器
	Handlers map[string]Handler `autowire:"auth,user?,order"`
}
```

```go
func NewService(handlers map[string]Handler) *Service {
	return &Service{Handlers: handlers}
}

func init() {
	gs.Provide(NewService, gs.TagArg("auth,user?,order"))
}
```

#### 通过配置项注入

通常情况下，我们在注入时就可以确定使用的 Bean 名称；
但在某些场景中，也可能需要根据配置项动态决定具体注入哪个（些） Bean。
Go-Spring 支持这种用法，只需要将 Bean 名称写为 `${...}` 形式的配置项表达式，
容器会在运行时从配置中解析出对应的 Bean 名称（列表）并完成注入。

这种方式适用于各种注入形式：
无论是结构体字段注入还是构造函数参数注入，
无论是单个 Bean 注入还是切片、Map 等集合收集，
都可以通过 `${...}` 表达式动态指定 Bean。

```go
type Service struct {
	// 从配置项 "storage.provider" 读取 Bean 名称
	// 这样就可以通过配置动态切换实现，而且无需修改代码
	Storage Storage `autowire:"${storage.provider}"`
}
```

```go
func NewService(storage Storage) *MyService {
	return &MyService{storage: storage}
}

func init() {
	// 从配置项 "storage.provider" 读取 Bean 名称
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

```go
type Chain struct {
	// 从配置项 "http.filters" 读取过滤器列表
	// 这样就可以通过配置动态切换实现，而且无需修改代码
	Filters []Filter `autowire:"${http.filters}"`
}
```

```go
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}

func init() {
	// 从配置项 "http.filters" 读取过滤器列表
	gs.Provide(NewChain, gs.TagArg("${http.filters}"))
}
```

`${...}` 语法本身也支持指定默认值；由于在 Bean 名称选择中较少使用，这里不展开介绍。

### 延迟注入

延迟注入主要用于解决某些场景下的循环依赖问题，并且仅适用于结构体字段注入。
其用法是在 tag 中添加 `,lazy` 标记。

```go
type Service struct {
	// 强制这个字段延迟注入，等所有非延迟注入完成后再处理
	Dep Dependency `autowire:",lazy"`
}
```

需要注意的是，标记为 `lazy` 的字段会在所有非延迟注入完成之后再统一处理。
由于这是一个独立阶段，因此在前一阶段的注入过程中，这些字段始终保持为空，请记住不要使用它们。

## Bean 类型

Go-Spring 在注册 Bean 时，支持三种形式的参数：

* **结构体指针**：将预先创建好的对象直接交由容器管理，使用方式最简单
* **构造函数**：由容器在启动时调用构造函数创建 Bean，更推荐这种方式
* **函数指针**：将函数本身作为 Bean 注册，支持函数式风格的使用场景

### 结构体指针

这是最简单的注册方式，直接传入一个已经创建好的结构体指针即可。
该对象既可以是临时创建的实例，也可以是全局创建并在其他地方复用的对象。

```go
// MyService 是你的业务结构体
type MyService struct {
	// ...
}

func init() {
	// 直接 new 出对象指针，注册到容器
	gs.Provide(new(MyService))
}
```

当我们将一个全局创建并被复用的对象注册到容器中时，容器仍然会对其进行管理，
包括在适当的生命周期阶段调用初始化和销毁方法。
这种能力在 Go-Spring 与其他框架或既有系统进行集成时尤为有用。

### 构造函数

通过构造函数创建 Bean 是 Go-Spring 推荐的方式。
在这种模式下，所有依赖都通过构造函数参数传入，由容器负责解析并提供。

```go
type MyService struct {
	dep Dep
}

// 构造函数参数接收依赖，返回实例
func NewMyService(dep Dep) *MyService {
	return &MyService{dep: dep}
}

func init() {
	gs.Provide(NewMyService)
}
```

上面的构造函数形式是 `func(...) T`，它直接返回 Bean，适用于创建过程确定不会失败的场景。

但在创建复杂对象时，初始化过程可能会失败，例如配置校验不通过、文件打开失败或数据库连接失败等情况。
此时我们可以通过构造函数返回 `error` 来表达失败，即使用 `func(...) (T, error)` 的形式。
容器会自动识别该模式，并在创建失败时终止启动。

```go
// 构造函数返回 error，用于表示 Bean 创建是否成功
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}

func init() {
	gs.Provide(NewMyService)
}
```

#### 参数绑定

在前面的示例中，我们已经展示了 `TagArg` 的一些用法，本节将进一步解释其背后的实现原理。

由于 Go 语言本身只支持在结构体字段上使用 tag，而无法直接在构造函数参数上声明 tag，
因此 Go-Spring 采用了一种替代方案：
在注册阶段通过 `Arg` 参数，为需要特殊处理的构造函数参数显式提供绑定信息。

这种在注册阶段将构造函数参数与具体依赖建立映射关系的机制，被称为**参数绑定（Arg binding）**。

Go-Spring 提供了多种 `Arg` 实现，用于不同类型的参数绑定场景：

* `TagArg`：用于绑定 Bean 依赖或配置属性
* `ValueArg`：用于绑定固定值
* `BindArg`：用于绑定 Option 模式构造函数参数
* `IndexArg`：用于按参数索引位置进行绑定

##### 注入 Bean

当一个构造函数 Bean 依赖其他 Bean 时，可以通过 `gs.TagArg` 来显式声明依赖关系。

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// TagArg 的参数为空字符串表示仅按类型匹配，不需要名称限定
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

`TagArg` 的字符串参数与结构体字段中的 `autowire` tag 在语义上是等价的。

另外，在上面的示例中，因为只有一个参数，而且按类型匹配就行，不需要限定名称，
所以我们可以省略 `TagArg` 参数。

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// 按类型自动匹配，此时可以省略 TagArg
	gs.Provide(NewUserController)
}
```

因为在前面章节的示例中已经展示了较多 `TagArg` 的用法，
例如按名称注入 Bean、通过配置项动态指定 Bean 名称，以及在切片和 Map 中进行收集等，
因此这里不再重复展开。相关细节可以参考 [**Bean 注入**](#bean-注入)章节。

##### 注入配置项

`TagArg` 不仅可以用于绑定 Bean 依赖注入，也可以用于绑定配置属性。
Go-Spring 支持从配置系统读取配置项的值，并自动转换为对应类型后注入到构造函数参数中。

```go
type RedisClient struct {
	host string
	port int
}

// 构造函数参数直接注入配置值
func NewRedisClient(host string, port int) *RedisClient {
	return &RedisClient{host: host, port: port}
}

func init() {
	// 使用 TagArg 指定配置项路径
	gs.Provide(NewRedisClient,
		gs.TagArg("${redis.host:=localhost}"), // 从配置项 "redis.host" 读取主机地址
		gs.TagArg("${redis.port:=6379}"), // 从配置项 "redis.port" 读取端口
	)
}
```

我们可以通过如下 YAML 配置来完成 `RedisClient` 的注入：

```yaml
redis:
  host: localhost
  port: 6379
```

我们也可以让容器将整个配置对象直接绑定到一个结构体参数中。

```go
// RedisConfig 定义 Redis 配置结构
type RedisConfig struct {
	Host     string        `value:"${host:=localhost}"`
	Port     int           `value:"${port:=6379}"`
	Password string        `value:"${password:=}"`
	DB       int           `value:"${db:=0}"`
	Timeout  time.Duration `value:"${timeout:=5s}"`
}

// 直接注入绑定好的 RedisConfig 对象
func NewRedisClient(cfg RedisConfig) *RedisClient {
	return &RedisClient{
		host: cfg.Host,
		port: cfg.Port,
		// ...
	}
}

func init() {
	// 前缀 "redis." 会自动添加到所有字段
	// 所以 cfg.Host 对应配置键 "redis.host"
	gs.Provide(NewRedisClient, gs.TagArg("redis"))
}
```

这种方式可以将一组相关配置项集中起来统一管理，比分散注入多个参数更清晰整洁。

`TagArg` 还支持更丰富的配置项绑定能力，例如 Map 和列表注入、配置引用、类型转换器以及嵌套配置等。
更多细节可以参考 [01-configuration.md](01-configuration.md)。

##### 注入固定值

如果参数的值在注册 Bean 时已经确定，既不需要从容器获取，也不需要从配置系统读取，
那么我们可以使用 `ValueArg` 来绑定一个固定值。

```go
type RedisClient struct {
	db int
}

func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	// db 参数绑定固定值 0
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

##### Option 绑定

**Functional Options** 是 Go 语言中非常流行的一种编程模式，
主要用于处理构造函数中的**可选参数**问题。

例如，一个服务可能包含多个配置项，但其中大部分都有合理的默认值，调用者通常只需要修改少数几个配置。
在这种情况下，Option 模式相比于定义多个构造函数或传入一长串零值参数，会更加清晰和优雅。

下面来看一个典型的 Option 模式定义：

```go
// Option 定义用于修改 Server 配置的函数类型
type Option func(*Server)

// WithPort 返回一个用于设置端口的 Option
func WithPort(port int) Option {
	return func(s *Server) {
		s.port = port
	}
}

// WithTimeout 返回一个用于设置超时的 Option
func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.timeout = timeout
	}
}

type Server struct {
	port    int
	timeout time.Duration
}

// NewServer 接受可变数量的 Option，用于配置 Server
func NewServer(opts ...Option) *Server {
	s := &Server{
		port:    8080,             // 默认端口
		timeout: 30 * time.Second, // 默认超时
	}

	// 依次应用所有 Option
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

上面的代码展示了标准的 Functional Options 模式写法：

1. 定义 `Option` 函数类型，用于接收 `*Server` 并修改其配置
2. 通过 `WithXXX` 函数创建具体的 Option，并设置对应配置项
3. `NewServer` 接受可变参数 `opts ...Option`，在创建实例后依次应用这些 Option

那么问题来了：**如果 Option 本身的创建依赖配置或其他 Bean，该如何处理？**
例如 `WithPort` 需要的端口来自配置文件，而不是在代码中写死。

在这种场景下，Go-Spring 的 `BindArg` 可以很好地解决这个问题：
它允许将每个 Option 的创建过程交由容器管理，在创建 Option 时自动注入所需依赖或配置，
最终再将生成好的 Option 传入 `NewServer` 构造函数中。

```go
func init() {
	// 使用 BindArg 为每个 Option 提供参数绑定
	// WithPort 需要从配置读取端口，用 TagArg 绑定
	// WithTimeout 使用固定超时 60 秒，用 ValueArg 绑定
	gs.Provide(NewServer,
		gs.BindArg(WithPort, gs.TagArg("${server.port:=8080}")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

此外，`BindArg` 还支持条件控制：
当条件满足时，对应的 Option 才会被创建并传入构造函数；
如果条件不成立，则该位置会被直接跳过，不会传入任何参数。

```go
func init() {
	gs.Provide(
		NewServer,
		// 只有配置了 server.port 才添加这个 Option
		gs.BindArg(WithPort, gs.TagArg("${server.port}")).
			Condition(gs.OnProperty("server.port")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

在上面的例子中，只有当配置系统中显式设置了 `server.port` 时，
`WithPort` 这个 Option 才会被创建并注入到构造函数中；
如果未配置，则会跳过该 Option，直接使用 `NewServer` 中定义的默认值。

这一机制在实际应用中非常有用，可以根据配置条件选择性地启用或关闭某些功能，实现更加灵活的装配方式。

#### 参数顺序

默认情况下，`gs.Provide()` 中的参数绑定是**按照顺序匹配**构造函数参数的：
第一个绑定对应第一个参数，第二个绑定对应第二个参数，以此类推。

但在构造函数参数较多、仅需对少量参数进行显式绑定，其余参数可以交由容器按类型自动推断的场景下，
我们可以使用 `IndexArg` 来显式指定绑定位置，而不必按照参数顺序逐一传入。

```go
// 构造函数有三个参数：a, b, c
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	// 仅为第三个参数（index=2，从 0 开始计数）绑定固定值
	// a 和 b 将由容器按类型自动推断并注入
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

通过这种方式，在不改变构造函数参数顺序的前提下，我们既可以精确绑定关键参数，
又可以将其余参数交由容器自动推断，从而使整体配置更加灵活、也更易于维护。

### 函数指针

除了结构体指针和构造函数，Go-Spring 还支持直接将**函数指针注册为 Bean**，
适用于函数本身作为可注入组件的场景。

由于在 Go 的类型系统中，函数类型无法与构造函数明确区分，
而容器又需要显式识别该函数是否作为 Bean 本身进行注册，
因此在注册 Bean 时需要使用 `reflect.ValueOf` 对函数进行封装。

```go
import "reflect"

// PasswordChecker 定义密码校验函数的类型
type PasswordChecker func(username, password string) bool

// Authenticator 需要注入密码校验函数
type Authenticator struct {
	checker PasswordChecker `autowire:""`
}

func NewAuthenticator(checker PasswordChecker) *Authenticator {
	return &Authenticator{checker: checker}
}

// 提供一个具体的密码校验函数
func BcryptPasswordChecker(username, password string) bool {
	// ... 具体的校验逻辑
	return true
}

func init() {
	// 使用 reflect.ValueOf() 封装函数指针，直接注册为 Bean
	// 这样任何需要 PasswordChecker 的组件都能注入这个函数
	gs.Provide(reflect.ValueOf(BcryptPasswordChecker))

	// 注册 Authenticator，它会自动注入上面的密码校验函数
	gs.Provide(NewAuthenticator)
}
```

## Bean 配置

在注册 Bean 的时候，我们通常还需要进行一些额外配置，
例如自定义名称、指定初始化方法、导出接口、附加条件或声明依赖等。

Go-Spring 提供了链式调用的方式，使我们能够更加方便地完成这些 Bean 配置。

### 设置 Bean 名称

为了唯一标识容器中的每个 Bean，Go-Spring 使用**类型 + 名称**的组合来生成唯一标识符。

如果在注册 Bean 的时候没有显式指定名称，Go-Spring 会自动使用类型的简短名称作为默认名称。例如：

* 对于结构体指针 `*UserService`，默认名称为 `"UserService"`
* 对于接口 `UserService`，默认名称为 `"UserService"`

当同一个类型需要注册多个不同实例时（例如主库与从库两个数据源），
我们可以通过 `.Name()` 方法为每个 Bean 显式指定名称，以示区分。

```go
func init() {
	// 同一个 DataSource 类型，注册两个不同名称的 Bean
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

在设置了名称之后，我们就可以通过该名称来明确指定所使用的实例了。

```go
type UserRepo struct {
	// 结构体字段注入在 autowire tag 中指定 Bean 名称
	ds *DataSource `autowire:"slave"`
}

func NewUserRepo(ds *DataSource) *UserRepo {
	return &UserRepo{ds: ds}
}

func init() {
	// 构造函数参数注入使用 TagArg 指定 Bean 名称
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}
```

### 设置生命周期回调

有时候，在 Bean 创建完成并且完成所有依赖注入之后，我们还需要执行一些自定义的初始化逻辑；
同样，在容器退出的时候，也需要对资源进行优雅释放。

为了解决这类问题，Go-Spring 提供了**生命周期回调机制**。

其中，**初始化回调**发生在 Bean 完成创建并且依赖注入结束之后，用于执行自定义初始化逻辑，
例如建立数据库连接、加载缓存数据到内存或校验配置的正确性等。

而**销毁回调**则发生在容器退出阶段，用于资源的优雅释放，
例如关闭数据库连接、持久化内存状态或停止后台任务等。

Go-Spring 支持两种配置生命周期回调的方式：通过函数指针显式指定，或通过方法名进行声明。

#### 方式一：通过函数指针设置

我们可以直接传入一个独立的函数指针作为生命周期回调，该函数需要接收 Bean 实例作为参数。
它可以没有返回值，也可以仅返回 `error`。

`init` 和 `destroy` 的函数签名规则是完全一致的。

```go
type MyService struct {
	client *redis.Client
}

func NewMyService() *MyService {
	return &MyService{}
}

// 初始化函数，接收 bean 作为参数
func InitMyService(s *MyService) error {
	s.client = redis.NewClient( /* ... */ )
	// 测试连接
	if err := s.client.Ping().Err(); err != nil {
		return err // 初始化失败，容器终止启动
	}
	return nil
}

// 销毁函数，接收 bean 作为参数
func DestroyMyService(s *MyService) error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		Init(InitMyService).      // 设置初始化函数
		Destroy(DestroyMyService) // 设置销毁函数
}
```

如果 `init` 回调返回错误，容器会终止启动，表示初始化失败。
如果 `destroy` 回调返回错误，容器会记录该错误日志，但不会阻塞容器退出。

#### 方式二：通过方法名指定

如果结构体本身已经定义了初始化方法和销毁方法，且函数签名符合要求，
那么我们可以通过指定方法名的方式来配置生命周期回调。

```go
type MyService struct {
	client *redis.Client
}

func NewMyService() *MyService {
	return &MyService{}
}

// Init 初始化方法，在依赖注入完成后调用
func (s *MyService) Init() error {
	s.client = redis.NewClient( /* ... */ )

	// 测试连接
	if err := s.client.Ping().Err(); err != nil {
		return err // 初始化失败，容器终止启动
	}
	return nil
}

// Destroy 销毁方法，在容器退出时调用
func (s *MyService) Destroy() error {
	return s.client.Close()
}

func init() {
	gs.Provide(NewMyService).
		InitMethod("Init").      // 设置初始化方法
		DestroyMethod("Destroy") // 设置销毁方法
}
```

### 导出为接口

在 Go 程序中，我们经常会使用接口。那么在 Go-Spring 中，如何注册并使用接口类型呢？

Go-Spring 的做法是：在注册 Bean 时，需要显式指定该 Bean 要导出的接口类型。
这样做是因为在 Go 中，一个结构体可能实现多个接口，甚至可能在无意中实现了某些接口。
所以为了避免自动推导带来的不确定性，需要明确声明对外暴露的接口类型。

```go
// 定义接口
type UserService interface {
	Get(id int) (*User, error)
}

// 具体实现
type UserServiceImpl struct {
	// ...
}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func (s *UserServiceImpl) Get(id int) (*User, error) {
	return &User{ID: id}, nil
}

func init() {
	// 将 UserServiceImpl 导出为 UserService 接口，供依赖方按接口注入
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

在上面的示例中，容器中会同时存在两个 Bean：一个是原始类型的 Bean，另一个是以接口形式导出的 Bean。
这两种形式都可以被注入，具体使用哪一个取决于注入点的类型声明。

当然，我们也可以直接在构造函数中返回接口类型，这样就不需要显式导出为接口了。

```go
// 定义接口
type UserService interface {
	Get(id int) (*User, error)
}

// 具体实现
type userServiceImpl struct {
	// ...
}

func NewUserService() UserService {
	return &userServiceImpl{}
}

func (s *userServiceImpl) Get(id int) (*User, error) {
	return &User{ID: id}, nil
}

func init() {
	// 构造函数直接返回接口类型，容器会按接口类型进行注册，因此无需再显式导出
	gs.Provide(NewUserService)
}
```

### 附加激活条件

有时候，我们需要根据环境变量、配置文件等条件，在特定情况下才注册 Bean。
此时可以通过 `.Condition()` 方法为 Bean 添加条件，使其仅在满足条件时才生效。

```go
func init() {
	// 仅在 dev profile 下注册该 Bean
	gs.Provide(NewDevLogger).Condition(
		gs.OnProperty("spring.profiles.active"). // 监听 spring.profiles.active 配置
			HavingValue("expr:contains($, 'dev')"). // 属性值需包含 dev
			MatchIfMissing(),                    // 属性不存在时默认匹配
	)
}
```

由于条件注册涉及的内容较多，本文后续会在[**条件注册**](#条件注册)章节专门介绍。

### 显式依赖声明

绝大多数情况下，Go-Spring 会通过**注入关系自动推断依赖顺序**——
你注入了哪个 Bean，容器就会保证该 Bean 先完成初始化。

但在某些情况下，两个 Bean 之间可能没有直接的依赖注入关系，却仍然需要控制它们的初始化顺序。
此时我们可以使用 `.DependsOn()` 方法显式声明依赖关系，以确保被依赖的 Bean 先完成初始化。

`.DependsOn()` 的作用是告诉容器：
**虽然当前 Bean 没有直接注入该依赖，但在初始化顺序上必须先完成它的创建**。

```go
type A struct {
	// ...
}

type B struct {
	// ...
}

func init() {
	// 声明 B 依赖 A，确保 A 在 B 之前完成初始化
	gs.Provide(NewB).DependsOn(gs.BeanIDFor[A]())
}
```

在上面的例子中，`B` 虽然没有直接依赖 `A`，但通过 `.DependsOn()` 方法，
可以确保 `A` 先于 `B` 完成初始化。

如果这两个间接关联的 Bean 都定义了销毁方法，那么容器在退出时也会按照相反的顺序执行销毁流程，
先销毁依赖的 Bean，再销毁被依赖的 Bean。

### 标记为根 Bean

Go-Spring 采用**按需创建**的机制，默认情况下只有被标记为 root 的 Bean 才会作为依赖注入的入口被触发。

但在与其他框架集成时，有时并不需要使用 Go-Spring 自带的 Runner 或 Server（它们是内置的 root Bean），
而只是将其作为一个独立的 Bean 容器来使用。

此时，如果没有显式指定 root Bean，容器将缺少初始化入口，从而无法自动触发用户注册 Bean 的实例化与依赖注入流程。

在这种情况下，可以通过 `app.Root()` 方法将某个 Bean 显式标记为根 Bean。
这样，Go-Spring 容器就会以该 Bean 为起点，沿依赖关系递归完成其自身及其依赖 Bean 的初始化与注入。

```go
func main() {
	bootstrap := &Bootstrap{}

	// ...
	// 在中间的代码中使用 bootstrap 对象
	// ...

	gs.Configure(func(app gs.App) {
		// 将已有的 Bootstrap 实例显式注册为 Root Bean
		// 容器会以它为入口，确保该对象一定被创建并纳入依赖体系
		app.Root(bootstrap)
	}).Run()
}
```

在上面的例子中，`bootstrap` 被标记为根 Bean，
即使在 IoC 容器外创建，也会被纳入容器管理并作为初始化入口使用。

## Bean 注册

Go-Spring 提供了多种 Bean 注册 API，以满足不同的使用场景。

### 通过 `gs.Provide()` 注册

我们可以在包的 `init()` 函数中调用 `gs.Provide()` 注册 Bean，这是最基础、最常用的方式。
但是需要注意，该方法必须在应用启动前调用，否则会直接 `panic`。

```go
func init() {
	gs.Provide(NewUserService)
}
```

`gs.Provide()` 会将 Bean 记录到全局注册表中，并在应用启动时统一合并。

对于绝大多数业务组件而言，这是推荐的注册方式。

### 通过 `gs.Module()` 注册

`gs.Provide()` 一次只能注册一个 Bean，而在实际场景中，我们也常常需要批量注册 Bean。
为此，Go-Spring 提供了 `gs.Module()` 用于统一组织和注册多个 Bean。

`gs.Module()` 同时支持**条件化注册**能力，可以根据配置项条件决定是否启用整个模块。

从本质上看，Module 是一组具备条件控制能力的 Bean 注册单元，非常适合用于按需启用功能模块，
因此它是 Starter 机制的完美抽象，第三方集成包通常通过 Module 对外暴露能力。

在实现上，`gs.Module()` 通过回调函数提供 `gs.BeanProvider`，
开发者可以通过其 `Provide(...)` 方法注册 Bean，其用法与 `gs.Provide()` 完全一致。

例如，在下面的模块中，只有当配置项 `enable.redis=true` 时，才会注册 Redis 相关 Bean；
否则整个 Redis 模块将不会生效。

```go
func RedisModule(r gs.BeanProvider, p flatten.Storage) error {
	var m map[string]RedisConfig

	// 从配置中绑定 Redis 实例配置（name -> config）
	if err := conf.Bind(p, &m); err != nil {
		return err
	}

	// 为每一个 Redis 配置注册一个独立的 Redis Client Bean
	for name, config := range m {
		r.Provide(NewRedisClient, gs.ValueArg(config)).Name(name)
	}
	return nil
}

func init() {
	gs.Module(
		// 当 enable.redis=true 时启用该模块，否则模块不生效
		gs.OnProperty("enable.redis").HavingValue("true"),
		RedisModule,
	)
}
```

### 通过 `gs.Group()` 注册

`gs.Group()` 是 `gs.Module()` 的一种特殊封装，用于批量创建同类型的 Bean。
它以配置 key 作为 Bean 名称，非常适合通过配置字典生成多个实例。

当需要基于配置创建多个同类型 Bean 时，
例如多个 HTTP 客户端、多个数据源或多个缓存客户端（每个实例都拥有独立的配置参数），
使用 `gs.Group()` 可以显著减少模板代码。

```go
// 定义 HTTP 客户端配置结构
type HTTPClientConfig struct {
	BaseURL string        `value:"${baseURL}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}

// 根据配置创建 HTTP 客户端
func NewHTTPClient(c HTTPClientConfig) (*http.Client, error) {
	return &http.Client{Timeout: c.Timeout}, nil
}

func init() {
	// 从配置 "http.clients" 读取 map
	// key 作为 Bean 名称，value 作为配置参数
	// 每个 entry 对应调用 NewHTTPClient 创建一个独立 Bean
	gs.Group("${http.clients}", NewHTTPClient, nil)
}
```

对应的 YAML 配置如下：

```yaml
http:
  clients:
    serviceA:  # 作为 Bean 名称
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:  # 作为 Bean 名称
      baseURL: "http://b.example.com"
      timeout: 60s
```

在上面的示例中，`gs.Group()` 处理完成后，容器中会生成两个 `*http.Client` Bean，
名称分别为 `serviceA` 和 `serviceB`，我们可以在服务中按名称注入使用。

```go
type MyService struct {
	ClientA *http.Client `autowire:"serviceA"`
	ClientB *http.Client `autowire:"serviceB"`
}
```

如果需要释放资源，还可以提供销毁函数，该函数会对每个 Bean 实例分别生效。

```go
func init() {
	gs.Group("${http.clients}",
		NewHTTPClient,
		// 容器在销毁阶段会对每个实例分别调用销毁函数
		func(c *http.Client) error { return c.Close() },
	)
}
```

### 通过 `Configuration` 注册

`Configuration` 模式允许一个**配置类（父 Bean）**导出多个**子 Bean**，
用于将同一功能领域的 Bean 进行集中组织管理。
导出的子 Bean 可以直接复用配置类中已注入的配置参数。

例如，一个数据库配置类可以统一导出数据源、Repository 以及 Mapper 等相关 Bean。

```go
// DatabaseConfiguration 是配置类（父 Bean），本身支持依赖注入
type DatabaseConfiguration struct {
	MaxOpenConns int `value:"${db.max-open-conns:=10}"`
}

// 导出 DataSource Bean
// 可使用配置类（父 Bean）中已注入的参数
func (c *DatabaseConfiguration) NewDataSource() *DataSource {
	return NewDataSource(c.MaxOpenConns)
}

// 导出 UserRepository Bean
// 方法参数参与依赖注入（仅基于类型匹配）
func (c *DatabaseConfiguration) NewUserRepository(ds *DataSource) *UserRepository {
	return NewUserRepository(ds)
}

// 导出 OrderRepository Bean
// 方法参数参与依赖注入（仅基于类型匹配）
func (c *DatabaseConfiguration) NewOrderRepository(ds *DataSource) *OrderRepository {
	return NewOrderRepository(ds)
}

func init() {
	// 注册配置类
	// 启用 Configuration 后，容器会自动扫描该对象的方法
	// 并将符合规则的方法返回值注册为 Bean
	gs.Provide(new(DatabaseConfiguration)).Configuration()
}
```

在上面的示例中，定义了一个 `DatabaseConfiguration` 作为配置类（父 Bean），
用于集中管理数据库相关的配置参数。
随后通过方法分别导出 `DataSource`、`UserRepository` 和 `OrderRepository` 这些 Bean。
这些 Bean 既可以使用配置类中已注入的参数，也可以接收其他 Bean 作为依赖（仅支持基于类型匹配）。

你可能会好奇 `Configuration` 模式是如何工作的：

1. 首先将配置类作为普通 Bean 注册到容器中，并通过 `.Configuration()` 方法将其标记为配置类
2. 在解析阶段，容器遍历所有已注册 Bean，筛选出启用 `Configuration` 模式的 Bean
3. 随后扫描该配置类的所有公开方法，并根据 `Includes` 和 `Excludes` 规则进行过滤
4. 对于符合条件的方法，容器会将其返回值自动注册为独立的子 Bean，并加入 Bean 列表
5. 最终，所有 Bean（无论来源）都会纳入统一的生命周期与规则管理

实际上，这些符合条件的方法被视为构造函数，因此既可以只返回一个对象，也可以额外返回一个 `error`。

下面是一些符合条件的方法示例：

```go
// ✅ 符合：返回指针，无 error
func (c *Config) NewDataSource() *DataSource

// ✅ 符合：返回指针 + error
func (c *Config) NewDataSource() (*DataSource, error)

// ❌ 不符合：返回值个数不对
func (c *Config) NewDataSource() (*DataSource, string, error)
```

如果需要自定义包含和排除规则，我们可以通过 `Includes` 和 `Excludes` 参数进行控制。
默认情况下，`Includes` 仅匹配方法名符合 `New.*` 的方法（即以 `New` 开头），
而 `Excludes` 不排除任何方法。

自定义包含和排除规则示例：

```go
func init() {
	gs.Provide(new(DatabaseConfiguration)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"}, // 包含匹配这些正则的方法
			Excludes: []string{".*Internal$"},       // 排除匹配这些正则的方法
		})
}
```

### 通过 `app.Provide()` 注册

除了前面几种注册方式外，我们还可以通过 `gs.Configure()` 提供的回调函数注册 Bean。
该方法会在回调中提供一个 `gs.App` 对象，开发者可以通过其 `Provide` 方法进行 Bean 注册。

```go
func main() {
	gs.Configure(func(app gs.App) {
		// 可以在回调中注册 Bean 或者设置应用级属性
		// 这些内容仅在当前应用实例中生效
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这种注册 Bean 的方式通常用于单元测试中，可以实现测试之间的数据隔离。

## 条件注册

有时候我们希望某些 Bean 仅在满足特定条件时才生效，这可以通过 **Condition** 机制来实现。

Go-Spring 提供了丰富的条件实现及组合工具，开箱即用。
我们既可以基于配置属性进行判断，也可以根据 Bean 是否存在进行判断，
还能通过 `And` / `Or` / `Not` / `None` 组合多个条件，实现更复杂的判断逻辑。

前面已经提到过了，在注册 Bean 的时候，我们可以通过 `.Condition()` 方法来为 Bean 绑定条件。

```go
gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
```

### 常用条件

Go-Spring 提供了一些常用的条件类型。

#### 属性条件

`OnProperty` 是最常用的条件类型之一，
既可以根据配置属性是否存在进行判断，
也可以根据配置属性的值是否等于特定值进行判断，
还支持通过 `expr` 表达式实现更灵活的条件判断。

```go
// 配置项存在即可满足条件（既可以是叶子节点，也可以是路径节点）
gs.OnProperty("enable.redis")

// 当配置项等于指定值时才满足条件
gs.OnProperty("env").HavingValue("prod")

// 配置项不存在时也满足条件（MatchIfMissing）
gs.OnProperty("optional.feature").MatchIfMissing()
```

`OnProperty` 还支持表达式判断，只需要在值前添加 `expr:` 前缀，
即可实现除存在性判断之外的更复杂校验逻辑。

```go
// 匹配端口大于 8080 的情况
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

##### 表达式语法

`OnProperty` 使用 [expr-lang/expr](https://github.com/expr-lang/expr) 引擎进行计算，
语法简洁直观：

* **`$`**：表示当前配置属性的值，所有表达式均围绕 `$` 进行比较判断
* **比较运算符**：支持 `>`、`<`、`>=`、`<=`、`==`、`!=` 等常见比较操作
* **逻辑运算符**：支持 `&&`、`||`、`!` 用于逻辑组合
* **字符串操作**：支持 `contains`、`hasPrefix`、`hasSuffix` 等字符串操作方法

常见表达式示例：

```go
// 端口大于 1024 且小于 65535
gs.OnProperty("server.port").HavingValue("expr: $ > 1024 && $ < 65535")

// 环境不是生产环境
gs.OnProperty("app.env").HavingValue("expr: $ != 'prod'")

// 配置项以 "http://" 开头
gs.OnProperty("app.base-url").HavingValue("expr: startsWith($, 'http://')")

// 配置项包含指定关键字
gs.OnProperty("app.features").HavingValue("expr: contains($, 'debug')")
```

我们可以通过 `gs.RegisterExpressFunc()` 注册自定义函数，并在表达式中使用。

```go
func init() {
	// 注册自定义表达式函数
	gs.RegisterExpressFunc("isValidPort", func(port int) bool {
		return port > 1024 && port < 65535
	})

	// 在条件表达式中使用自定义函数
	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").HavingValue("expr:isValidPort($)"),
	)
}
```

#### Bean 存在条件

Go-Spring 提供了一些用于根据容器中是否存在特定 Bean 来进行判断的条件：

```go
// 当容器中存在 UserService（至少一个匹配）时满足条件
gs.OnBean[*UserService]()

// 当容器中不存在 UserService 时满足条件
gs.OnMissingBean[*UserService]()

// 当容器中恰好存在一个 UserService 时满足条件
gs.OnSingleBean[*UserService]()

// 按名称匹配，判断指定名称的 DataSource 是否存在
gs.OnBean[*DataSource]("master")
```

- `OnBean[T]()` 表示容器中至少存在一个匹配的 Bean。
- `OnMissingBean[T]()` 表示容器中不存在匹配的 Bean。
- `OnSingleBean[T]()` 表示容器中恰好存在一个匹配的 Bean。

我们在使用这三个条件的时候，可以选择传入 Bean 名称或者不传入。
不传 Bean 名称时表示仅按类型匹配，传入 Bean 名称时表示同时按类型和名称进行匹配。

#### 自定义函数条件

在一些简单情况下，我们可以使用 `OnFunc` 包装自定义函数来实现条件判断。

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	// 自定义任意条件判断逻辑
	return myCustomCheck(ctx)
})
```

#### 组合条件

Go-Spring 提供了四种条件逻辑组合：`And` / `Or` / `Not` / `None`。

```go
// 当需要同时满足所有条件时，使用 gs.And
gs.Provide(NewService).Condition(gs.And(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))

// 当需要任一条件满足时，使用 gs.Or
gs.Provide(NewService).Condition(gs.Or(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))

// 当需要条件取反时，使用 gs.Not
gs.Provide(NewFallbackService).Condition(gs.Not(
	gs.OnBean[RealService](),
))

// 当所有条件都不满足时，使用 gs.None
gs.Provide(NewService).Condition(gs.None(
	gs.OnProperty("profile.dev"),
	gs.OnProperty("profile.test"),
))
```

- **`gs.And`**：要求所有子条件同时满足时才成立
- **`gs.Or`**：只要任一子条件满足即可成立
- **`gs.Not`**：对子条件结果取反
- **`gs.None`**：所有子条件均不满足时才成立

这些组合条件还可以相互嵌套使用，实现更复杂的条件逻辑。

```go
// 生产环境 且 (启用了 A 或 启用了 B)
gs.And(
	gs.OnProperty("env").HavingValue("prod"),
	gs.Or(
		gs.OnProperty("enable.a"),
		gs.OnProperty("enable.b"),
	),
)
```

需要注意的是，虽然组合条件可以实现复杂的逻辑，但一般不建议使用过于复杂的条件表达式。
如果逻辑过于复杂，应当优先考虑是否可以进行简化或重构方案。

#### 缓存条件结果 `OnOnce`

有时候条件计算可能较为复杂，并且需要在多处复用。
为了避免重复计算，我们可以使用 `OnOnce` 对结果进行缓存，后续判断将直接复用缓存结果。

```go
// 条件只会计算一次，后续直接复用缓存结果
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

绝大多数情况下，简单条件不需要缓存结果；
只有在条件较为复杂且需要在多处复用时，才需要使用缓存机制。

### Profile 条件

对于按环境（开发/测试/生产）选择性启用 Bean 的场景，
Go-Spring 提供了 `.OnProfiles()` 链式方法，相比显式条件设置更加简洁直观。

```go
func init() {
	// 仅在 dev 环境下启用该 Bean
	gs.Provide(NewDevLogger).OnProfiles("dev")
}
```

本质上，Profile 条件是基于 `spring.profiles.active` 配置项进行判断的。
当设置的 profiles 与当前激活的 profiles 中有任意一个匹配时，条件即可满足。

> `spring.profiles.active` 表示当前激活的 profiles，例如 `"dev"`、`"test"` 或 `"prod"`。

## 容器原理

本章节介绍 Go-Spring 的一些实现细节。
了解容器的执行原理，有助于我们更好地理解和使用 Go-Spring。

### 运行流程

Go-Spring 最核心的是 IoC 容器的运行流程。
下面介绍容器从启动到关闭的完整过程。

#### 注册阶段

Bean 的注册分为 `全局注册` 和 `容器注册` 两个独立阶段，
其根本目的在于支持单元测试中的数据隔离。

##### 全局注册

我们在 `init()` 中调用 `gs.Provide()`、`gs.Module()` 完成 Bean 注册，
这些注册信息会被保存到全局注册表中，并作为所有容器首要的注册来源。

在创建容器时，Go-Spring 会从全局注册表中拷贝一份注册信息生成独立的容器实例，
因此每个容器之间互不影响。

这也是单元测试能够实现数据隔离的基础：
每个测试用例都会创建独立容器，并基于全局注册表的拷贝构建自己的 Bean 集合。

##### 容器注册

容器注册主要用于解决单元测试场景下的 Bean 补充问题。
Go-Spring 支持在不启动完整应用的情况下运行单测，
此时可以通过 `app.Provide()` 直接向当前容器注册所需的 Bean，
以补齐测试环境中的依赖，这些 Bean 仅在当前容器中生效。

#### 解析阶段

容器启动后，第一步是对所有 Bean 进行合并与解析处理，该阶段按顺序执行以下几个步骤：

##### Bean 合并

第一步是将所有来源的 Bean 统一合并到一起，包括：

* 通过 `app.Provide()` 注册的 Bean
* 通过 `gs.Provide()` 注册的 Bean
* 通过 `gs.Module()` 或 `gs.Group()` 注册的 Bean
* 通过 `Configuration` 模式导出的 Bean

合并完成后，容器会得到一份完整的待处理 Bean 列表。

##### 条件裁剪

这一步会遍历所有 Bean，并依次执行其 Condition 判断。
最终，满足条件的 Bean 被保留，不满足条件的 Bean 被剔除，
只有保留下来的 Bean 才会参与后续的创建过程。

通过这一过程，实现了“根据条件动态决定哪些 Bean 生效”的能力，也是多环境配置的基础。

##### 冲突检测

在 Go-Spring 中，类型与名称完全一致的 Bean 被视为冲突。
Go-Spring 严格遵循**不允许覆盖**的原则，并在解析阶段的最后进行冲突检测。
一旦检测到冲突，容器将直接启动失败并报错。

#### 注入阶段

解析完成后，容器会从 root beans 出发，按照依赖关系递归创建 Bean 实例并完成依赖注入。
这一阶段的核心步骤如下：

1. 首先基于类型和名称为所有 Bean 建立索引，用于后续快速匹配依赖
2. 从 root beans 开始，按照依赖顺序递归创建 Bean 并完成注入。
	**在此过程中，每个 Bean 的处理流程如下：**
   - 创建 Bean 实例后立即进行依赖注入
   - 注入过程中检测循环依赖，如果存在则直接报错
   - 同时记录需要延迟注入的字段，供后续统一处理
   - 记录 Bean 之间的 destroy 执行顺序依赖关系
   - 注入完成后调用 Bean 的 `init` 方法完成初始化
3. 所有 Bean 处理完成后，统一处理延迟注入字段，完成最终依赖绑定
4. 对 destroy 方法依赖关系进行拓扑排序，确保被依赖的 Bean 优先销毁
5. 清理解析阶段的临时元数据，释放不必要的内存资源

注入阶段完成后，所有 Bean 均已创建并完成依赖注入与初始化，容器进入运行阶段。

#### 运行阶段

Go-Spring 在运行阶段不参与业务逻辑处理，也不支持在运行时动态获取 Bean 实例。

这一设计是有意为之的。
一方面，从 Java Spring 的实现来看，支持运行时获取 Bean 会显著增加框架实现复杂度；
另一方面，Go 语言更强调静态明确的依赖关系，这种设计可以避免运行时反射带来的不确定性。
因此，Go-Spring 在设计上刻意避免了运行时的反射式 Bean 访问能力。

换句话说，Go-Spring 采用的是**启动期模型（startup-time model）**：
即所有依赖关系仅在启动阶段完成注入，注入完成后容器不再参与运行过程。

这一设计也带来了一些额外优势，例如：

* 所有依赖错误都能在启动阶段暴露，避免运行时才发现问题
* 更适合资源受限环境，例如嵌入式系统或者轻量级服务
* 运行时无需容器参与，整体执行更轻量、可预测性更强

#### 关闭阶段

当应用收到退出信号（例如 SIGINT）时，会通知容器进入关闭阶段。
此时容器会按照依赖关系的逆序销毁所有 Bean，确保依赖安全释放。

销毁过程中会依次调用各 Bean 的 `Destroy` 回调方法用于释放资源，
全部完成后容器才会彻底退出。

### 核心设计

Go-Spring 在实现过程中做出了一系列关键设计决策，这些决策从根本上塑造了容器的整体运行机制。
深入理解这些设计理念，有助于我们更加高效、灵活地使用 Go-Spring。

#### 接口分离

无论一个结构体实现了多少接口，Go-Spring 都会将**接口 Bean**与**原始 Bean**视为彼此独立的实体。
如果希望通过接口进行注入，必须使用 `Export` 方法显式地导出对应接口。

之所以采用这种设计，是因为 Go 语言没有 `implements` 关键字，若依赖自动推导，容易引发意料之外的行为。
例如，一个结构体可能“恰好”实现了某个接口，但这并非我们的本意，从而导致错误的依赖被注入。

```go
type Service interface {
	Do()
}

type ServiceImpl struct{}

func (s *ServiceImpl) Do() {
	println("ServiceImpl.Do")
}

func NewServiceImpl() *ServiceImpl {
	return &ServiceImpl{}
}

func init() {
	// 仅注册 *ServiceImpl，没有导出 Service 接口
	// 其他组件无法通过 Service 接口注入，只能依赖具体类型 *ServiceImpl
	gs.Provide(NewServiceImpl)
}

func init() {
	// 正确方式：显式导出接口
	// 这样既可以按接口注入，也保留了对具体类型的支持
	gs.Provide(NewServiceImpl).Export(gs.As[Service]())
}
```

#### 按需创建

在大型项目中，依赖关系和组织结构往往非常复杂，很容易引入一些实际上并未使用的 Bean。
为了解决这一问题，Go-Spring 使用了**按需创建（lazy creation）**的策略。

也就是说，在 Go-Spring 中，只有被依赖的 Bean 才会被创建，而未被依赖的 Bean 则不会实例化，
从而避免了不必要的资源开销。

但这也引出了一个关键问题：容器应该从哪些 Bean 开始分析依赖关系，才能确保不会遗漏真正需要的 Bean？

为此，Go-Spring 引入了 **root bean**（根 Bean）的概念，即依赖树的起点。
容器会从这些 root bean 出发，递归解析并创建其依赖的所有 Bean。

那么，哪些 Bean 会被视为 root bean 呢？

- 首先，实现了 `gs.Runner` 或 `gs.Server` 接口的 Bean 会被自动识别为 root bean。
  这类 Bean 在应用启动过程中具有特殊作用，由容器自动收集。
- 其次，通过 `app.Root()` 显式标记的 Bean 也会被视为 root bean。
  这类 Bean 通常是在 IoC 容器之外使用，但借助容器完成依赖注入。

通过这种机制，Go-Spring 能够在保证依赖完整性的同时，实现精确、可控的 Bean 创建策略。

#### 循环依赖

Go-Spring 可以**有限支持**循环依赖，具体取决于依赖注入的方式：

- 如果 A 通过**字段注入**依赖 B，而 B 同样通过**字段注入**依赖 A，这种情况是支持的。
  容器可以先创建对象实例，再进行属性填充，从而完成依赖闭环。

- 如果 A 通过**字段注入**依赖 B，而 B 通过**构造函数注入**依赖 A，这种情况属于**有限支持**。
  在依赖注入过程中，A 或 B 可能处于“未完全初始化”的状态，因此需要谨慎使用。

- 如果 A 和 B **都通过构造函数注入**互相依赖，则这种情况**不被支持**。
  因为构造函数要求依赖在创建时就必须是完整的，而 A 和 B 无法同时先于对方完成初始化，导致无法解析。

#### 销毁顺序

Go-Spring 严格遵循**依赖逆序**原则来管理 Bean 的销毁顺序，即：**被依赖的 Bean 先初始化，后销毁**。

举个例子，如果 A 依赖 B，那么：

* 在初始化阶段：必须先创建 B，才能将其注入到 A 中；
* 在销毁阶段：则先销毁 A，释放其对 B 的依赖，最后再销毁 B。

这种“先建后用、先停后拆”的顺序，确保了在整个销毁过程中，依赖始终是可用的，
从而避免出现“访问已销毁 Bean”的问题。

## 常见问题

### 1. 支持运行时动态获取 Bean 吗？为什么不提供 `getBean()`？

Go-Spring **不支持**在运行时动态获取 Bean（即不提供 `getBean()` 这类 API），
这是一个有意为之的设计选择，主要基于以下考虑：

1. **实际需求较少**：在大多数服务端应用中，组件依赖关系在启动时就已经确定，很少需要在运行时动态获取 Bean。
2. **避免引入复杂性**：所有依赖在启动阶段完成解析，运行时代码无需感知容器，调用关系更加直接、清晰。
3. **运行时零开销**：应用启动完成后，容器不再参与运行逻辑，相关元数据也会被清理，性能与手写代码基本一致。
4. **问题尽早暴露**：依赖错误会在启动阶段立即暴露，而不是延迟到运行时才发现。
5. **依赖显式声明**：所有依赖关系都在代码中明确体现，相比运行时动态获取，更易理解和调试。

如果确实存在运行时创建实例的需求，推荐使用**工厂模式**：
即注册一个工厂单例，由工厂负责创建和管理所需的实例。

### 2. 为什么接口需要显式导出？不能自动推导吗？

在 Go 语言中，并不存在 `implements` 关键字，接口的实现是**隐式的**。
这意味着如果依赖自动推导，一个结构体只要“恰好”实现了某个接口，就可能被错误地匹配，从而引入难以察觉的问题。

此外，在大多数场景下，我们并不一定需要通过接口进行解耦，
Go 更推荐在合适的范围内直接使用具体类型，以保持代码简洁。

因此，Go-Spring 要求通过 `Export` 显式声明接口实现关系，使依赖关系更加**明确、可控、可读**。

### 3. 支持循环依赖吗？

Go-Spring 对循环依赖提供**有限支持**，可以自动处理一部分常见场景（例如基于字段注入的循环依赖）。

但对于某些情况（如构造函数之间的循环依赖），由于对象在创建时必须是完整的，容器无法解析，因此不被支持。

### 4. 怎么进行单元测试？

详见独立文档 [07-testing.md](07-testing.md)，其中对测试相关内容做了系统说明，主要包括：

* 不依赖 IoC 的纯单元测试写法（推荐，最简单直接）
* 基于 IoC 容器的集成测试方式
* 如何在测试中通过 Mock Bean 覆盖全局注册的默认实现
* Go-Spring 内置断言工具及 Mock 框架的使用方法

### 5. 支持原型模式（多实例）吗？

Go-Spring 核心**只支持单例模式**，不直接提供原型（prototype）作用域。
这是一个与 Go 语言特性相匹配的设计选择：

1. **实际需求较少**：在 Java 生态中原型模式有一定使用场景，但在 Go 的服务端开发中，
大多数组件天然是无状态或可复用的，单例已经足够覆盖绝大部分需求。

2. **Go 提供更自然的替代方式**：如果确实需要创建多个实例，更符合 Go 风格的做法是使用工厂模式，
而不是依赖容器的作用域机制。

典型做法是将工厂本身注册为单例，由工厂负责创建新的实例：

```go
// 工厂本身是单例
type MyServiceFactory struct {
	// 工厂可以依赖容器中的其他组件
	config Config `autowire:""`
}

// 通过工厂方法创建新实例
func (f *MyServiceFactory) NewMyService(...) *MyService {
	return &MyService{
		// 工厂负责初始化
		config: f.config,
		// ...
	}
}

func init() {
	gs.Provide(new(MyServiceFactory))
}
```

简而言之：如果需要多实例，直接用工厂模式解决即可，
IoC 容器的核心职责不需要引入额外的作用域复杂性。

### 6. 性能怎么样？运行时有反射开销吗？

Go-Spring 在设计上保证了**运行时零反射开销**：

- **依赖注入仅在启动期执行一次**：所有 Bean 的解析与注入都发生在启动阶段，
  完成后容器会释放大部分元数据，不再参与运行时逻辑。
- **运行时代码直接调用**：应用启动完成后，组件之间的调用与普通 Go 代码无异，
  不存在运行时的反射查找或动态解析。
- **反射仅用于初始化阶段**：唯一使用反射的地方是在启动期进行依赖解析与字段注入，
  这一过程只执行一次。

因此，在应用启动完成后，Go-Spring 的运行性能与手写 Go 代码基本一致，无需额外担心性能损耗。

### 7. 为什么用 `init()` 手动注册，而不是自动包扫描？

首先，从 Go 语言本身来看，**并不存在真正意义上的“自动包扫描”能力**：

- Go 的编译模型决定了：只有被 `import` 的包才会被编译进二进制；未导入的包即使存在代码，也不会被加载。
- 即便要实现“自动注册”，本质上仍然需要通过 `import _ xxx` 的方式触发 `init()` 执行。

既然最终都必须依赖 `import` 才能引入包，那么 Go-Spring 选择直接在 `init()` 中显式注册 Bean，
而不是引入额外的扫描机制。这样实现也有一些好处：

1. **完全显式**：哪些 Bean 被注册是明确可见的，不存在“隐式扫描”带来的不确定性。
2. **清晰可控**：无需注解、配置文件或运行时扫描文件系统，符合 Go “显式优于隐式”的设计哲学。
3. **启动更简单**：所有注册逻辑在程序启动时通过 `init()` 自动完成，无需额外扫描或解析阶段。

当然，如果希望减少手动编写注册代码，也可以通过代码生成工具在构建期生成 `Provide` 调用，但这不会改变核心模型。

### 8. 和 Wire 这类编译期 DI 有什么区别？

| 特性     | Go-Spring   | Wire    |
| ------ | ----------- | ------- |
| 处理时机   | 运行期（启动阶段反射） | 编译期代码生成 |
| 条件注册   | ✅ 原生支持      | ❌ 不支持   |
| 动态配置   | ✅ 原生支持      | ❌ 不支持   |
| 生命周期管理 | ✅ 完整支持      | ❌ 不支持   |
| 启动速度   | 足够快         | 更快      |

从本质上看，Wire 属于**编译期依赖注入工具**，在构建阶段就完成依赖关系的生成，因此具备更强的静态约束能力，
在启动速度和二进制体积方面也更有优势，适合对极致性能和编译期安全性要求较高的场景。

Go-Spring 则是**运行期（启动阶段）依赖注入框架**，虽然使用反射完成依赖解析，但仅发生在应用启动阶段，
一旦启动完成，运行时不再依赖反射，因此不会带来持续性能开销。

它的优势主要体现在：

* 支持条件注册，依赖可以根据配置灵活启用或替换
* 支持动态配置，更适合复杂业务环境
* 提供完整生命周期管理能力（初始化、销毁、依赖顺序等）
* 更贴近大型服务端应用的工程化需求

因此可以简单理解为：

* Wire 更偏向“**编译期确定一切**”
* Go-Spring 更偏向“**运行期灵活管理系统结构**”

两者并非直接优劣关系，而是针对不同工程约束下的设计取舍。

### 9. 什么是 root bean？什么时候需要手动标记？

**root bean** 是依赖树的起点。容器会从 root bean 出发递归解析其依赖，
并确保这些 Bean 一定会被创建，即使它们没有被其他 Bean 显式依赖。

以下情况会**自动成为 root bean**，无需手动配置：

* 实现了 `gs.Runner` 接口的 Bean
* 实现了 `gs.Server` 接口的 Bean（例如 HTTP Server）

这类组件通常代表应用的入口或核心运行单元，容器会在启动阶段自动收集并处理。

如果某个组件需要在容器启动时被主动创建，但它既不是 `Runner` 也不是 `Server`，
则可以通过 `app.Root()` **手动标记为 root bean**：

```go
func main() {
	c := &MyComponent{}
	// ...
	gs.Configure(func(app gs.App) {
		app.Root(c)
	}).Run()
}
```
