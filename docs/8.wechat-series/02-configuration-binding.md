# 配置绑定的核心机制

## 本篇要解决的问题

配置只有被绑定到 Go 代码里，才真正进入应用逻辑。Go-Spring 提供两种主要绑定方式：

- 结构体标签绑定，适合绝大多数业务代码。
- 手动 `Bind` 函数绑定，适合模块化注册、批量创建 Bean 等更底层的场景。

这两种方式共享同一套配置 path 和类型转换机制，只是使用位置不同。

## 结构体标签绑定

最常用的方式是在结构体字段上使用 `value` 标签：

```go
type ServerConfig struct {
	Port      int           `value:"${port:=8080}"`
	Timeout   time.Duration `value:"${timeout:=30s}"`
	EnableSSL bool          `value:"${enable-ssl:=true}"`
	Endpoints []string      `value:"${endpoints}"`
}

type App struct {
	Config ServerConfig `value:"${server}"`
}
```

`value:"${key:=defaultValue}"` 可以拆成三部分：

- `key` 是配置 path。
- `:=defaultValue` 是可选默认值。
- 如果没有默认值且配置不存在，该字段就是必填字段，绑定阶段会失败。

上面的 `App.Config` 使用 `${server}` 作为前缀，因此 `ServerConfig.Port` 对应的是 `server.port`，`ServerConfig.Timeout` 对应的是 `server.timeout`。

如果写成 `${:=default}`，表示 key 为空，不从配置中查找值，而是直接使用默认值。这适合需要保留标签形式、但当前值固定的场景。

## 字段映射方式

结构体绑定不是把字段名简单拼接到配置 key 上，而是以 `value` 标签作为显式声明。这样可以避免字段名变化影响配置协议，也能明确表达默认值和必填语义。

没有内置转换器的结构体会递归绑定字段，因此可以自然表达嵌套配置：

```go
type DatabaseConfig struct {
	Host string `value:"${host}"`
	Port int    `value:"${port:=5432}"`
}

type AppConfig struct {
	DB DatabaseConfig `value:"${database}"`
}
```

对应配置：

```yaml
database:
  host: localhost
  port: 5432
```

## 手动 Bind 函数绑定

在普通业务代码中，结构体标签绑定通常已经足够。手动 `Bind` 更多出现在 `Module` 这类注册逻辑里：模块先读取配置，再根据配置注册一个或多个 Bean。

```go
func init() {
	gs.Module(nil, func(r gs.BeanProvider, p flatten.Storage) error {
		var config ServerConfig
		if err := conf.Bind(p, &config, "${server}"); err != nil {
			return err
		}
		// 使用 config 注册相关 Bean
		return nil
	})
}
```

`Bind` 的函数签名是：

```go
func Bind(storage flatten.Storage, target any, tag ...string) error
```

参数含义：

- `storage`：已经加载完成的配置存储。
- `target`：绑定目标，必须传指针。
- `tag`：可选绑定 path，支持完整标签语法；不传时绑定整个配置。

## 使用建议

业务 Bean 的配置优先使用结构体标签绑定，因为依赖关系和配置协议都直接声明在类型上，更容易阅读和测试。

需要根据配置动态注册 Bean、批量生成实例或封装 Starter 时，再使用 `conf.Bind`。这类代码通常位于模块注册层，而不是业务处理路径。

## 下一篇

本篇只讲绑定入口。下一篇会展开绑定中的类型系统，包括基础类型、特殊转换器、自定义转换器、slice、array 和 map。

