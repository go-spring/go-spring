# Go-Spring 实战第 2 课 —— 配置绑定：Properties 映射 Go 代码

在上一篇中，Go-Spring 把不同来源、不同格式的配置统一到了 `Properties` 和 Path 语法，那么接下来就是这些配置如何映射到 Go 代码，这样才能真正开始影响应用行为。 // 这里是引子。

Go-Spring 把最常见的业务场景放到结构体标签上处理。如果绑定发生在模块化注册、批量创建 Bean 或 Starter 封装里，也可以手动调用 `Bind`。这两种方式共享同一套配置 path 和类型转换机制。 // 这里是前言

## value 标签

配置绑定最常用的方式，是在结构体字段上使用 `value` 标签。举个例子： // 首先讲最一般的情况

```go
type ServerConfig struct {
	Port      int           `value:"${port:=8080}"`
	Timeout   time.Duration `value:"${timeout:=30s}"`
	EnableSSL bool          `value:"${enable-ssl:=true}"`
	Endpoints []string      `value:"${endpoints}"`
}
```

value 标签的语法是 `value:"${key:=defaultValue}"`，其中 `key` 是配置 key 的 path，`:=defaultValue` 是可选的默认值。如果我们没有默认值并且配置也不存在，那么该字段就认为是必填字段，绑定的时候会失败。

我们也可以写成 `${:=default}` 形式，表示 key 虽然为空，但不从配置中查找值，而是直接使用默认值。这样，我们可以避免使用 Go 代码进行赋值，同时保留了对 key 的定义权。

## 嵌套结构体

如果配置绑定的目标字段本身是结构体，并且没有注册类型转换器，那么 Go-Spring 会递归绑定目标字段。这样，我们可以很自然的表达嵌套配置。举个例子： // 然后讲嵌套结构体的情况

```go
type DatabaseConfig struct {
	Host string `value:"${host}"`
	Port int    `value:"${port:=5432}"`
}

type AppConfig struct {
	DB DatabaseConfig `value:"${database}"`
}
```

对于 `DatabaseConfig` 的 `Host` 和 `Port` 字段，在单独使用时，它们分别使用 host 和 port 作为 key。但在表达 `AppConfig` 的 `DB` 字段时，它们分别需要对应到 `database.host` 和 `database.port`。此时，对应的配置文件如下：

```yaml
database:
  host: localhost
  port: 5432
```

## Bind 函数

当我们使用 Go-Spring 的 `Module` 模块化机制注册 Bean 时，通常需要根据获取的配置，来决定如何注册 Bean。举个例子： // 手动调用 Bind 函数的情况

```go
func init() {
	gs.Module(nil, func(r gs.BeanProvider, p flatten.Storage) error {
		var config ServerConfig
		if err := conf.Bind(p, &config, "${server}"); err != nil {
			return err
		}
		// 在这里完成使用 config 注册相关 Bean 的动作
		// 这里最好是一个完整的例子
		return nil
	})
}
```

// 在这里需要总结一下上面的例子，说明一下 `Bind` 函数的使用场景。

`Bind` 函数的签名如下：

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

其中 `storage` 是已经加载完成的配置存储，`target` 是绑定目标，`tag` 是可选的标签。
target 必须是指针类型，否则不能修改目标值。tag 支持完整的标签语法，可以设置默认值。
