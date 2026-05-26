# Go-Spring 实战第 12 课 —— Bean 类型：结构体指针、构造函数和函数

前面两篇文章，咱们讲了依赖注入的写法和注入目标，本篇咱们来讲一下 Bean 的类型。不过需要澄清的是，这里的类型不是指 Bean 元数据里面的类型，而是指注册的时候 `gs.Provide()` 第一个参数的类型。因为用文字很难表述二者的区别，或者说我没有找到很好的表述方式，所以目前就这么说。这一点提前跟大家讲明白。

换句话说，本篇不是在讨论 Bean 最终以什么 Go 类型被依赖方引用，而是在讨论 `gs.Provide(...)` 里第一个参数可以怎么写。为了讲起来方便，后面我会把它们暂时称为三种 Bean 类型，即结构体指针、构造函数和函数。

## 结构体指针

它的特点是对象在注册前已经创建完成，`gs.Provide()` 接收到的是这个对象的地址，Go-Spring 接手的是后续的管理。

看个例子。

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

在上面的注册语句里，`new(MyService)` 是传给 `gs.Provide()` 的参数，它是一个结构体指针。这种情况下，Go-Spring 在注入阶段不会再创建 `MyService` 对象，因为对象已经存在了，我们拿来复用就好了。容器要做的是接管这个对象，继续完成字段注入、生命周期回调、条件判断、名称和接口导出等容器语义。

这种写法非常适合两类场景：
- 一类是简单组件，对象没有复杂的构造过程，只需要让容器补齐字段依赖即可。
- 另一类是集成场景，对象已经由外部代码创建，但仍希望交给 Go-Spring 管理。

## 构造函数

当我们为 `gs.Provide()` 传入一个构造函数时，Go-Spring 会在注入阶段调用它来创建对象。

看个例子。

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

这里 `NewMyService` 不是要被注入的 Bean，而是容器创建 Bean 的入口（构造函数）。Go-Spring 在解析到这个 Bean 时，会先根据函数参数准备好 `Dep`，然后调用构造函数得到 `*MyService`。也就是说，传进去的是构造函数，最终被容器管理的是 `*MyService`。

构造函数把对象创建过程留在普通 Go 函数里。依赖关系写在参数列表中，创建结果写在返回值中，容器只负责解析参数并调用它。

既然创建过程放在构造函数里，创建失败也应该从这里表达。构造函数可以返回 `error`。

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

配置校验、打开文件、建立连接这类动作都适合通过 `(T, error)` 表达失败。Go-Spring 收到非 nil 错误后会终止启动，而不是让半初始化对象进入运行期。

### 参数绑定

构造函数有了参数以后，下一件事就是参数从哪里来。默认情况下，构造函数参数按类型从容器里解析。但函数参数不像结构体字段那样可以直接写 tag，所以当参数需要额外语义时，Go-Spring 使用 `Arg` 在注册阶段补齐。

比如，`TagArg` 可以让构造函数参数直接从配置读取值。

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

并不是所有参数都应该进入配置系统。有些值在注册的时候就已经确定了，这时可以使用 `ValueArg`。

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合常量、测试替身或无需按环境变化的参数。它表达的是“这个值在注册时已经确定”，所以不要把需要由部署环境控制的参数硬塞进这里。

如果只想覆盖某个位置，可以使用 `IndexArg`。没有显式绑定的位置，仍然由容器自动推断。

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

还有一类常见情况是 Functional Options。很多既有库都使用这种 API，Go-Spring 不需要改造它们，而是通过 `BindArg` 生成对应的 Option 参数。

也就是说，`BindArg` 可以先解析参数，再调用 `WithPort` 或 `WithTimeout` 生成构造函数真正需要的 `Option`。

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

这里 `NewServer` 仍然是构造函数类型的 Bean 声明。`WithPort` 和 `WithTimeout` 不是新的 Bean，而是参数绑定过程中的辅助函数：`WithPort` 的参数来自配置，`WithTimeout` 的参数来自注册期固定值，最后生成构造函数真正接收的 `Option`。

`BindArg` 还可以附加条件，只有条件满足时才生成对应 Option。这样已有的 Functional Options API 可以纳入容器注册，同时保留原来的调用形态。

## 函数

最后再回到一个容易混淆的地方：如果 `gs.Provide()` 里传的是函数，它通常会被理解成构造函数。但函数本身也可以是 Bean。

Go-Spring 默认会把普通函数理解为构造函数，因为大多数 `gs.Provide(fn)` 都是在表达“调用这个函数得到 Bean”。如果调用方需要注入的正是这个函数本身，就要用 `reflect.ValueOf` 明确告诉 Go-Spring：这里注册的是函数，而不是函数调用结果。

比如下面这段代码，注册目标就是 `BcryptPasswordChecker` 这个函数本身。

```go
type PasswordChecker func(username, password string) bool

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(PasswordChecker(BcryptPasswordChecker)))
}
```

这个例子里，传给 `gs.Provide()` 的是经过 `reflect.ValueOf` 包装的函数，`PasswordChecker(BcryptPasswordChecker)` 明确了 Bean 自身的类型。依赖方要拿到的是一段可调用能力，而不是调用 `BcryptPasswordChecker` 以后得到的 `bool`。

策略函数、校验函数、编码函数、签名函数这类函数式组件适合用这种方式进入容器。它们没有对象状态，也不需要生命周期，但它们本身就是业务依赖的一部分。

这个边界很重要：构造函数注册的是“用这个函数创建 Bean”，函数注册的是“这个函数就是 Bean”。两者在 Go 代码里都长得像函数，但在 Go-Spring 的注册语义里不是同一件事。

## Bean 类型

回到开头那句话，结构体指针、构造函数和函数，是本文为了行文方便使用的三种 Bean 类型说法。严格一点讲，它们描述的是 `gs.Provide()` 第一参数的类型，而不是 Bean 最终被依赖方引用时的 Go 类型。

结构体指针表示对象已经创建，容器负责后续管理。构造函数表示对象由容器解析参数后创建，返回值才是 Bean 自身类型。函数表示函数本身就是可注入能力，需要用 `reflect.ValueOf` 和构造函数语义区分开。

把这层区别说清楚，是为了避免把 `gs.Provide()` 的参数写法误认为 Bean 的运行类型。读本文里的 Bean 类型时，可以先把它理解成一个注册入口的问题：这个参数告诉容器怎样得到对象，Bean 自身类型告诉依赖方怎样声明依赖。把这两件事分开，`gs.Provide()` 的几种写法就容易理解了。
