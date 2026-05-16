# Go-Spring 实战第 2 课 —— 配置绑定：Properties 映射 Go 代码

在上一篇中，Go-Spring 把不同来源、不同格式的配置统一到了 `Properties` 和 Path 语法，那么接下来就是这些配置如何映射到 Go 代码，这样才能真正开始影响应用行为。

Go-Spring 把最常见的业务场景放到结构体标签上处理。如果绑定发生在模块化注册、批量创建 Bean 或 Starter 封装里，也可以手动调用 `Bind`。这两种方式共享同一套配置 path 和类型转换机制。

## value 标签

配置绑定最常用的方式，是在结构体字段上使用 `value` 标签。举个例子：

```go
type ServerConfig struct {
	Port      int           `value:"${port:=8080}"`
	Timeout   time.Duration `value:"${timeout:=30s}"`
	EnableSSL bool          `value:"${enable-ssl:=true}"`
	Endpoints []string      `value:"${endpoints}"`
}
```

value 标签的语法是 `value:"${key:=defaultValue}"`，其中 `key` 是配置 key 的 path，`:=defaultValue` 是可选的默认值。如果没有默认值且配置不存在，则该字段就是必填字段，绑定阶段会失败。

如果写成 `${:=default}`，表示 key 虽然为空，但不从配置中查找值，而是直接使用默认值。这样，我们可以避免使用 Go 代码进行赋值，同时保留了对 key 的定义权。

## 嵌套结构体

如果配置绑定的目标字段本身还是结构体，并且没有注册转换器，那么 Go-Spring 会继续递归绑定字段。这样，我们可以很自然的表达嵌套配置。看下面的例子：

```go
type DatabaseConfig struct {
	Host string `value:"${host}"`
	Port int    `value:"${port:=5432}"`
}

type AppConfig struct {
	DB DatabaseConfig `value:"${database}"`
}
```

对于 `DatabaseConfig` 的 `Host` 和 `Port` 字段，在单独使用时，它们分别使用 host 和 port 作为 key。但在表达 `AppConfig` 的 `DB` 字段时，它们分别对应到 `database.host` 和 `database.port`。对应的配置文件如下：

```yaml
database:
  host: localhost
  port: 5432
```

## Bind 函数

当我们在 `Module` 中注册 Bean 时，需要获取配置，然后根据配置来注册 Bean。举个例子：

```go
func init() {
	gs.Module(nil, func(r gs.BeanProvider, p flatten.Storage) error {
		var config ServerConfig
		if err := conf.Bind(p, &config, "${server}"); err != nil {
			return err
		}
		// 在这里完成使用 config 注册相关 Bean 的动作
		return nil
	})
}
```

`Bind` 的函数签名如下：

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

其中 `storage` 是已经加载完成的配置存储，`target` 是绑定目标，必须传指针，`tag` 支持完整的标签语法。
