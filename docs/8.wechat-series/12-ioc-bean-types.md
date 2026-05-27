# Go-Spring 实战第 12 课 —— Bean 注册类型：结构体指针、构造函数和函数

前面两篇文章，咱们讲了依赖注入的写法和注入目标，本篇咱们来详细讲讲 Bean 的注册类型。

所谓注册类型，指的是注册 Bean 时使用的 `gs.Provide()` 函数的第一个参数的类型。这个参数可以是一个已经创建好的结构体指针，可以是一个用来创建对象的构造函数，也可以是一个函数本身。

## 结构体指针

结构体指针这种注册类型，特点是对象在注册前已经创建完成，`gs.Provide()` 接收到的是这个 Bean 对象的地址，Go-Spring 只需要负责后续的管理即可。

看个例子。

```go
type MyService struct {
	// ...
}

func init() {
	gs.Provide(new(MyService))
}
```

在上面的注册语句里，`new(MyService)` 是传给 `gs.Provide()` 的参数，它是一个结构体指针。这种情况下，Go-Spring 不会再创建 `MyService` 对象，因为对象已经存在了，我们直接拿来复用就好了。容器要做的是接管这个对象，继续完成字段注入、生命周期回调、条件判断、名称和接口导出等容器语义。

这种写法非常适合两类场景：
- 一类是简单组件，对象没有复杂的构造过程，只需要让容器补齐字段依赖即可。
- 另一类是集成场景，对象已经由外部代码创建了，但仍希望交给 Go-Spring 管理，这在和外部代码合作时非常常见。

## 构造函数

构造函数这种注册类型，是指把创建对象的函数交给容器。Go-Spring 会根据这个函数创建真正的 Bean 实例。也就是说，`gs.Provide()` 收到的是一个函数，但真正管理的是这个函数的返回值。

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

这里 `NewMyService` 是容器创建 Bean 的入口。Go-Spring 在解析到这个 Bean 时，会先根据函数的参数准备好 `Dep` 依赖，然后调用构造函数得到 `*MyService` 对象。

对于复杂的构造过程，我们通常会返回 `error` 来表达失败。Go-Spring 也支持此类构造函数，即返回值可以是 `(T, error)`。

示例如下：

```go
func NewMyService(dep Dep) (*MyService, error) {
	return &MyService{dep: dep}, nil
}
```

像配置校验、打开文件、建立连接这类动作都适合通过 `(T, error)` 来表达失败。Go-Spring 在收到非 nil 错误后会立即终止启动，而不是让半初始化的对象进入运行期，免得带来更复杂的错误。

### 参数绑定

在真实的代码中，构造函数的参数是多种多样的，可能是需要从容器里查找的 Bean，可能是需要从配置里读取的值，还可能是注册时就已经确定的固定值。Go-Spring 不要求我们为了容器必须修改构造函数的签名，而是通过参数绑定来补齐这些数据。

针对不同的参数绑定需求，Go-Spring 提供了 `TagArg`、`ValueArg`、`IndexArg`、`BindArg` 四种方案。

### `TagArg`

在为构造函数绑定参数的时候，最常见的是 `TagArg`。它不仅可以直接从配置里读取值，还可以从容器中获取需要的 Bean。

本质上，`TagArg` 是在模拟 Go 结构体字段上的 tag 语义。当我们使用 `TagArg("${...}")` 的时候，表示从配置中读取值。当我们使用 `TagArg("...")` 的时候，表示从容器中获取 Bean。

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

在上面的代码中，我们通过 `TagArg` 分别为构造函数的两个参数 `host` 和 `port` 绑定了配置值。如果配置值不存在，还可以使用默认值。

如果需要绑定的配置很多，我们可以把相关配置聚合成结构体，这样代码会更清楚。

示例如下：

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

在上面的代码里，`TagArg("${redis}")` 表示把 `redis` 配置子树绑定到 `RedisConfig` 结构体的字段上。

如果 `TagArg` 传递的是普通字符串，比如 `gs.TagArg("slave")`，那么它表达的就是按照名称来注入 Bean。由于在上一篇中，我们详细介绍了 `TagArg` 在不同注入目标下的使用方式，本篇就不再重复展开了。

