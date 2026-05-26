# Go-Spring 实战第 12 课 —— Bean 类型：结构体指针、构造函数和函数

前面两篇文章，咱们讲了依赖注入的写法和注入目标，本篇咱们来讲一下 Bean 的类型，也就是，咱们可以向 IoC 容器中注册哪些类型的 Bean。

## 结构体指针

先看一个证明已有实例语义的例子：`new(MyService)` 在注册前已经创建了对象，Go-Spring 接手的是后续管理。

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

注册后，Go-Spring 仍然会对这个对象执行依赖注入和生命周期回调。也就是说，对象创建已经发生，但对象管理仍然交给容器。

这种方式适合简单组件，或者把既有对象纳入 Go-Spring 管理的集成场景。它的边界也很清楚：构造阶段已经结束，后续依赖只能通过字段注入或生命周期回调补齐。

## 构造函数

更常见的方式是注册构造函数。下面的例子证明 Go-Spring 会在创建 Bean 前先解析构造函数参数。

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

这里 `Dep` 是 `MyService` 创建的前提。Go-Spring 解析到 `NewMyService` 时，会先准备好 `Dep`，再调用构造函数得到 `*MyService`。

如果创建过程可能失败，构造函数可以返回 `error`。

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

配置校验、打开文件、建立连接这类动作都适合通过 `(T, error)` 表达失败。Go-Spring 收到错误后会终止启动，而不是让半初始化对象进入运行期。

构造函数的价值在于让依赖关系、创建结果和创建失败都留在普通 Go 函数签名里。容器只负责解析参数并调用它。

### 参数绑定

构造函数参数需要额外语义时，Go-Spring 使用 `Arg` 在注册阶段补齐。下面这个例子证明 `TagArg` 可以让参数直接从配置读取值。

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

这里 `WithPort` 的参数来自配置，`WithTimeout` 的参数来自注册期固定值。`BindArg` 还可以附加条件，只有条件满足时才生成对应 Option。这样已有的 Functional Options API 可以纳入容器注册，同时保留原来的调用形态。

## 函数

最后一种容易混淆的形态是函数值。Go-Spring 默认会把函数理解为构造函数；如果函数本身就是要注入的能力，需要用 `reflect.ValueOf` 明确表达。下面的例子证明注册目标是函数本身，而不是函数调用结果。

```go
type PasswordChecker func(username, password string) bool

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(BcryptPasswordChecker))
}
```

这个例子里，调用方需要注入的是 `BcryptPasswordChecker` 这个函数本身，而不是调用它得到的返回值。策略函数、校验函数、编码函数等函数式组件适合用这种方式进入容器。
