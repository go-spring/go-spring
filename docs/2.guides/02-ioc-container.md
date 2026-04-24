# IoC 容器

> 控制反转（Inversion of Control，IoC）和依赖注入（Dependency Injection，DI）是 Spring 框架的核心基础。
> Go-Spring 沿用了 Spring 的设计理念，同时保持 Go 语言的原生风格，为 Go 开发者提供了一个简洁高效的依赖注入容器。

## 什么是依赖注入？

如果你已经熟悉 Java Spring，可以直接跳过这一节。

依赖注入是一种设计模式，它能帮助我们写出更优雅、更易维护的代码：

1. **解耦组件依赖**：组件不需要自己创建依赖对象，由容器统一提供
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

但当应用有几十上百个组件，而且它们之间存在复杂的依赖关系时，我们还需要一个个手动组装吗？当然不需要。
**这时候 IoC 容器就派上用场了** —— 它可以帮我们自动完成以下所有事情：

1. 我们把所有组件告诉容器（这一步叫做**注册 Bean**）
2. 容器自动分析组件之间的依赖关系
3. 容器按照正确的顺序创建所有对象
4. 容器自动把依赖注入到需要的地方
5. 容器全程管理组件从创建到销毁的整个生命周期

一句话概括：所有组件的组装和管理工作，都可以交给 IoC 容器自动完成，让我们更专注于编写业务逻辑。
这，就是 Go-Spring 的使命！

---

## 快速开始

让我们通过一个简洁的示例快速了解 Go-Spring IoC 的用法：

```go
package main

import (
	"fmt"
	"net/http"

	// 引入 Go-Spring 核心包
	"github.com/go-spring/spring-core/gs"
)

// 定义 UserService 接口
// 接口定义了组件对外暴露的能力，实现解耦
type UserService interface {
	GetUser() string
}

// UserServiceImpl 是 UserService 的具体实现
type UserServiceImpl struct {
	// ...
}

func NewUserServiceImpl() *UserServiceImpl {
	return &UserServiceImpl{}
}

func (s *UserServiceImpl) GetUser() string {
	return "Alice"
}

// UserController 依赖 UserService 来处理请求
type UserController struct {
	service UserService
}

// UserService 依赖由容器自动提供
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

// ServeHTTP 处理 HTTP 请求
func (c *UserController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 直接使用注入的 service，不需要关心它是如何创建的
	fmt.Fprintf(w, "Hello, %s!", c.service.GetUser())
	fmt.Println("Hello,", c.service.GetUser())
}

// 所有 Bean 都在 init() 函数中注册
func init() {
	// 1. 注册 UserServiceImpl，并导出为 UserService 接口
	// 这样其他组件就可以注入 UserService 接口，而不依赖具体实现
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())

	// 2. 注册 UserController
	// 容器会自动分析构造函数参数，找到并注入对应的依赖
	gs.Provide(NewUserController)

	// 3. 注册 HTTP 路由入口
	gs.Provide(func(c *UserController) *gs.HttpServeMux {
		return &gs.HttpServeMux{Handler: c}
	})
}

func main() {
	// 启动 Go-Spring 容器
	// 容器会自动完成所有 Bean 的创建和依赖注入，然后启动 HTTP 服务
	gs.Run()
}
```

运行后访问 `http://localhost:9090`，你会看到：

```
Hello, Alice!
```

控制台也会输出：

```
Hello, Alice
```

可以看到整个过程非常简洁：定义接口和实现，声明依赖，在 `init()` 中注册 Bean，
最后调用 `gs.Run()` 启动容器，剩下的事情都由 Go-Spring 自动完成。

---

## Bean 定义

Bean 是由容器统一管理的组件，在容器中通常以单例形式存在。

"Bean" 的概念来自 Java Spring，但 Go-Spring 对它做了更贴近 Go 语言特点的诠释。
在我们看来，**Bean 本质上是一种比 Go 包更细粒度的功能组织单元**。

Go 的包在很多情况下粒度偏粗，一个包里往往包含多个彼此独立、需要注入的组件，而 Bean 正好弥补了这一不足。
我们可以把每个 Bean 看作一个独立的功能单元，它的创建、初始化以及销毁的整个生命周期都由容器统一管理。

---

## Bean 注入

Go-Spring 的依赖注入体系可以从两个维度来理解：

- **注入方式**：用什么语法形式声明依赖（构造函数注入 vs 结构体字段注入）
- **注入目标**：依赖最终以什么形式被接收（单个 Bean、Bean 集合、动态匹配等）

### 注入方式

Go-Spring 支持**两种注入方式**：构造函数注入和结构体字段注入。

#### 结构体字段注入

结构体字段注入是最直观的注入方式：
只需要在结构体字段上通过 `autowire`（或 `inject`）标签声明依赖，
容器就会自动把匹配的 Bean 注入到该字段中。

这种方式简洁明了，不需要手动写构造函数，适用于大多数业务场景。
而且它天然支持循环依赖，不需要重构代码也能正常工作。

示例：

```go
// UserController 依赖 UserService
type UserController struct {
	service UserService `autowire:""`
}
```

#### 构造函数参数注入

构造函数注入是指依赖通过构造函数的参数传入，
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

> Go-Spring 使用标准的 Go 构造函数，不需要特殊约定。

#### 选择哪种注入方式

从解耦的角度来看，推荐大多数场景使用**构造函数注入**，
但是我们也不特别排斥使用**结构体字段注入**。

### 注入目标

无论是构造函数注入还是结构体字段注入，Go-Spring 都支持把 Bean 注入到多种形式的目标中。
最常见的是**注入单个 Bean**，即只需要一个符合类型条件的 Bean。
也可以**注入多个 Bean**，此时容器会自动把符合条件的 Bean 收集到切片或 Map 中。

#### 注入单个 Bean

这是最基础、也是最常见的使用方式，即注入**唯一**符合类型条件的 Bean。
绝大多数业务依赖都属于这种场景。

**按类型注入**

我们可以在结构体字段上添加 `autowire` 或者 `inject` 标签，
容器就会自动把匹配的 Bean 注入到该字段中。

```go
type Service struct {
	// 按类型自动匹配，注入唯一的 UserRepository 接口
	Repo UserRepository `autowire:""`
}
```

我们也可以在注册构造函数 Bean 时使用 `TagArg` 来指定 Bean 的名称。
实际上我们就是用 `TagArg` 来模拟和实现结构体字段 tag 的功能。

