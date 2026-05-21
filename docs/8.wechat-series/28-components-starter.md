# Go-Spring 实战第 28 课 —— Starter 机制：把组件注册、配置和生命周期封装成可复用包

上一课的 HTTP Server 是 Go-Spring 内置组件。它给应用提供的不只是一个 `http.Server`，还包括配置前缀、默认启用条件、路由入口和关闭语义。把视角放到业务项目里，数据库、Redis、客户端、pprof 这类基础设施组件也会遇到同样的问题。

如果每个项目都手写一遍初始化代码，短期看只是多几行 `Provide`。但复制出去的其实还有配置命名、启用条件、默认 Bean 名称、多实例规则和资源释放方式。项目越多，这些约定越容易分叉。

Starter 要解决的是组件接入的复用问题。它不是另一套容器机制，而是把 Go-Spring 已有的 Bean 注册、配置绑定、条件判断和生命周期回调放进一个独立包。应用侧通过一次导入获得组件能力，组件作者则把接入规则稳定在 Starter 内部。

## 空白导入

应用使用 Starter 时，最常见的入口是空白导入。这个例子要证明的是：导入 Starter 的动作只让注册逻辑进入 Go-Spring，并不等于马上创建资源。

```go
import _ "github.com/go-spring/starter-gorm-mysql"
```

空白导入会触发 starter 包的 `init()`。在 `init()` 里，Starter 可以调用 `gs.Provide`、`gs.Module` 或 `gs.Group`，把 Bean 定义和模块函数放入 Go-Spring 的注册表。

但 Bean 的实例化仍然发生在应用启动和容器解析阶段，并且要满足对应条件。也就是说，Starter 的导入语义是“让组件接入规则可见”，不是“立刻打开数据库连接”。这点很重要，因为 Starter 往往会被多个项目共享，导入本身不应该产生不可控副作用。

## 默认单实例

很多基础设施组件有一个默认实例，例如默认数据库连接或默认 Redis 客户端。这里最适合用 `gs.Provide` 封装，因为构造函数、配置绑定、启用条件和销毁函数都可以挂在同一个 Bean 定义上。

下面的例子要证明的是：默认单实例可以由关键配置项触发，并在容器关闭时释放资源。

```go
func init() {
	gs.Provide(NewDB, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__").
		Destroy(CloseDB)
}

type Config struct {
	DSN string `value:"${dsn}"`
}

func NewDB(config Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(config.DSN), &gorm.Config{})
}

func CloseDB(db *gorm.DB) error {
	sqlDB, _ := db.DB()
	return sqlDB.Close()
}
```

`gs.TagArg("${spring.gorm}")` 表示把 `spring.gorm` 子树绑定到构造参数 `Config`。`Condition(gs.OnProperty("spring.gorm.dsn"))` 表示只有关键配置存在时才启用这个 Bean。`Name("__default__")` 给默认实例一个稳定名称，`Destroy(CloseDB)` 则把关闭逻辑交给容器生命周期。

因此，应用侧只需要提供配置。是否启用、怎样构造、叫什么名字、何时释放，都由 Starter 统一表达。

## 动态注册

有些组件不能只靠一个静态 Bean 定义表达。比如 Starter 需要读取配置后决定注册哪一种实现，或者需要根据一个开关注册一组相关 Bean。此时 `gs.Module` 更合适，因为模块函数会在应用启动时拿到已加载的配置存储和 Bean 注册入口。

下面的例子要证明的是：当注册结果依赖配置内容时，可以把判断放进模块函数里。

```go
func init() {
	gs.Module(
		gs.OnProperty("enable-mysql").HavingValue("true"),
		func(r gs.BeanProvider, p flatten.Storage) error {
			if s, _ := p.Value("enable-readonly"); s == "true" {
				r.Provide(NewReadOnlyDB)
			} else {
				r.Provide(NewDB)
			}
			return nil
		})
}
```

