# Go-Spring 实战第 3 课 —— 复杂类型的配置绑定：Duration、Time、Slice、Map

上一篇我们讲了配置绑定。在配置从 `Properties` 进入业务结构体以后，应用代码拿到的就不再是一组字符串，而是已经转换过的 Go 值。

但真实服务的配置不会只停留在 `string`、`int`、`bool` 等基础类型上。比如超时时间通常要绑定成 `time.Duration`，发布时间可能要绑定成 `time.Time`，服务地址可能是一组列表，多数据源、多 Redis、多 HTTP 客户端又经常按名称组织成一组实例。

如果这些复杂结构都需要交给业务代码自己解析，那么配置绑定只能算完成了一半。所以，Go-Spring 的配置绑定不只负责把单个 key 转成基础类型，还要支持时间、枚举、slice、map 等更复杂的类型。

## 基础类型

Go-Spring 开箱支持以下常见的基础类型。

- 支持 `bool` 类型，支持 `true`/`false`、`1`/`0`、`t`/`f` 等多种常见写法。
- 支持整数类型，支持 `int`、`int8`、`int16`、`int32`、`int64` 以及对应的无符号类型。
- 支持浮点类型，支持 `float32`、`float64`，支持科学计数法。
- 支持 `string` 类型。

这些基础类型可以直接写在配置结构体字段上。

```go
type ServerConfig struct {
	Host      string `value:"${host:=localhost}"`
	Port      int    `value:"${port:=8080}"`
	EnableTLS bool   `value:"${enable-tls:=false}"`
}
```

## 时间类型

时间和时长是最常见的复杂配置。它们虽然经常以字符串形式写在配置文件里，但业务代码真正需要的是 `time.Duration` 或 `time.Time`。Go-Spring 内置了这两类转换器。

| 类型 | 用途 | 示例 |
|------|------|------|
| `time.Duration` | 时间时长 | `30s`、`5m`、`1h30m` |
| `time.Time` | 时间点 | `2006-01-02`、`2006-01-02 15:04:05` |

下面这个配置结构体把超时配置直接声明成 `time.Duration`，使用方式和基础类型完全一致。

```go
type Config struct {
	Host    string        `value:"${host:=localhost}"`
	Port    int           `value:"${port:=8080}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}
```

当我们在配置里写 `timeout=5m` 时，绑定后的 `Timeout` 字段就是 `5 * time.Minute`。业务代码不需要自己调用 `time.ParseDuration`，也不需要在每个模块里约定时长的表达格式。

## 自定义转换器

内置转换器覆盖不了所有业务类型。像日志级别、运行模式、灰度状态，以及第三方库里的专用类型，往往都有自己的字符串格式。这时候需要通过自定义转换器，把这些类型接入到 Go-Spring 的配置绑定体系里面来。

下面这个示例展示了自定义转换器的使用。这里用一个转换器把字符串转换成了枚举类型 `Status`。首先我们需要注册这个转换器。

```go
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

func init() {
	conf.RegisterConverter(func(s string) (Status, error) {
		switch s {
		case "disabled", "off":
			return StatusDisabled, nil
		case "enabled", "on":
			return StatusEnabled, nil
		default:
			v, err := strconv.Atoi(s)
			if err != nil {
				return 0, err
			}
			return Status(v), nil
		}
	})
}
```

然后字段就可以直接声明成 `Status` 类型。

```go
type AppConfig struct {
	Status Status `value:"${app.status:=enabled}"`
}
```

此时，如果配置是 `app.status=on`，那么绑定后的 `Status` 就是 `StatusEnabled`。如果配置值既不是约定字符串，也不能转成数字，转换器会返回错误。这个错误会继续向外传播，应用也会在启动阶段失败。

注意，转换器应该在 `init` 阶段注册完成，这样业务 Bean 开始绑定配置之前，Go-Spring 已经知道该如何处理这个类型。

## Slice

我们可以把一组相同类型的配置绑定成 slice。比如应用白名单、端口列表、下游端点列表，它们在业务代码里天然就是 `[]string`、`[]int` 或者 `[]EndpointConfig`。

对于支持列表的配置格式，可以直接写成自然的层级结构。

```yaml
apps:
  - a
  - b
  - c
```

对应的结构体只需要把字段声明成 slice。

```go
type AppConfig struct {
	Apps []string `value:"${apps}"`
}
```

绑定之后，`Apps` 的值就是 `[]string{"a", "b", "c"}`。

如果使用 properties 格式，也可以使用下标表达列表。

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

这种写法和 YAML 列表的语义一致，只是需要手动维护下标。

> Go-Spring 要求下标从 `0` 开始并且连续。如果缺失了中间的下标，Go-Spring 只会绑定到最后连续的下标位置。

对于短字符串列表，我们还可以使用逗号分隔的写法。Go-Spring 会按逗号拆分，并对每个元素继续执行目标类型转换。

```properties
apps=a,b,c
```

这种写法更加紧凑，也适合环境变量或命令行参数这类不方便表达层级列表的来源。

如果列表元素本身是结构体，slice 仍然可以承接。比如下面的配置是把多个下游端点写成列表，每个元素都有自己的字段。

```yaml
endpoints:
  - name: user
    url: https://user.example.com
    timeout: 500ms
  - name: order
    url: https://order.example.com
    timeout: 1s
```

对应的结构体如下。

```go
type EndpointConfig struct {
	Name    string        `value:"${name}"`
	URL     string        `value:"${url}"`
	Timeout time.Duration `value:"${timeout:=1s}"`
}

type ClientConfig struct {
	Endpoints []EndpointConfig `value:"${endpoints}"`
}
```

绑定 `Endpoints[0]` 时，Go-Spring 会把父路径 `endpoints[0]` 和元素字段上的 `name`、`url`、`timeout` 组合起来。也就是说，`Endpoints[0].Name` 会读取 `endpoints[0].name`。

换句话说，slice 负责表达“有多个”，结构体字段负责表达“每个元素内部有哪些配置”。

## Map

有些配置不是单纯的有序列表，而是按名字区分的一组实例。比如 `master` 和 `slave` 数据源、多个 Redis 客户端、不同业务线的 HTTP 客户端。

这类配置适合绑定成 `map[string]T`。比如下面这组 properties 把 `master` 和 `slave` 放在路径中间，那么这一层就会成为 map 的 key。

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

对应的 Go 结构可以这样写。

```go
type DBConnectionConfig struct {
	Host string `value:"${host}"`
	Port int    `value:"${port:=5432}"`
}

type DatabaseConfig struct {
	Connections map[string]DBConnectionConfig `value:"${database.connections}"`
}
```

在实现配置绑定时，Go-Spring 会收集 `database.connections` 下面的第一层子 key。于是 `master` 和 `slave` 会成为 `Connections` 里的两个条目。

```go
cfg.Connections["master"].Host // localhost
cfg.Connections["slave"].Port  // 5433
```

接着，对 map value 的绑定会沿用结构体规则。比如对 `master` 这个条目来说，`Host` 字段实际读取的是 `database.connections.master.host`，`Port` 字段实际读取的是 `database.connections.master.port`。

`map[string]T` 中的 `T` 既可以是基础类型，也可以是结构体，还可以继续组合其他复杂类型。

## 复杂类型绑定

复杂类型绑定让业务结构可以自然的承接配置结构，而自定义转换器则大大增加了 Go-Spring 的表达能力。
