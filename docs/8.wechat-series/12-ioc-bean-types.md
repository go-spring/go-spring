# Go-Spring 实战第 12 课 —— Bean 注册形态：结构体指针、构造函数和函数

前面两篇文章，咱们讲了依赖注入的写法和注入目标。到了注册阶段，还有一个容易说混的概念：`gs.Provide()` 接收的参数到底代表什么。

很多时候我们会顺口说“Bean 类型”，但这里的“类型”并不准确。Bean 自身的类型，是依赖图里最终可以被注入的类型，比如 `*MyService`、`UserService` 或 `PasswordChecker`。而 `gs.Provide()` 看到的，是注册时传进来的参数形态：可能是一个已经创建好的结构体指针，可能是一个用来创建 Bean 的构造函数，也可能是一个函数值本身。

所以这里把这个概念叫做 **Bean 注册形态**。它描述的是“容器怎样理解这条注册语句”，而不是“Bean 自己是什么类型”。这层概念分清以后，结构体指针、构造函数和函数这三种写法的边界就会清楚很多。

## 结构体指针

最直接的注册形态，是把一个结构体指针交给 Go-Spring。下面的例子证明对象在注册前已经创建完成，Go-Spring 接手的是后续管理。

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

这条注册语句里，`new(MyService)` 是注册参数的形态，Bean 自身的类型是 `*MyService`。Go-Spring 不会再调用构造函数创建这个对象，因为对象已经存在。容器要做的是把这个对象纳入依赖图，继续完成字段注入、生命周期回调、条件判断、名称和接口导出等容器语义。

因此，结构体指针适合两类场景。

一类是简单组件，对象没有复杂构造过程，只需要让容器补齐字段依赖。

另一类是集成场景，对象已经由外部代码创建，但仍希望交给 Go-Spring 管理。

它的边界也很明确：构造阶段已经结束，所以构造参数、构造失败和创建顺序都不能再通过这个注册形态表达。后续依赖只能通过字段注入或生命周期回调补齐。如果对象必须在创建时拿到依赖，或者创建过程可能失败，就应该把注册形态换成构造函数。

## 构造函数

更常见的注册形态，是把构造函数交给 Go-Spring。下面的例子证明注册参数是 `NewMyService`，但 Bean 自身的类型来自构造函数返回值。

```go
type MyService struct {
	dep Dep
}

func NewMyService(dep Dep) *MyService {
	return &MyService{dep: dep}
}

func init() {
	gs.Provide(NewMyService)
}
```

这里 `NewMyService` 不是要被注入的 Bean，而是容器创建 Bean 的入口。Go-Spring 解析到这个构造函数时，会先根据函数参数准备好 `Dep`，再调用构造函数得到 `*MyService`。也就是说，注册形态是构造函数，Bean 类型是 `*MyService`。

构造函数把对象创建过程留在普通 Go 函数里。依赖关系写在参数列表中，创建结果写在返回值中，容器只负责解析参数并调用它。

如果创建过程可能失败，构造函数可以返回 `error`。

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

配置校验、打开文件、建立连接这类动作都适合通过 `(T, error)` 表达失败。Go-Spring 收到非 nil 错误后会终止启动，而不是让半初始化对象进入运行期。

### 参数绑定

构造函数参数默认按类型从容器里解析。但函数参数不像结构体字段那样可以直接写 tag，所以当参数需要额外语义时，Go-Spring 使用 `Arg` 在注册阶段补齐。

下面的例子证明 `TagArg` 可以让构造函数参数直接从配置读取值。

```go
func NewRedisClient(host string, port int) *RedisClient {
	return &RedisClient{host: host, port: port}
}

func init() {
	gs.Provide(NewRedisClient,
		gs.TagArg("${redis.host:=localhost}"),
		gs.TagArg("${redis.port:=6379}"),
	)
}
```

`TagArg` 对基础类型参数执行配置绑定，对 Bean 类型参数执行依赖注入。它和字段上的 `value`、`autowire` 标签处在同一类语义里，只是函数参数不能写 tag，所以把声明放到了注册语句中。

基础类型参数很少时，这样写直接。参数继续增加以后，把相关配置聚合成结构体会更清楚。

```go
type RedisConfig struct {
	Host     string        `value:"${host:=localhost}"`
	Port     int           `value:"${port:=6379}"`
	Password string        `value:"${password:=}"`
	DB       int           `value:"${db:=0}"`
	Timeout  time.Duration `value:"${timeout:=5s}"`
}

func NewRedisClient(cfg RedisConfig) *RedisClient {
	return &RedisClient{host: cfg.Host, port: cfg.Port}
}

func init() {
	gs.Provide(NewRedisClient, gs.TagArg("${redis}"))
}
```

