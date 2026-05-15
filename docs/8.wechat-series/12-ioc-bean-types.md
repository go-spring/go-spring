# Go-Spring 实战第 12 课：Bean 创建方式：结构体、构造函数和函数值怎样决定容器创建策略

依赖注入解决的是对象之间怎么连接。再往前一步，Go-Spring 容器还要知道 Bean 自己从哪里来。

同样是注册到容器里，直接给一个结构体指针、给一个构造函数、给一个函数值，含义并不相同。它们分别决定对象是否已经创建、依赖是否要在创建时注入、失败是否能在启动阶段返回，以及函数本身是不是一个可注入能力。

所以看 Bean 注册时，先不要只看 `gs.Provide()`。更关键的是传进去的值是什么形态，因为形态决定了 Go-Spring 容器何时接手对象。

## 已有实例进入容器后只交出后续管理

最直接的方式是注册一个已经创建好的对象。下面的 `MyService` 在注册前已经完成实例化，Go-Spring 容器接手的是后续注入和生命周期。

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

注册后，Go-Spring 仍然会对这个对象执行依赖注入和生命周期回调。也就是说，对象创建已经发生，但对象管理仍然交给容器。

这种方式适合简单组件，或者需要把既有对象纳入 Go-Spring 管理的集成场景。它的边界也很明确——构造阶段已经结束，后续依赖只能通过字段注入或生命周期回调补齐。

## 构造函数把依赖和创建失败放进签名

更常见的方式是注册构造函数。下面这个例子把 `Dep` 放进构造函数参数，Go-Spring 容器在创建 `MyService` 前必须先解析出这个依赖。

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

如果创建过程可能失败，构造函数可以返回 `error`。

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

配置校验、打开文件、建立连接这类动作都适合通过 `(T, error)` 表达失败。这样错误会停留在启动阶段，容器收到错误后终止启动，而不是让半初始化对象进入运行期。

构造函数的价值在于让依赖关系、创建结果和创建失败都留在普通 Go 函数签名里。Go-Spring 容器只负责解析参数并调用它。

## Arg 的作用是补齐函数参数缺少的标签语义

Go 不能给函数参数写 tag，所以构造函数参数需要额外绑定信息时，Go-Spring 会在注册阶段通过 `Arg` 补充声明。

`Arg` 的选择可以按参数来源来判断。来自容器 Bean 或配置项时用 `TagArg`，注册时已经确定的值用 `ValueArg`，接入 Functional Options 时用 `BindArg`，只需要覆盖某个位置时用 `IndexArg`。

这个设计让构造函数保持普通 Go 写法，同时把容器绑定语义集中在注册语句里。

## TagArg 让构造参数参与 Bean 注入和配置绑定

当构造函数参数来自容器中的 Bean 时，`TagArg` 可以补充字段标签里原本能表达的注入语义。

```go
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

func init() {
	gs.Provide(NewUserController, gs.TagArg(""))
}
```

`TagArg("")` 表示仅按类型匹配。参数只有一个且可按类型自动推断时，这个 `TagArg` 可以省略。

同类型有多个候选时，把名称写进 `TagArg`，容器就会按这个名称选择目标 Bean。

```go
func init() {
	gs.Provide(NewRepository, gs.TagArg("slave"))
}
```

`TagArg` 也可以读取配置项并转换成目标类型。下面这个构造函数没有配置结构体，两个基础类型参数直接来自配置。

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

基础类型参数少时，这样写很直接。参数继续增加以后，相关配置聚合成结构体会更清楚。

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

这时问题已经不是 Go-Spring 能不能绑定，而是调用方能不能读懂这一组参数的含义。配置结构体把协议集中在一个类型里，后续新增字段也更稳。

## ValueArg 只承载注册期固定值

当参数值在注册时已经确定，可以使用 `ValueArg`。

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合常量、测试替身或无需进入配置系统的参数。它表达的是注册期固定值，所以不要把需要按环境变化的参数硬塞进这里。

## BindArg 把 Functional Options 转成容器可解析的参数

有些已有 API 使用 Functional Options。Go-Spring 不需要改变这些 API，而是通过 `BindArg` 生成对应的 Option 参数。

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

## IndexArg 只在指定位置需要覆盖时使用

默认参数绑定按顺序匹配。如果只想绑定某个位置，可以使用 `IndexArg`。

```go
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

索引从 0 开始。未显式绑定的参数仍由容器自动推断。`IndexArg` 适合少量参数需要覆盖的场景，如果每个位置都要靠索引解释，构造函数签名通常已经不够清楚。

## 函数值注册要明确它本身就是能力

最后一种容易混淆的形态是函数值。Go-Spring 默认会把函数理解为构造函数；如果函数本身就是要注入的能力，需要用 `reflect.ValueOf` 明确表达。

```go
type PasswordChecker func(username, password string) bool

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(BcryptPasswordChecker))
}
```

这种能力适合策略函数、校验函数、编码函数等函数式组件。关键判断是，调用方需要注入的是函数返回值，还是函数本身。

## 创建方式决定的是容器何时接手对象

结构体指针表示对象已经创建，Go-Spring 负责后续注入和生命周期。构造函数表示对象创建也交给容器，依赖和失败都在启动期处理。函数值表示函数本身就是运行期能力，需要被当作 Bean 注入。

Bean 创建方式确定以后，还需要通过名称、生命周期回调、接口导出、条件和 root 入口这些元信息，继续影响容器行为。