```go
// UserController 需要注入一个 UserService
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

// 按类型自动匹配，可以省略 TagArg
func init() {
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

**按名称注入**

如果候选 Bean 只有一个，我们通常不需要指定名称。
但如果容器中存在多个相同类型的 Bean，就需要通过名称明确指定。

```go
func init() {
	// 注册 master 和 slave 两个命名 Bean
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

type Service struct {
	// 只注入名为 "slave" 的 Bean
	ds DataSource `autowire:"slave"`
}
```

```go
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

// 注入名称为 "slave" 的 DataSource
func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

**可空注入**

默认情况下，如果找不到匹配的 Bean，容器会启动失败。
如果我们希望找不到 Bean 时注入零值而不是报错，可以使用 `?` 标记，
它表示这个字段是可空的。

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
func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

// 按类型自动匹配，可以省略 TagArg
func init() {
	gs.Provide(NewUserController, gs.TagArg("?"))
}
```

#### 注入多个 Bean

当需要获取**多个符合条件的 Bean**时，
我们可以把依赖声明为 `[]T`（切片）或者 `map[string]T`（Map）。
这时，容器会自动把符合条件的同类型 Bean 收集到集合中。

##### 切片收集 `[]T`

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
// 注入所有 Plugin 到切片
func NewApplication(plugins []Plugin) *Application {
	return &Application{plugins: plugins}
}

func init() {
	gs.Provide(NewApplication)
}
```

如果没有指定 tag，容器会按照 **Bean 名称的字典序** 对切片中的元素进行排序，
这样每次启动结果都是确定的，保证了行为的一致性。

如果需要**精确控制顺序**，我们可以在 tag 中用 Bean 的名称显式指定：

```go
type Chain struct {
	// 顺序: auth(可空) -> trace -> recovery
	// auth 不存在也没关系，标记为可空跳过
	filters []Filter `autow:"auth?,trace,recovery"`
}
```

```go
// 也可以指定筛选，和字段注入语法相同
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}

func init() {
	// 顺序: auth -> trace -> 剩余所有 -> recovery
	gs.Provide(NewChain, gs.TagArg("auth,trace,*,recovery"))
}
```

我们也可以对列表中的每个 Bean 使用可空语法 `name?`，意思是找不到就跳过。

此外，我们还可以用通配符 `*` 来表示包含**所有未显式列出**的剩余 Bean。
需要注意的是，通配符 `*` 只能在表达式中出现一次。

当使用通配符 `*` 时，Bean 的收集顺序如下：

1. **`*` 之前的显式 Bean** 按 tag 中声明的顺序排列；
2. **`*` 匹配的剩余 Bean** 按名称字典序排列；
3. **`*` 之后的显式 Bean** 按 tag 中声明的顺序排列。

例如 `autowire:"a,*,c?,b"` 表示：首先收集 a，然后是除 c、b 之外的其他 Bean，
然后是 c，最后是 b。同时 c 可以为空，找不到也不报错。

##### Map 收集 `map[string]T`

```go
type Router struct {
	// name -> Handler 映射，key 就是 Bean 名称
	handlers map[string]Handler `autowire:""`
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

当使用 `map[string]T` 进行收集时，容器会自动用每个 Bean 的名称作为键，建立名称到 Bean 的映射。
如果我们只需要包含特定的 Bean，也可以用和切片相同的 tag 语法进行筛选，不过 Map 结果不保证顺序。

```go
type Service struct {
	// 只包含指定名称的处理器
	myHandlers map[string]Handler `autowire:"auth,user,order"`
}
```

#### 通过配置项注入

通常，我们在注入时就能确定要注入的 Bean 名称，但有时候我们可能需要根据配置项动态指定注入的 Bean 名称。
Go-Spring 也支持这种需求。只需要把 Bean 名称改为 `${...}` 表达式即可，
这表示容器会从配置项中读取需要注入的 Bean 名称。

无论是结构体字段注入还是构造函数注入，无论是切片收集还是 Map 收集，都可以使用 `${...}` 表达式动态指定 Bean 名称。

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
	// 从配置 "storage.provider" 读取 Bean 名称
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

```go
type Chain struct {
	// 从配置 "http.filters" 读取过滤器列表
	filters []Filter `autowire:"${http.filters}"`
}
```

```go
func NewChain(filters []Filter) *Chain {
	return &Chain{filters: filters}
}
```

```go
func init() {
	gs.Provide(NewChain, gs.TagArg("${http.filters}"))
}
```

当然，我们也可以在 `${}` 语法中指定默认值，但不常用，这里就不介绍了。

---

### 延迟注入

延迟注入是为了解决某些情况下的循环依赖问题，它只能用在结构体字段注入中。
语法是在 tag 中添加 `,lazy` 标记。

```go
type Service struct {
	// 强制这个字段延迟注入，等所有非延迟注入完成后再处理
	dep Dependency `autowire:",lazy"`
}
```

需要注意的是，被标记为 lazy 注入的字段会在所有非延迟注入完成后再处理，
这是一个独立的过程，所以在上一个阶段的处理过程中，这些标记为 lazy 注入的字段都是空的。

## Bean 类型

Go-Spring 支持三种类型的 Bean 定义：

- **结构体指针**：将预先创建好的对象直接交给容器管理，这种写法最简单
- **构造函数**：由容器在启动时调用构造函数创建 Bean，推荐使用这种方式
- **函数指针**：直接将函数指针作为 Bean，适配函数式风格

### 结构体指针

这是写法最简单的注册方式，**直接传一个已经创建好的结构体指针就行**：

```go
package mypackage

import (
	"github.com/go-spring/spring-core/gs"
)

// MyService 是你的业务结构体
type MyService struct {
	// ...
}

func init() {
	// 直接 new 出对象指针，注册到容器
	gs.Provide(new(MyService))
}
```

虽然需要提前创建对象，可能会浪费一点点内存，但这点开销在绝大多数场景下完全可以忽略。

另外，它还有一个独特优点：**可以把容器外已经创建好的对象直接放进容器里管理**。
这在 Go-Spring 和其他框架或已有程序协作时非常有用。

### 构造函数

通过构造函数创建 Bean 是 Go-Spring 推荐的方式。
在这种模式下，**所有依赖都通过构造函数的参数传入，由容器负责解析并提供**：

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

上面的构造函数是 `func(...) T`，直接返回 Bean，适用于创建过程一定不会失败的场景。

当我们创建复杂对象时，创建过程可能会失败，比如配置验证不通过、打开文件失败、连接数据库失败等。
这时候我们可以返回 `error` 来表示创建失败，格式是 `func(...) (T, error)`，容器会自动处理错误并启动失败。

```go
// 构造函数参数接收依赖，返回实例
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}

func init() {
	gs.Provide(NewMyService)
}
```

#### 参数绑定

Go 语言不支持在函数参数上添加 tag 元数据，所以 Go-Spring 选择了
**在注册时通过 `Arg` 接口为每个需要特殊处理的参数显式声明绑定信息**。

为构造函数参数提供具体值的过程我们称之为**参数绑定 (Arg binding)**。

这种方式借鉴了结构体字段注入的 `autowire`/`value` tag，
但不需要在函数参数上加标记，也不需要额外的语法标识，容易理解和扩展。

Go-Spring 提供了几种不同的 `Arg` 实现：

- `TagArg`：用来绑定 Bean 依赖注入，或者绑定配置属性
- `ValueArg`：用于绑定一个固定值
- `BindArg`：用于绑定 Option 模式构造函数的参数

##### 注入 Bean

当一个 Bean 需要另一个 Bean 才能工作时，我们可以用 `gs.TagArg` 来声明。

```go
// UserController 需要 UserService 才能工作
type UserController struct {
	service *UserService
}

func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// TagArg 的参数为空字符串表示仅按类型匹配，不需要名称限定
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