这里 `TagArg("${redis}")` 把 `redis` 子树绑定到 `RedisConfig`。问题已经不是 Go-Spring 能不能绑定，而是调用方能不能读懂这一组参数的含义。配置结构体把协议集中在一个类型里，后续新增字段也更稳。

### 固定值和位置绑定

并不是所有参数都应该进入配置系统。下面的例子证明 `ValueArg` 表达的是注册期已经确定的固定值。

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合常量、测试替身或无需按环境变化的参数。它表达的是“这个值在注册时已经确定”，所以不要把需要由部署环境控制的参数硬塞进这里。

如果只想覆盖某个位置，可以使用 `IndexArg`。下面的例子证明未显式绑定的位置仍然由容器自动推断。

```go
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

索引从 0 开始。`IndexArg` 适合少量参数需要覆盖的场景；如果每个位置都要靠索引解释，构造函数签名通常已经不够清楚。

### Functional Options

很多既有库使用 Functional Options。Go-Spring 不需要改造这些 API，而是通过 `BindArg` 生成对应的 Option 参数。

下面的例子证明 `BindArg` 可以先解析参数，再调用 `WithPort` 或 `WithTimeout` 生成构造函数真正需要的 `Option`。

```go
type Option func(*Server)

func WithPort(port int) Option {
	return func(s *Server) { s.port = port }
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) { s.timeout = timeout }
}

func NewServer(opts ...Option) *Server {
	s := &Server{port: 8080, timeout: 30 * time.Second}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func init() {
	gs.Provide(NewServer,
		gs.BindArg(WithPort, gs.TagArg("${server.port:=8080}")),
		gs.BindArg(WithTimeout, gs.ValueArg(60*time.Second)),
	)
}
```

这里 `NewServer` 仍然是构造函数注册形态。`WithPort` 和 `WithTimeout` 不是新的 Bean，而是参数绑定过程中的辅助函数：`WithPort` 的参数来自配置，`WithTimeout` 的参数来自注册期固定值，最后生成构造函数真正接收的 `Option`。

`BindArg` 还可以附加条件，只有条件满足时才生成对应 Option。这样已有的 Functional Options API 可以纳入容器注册，同时保留原来的调用形态。

## 函数

最后一种注册形态最容易和构造函数混淆：函数值本身也可以是 Bean。

Go-Spring 默认会把普通函数理解为构造函数，因为大多数 `gs.Provide(fn)` 都是在表达“调用这个函数得到 Bean”。如果调用方需要注入的正是这个函数本身，就要用 `reflect.ValueOf` 明确告诉 Go-Spring：这里注册的是函数值，而不是函数调用结果。

下面的例子证明注册目标是 `BcryptPasswordChecker` 这个函数本身。

```go
type PasswordChecker func(username, password string) bool

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(PasswordChecker(BcryptPasswordChecker)))
}
```

这个例子里，注册参数的形态是经过 `reflect.ValueOf` 包装的函数值，`PasswordChecker(BcryptPasswordChecker)` 明确了 Bean 自身的类型。依赖方要拿到的是一段可调用能力，而不是调用 `BcryptPasswordChecker` 以后得到的 `bool`。

策略函数、校验函数、编码函数、签名函数这类函数式组件适合用这种方式进入容器。它们没有对象状态，也不需要生命周期，但它们本身就是业务依赖的一部分。

这个边界很重要：构造函数注册的是“用这个函数创建 Bean”，函数值注册的是“这个函数就是 Bean”。两者在 Go 代码里都长得像函数，但在 Go-Spring 的注册语义里不是同一件事。

## Bean 注册形态

结构体指针、构造函数和函数，并不是三种 Bean 自身类型，而是三种 Bean 注册形态。

结构体指针表示对象已经创建，容器负责后续管理。构造函数表示对象由容器解析参数后创建，返回值才是 Bean 自身类型。函数值表示函数本身就是可注入能力，需要用 `reflect.ValueOf` 和构造函数语义区分开。

把这个概念单独拎出来，是为了避免把 `gs.Provide()` 的参数形态误认为 Bean 的运行类型。Go-Spring 真正装配的是 Bean 定义：注册形态说明容器怎样得到对象，Bean 类型说明对象怎样被依赖方引用。两者配合起来，才构成一条完整的注册语义。
