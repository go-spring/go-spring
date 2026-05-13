# Bean 类型与构造函数绑定

前面几篇一直在讲“依赖怎么注入”。现在我们把视角往前挪一格：Bean 本身是怎么来的。

Bean 注册看起来只是“把对象放进容器”，但对象从哪里来、什么时候创建、参数怎么注入，都会影响后续行为。Go-Spring 注册 Bean 时支持多种输入形态，不同形态对应不同创建方式，也影响依赖注入和生命周期管理。

主要包括：

- 结构体指针。
- 构造函数。
- 函数指针。

其中构造函数 Bean 还涉及参数绑定规则，是理解 Go-Spring Bean 创建机制的重点。

## 结构体指针

最简单的方式是直接注册一个已经创建好的对象：

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

对象可以是临时创建的，也可以是外部已有的实例。注册后，容器仍会对它执行依赖注入和生命周期回调。

这种方式适合简单组件，或需要把既有对象纳入 Go-Spring 管理的集成场景。它的特点是对象创建已经发生，容器主要负责后续注入和管理。

## 构造函数

更推荐的方式是注册构造函数：

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

构造函数也可以返回 `error`：

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

当创建过程可能失败时，例如配置校验、打开文件、建立连接，应该使用 `(T, error)` 表达失败。容器会在错误时终止启动。

构造函数让我们把依赖关系和创建失败都放在普通 Go 函数签名里，容器只负责解析参数并调用它。

## 参数绑定

Go 不能给函数参数写 tag，因此 Go-Spring 通过注册阶段的 `Arg` 对参数进行绑定。

常用 `Arg` 包括：

- `TagArg`：绑定 Bean 依赖或配置属性。
- `ValueArg`：绑定固定值。
- `BindArg`：绑定 Functional Options 参数。
- `IndexArg`：按参数索引绑定。

### TagArg 注入 Bean

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

`TagArg("")` 表示仅按类型匹配。参数只有一个且可按类型自动推断时，可以省略。

按名称注入时：

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

### TagArg 注入配置

`TagArg` 也可以读取配置项并转换成目标类型：

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

也可以把一组配置绑定成结构体参数：

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
	gs.Provide(NewRedisClient, gs.TagArg("redis"))
}
```

当参数变多时，我们通常应该优先聚合成配置结构体，而不是把大量基础类型直接塞进构造函数参数列表。

## ValueArg

当参数值在注册时已经确定，可以使用固定值：

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合常量、测试替身或无需进入配置系统的参数。

## BindArg

`BindArg` 用于 Functional Options 模式：

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

`BindArg` 还可以附加条件，只有条件满足时才生成对应 Option。这让已有的 Functional Options API 可以比较自然地纳入容器注册。

## IndexArg

默认参数绑定按顺序匹配。如果只想绑定某个位置，可以使用 `IndexArg`：

```go
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

索引从 0 开始。未显式绑定的参数由容器自动推断。

## 函数指针 Bean

函数本身也可以作为 Bean 注入，但需要用 `reflect.ValueOf` 明确表示注册的是函数值，而不是构造函数：

```go
type PasswordChecker func(username, password string) bool

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(BcryptPasswordChecker))
}
```

这种能力适合策略函数、校验函数、编码函数等函数式组件。

## Bean 形态背后是创建策略

常规业务组件优先使用构造函数。结构体指针适合简单对象和兼容集成。函数指针只在函数本身确实是可注入组件时使用。

构造函数参数较多时，不要把所有配置拆成大量基本类型参数。我们应该优先将相关配置聚合为结构体，这样配置协议会更清楚，后续演进也更稳。

下一步看这些 Bean 注册之后，还能附加哪些名称、回调、条件和入口信息。
