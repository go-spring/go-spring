# Go-Spring 实战第 12 课：Bean 创建方式：结构体、构造函数和函数指针如何进入容器

前面几篇一直在讲 Go-Spring 里“依赖怎么注入”。现在我们把视角往前挪一格，看看 Bean 本身是怎么来的。

Bean 注册看起来好像只是“把对象放进容器”，但对象从哪里来、什么时候创建、参数怎么注入，都会影响后续行为。所以，Go-Spring 注册 Bean 时支持多种输入形态，不同形态对应不同创建方式，也影响依赖注入和生命周期管理。

主要包括：

- 结构体指针。
- 构造函数。
- 函数指针。

其中构造函数 Bean 还涉及参数绑定规则，是理解 Go-Spring Bean 创建机制的重点。

这一篇的信息量会比前几篇更密一些。阅读时不用一次记住所有 `Arg`，先分清“对象怎么创建”和“构造参数怎么补充绑定信息”两件事，后面的 API 就容易归位。

实际选型可以先按这条顺序走：普通业务组件优先构造函数；已有对象或简单组件用结构体指针；只有函数本身就是策略或能力时，才把函数指针注册为 Bean。

## 直接注册已有结构体实例

最简单的方式是直接注册一个已经创建好的对象：

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

对象可以是临时创建的，也可以是外部已有的实例。注册后，Go-Spring 容器仍会对它执行依赖注入和生命周期回调。也就是说，对象虽然已经创建，但还没有脱离容器管理。

这种方式适合简单组件，或需要把既有对象纳入 Go-Spring 管理的集成场景。它的特点是对象创建已经发生，容器主要负责后续注入和管理。

## 构造函数是更推荐的创建入口

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

如果创建过程可能失败，例如配置校验、打开文件、建立连接，应该使用 `(T, error)` 表达失败。容器会在错误时终止启动。

构造函数让我们把依赖关系和创建失败都放在普通 Go 函数签名里，Go-Spring 容器只负责解析参数并调用它。

## Arg 用来补充参数绑定信息

Go 不能给函数参数写 tag，因此 Go-Spring 通过注册阶段的 `Arg` 对参数进行绑定。接下来几个 `Arg`，不妨理解成给构造函数参数补充声明信息。

常用 `Arg` 包括：

- `TagArg`：绑定 Bean 依赖或配置属性。
- `ValueArg`：绑定固定值。
- `BindArg`：绑定 Functional Options 参数。
- `IndexArg`：按参数索引绑定。

参数绑定也可以先按使用目的来选：需要容器或配置参与时用 `TagArg`，注册时已经确定的值用 `ValueArg`，接入 Functional Options 用 `BindArg`，只想补充某个位置的参数时用 `IndexArg`。

### TagArg：按类型或名称注入 Bean

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

### TagArg：从配置读取参数

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

如果参数变多，我们通常应该优先聚合成配置结构体，而不是把大量基础类型直接塞进构造函数参数列表。

## ValueArg：传入注册期固定值

当参数值在注册时已经确定，可以使用固定值：

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合常量、测试替身或无需进入配置系统的参数。这样简单值不用绕进配置绑定流程。

## BindArg：接入 Functional Options

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

`BindArg` 还可以附加条件，只有条件满足时才生成对应 Option。这样已有的 Functional Options API 就可以比较自然地纳入容器注册。

## IndexArg：按参数位置补充绑定

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

## 函数指针也可以作为 Bean

最后还有一种容易混淆的形态：函数本身也可以作为 Bean 注入，但需要用 `reflect.ValueOf` 明确表示注册的是函数值，而不是构造函数：

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

## Bean 形态决定创建策略

常规业务组件优先使用构造函数。结构体指针适合简单对象和兼容集成。函数指针只在函数本身确实是可注入组件时使用。

构造函数参数一多，就不要把所有配置拆成大量基本类型参数。我们应该优先将相关配置聚合为结构体，这样配置协议会更清楚，后续演进也更稳。

Bean 创建方式确定以后，还需要通过名称、生命周期回调、接口导出、条件和 root 入口这些元信息，继续影响容器行为。
