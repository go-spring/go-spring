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
- 另一类是集成场景，对象已经由外部代码创建，但仍希望交给 Go-Spring 管理，这在和外部代码合作时非常常见。

## 构造函数

当我们为 `gs.Provide()` 传入一个构造函数时，Go-Spring 会在注入阶段调用它来创建对象。todo 不流畅。

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

这里 `NewMyService` 不是要被注入的 Bean，而是容器创建 Bean 的入口（即构造函数）。Go-Spring 在解析到这个 Bean 时，会先根据函数的参数准备好 `Dep` 依赖，然后调用构造函数得到 `*MyService` 对象。也就是说，我们注册的时候传入的是构造函数，最终被容器管理的是 `*MyService`。

对于复杂的构造过程，我们通常会返回 `error` 来表达失败。Go-Spring 也支持此类构造函数，即返回值可以是 `(T, error)`。

示例如下：

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

如配置校验、打开文件、建立连接等动作都适合通过 `(T, error)` 来表达失败。同时，Go-Spring 在收到非 nil 错误后会终止启动，而不是让半初始化的对象进入运行期。

构造函数的参数可能多种多样，Go-Spring 不对构造函数的参数做任何约束。只要 Go 支持的构造函数，Go-Spring 都能支持。

默认情况下，构造函数参数按类型从容器里解析。但函数参数不像结构体字段那样可以直接写 tag，所以当参数需要额外语义时，Go-Spring 使用 `Arg` 在注册阶段补齐。
为了支持任意类型的参数，Go-Spring 为 `gs.Provide(...)` 提供了 `Arg` 来绑定参数。具体实现有 `TagArg`、`ValueArg`、`IndexArg`、`BindArg` 等。


### `TagArg`

在为构造函数绑定参数的时候，最常见的是 `TagArg` 。它不仅可以让构造函数参数直接从配置读取值，还可以从容器中获取需要的 Bean 实例。

下面是读取配置的示例。

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

如果需要绑定的配置太多，我们可以把相关配置聚合成结构体，这样会更清楚。示例如下：

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

这里 `TagArg("${redis}")` 表示把 `redis` 子树绑定到 `RedisConfig`。

在上一篇中，我们详细介绍了 `TagArg` 在不同注入目标下的使用方式，这里就不重复展开了。

### `ValueArg`

如果构造函数的参数需要绑定一个固定值，我们可以使用 `ValueArg`。示例如下：

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

### `BindArg`

在 Go 里面，使用 Functional Options 模式的构造函数随处可见，Go-Spring 对这种情况也能很好的支持。

我们可以使用 `BindArg` 来绑定 Functional Options 模式的构造函数参数。示例如下：

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

在上面的代码中，`NewServer` 就是一个使用了 Functional Options 模式的构造函数。`gs.Provide(...)` 在注册 `NewServer` 的时候，使用 `BindArg` 为它绑定了 `WithPort` 和 `WithTimeout` 两个参数。Go-Spring 在创建 `*Server` Bean 时，如果发现使用了 `BindArg`，它就会根据 `BindArg` 的参数绑定规则，来调用对应的 `Option` 函数，然后将执行的结果作为构造函数的参数。

`BindArg` 还可以附加**条件（Condition）**，表示只有条件满足时才能生成对应的 Option。这种情况不太常见，这里就先不展开了。

### `IndexArg`

如果构造函数的参数比较少，我们可能会按顺序逐个对参数进行绑定。如果参数比较多，而且有些参数可以使用默认值，那么这时候我们可以使用 `IndexArg` 来绑定参数的下标，从而跳过不需要显式绑定的参数。

> 参数可以使用默认值通常是指参数是一个 Bean 注入，而且可以完全通过类型完成 Bean 的注入。像配置注入、固定值、Option 绑定，一般都是不能跳过的。

示例如下：

```go
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

在上面的代码中，我们只是将 `c` 的参数绑定为特殊指定的 `custom-value` Bean，`a` 和 `b` 都是由容器根据类型进行自动推断。

`IndexArg` 的下标符合 Go 语言的惯例，从 0 开始。

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