### `ValueArg`

如果构造函数的参数需要绑定一个固定值，我们可以使用 `ValueArg`。

示例如下：

```go
func NewRedisClient(db int) *RedisClient {
	return &RedisClient{db: db}
}

func init() {
	gs.Provide(NewRedisClient, gs.ValueArg(0))
}
```

`ValueArg` 适合用于常量、测试替身，或者不需要随环境变化的参数。如果一个值需要由配置文件、环境变量或启动参数控制，就不应该硬写在 `ValueArg` 里，而是交给 `TagArg` 来绑定。

### `BindArg`

在 Go 里面，使用 Functional Options 模式的构造函数随处可见，Go-Spring 对这种情况也能很好地支持。我们可以使用 `BindArg` 为 Functional Options 模式的构造函数绑定参数。

示例如下：

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

在上面的代码中，`NewServer` 就是一个使用了 Functional Options 模式的构造函数。`gs.Provide(...)` 在注册 `NewServer` 的时候，使用 `BindArg` 绑定了 `WithPort` 和 `WithTimeout` 两个 Option。Go-Spring 在创建 `*Server` Bean 时，如果发现注册时使用了 `BindArg`，就会先调用 `WithPort`、`WithTimeout` 这两个函数，再把返回的 Option 作为构造函数的参数。

`BindArg` 自己的参数也可以是 `Arg` 类型，比如这里的 `TagArg` 和 `ValueArg`。原因也很直接：`WithPort`、`WithTimeout` 本身也是函数，它们的参数同样需要说明来源。

另外，`BindArg` 还可以附加**条件（Condition）**，表示只有条件满足时才能生成对应的 Option。这种情况不太常见，就先不展开了。

### `IndexArg`

如果构造函数的参数比较少，我们可能会按顺序逐个对参数进行绑定。但如果参数较多，而且有些参数可以由容器自动推断，那么这时候我们可以使用 `IndexArg` 绑定参数的下标，来跳过不需要显式绑定的参数。

> 这里说的跳过，通常是指参数可以完全通过类型完成 Bean 注入。像配置注入、固定值、Option 绑定，这些通常是不能跳过的。

示例如下：

```go
func NewBean(a *ServiceA, b *ServiceB, c string) *Bean {
	return &Bean{a: a, b: b, c: c}
}

func init() {
	gs.Provide(NewBean, gs.IndexArg(2, gs.ValueArg("custom-value")))
}
```

在上面的代码中，我们只是把下标为 2 的 `c` 参数绑定为固定值 `custom-value`，`a` 和 `b` 都是由容器根据类型自动推断的。

`IndexArg` 的下标符合 Go 语言的惯例，从 0 开始。

## 函数

在 Go 里面，函数也是一等公民，我们经常看到将函数作为参数传递和使用。因此，函数本身也可以作为 Bean 使用。

不过构造函数本质上也是函数。为了区分“调用这个函数创建 Bean”和“这个函数本身就是 Bean”这两种不同的语义，我们在注册函数 Bean 的时候需要使用 `reflect.ValueOf` 来包裹函数。

> 考虑到使用函数 Bean 的情况不多，Go-Spring 就不再提供专门的注册方法了。

看个例子。

```go
type PasswordChecker func(username, password string) bool

type Authenticator struct {
	checker PasswordChecker
}

func NewAuthenticator(checker PasswordChecker) *Authenticator {
	return &Authenticator{checker: checker}
}

func (a *Authenticator) Check(username, password string) bool {
	return a.checker(username, password)
}

func BcryptPasswordChecker(username, password string) bool {
	return true
}

func init() {
	gs.Provide(reflect.ValueOf(PasswordChecker(BcryptPasswordChecker)))
	gs.Provide(NewAuthenticator)
}
```

在上面的例子里，`Authenticator` 依赖的是 `PasswordChecker` 类型的 Bean。注册时，`PasswordChecker(BcryptPasswordChecker)` 明确了函数 Bean 的类型，Go-Spring 启动时就能把这个函数注入到 `Authenticator` 中。

通常来说，策略函数、校验函数、编码函数、签名函数这类函数式组件都很适合用这种方式进入容器。
