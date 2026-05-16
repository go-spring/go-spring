# Go-Spring 实战第 2 课 —— 配置绑定：Properties 映射到 Go 类型

上一篇讲了 Go-Spring 的统一配置模型，把配置收敛到了 `Properties` 和 Path 语法。这样一来，不同格式、不同来源的配置就进入了同一棵配置树，但业务代码还不能直接使用这些配置。因为在线服务真正需要的，通常不是某个字符串形式的 key，而是映射到 Go 类型上的配置结构。

如果每个 Bean 都自己从 `Properties` 里取值、转换类型、处理默认值，那么配置逻辑很快就会散落在业务代码中。这不是我们希望的。因此 Go-Spring 必须能够根据 path 找到配置值，然后按照目标字段类型完成转换，最后把结果写入 Go 对象。

Go-Spring 提供了两种配置绑定方式。一种适合日常业务代码，即在结构体字段上使用 `value` 标签。另一种适合模块注册、Starter 封装或批量创建 Bean，即手动调用 `conf.Bind` 函数。这两种方式共享同一套 path、默认值和类型转换规则，底层实现并无不同。

## value 标签

配置绑定最常见的场景，是一个 Bean 或者配置结构体需要从配置树中取一组字段。此时我们可以把绑定关系写在字段的 `value` 标签上，这样可以在一个地方统一表达配置 key、默认值和目标类型。

```go
type ServerConfig struct {
	Host      string `value:"${host:=0.0.0.0}"`
	Port      int    `value:"${port:=8080}"`
	EnableTLS bool   `value:"${enable-tls:=false}"`
}
```

`value` 标签的基本形式是 `value:"${key:=defaultValue}"`。其中 `key` 是配置 path，`:=defaultValue` 是可选默认值。如果没有默认值，并且配置里也找不到这个 key，那么这个字段会被视为必填字段，绑定阶段会返回错误。

默认值也可以不绑定任何 key，写成 `${:=default}`。这种写法不会从配置树中查找值，而是直接使用标签中的默认值。它适合用于表达一个固定的初始值，同时仍然让这个值留在配置绑定的语义里，而不是散落在 Go 代码中进行额外的赋值。

## 嵌套结构体

字段 value 标签里的 key 不是在任何情况下都表示完整的配置路径。如果目标字段是结构体，并且没有对应的类型转换器，Go-Spring 就会把父字段的 path 作为前缀，和子结构体字段的 path 组合成完整的配置路径。这样子结构体可以保持局部命名，在放到不同父路径下面时能够复用。

```go
type DatabaseConfig struct {
	Host string `value:"${host}"`
	Port int    `value:"${port:=5432}"`
}

type AppConfig struct {
	DB DatabaseConfig `value:"${database}"`
}
```

在 `DatabaseConfig` 内部，`Host` 和 `Port` 只关心 `host`、`port`。当它作为 `AppConfig.DB` 出现时，父字段上的 `${database}` 会把实际查找路径变成 `database.host` 和 `database.port`。

对应的 YAML 可以保持自然的层级结构。

```yaml
database:
  host: localhost
  port: 5432
```

## Bind 函数

结构体标签适合 Bean 已经进入容器生命周期后的字段绑定。但 Starter 或者模块注册经常要先读取配置，然后再决定注册哪些 Bean，或者把同一份配置传给多个构造函数。这个阶段还没有现成字段可以让容器自动注入，所以更适合手动调用 `conf.Bind`。

下面的代码表示模块在 `bookprice.base-url` 存在时才能启用。模块函数先把 `bookprice` 前缀下的配置绑定到 `BookPriceConfig`，然后再把配置作为构造参数用于注册客户端 Bean。

```go
type BookPriceConfig struct {
	BaseURL string `value:"${base-url}"`
	Trace   bool   `value:"${trace:=false}"`
}

type Client struct {
	baseURL string
	trace   bool
}

func NewClient(cfg BookPriceConfig) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		trace:   cfg.Trace,
	}
}

func init() {
	gs.Module(gs.OnProperty("bookprice.base-url"), func(r gs.BeanProvider, p flatten.Storage) error {
		var cfg BookPriceConfig
		if err := conf.Bind(p, &cfg, "${bookprice}"); err != nil {
			return err
		}
		r.Provide(NewClient, gs.ValueArg(cfg))
		return nil
	})
}
```

对应配置如下。

```yaml
bookprice:
  base-url: https://price.example.com
  trace: true
```

这里的关键点是绑定发生在模块函数内部。`p flatten.Storage` 是 Go-Spring 已经加载并合并后的配置存储，`conf.Bind(p, &cfg, "${bookprice}")` 表示只绑定 `bookprice` 这棵子树。绑定成功后，模块可以使用 `cfg` 注册一个或多个 Bean。

`conf.Bind` 的函数签名如下。

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

`storage` 是配置存储，`target` 是绑定目标，通常传入非 nil 指针。`tag` 是可选绑定标签，不传时默认从根路径进行绑定。这个标签仍然支持默认值语法。