在这种情况下，TagArg 的字符串参数和结构体字段的 `autowire` tag 是等价的。
具体语法参考 [结构体字段注入](#结构体字段注入)。

另外，对于上面的示例，因为只有一个参数，而且默认按类型匹配就行，不需要限定名称，
所以我们可以省略 `TagArg` 参数：

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	// 按类型自动匹配，可以省略 TagArg
	gs.Provide(NewUserController)
}
```

我们也可以按名称注入同一个类型的多个不同 Bean：

```go
// 注册两个不同名称的 DataSource
func init() {
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}

// 注入名称为 "slave" 的 DataSource
func NewRepository(ds *DataSource) *Repository {
	return &Repository{ds: ds}
}

func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

我们还可以从配置中读取 Bean 的名称：

```go
// 从配置 "storage.provider" 读取要注入的 Bean 名称
func NewService(storage Storage) *MyService {
	return &MyService{storage: storage}
}

func init() {
	// 配置文件中指定 storage.provider=redis，就会注入名称为 redis 的实现
	gs.Provide(NewService, gs.TagArg("${storage.provider}"))
}
```

##### 注入配置项

`TagArg` 除了用来绑定 Bean 依赖注入，也可以用来绑定配置属性。
Go-Spring 支持从配置系统读取配置项的值，自动转换为对应参数类型，注入到构造函数参数中。

```go
// RedisClient 需要从配置读取端口和地址
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
		gs.TagArg("${redis.host}"), // 从配置项 "redis.host" 读取主机地址
		gs.TagArg("${redis.port}"), // 从配置项 "redis.port" 读取端口
	)
}
```

我们可以使用如下的 YAML 配置来注入 RedisClient：

```yaml
redis:
  host: localhost
  port: 6379
```

TagArg 支持完整的配置项绑定语法，因此也支持默认值。

```go
func NewRedisClient(host string, port int) *RedisClient {
	return &RedisClient{host: host, port: port}
}

func init() {
	gs.Provide(NewRedisClient,
		// 从配置项 "redis.host" 读取主机地址，默认值为 localhost
		gs.TagArg("${redis.host:=localhost}"), 
		// 从配置项 "redis.port" 读取端口，默认值为 6379
		gs.TagArg("${redis.port:=6379}"), 
	)
}
```

我们也可以直接让容器绑定整个配置对象到一个结构体参数上：

```go
// RedisConfig 定义 Redis 配置结构
type RedisConfig struct {
	Host        string        `value:"host,default=localhost"`
	Port        int           `value:"port,default=6379"`
	Password    string        `value:"password"`
	DB          int           `value:"db,default=0"`
	Timeout     time.Duration `value:"timeout,default=5s"`
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

这种方式可以把一组相关配置集中管理，比分散注入多个参数更清晰整洁。

其他更高级的示例和说明可以参考 [01-configuration.md]。

##### 注入固定值

如果参数的值在注册 Bean 时就已经确定，不需要从容器获取，也不需要从配置读取，
那么我们可以用 `gs.ValueArg` 绑定一个固定值。

示例：db 参数固定绑定为 0

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
专门用来处理构造函数的**可选参数**问题。

例如，一个服务有很多配置项，但大部分配置项都有合理的默认值，
调用者往往只需要修改少数几个配置，
这时 Option 模式比重载多个构造函数或传入一大堆零值参数要优雅得多。

我们先来看一个典型的 Option 模式定义：

```go
// Option 定义配置函数类型，用于修改 Server 配置
type Option func(*Server)

// WithPort 创建一个设置端口的 Option
func WithPort(port int) Option {
	return func(s *Server) {
		s.port = port
	}
}

// WithTimeout 创建一个设置超时的 Option
func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// Server 是我们要创建的服务
type Server struct {
	port    int
	timeout time.Duration
}