`gs.Module` 的条件先决定模块是否执行。模块执行后，函数内部可以读取 `flatten.Storage`，再通过 `gs.BeanProvider` 注册 Bean。这里的语义比普通 `Provide` 多了一层：注册动作本身也是配置驱动的。

所以，`Module` 适合封装“先看配置，再决定注册什么”的组件接入逻辑。它不应该被滥用成普通 Bean 注册的替代品；如果一个构造函数和条件已经能表达清楚，`Provide` 会更直接。

## 多实例配置

默认单实例不能覆盖所有组件。多数据源、多 Redis、多外部客户端这类场景，通常希望配置字典里的每个条目生成一个同类 Bean，并且字典 key 成为 Bean 名称。

下面的例子要证明的是：`gs.Group` 把“配置 Map -> 多个 Bean”的模式固定下来。

```go
func init() {
	gs.Group("${spring.gorm.instances}", NewDB, CloseDB)
}
```

对应配置通常写成字典。

```yaml
spring:
  gorm:
    instances:
      db1:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
      db2:
        dsn: "root:123456@tcp(localhost:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local"
```

`gs.Group` 要求 tag 使用 `${...}` 形式。它会读取 `spring.gorm.instances`，绑定成 `map[string]T`，然后对每个条目调用构造函数。Map 的 key 会成为 Bean 名称，Map 的 value 会作为构造参数传入。如果提供了销毁函数，生成的每个 Bean 都会绑定对应 Destroy 回调。

这让多实例 Starter 不需要自己重复写“遍历配置、命名 Bean、绑定关闭函数”的样板逻辑。配置结构也会更稳定：默认实例放在组件前缀下，多实例放在 `instances` 下。

## 配置与命名约定

Starter 的复用价值不只来自少写代码，更来自约定一致。官方 Starter 通常采用“默认单实例 + 可选多实例”的组织方式。

下面的组合证明的是：同一个组件可以同时提供默认实例和多实例入口，但两者要有清晰命名边界。

```go
func init() {
	gs.Provide(newClient, gs.TagArg("${spring.gorm}")).
		Condition(gs.OnProperty("spring.gorm.dsn")).
		Name("__default__")

	gs.Group("${spring.gorm.instances}", newClient, nil)
}
```

默认单实例使用组件前缀下的关键配置触发，例如 `spring.gorm.dsn`。多实例使用 `spring.gorm.instances` 这样的字典结构，每个字典 key 对应一个 Bean 名称。资源型组件应提供 Destroy 函数，让连接池、客户端或后台资源在容器关闭时释放。

这些约定让不同 Starter 在应用侧看起来一致。反过来，如果每个 Starter 都使用不同前缀、不同默认名称和不同多实例结构，组件封装只是把复杂性从业务代码移动到了配置里。

## 组件接入边界

Go-Spring 已经提供了一些常见基础设施 Starter，例如 `starter-gorm-mysql`、`starter-go-redis`、`starter-redigo` 和 `starter-pprof`。内置 HTTP Server 也可以看作框架自带的组件接入示例。

使用 Starter 时，边界要保持清楚：Starter 负责组件怎样被注册、怎样读取配置、什么时候启用、关闭时怎样释放；业务代码负责使用这些 Bean 完成业务流程。Starter 不应该把业务策略、租户规则或场景化编排强塞进通用组件包。

当一个组件只在单个项目里使用，而且初始化逻辑很短，直接在项目里 `Provide` 可能更清楚。只有当配置约定、生命周期和多项目复用开始出现重复时，Starter 才值得抽出来。

## Starter 机制

Starter 本质上是 Go-Spring 注册能力的封装。`Provide` 适合默认单实例，`Module` 适合配置驱动的动态注册，`Group` 适合从配置字典生成多实例。条件、配置绑定、命名和 Destroy 回调仍然沿用容器原有语义。

因此，Starter 解决的是“组件接入规则怎样复用”，不是替代业务代码。组件封装稳定之后，还需要测试体系来验证装配是否符合预期：哪些逻辑留在纯单测，哪些行为需要启动容器，外部依赖又该怎样隔离。