// NewServer 接受可变参数 Option 来配置服务
func NewServer(opts ...Option) *Server {
	s := &Server{
		port:    8080,          // 默认端口
		timeout: 30 * time.Second, // 默认超时
	}
	// 应用所有 option
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

上面的代码展示了标准的 Functional Options 模式写法：
1. 定义 `Option` 函数类型，它接收 `*Server` 并修改其配置
2. 每个 `WithXXX` 函数创建一个 Option，设置对应的配置项
3. `NewServer` 接受可变参数 `opts ...Option`，创建 Server 后逐个应用所有 Option

现在问题来了：**如果 Option 创建本身需要依赖配置或其他 Bean 怎么办？**
例如 `WithPort` 需要的端口号来自配置文件，而不是写死在代码里。

Go-Spring 的 `BindArg` 可以解决这个问题：
它允许你把每个 Option 的创建交给容器处理，Option 创建过程中可以注入依赖或配置，
最终把生成好的 Option 传入 `NewServer` 构造函数。

来看一下具体用法：

```go
func init() {
	// 使用 BindArg 绑定每个 Option
	// WithPort 需要从配置读取端口，用 TagArg 绑定
	// WithTimeout 使用固定超时 60 秒，用 ValueArg 绑定
	gs.Provide(NewServer,
		gs.BindArg(WithPort, gs.TagArg("${server.port:=8080}")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

对于每个 `BindArg`，Go-Spring 会先执行绑定函数，为绑定函数的参数注入配置和依赖，
然后执行得到 Option 的值，再把这个 Option 作为参数传给主构造函数，
最终创建出配置好的 Server。

我们还可以为 `BindArg` 添加条件，**只有当条件满足时，这个 Option 才会被生成并传入构造函数**。
如果条件不满足，这个位置就会直接跳过，不传入任何 Option。

看下面这个例子：

```go
func init() {
	gs.Provide(NewServer,
		gs.BindArg(
			WithPort,
			// 只有配置了 server.port 才添加这个 Option
			gs.TagArg("${server.port:8080}")).Condition(gs.OnProperty("server.port")),
		),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

在上面这个例子中，只有当配置文件里明确设置了 `server.port` 配置项，`WithPort` Option 才会被添加进去；
如果没有配置，就直接使用 `NewServer` 里的默认值 8080。

这个特性非常有用，我们可以根据配置条件选择性地启用某些功能。

---

#### 参数顺序

默认情况下，`gs.Provide()` 传入的参数绑定是**按顺序匹配**构造函数的参数：
第一个绑定匹配第一个参数，第二个绑定匹配第二个参数，依此类推。

但如果构造函数参数很多，只需要给少数参数显式绑定，其他的让容器按类型自动推断，
这时可以用 `gs.IndexArg` 来指定绑定位置，不需要按顺序传入。

```go
// 构造函数有三个参数：a, b, c
func NewBean(a ServiceA, b ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	// 只给第三个参数（index=2，从 0 开始计数）绑定固定值
	// a 和 b 自动按类型推断为 Bean 依赖注入
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

通过这种方式，我们可以在保持构造函数原有参数顺序的同时，灵活选择需要显式绑定的参数，
剩下的交给容器自动按类型推断就行。

### 函数指针

除了结构体指针和构造函数，Go-Spring 还支持直接注册**函数指针**作为 Bean。
这种方式适用于**函数本身就是需要被注入的组件**的场景。

由于函数指针和构造函数在 Go 类型系统中都是函数类型，容器无法直接区分。
因此，**必须用 `reflect.ValueOf` 对函数指针进行封装**，明确告诉容器这个函数就是 Bean 本身。

最常见的场景是：我们需要在某个组件中注入一个函数，而这个函数由其他地方提供：

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

// -------------- 实现 --------------
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

---

## Bean 配置

注册 Bean 之后，我们常常还需要做一些额外配置，比如
自定义名称、指定初始化方法、导出接口、附加条件、声明依赖等。

`gs.Provide()` 在注册完成后会返回 `*BeanDefinition`，
以上所有配置都可以通过**链式调用**来完成。

### 设置 Bean 名称

为了唯一标识容器中的每个 Bean，
Go-Spring 用**类型**和**名称**的组合来生成唯一标识符。

如果注册 Bean 时没有显式指定名称，
Go-Spring 会自动用类型的简短名称作为默认名称。
例如：
- 对于结构体指针 `*UserService`，默认名称是 `"UserService"`
- 对于接口 `UserService`，默认名称是 `"UserService"`

当同一个类型需要注册多个不同实例时（比如主库和从库两个数据源），
我们可以通过 `.Name()` 方法为每个 Bean 设置自定义名称：

```go
func init() {
	// 同一个 DataSource 类型，注册两个不同名称的 Bean
	gs.Provide(NewMasterDataSource).Name("master")
	gs.Provide(NewSlaveDataSource).Name("slave")
}
```

设置名称后，我们就可以通过名称明确指定要使用哪个实例：

```go
// 构造函数注入使用 TagArg 指定名称
func NewUserRepo(ds *DataSource) *UserRepo {
	return &UserRepo{ds: ds}
}

func init() {
	gs.Provide(NewUserRepo, gs.TagArg("slave"))
}

// 字段注入直接在 autowire tag 中指定名称
type UserRepo struct {
	ds *DataSource `autowire:"slave"`
}
```

### 设置生命周期回调

有时候，在 Bean 创建完成，并且所有依赖都注入完成后，我们还需要执行一些自定义的初始化逻辑。
同样，当容器退出时，我们也需要优雅地关闭资源。

Go-Spring 提供了**生命周期回调**机制来解决这两个问题。

**初始化回调** 是指在 Bean 创建并且**所有依赖注入完成之后**调用，
我们可以在这里做自定义初始化工作，比如建立数据库连接、把缓存数据加载到内存、
验证配置的正确性等。

**销毁回调** 是指在**容器退出**时调用，我们可以在这里优雅释放资源，
比如关闭数据库连接，把内存状态持久化到磁盘，停止后台任务等。

Go-Spring 支持两种配置生命周期回调的方式：一种是通过函数指针设置，另一种是通过方法名指定。

---

#### 方式一：通过函数指针设置

我们可以直接传入一个独立的函数指针，这个函数需要接收 Bean 实例作为参数。
可以没有返回值，也可以只返回 `error` 类型。

init 和 destroy 的函数签名规则都是一致的。

```go
type MyService struct {
	client *redis.Client
}

func NewMyService() *MyService {
	return &MyService{}
}

// 初始化函数，接收 bean 作为参数
func InitMyService(s *MyService) error {
	s.client = redis.NewClient(/* ... */)
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

如果 init 返回错误，容器会终止启动，初始化失败。
如果 destroy 返回错误，容器会记录日志，但不会阻塞容器退出。

#### 方式二：通过方法名指定

有时候，结构体可能已经有了自己的初始化方法和销毁方法，而且函数签名也符合要求，
这时候我们就可以用指定方法名的方式来配置生命周期回调。

```go
type MyService struct {
	client *redis.Client
}

// DoInit 自定义初始化方法，签名支持 `func()` 或 `func() error`
func (s *MyService) DoInit() error {
	s.client = redis.NewClient(/* ... */)
	return s.client.Ping().Err()
}

// Cleanup 自定义销毁方法
func (s *MyService) Cleanup() error {
	return s.client.Close()
}

func init() {
	gs.Provide((*MyService)(nil)).
		InitMethod("DoInit").     // 指定 DoInit 为初始化方法
		DestroyMethod("Cleanup")  // 指定 Cleanup 为销毁方法
}
```

### 导出为接口

我们经常会在 Go 程序中使用接口，那在 Go-Spring 中怎么注册接口类型呢？
Go-Spring 要求我们在注册 Bean 时，必须显式指定要导出的接口。代码如下：

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

func init() {
	// 注册结构体，并导出为 UserService 接口
	// 这样其他组件可以注入 UserService 接口，而不依赖具体实现
	gs.Provide(NewUserServiceImpl).Export(gs.As[UserService]())
}
```

上面的示例中，容器中会同时存在两个 Bean：一个是原始 Bean，一个是接口 Bean。
这两个 Bean 都可以被注入，具体用哪个由注入点指定。

为什么必须显式导出接口呢？
因为在 Go 中，结构体可能实现多个接口，甚至可能不小心就实现了某个接口，
为了避免自动推导出现意外，所以我们需要明确导出哪个接口给外界使用。

### 附加激活条件

有时候，我们需要根据环境变量、配置文件等条件，在特定情况下才注册 Bean。
这时我们可以通过 `.Condition()` 方法给 Bean 添加条件，让 Bean 在特定条件下才生效。

```go
func init() {
	// 只有在 dev profile 下才注册这个 Bean
	gs.Provide(NewDevLogger).Condition(gs.OnProfiles("dev"))
}
```

我们将在本文「条件注册」章节详细介绍条件的用法。

### 显式依赖声明

绝大多数情况下，Go-Spring 通过**注入关系自动推断依赖顺序**
—— 你注入了哪个 Bean，自然就会保证那个 Bean 先初始化。

但有时候，可能两个 Bean 之间并没有明确的依赖关系，但又需要控制彼此的初始化顺序。
这时我们可以用 `.DependsOn()` 方法来**强制声明 Bean 之间的依赖关系**，
确保被依赖的 Bean 先完成初始化。

`.DependsOn()` 用来告诉容器：**我虽然没有注入它，但我要求它先初始化完成**。

```go
type A struct {
	// ...
}
type B struct {
	// ...
}
func init() {
	// B 依赖 A，确保 A 先初始化
	gs.Provide(NewB).DependsOn(gs.BeanID[A]())
}
```

在上面的例子中，`B` 虽然没有直接依赖 `A`，但通过 `.DependsOn()` 方法，
确保 `A` 先初始化完成。

如果两个间接依赖的 Bean 都有销毁方法，容器也会保证先销毁依赖的 Bean，
再销毁被依赖的 Bean。

### 标记为根 Bean

在 Go-Spring 和其他框架集成时，可能不需要 Go-Spring 的 Runner 或 Server，
只是把 Go-Spring 当作一个 Bean 容器来用。
但由于 Go-Spring 采用**按需初始化**的机制，只有被标记为 root 的 Bean 才会被初始化。
这时候我们可以用 `app.Root()` 方法把一个 Bean 标记为根 Bean。

这样 Go-Spring 容器就会确保这个 Bean 及其依赖的 Bean 都被初始化。

```go
func main() {
	bootstrap := &Bootstrap{}

	// ...
	// 在中间的代码中使用 bootstrap 对象
	// ...

	gs.Configure(func(app gs.App) {
		// 将 Bootstrap 标记为根 Bean，容器一定会创建它
		app.Root(bootstrap)
	}).Run()
}
```

在上面的例子中，`bootstrap` 被标记为根 Bean，我们可以在 IoC 容器之外放心地使用它。

`---

## Bean 注册

Go-Spring 提供了多种注册 Bean 的 API，满足不同的使用场景。

### 通过 `gs.Provide()` 注册

我们可以在包的 `init()` 函数中调用 `gs.Provide()` 注册一个 Bean。
这是最基础、最常用的方式。但要注意，不能在应用启动之后调用，否则会直接 `panic`。

```go
package mypackage

import (
	"github.com/go-spring/spring-core/gs"
)

func init() {
	// 注册一个 Bean
	gs.Provide(NewUserService)
}
```

`gs.Provide()` 会把当前 Bean 记录到全局注册表中，然后在应用启动时被合并。
详细的工作原理可以参考 [Bean 注册原理](#bean-注册原理)。

绝大多数业务组件都应该使用这种方式，因为清晰简单。

### 通过 `gs.Module()` 注册

`gs.Provide()` 一次只能注册一个 Bean，但有时候我们需要批量注册 Bean 的功能。
所以 Go-Spring 提供了 `gs.Module()` 函数，用于批量注册 Bean。

`gs.Module()` 还支持**条件化注册**能力，方便根据配置动态决定注册哪些 Bean。

本质上，Module 是一组 Bean 的条件化注册单元，非常适合按条件按需启用功能。
因此它是 Starter 机制的完美抽象，第三方集成包通常用这种方式暴露功能。

`gs.Module()` 通过回调暴露的 `gs.BeanProvider` 对象来提供注册 Bean 的能力。
它也提供了一个 `r.Provide(...)` 方法，而且和 `gs.Provide()` 用法完全相同。

例如，在下面的模块中，只有配置了 `enable.redis=true` 才会注册 Redis 相关 Bean，
否则整个 Redis 模块都不生效。

```go
// RedisModule 根据配置决定是否注册 Redis 相关 Bean
func RedisModule(r gs.BeanProvider, p flatten.Storage) error {
	// 检查配置是否启用 Redis
	enabled, _ := conf.Get[bool](p, "enable.redis")
	if enabled {
		// 根据条件注册 Bean
		r.Provide(NewRedisClient)
		r.Provide(NewRedisCache)
		// ... 可以注册任意多个 Bean
	}
	return nil
}

func init() {
	// 注册 Module，只有条件满足才会执行 Module 函数
	gs.Module(gs.OnProperty("enable.redis"), RedisModule)
}
```

与 `gs.Provide()` 不同的是，`gs.Module()` 注册的 Bean 只对当前容器生效。
通常来说，我们不需要关注这个差异。

### 通过 `gs.Group()` 注册

`gs.Group()` 是 `gs.Module()` 的一个特殊用法，只能用于批量创建同类型的 Bean。
`gs.Group()` 用配置 key 作为 Bean 名称。

当我们需要根据配置字典创建多个同类型 Bean 时，用 `gs.Group()` 可以大大减少模板代码。
比如，配置多个 HTTP 客户端、多个数据源、多个缓存客户端，每个都有独立的配置参数。

```go
// 定义 HTTP 客户端配置结构
type HTTPClientConfig struct {
	BaseURL string        `value:"baseURL"`
	Timeout time.Duration `value:"timeout,default=30s"`
}

// 根据配置创建 HTTP 客户端
func NewHTTPClient(c HTTPClientConfig) (*http.Client, error) {
	return &http.Client{
		Timeout: c.Timeout,
	}, nil
}

func init() {
	// 从配置 "http.clients" 读取 map，
	// 每个 entry 创建一个 Bean，key 作为 Bean 名称
	gs.Group("${http.clients}", NewHTTPClient, nil)
}
```

对应的 YAML 配置：

```yaml
http:
  clients:
    serviceA:  # "serviceA" 自动成为 Bean 名称
      baseURL: "http://a.example.com"
      timeout: 30s
    serviceB:  # "serviceB" 自动成为 Bean 名称
      baseURL: "http://b.example.com"
      timeout: 60s
```

`gs.Group()` 处理完成后，容器中就有了两个 `*http.Client` Bean：
`serviceA` 和 `serviceB`，我们可以直接按名称注入。

```go
type MyService struct {
	clientA *http.Client `autowire:"serviceA"`
	clientB *http.Client `autowire:"serviceB"`
}
```

如果需要释放资源，我们还可以提供销毁函数，它对每个 Bean 都生效：

```go
func init() {
	gs.Group[HTTPClientConfig, *http.Client]("${http.clients}",
		NewHTTPClient,
		func(c *http.Client) error {
			return c.CloseIdleConnections() // 清理资源
		},
	)
}
```

### 通过 `Configuration` 注册

Configuration 模式允许一个**配置类（父 Bean）**导出多个**子 Bean**，
这样方便把同一功能领域的多个 Bean 组织在一起，
子 Bean 可以共享配置类中已经注入的配置参数。

例如，一个数据库配置类可以导出数据源、多个仓库、多个 mapper 等所有相关 Bean。

```go
// DatabaseConfig 是配置类（父 Bean），本身可以依赖注入
type DatabaseConfig struct {
	MaxOpenConns int `value:"db.max-open-conns,default=10"`
}

// 导出 DataSource Bean - 方法接收者 c 就是父 Bean
func (c *DatabaseConfig) NewDataSource() *DataSource {
	// 使用父 Bean 中的配置创建子 Bean
	return NewDataSource(c.MaxOpenConns)
}

// 导出 UserRepository Bean - 可以依赖其他子 Bean
func (c *DatabaseConfig) NewUserRepository(ds *DataSource) *UserRepository {
	return NewUserRepository(ds)
}

// 导出 OrderRepository Bean
func (c *DatabaseConfig) NewOrderRepository(ds *DataSource) *OrderRepository {
	return NewOrderRepository(ds)
}

func init() {
	// 注册配置类，设置 Configuration 后容器会自动扫描并导出所有符合条件的方法
	gs.Provide((*DatabaseConfig)(nil)).Configuration()
}
```

Configuration 模式是如何工作的呢？

1. 首先把配置类本身作为一个普通 Bean 注册到容器，然后调用 `.Configuration()` 方法标记它是一个配置类
2. 在解析阶段，容器会遍历所有已注册的 Bean，找出那些设置了 `Configuration` 的 Bean（即配置类）
3. 然后容器扫描该配置类的所有公开方法，根据 Includes 和 Excludes 设置的规则进行筛选
4. 扫描到的符合条件的方法都会被自动注册为一个独立的子 Bean，添加到容器的 Bean 列表中
5. 最后容器不管 Bean 是怎么来的，都按照统一的规则进行处理

方法的签名和构造函数的签名是一样的，可以只返回对象，也可以额外返回一个 error。
下面是一些符合条件的方法示例：

```go
// ✅ 符合：返回指针，无 error
func (c *Config) NewDataSource() *DataSource

// ✅ 符合：返回指针 + error
func (c *Config) NewDataSource() (*DataSource, error)

// ❌ 不符合：返回值不是指针
func (c *Config) DataSource() DataSource

// ❌ 不符合：返回值个数不对
func (c *Config) NewDataSource() (*DataSource, string, error)
```

如果我们需要自定义包含和排除规则，可以通过 `Includes` 和 `Excludes` 参数来实现。
Includes 默认只导出方法名匹配 `New.*` 的方法（即以 `New` 开头），
Excludes 默认不排除任何方法。

自定义包含和排除规则示例：

```go
func init() {
	gs.Provide((*DatabaseConfig)(nil)).
		Configuration(gs.Configuration{
			Includes: []string{"New.*", "Create.*"}, // 包含匹配这些正则的方法
			Excludes: []string{".*Internal$"},       // 排除匹配这些正则的方法
		})
}
```

### 通过 `app.Provide()` 注册

除了前面几种注册方式，我们还可以用 `gs.Configure()` 提供的回调函数来注册 Bean。
`gs.Configure()` 通过回调暴露了一个包含 provide 方法的 `gs.App` 对象。
我们可以用这个 provide 方法来注册 Bean。
而且这种方式注册的 Bean 只对当前应用实例可见。

```go
func main() {
	gs.Configure(func(app gs.App) {
		// 在这个回调中注册的 Bean 只对当前应用可见
		app.Provide(NewAppSpecificComponent)
		app.Property("server.port", "8080")
	}).Run()
}
```

这种方式通常用来注册应用特定的入口组件和配置。
在单元测试中也经常用到，比如可以注册 Mock Bean，这样可以保持单测之间的数据隔离。

---

## 条件注册

有时候，我们想让 Bean 只在满足特定条件时才生效，就可以通过 **Condition** 机制来实现。

Go-Spring 提供了丰富的条件实现和组合工具，开箱即用。
我们可以根据配置属性判断，也可以根据 Bean 是否存在判断，
还可以通过 `And`/`Or`/`Not`/`None` 组合多个条件，实现复杂的判断逻辑。

#### 绑定条件

在注册 Bean 的时候，我们可以通过 `.Condition()` 方法为注册的 Bean 绑定条件：

```go
gs.Provide(NewMyService).Condition(gs.OnProperty("my.condition"))
```

### 常用条件

Go-Spring 提供了一些经常使用的条件。

#### 属性条件

`OnProperty` 是最常用的条件类型，
它可以根据配置属性是否存在或等于特定值来判断是否满足条件。

```go
// 只要配置项存在就满足条件，可以是叶子节点，也可以是路径节点
gs.OnProperty("enable.redis")

// 只有当配置项等于特定值时才满足条件
gs.OnProperty("env").HavingValue("prod")

// 如果配置项不存在也满足（匹配缺失）
gs.OnProperty("optional.feature").MatchIfMissing()
```

`OnProperty` 还支持表达式判断，只要在值前面加上 `expr:` 前缀就行：

```go
// 匹配端口大于 8080 的情况
gs.OnProperty("server.port").HavingValue("expr:$ > 8080")
```

##### 表达式语法

`OnProperty` 支持表达式语法，可以实现除了存在性判断以外的其他校验逻辑。
表达式使用 [expr-lang/expr](https://github.com/expr-lang/expr) 引擎进行计算，语法简洁自然：

- **`$`** - 代表当前配置属性的值，所有表达式围绕 `$` 进行比较判断
- **比较运算符**：支持 `>` `<` `>=` `<=` `==` `!=` 等常见比较操作
- **逻辑运算符**：支持 `&&` `||` `!` 进行逻辑组合
- **字符串操作**：支持 `contains` `hasPrefix` `hasSuffix` 等方法

**常用示例：**

```go
// 端口大于 1024 且小于 65535
gs.OnProperty("server.port").HavingValue("expr:$ > 1024 && $ < 65535")

// 环境不是生产环境
gs.OnProperty("app.env").HavingValue("expr:$ != 'prod'")

// 配置项以 "http://" 开头
gs.OnProperty("app.base-url").HavingValue("expr:$ hasPrefix 'http://'")

// 配置项包含指定关键字
gs.OnProperty("app.features").HavingValue("expr:$ contains 'debug'")
```

我们也可以用 `gs.RegisterExpressFunc()` 注册自定义函数，然后在表达式中使用：

```go
func init() {
	// 注册自定义判断函数
	gs.RegisterExpressFunc("isValidPort", func(port int) bool {
		return port > 1024 && port < 65535
	})
	
	// 在表达式中使用自定义函数
	gs.Provide(NewServer).Condition(
		gs.OnProperty("server.port").HavingValue("expr:isValidPort($)"),
	)
}
```

表达式必须返回布尔值，这样才能作为条件使用。

#### Bean 存在条件

Go-Spring 提供了一些根据容器中是否已存在特定 Bean 来判断的条件：

```go
// 只有当 UserService 已经注册且至少有一个匹配时才满足
gs.OnBean[UserService]()

// 只有当 UserService 未注册时才满足
gs.OnMissingBean[UserService]()

// 只有当恰好有一个 UserService 注册时才满足
gs.OnSingleBean[UserService]()

// 精确到名称，检查指定名称的 Bean 是否存在
gs.OnBean[DataSource]("master")
```

`OnBean[T]()` 表示容器中至少有一个匹配的 Bean。
`OnMissingBean[T]()` 表示容器中没有匹配的 Bean。
`OnSingleBean[T]()` 表示容器中恰好有一个匹配的 Bean。

我们在使用这三个条件的时候，可以传 Bean 的名字，也可以不传 Bean 的名字。
不传 Bean 名字表示仅按类型匹配，传 Bean 名字表示按类型和名称一起匹配。

#### 自定义函数条件

在一些简单的情况下，我们可以用 `OnFunc` 包装自定义函数来实现条件判断：

```go
gs.OnFunc(func(ctx gs.ConditionContext) (bool, error) {
	// 自定义任意判断逻辑
	return myCustomCheck(ctx)
})
```

#### 组合条件

Go-Spring 还提供了四种逻辑组合：`And`/`Or`/`Not`/`None`。

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

这些组合条件还可以配合使用，实现更复杂的逻辑。

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

当多个条件之间只有 and 逻辑时，我们可以省略 `gs.And` 封装：

```go
// 启用了服务且配置了 Config Bean 时才满足
gs.And(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
)
```

需要提醒的是，虽然组合条件可以实现复杂的逻辑，但通常来说，我们不应该使用复杂的条件。
如果实现比较复杂，应该考虑方案是否可以简化。

#### 缓存条件结果 `OnOnce`

有时候，条件的计算可能比较复杂，而且需要在多处共享使用，为了避免重复计算，
我们可以用 `OnOnce` 来把结果缓存起来，后续直接返回：

```go
// 条件只计算一次，之后缓存结果
gs.Provide(NewService).Condition(gs.OnOnce(
	gs.OnProperty("enable.service"),
	gs.OnBean[Config](),
))
```

绝大多数情况下，简单条件不需要缓存结果，只有当条件比较复杂，而且需要在多处共享使用时才需要。

### Profile 条件

对于按环境（开发/测试/生产）选择性启用 Bean 的场景，
Go-Spring 提供了直接的链式调用 `.OnProfiles()`，比显式设置条件更简洁。

```go
func init() {
	// 只在 dev 环境启用
	gs.Provide(NewDevLogger).OnProfiles("dev")
	// 同时支持多个 profiles，满足任一即可
	gs.Provide(NewTestLogger).OnProfiles("dev", "test")
}
```

本质上，Profile 条件是基于 `spring.profiles.active` 配置项来判断的。
如果 Profile 条件设置的 profiles 与激活的 profiles 中有任意一个匹配，条件就满足。

> `spring.profiles.active` 表示当前激活的 profiles 列表，如 `"dev,test,local"`。

---

## 容器原理

了解容器的执行原理可以帮助我们更好地理解和使用 Go-Spring。

### 生命周期

Go-Spring IoC 容器从程序启动到退出，会经历如下完整的**生命周期**阶段。

#### 注册阶段

Bean 的注册分为两个独立阶段，从根本上是为了支持单元测试的数据隔离而设计的。

##### 全局注册

我们在 `init()` 中调用 `gs.Provide()`、`gs.Module()` 完成 Bean 注册。
这种方式产生的注册信息保存在全局注册表中，对后续所有新建的容器都可见。
这是单元测试能够隔离的基础：每个测试用例都会创建新容器，只会使用全局注册的 Bean。

##### 容器注册

最典型的使用场景是单元测试：你可以注册自定义的 Mock Bean 来覆盖全局注册的默认实现。
还有一种使用场景是和其他框架融合使用的时候，仅作为 IoC 容器使用，对外部注册的 Bean 进行注入。

#### 解析阶段

容器启动后，第一步是对所有 Bean 进行合并和解析处理，这个阶段按顺序执行四个步骤：

##### Bean 合并

这一步把所有来源的 Bean 合并到一起：

- 使用 `app.Provide()` 注册的 Bean
- 使用 `gs.Provide()` 注册的 Bean
- 通过 `gs.Module()` 或 `gs.Group` 注册的 Bean
- 通过 `Configuration` 模式导出的 Bean

合并之后得到一份完整的等待处理的 Bean 列表，进入下一步处理。

##### 条件裁剪

这一步会遍历所有的 Bean，依次执行它们的 Condition 判断。
最终，满足条件的 Bean 保留，不满足条件的 Bean 被剔除，后续也不会被创建。
这样就实现了"根据条件动态决定哪些 Bean 生效"，这是多环境配置的基础。

##### 冲突检测

在 Go-Spring 中，类型和名称完全一致的 Bean 被视为冲突。
Go-Spring 严格遵循**不允许覆盖**的原则，一旦检测到冲突，容器直接启动失败并报错。

---

#### 注入阶段

解析完成后，容器会从 root beans 出发，按照依赖顺序递归创建 Bean 实例并完成依赖注入。
核心步骤如下：

1. 首先对所有 Bean 建立按类型和名称的索引，方便后续快速查找匹配的 Bean。
2. 然后从 root beans 开始，按照依赖顺序递归创建 Bean 实例并完成依赖注入。
   - 处理每个 Bean 时，先创建 Bean 实例，然后完成依赖注入。
   - 注入过程中会检查是否存在循环依赖，如果发现存在循环依赖，就会直接报错。
   - 注入过程中也会记录需要延迟注入的字段，以便后续统一处理。
   - 注入过程中容器还会记录 Bean 之间 destroy 方法的执行顺序。
   - 注入完成后，容器会调用 Bean 的 `init` 方法，完成初始化。
3. 当所有 Bean 都处理完成后，容器会统一处理标记为延迟注入的字段，至此所有依赖注入完成。
4. 对注入过程中记录的 destroy 方法进行拓扑排序，确保**被依赖的 Bean 先销毁**。
5. 容器清理不再需要的元数据，释放不必要的内存。

注入阶段完成后，**所有需要的 Bean 都已创建完成，依赖注入和初始化全部完成**，应用进入运行阶段。

---

#### 运行阶段

Go-Spring 在运行阶段不参与业务逻辑的处理，也就是说你不能在运行时动态获取 Bean 实例。
这种设计是刻意为之的：
一方面从 Java Spring 的实现来看，支持运行时获取 Bean 实例会让实现变得极度复杂，
另一方面 Go 开发者非常讨厌运行时使用反射，这种设计从根本上杜绝了运行时的反射操作。

换句话说，Go-Spring 遵循**启动期模型**设计：
即**所有依赖只在启动期完成注入，注入完成后容器不再参与运行**。

这种设计还有其他优点，
比如所有依赖错误在启动阶段就能发现，而不会带到运行时才出问题，
比如 Go-Spring 可以在内存受限的环境中使用，比如嵌入式系统。

---

#### 关闭阶段

当应用收到退出信号（比如 SIGINT）时，会通知容器进入关闭阶段，
这时容器会保证按正确顺序销毁所有 Bean。

---

### 核心设计

在实现完整生命周期的过程中，Go-Spring 做了一系列重要的设计决策，
这些决策决定了容器的整体工作方式。理解这些设计决策能帮助我们更好地使用 Go-Spring。

#### 接口分离

无论一个结构体实现了多少接口，Go-Spring 都会把**接口 Bean** 和**原始 Bean** 当作不同实体，
如果我们需要用接口进行注入，就必须用 Export 方法显式导出接口。

这样设计是因为 Go 没有 `implements` 关键字，自动推导容易产生意外，
如果有个结构体碰巧实现了接口，但它并不是我们想要的，就会导致问题。

```go
type Service interface {
	Do()
}

typeServiceImpl struct {}

func init() {
	// 只注册了 *ServiceImpl，没有导出 Service 接口
	// 其他组件无法注入 Service 接口，只能注入 *ServiceImpl
	gs.Provide(NewServiceImpl)
}

// 正确方式：显式导出
func init() {
	gs.Provide(NewServiceImpl).Export(gs.As[Service]())
}
```

#### 按需创建

在大型项目中，依赖和组织方式非常复杂，很可能引入了一些 Bean 但实际并没有用到。
为了解决这种情况，Go-Spring 支持了按需创建的策略。
也就是说，在 Go-Spring 中，只有被依赖的 Bean 才会创建，没有被依赖的 Bean 不会实例化。

那么这时候就有一个问题：我们要从哪些 Bean 开始处理，才不会导致用到的 Bean 被遗漏呢？
Go-Spring 引入了 **root bean** 的概念，也就是依赖树的根节点，
容器会从这些 Bean 开始递归处理被依赖的 Bean。

那什么样的 Bean 才是 root bean 呢？
- 首先，实现了 `gs.Runner` 或 `gs.Server` 接口的 Bean 是 root bean，它们由容器自动收集，
在应用启动过程中具有特殊作用。
- 其次，通过 `app.Root()` 显式标记的 Bean 也是 root bean，这些 Bean 通常在 IoC 容器之外使用，
把它注册进容器只是为了完成依赖注入。

#### 循环依赖

Go-Spring 能有限支持几种循环依赖场景：

- 假设 A 通过字段注入依赖了 B，而 B 又通过字段注入依赖了 A，这种情况是支持的。
- 假设 A 通过字段注入依赖了 B，而 B 又通过构造函数注入依赖了 A，这种情况下在注入过程中，A 或者 B 可能是不完整的，算是有限支持。
- 假设 A 通过构造函数依赖了 B，而 B 又通过构造函数注入依赖了 A，这种情况是不支持的，因为 A 和 B 都不能是完整的。

那么，如何解决循环依赖呢？(暂时不答)

#### 销毁顺序

Go-Spring 严格遵循**依赖逆序**原则管理销毁顺序：即**被依赖的 Bean 先初始化，后销毁**。

举个例子：如果 A 依赖 B，那么：

- 初始化顺序是 B 必须先创建好，才能注入给 A，
- 销毁顺序是 A 先销毁，释放对 B 的依赖，B 最后再销毁。

这种顺序保证了**销毁过程中依赖始终可用**，不会出现"在已经销毁的 Bean 上访问资源"的问题。

---

## 常见问题

1. 支持运行时动态获取 Bean 吗？为什么不提供 `getBean()`？

Go-Spring **不支持**运行时动态获取 Bean（即不提供 `getBean()` 这类 API），这是主动的设计选择：

1. **实际需求很少**：绝大多数服务端应用，所有组件都在启动时确定，不需要运行时动态获取
2. **避免复杂魔法**：所有依赖都在启动期解析完成，运行时代码不依赖容器，可以直接调用，更加清晰简单
3. **运行时零开销**：启动完成后容器清理元数据不再介入，性能和手写代码完全一致
4. **问题早暴露**：所有依赖错误在启动阶段就能发现，不会带到运行时才出问题
5. **依赖显式声明**：所有依赖都在代码中显式声明，比运行时动态获取更清晰，更容易调试

如果确实需要在运行时动态创建实例，推荐使用**工厂模式**：
即注册一个工厂单例，然后由工厂帮我们创建新实例。

---

2. 为什么接口需要显式导出？不能自动推导吗？

在 Go 里面，没有 `implements` 关键字，自动推导容易产生意外。
另外，大部分情况下我们不需要使用接口，Go 更推荐直接使用结构体。

---

3. 支持循环依赖吗？

Go-Spring 能自动解决部分循环依赖。

---

4. 怎么进行单元测试？

详见独立文档 [07-testing.md](./07-testing.md)，其中详细讲解了：

- 不使用 IoC 的纯粹单元测试写法（推荐，最简单直接）
- 使用 IoC 容器的集成测试写法
- 如何在测试中使用 Mock Bean 覆盖全局注册的默认实现
- Go-Spring 内置的断言工具和 Mock 框架的使用

---

5. 支持原型模式（多实例）吗？

Go-Spring 核心**只支持单例**，不直接提供原型模式。
这是贴合 Go 语言特点的设计选择：

1. **需求场景不同**：原型模式在 Java 中有时会用到，但在 Go 服务端开发中，绝大多数场景下单例就足够了，动态创建多实例的需求很少
2. **Go 有更自然的方式**：如果确实需要动态创建多个实例，我们可以自己实现一个工厂 Bean，把工厂注册为单例，然后由工厂帮我们创建新实例：

```go
// 工厂本身是单例
type MyServiceFactory struct {
	// 工厂可以依赖容器中的其他组件
	config Config `autowire:""`
}

// 需要新实例时，调用工厂方法创建
func (f *MyServiceFactory) NewMyService(...) *MyService {
	return &MyService{
		// 工厂负责创建和初始化
		config: f.config,
		// ...
	}
}

func init() {
	gs.Provide(new(MyServiceFactory))
}
```

简言之：真需要多实例，用工厂模式自己解决就行，IoC 容器核心不需要做这个。

---

6. 性能怎么样？运行时有反射开销吗？

Go-Spring 的设计保证了**运行时零反射开销**：

- **所有依赖注入只在启动期执行一次**，启动完成后容器就清理了大部分元数据，不再介入运行
- **运行时代码直接调用方法**，和手写代码没有区别，没有任何反射查找
- 唯一用到反射的地方是启动期解析依赖和注入字段，这只执行一次

所以启动后性能和不用 IoC 容器完全一致，不用担心性能问题。

---

7. 为什么用 `init()` 手动注册，而不是自动包扫描？

首先，**Go 本身从技术上就做不到真正的"自动包扫描"**：

- Go 链接器只会把我们 `import` 的包包含进二进制，如果没有 `import`，包代码根本不会被编译进去
- 即便是做自动注册，我们也总得写 `import _ xxx` 把包引入，才会执行注册逻辑

既然都必须写 `import` 了，不如更进一步，直接在 `init()` 里手动调用 `gs.Provide()` 完成注册：

1. **完全显式**：哪些 Bean 需要注册，哪些不需要，代码里一目了然，不用猜
2. **清晰可控**：不需要额外的注解配置，也不需要运行时扫描文件系统，符合 Go "显式优于隐式" 的哲学
3. **启动更快**：所有注册信息在 Go 程序启动时通过 `init()` 自动就准备好了，不需要再做一遍扫描

当然，如果我们嫌手动写麻烦，也可以用第三方工具扫描代码生成注册代码，这并不影响核心设计。

这是 Go-Spring 的核心设计原则：**所有 Bean 必须在启动前完成注册**。好处是：

1. **可预测**：所有 Bean 在程序启动时就已经注册完毕，后续不会动态添加
2. **单元测试隔离**：每个测试用例创建新容器时，都是从干净的全局注册开始，保证测试隔离
3. **符合 Go 语言习惯**：Go 本身就是通过 `init()` 做包级初始化，这是最自然的方式

---

8. 和 Wire 这类编译期 DI 有什么区别？

| 特性 | Go-Spring | Wire |
|------|-----------|------|
| 处理时机 | 运行期反射 | 编译期生成代码 |
| 条件注册 | ✅ 原生支持 | ❌ 不支持 |
| 动态配置 | ✅ 原生支持 | ❌ 不支持 |
| 生命周期管理 | ✅ 完整支持 | ❌ 不支持 |
| 启动速度 | 足够快 | 更快 |
| 二进制大小 | 适中 | 更小 |

Wire 的优势是完全编译期检查，对启动速度和二进制大小要求极高的场景很适合。  
Go-Spring 的优势是支持条件注册、动态配置、完整的生命周期管理，更适合开发大型服务端应用。

Go-Spring 虽然用了运行期反射处理依赖注入，但只在启动阶段使用，运行时没有任何开销，所以性能上和编译期 DI 没有区别。

---

9. 什么是 root bean？什么时候需要手动标记？

root bean 是依赖树的起点，容器一定会创建它，即使没有其他 Bean 依赖它。

以下情况**自动成为 root bean**，不需要手动标记：
- 实现了 `gs.Runner` 接口的 Bean
- 实现了 `gs.Server` 接口的 Bean（比如 HTTP 服务器）

如果我们有一个组件需要容器主动创建，但它不属于以上类型，可以手动标记：

```go
func main() {
	gs.Configure(func(app gs.App) {
		app.Root(MyComponent)
	})
}
```

---

掌握了 IoC 容器，我们就掌握了 Go-Spring 的核心基础。下一篇我们介绍配置系统如何使用。
