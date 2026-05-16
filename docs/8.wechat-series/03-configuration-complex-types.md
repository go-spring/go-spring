# Go-Spring 实战第 3 课 —— 复杂类型的配置绑定：Duration、Time、Slice、Map

真实的配置中很少只会有字符串和整数，时间、时长、枚举、列表、对象数组、字典等等，都是很常见的。如果 Go-Spring 只支持整数和字符串这些简单类型，那么我们就需要在业务代码中手动处理这些复杂类型。这必然是不行的。因此，Go-Spring 需要支持更复杂的数据类型，这样才能让配置结构自然映射到业务结构体，而不是把解析细节散落到业务代码里。

## 基础类型

复杂数据类型的绑定首先建立在稳定的基础类型转换之上。Go-Spring 开箱支持下列常见基础类型：

- 支持 `bool` 类型，支持 `true`/`false`、`1`/`0`、`t`/`f` 等多种常见写法。
- 支持整数类型，支持 `int`、`int8`、`int16`、`int32`、`int64` 以及对应的无符号类型。
- 支持浮点类型，支持 `float32`、`float64`，支持科学计数法。
- 支持 `string` 类型。

## 时间类型

Go-Spring 内置了对 `time.Duration` 和 `time.Time` 类型的转换器。

| 类型 | 用途 | 示例 |
|------|------|------|
| `time.Duration` | 时间时长 | `30s`、`5m`、`1h30m` |
| `time.Time` | 时间点 | `2006-01-02`、`2006-01-02 15:04:05` |

下面这个示例展示了 `time.Duration` 类型的使用，看起来和基础类型没有区别。

```go
type Config struct {
	Host    string        `value:"${host:=localhost}"`
	Port    int           `value:"${port:=8080}"`
	Timeout time.Duration `value:"${timeout:=30s}"`
}
```

当我们在配置里写 `timeout=5m` 的时候，经过配置绑定后 `Timeout` 字段的值就是 `5 * time.Minute`。也就是说，业务代码拿到的已经是可直接使用的类型，不需要自己手动解析。

## 类型转换器

当业务配置里有枚举、第三方库类型或特殊的字符串格式时，我们可以使用自定义类型转换器。

下面这个示例展示了自定义类型转换器的使用。我们使用一个类型转换器将字符串转换成了枚举类型 `Status`。首先我们需要注册这个类型转换器。

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

注意，我们一般在 init 函数里注册类型转换器，这样在业务代码生效前就可以使用了。然后我们将字段声明为自定义的 `Status` 类型。

```go
type AppConfig struct {
	Status Status `value:"${app.status:=enabled}"`
}
```

// 这里需要补充一些解释性的内容。

## Slice & Array

对于 slice 和 array 类型，我们可以使用 yaml、toml 等支持列表的配置格式，这样可以避免使用下标。

```yaml
apps:
  - a
  - b
  - c
```

当然也可以使用 properties 格式，但是就需要手动维护下标。两种方式是等价的，选择哪种都可以，方便为主。

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

对于短字符串列表，Go-Spring 支持使用逗号分隔的写法，这样写起来更加紧凑。

```properties
apps=a,b,c
```

上面所有方式都可以绑定到 `[]string` 类型，值为 `[]string{"a", "b", "c"}`。

// 这里没有对结构体的展示，我觉得可以加一下。

## Map

字段为 map 类型时，表示收集指定 path 下的所有子节点。例如，下面这组 key 把 `master` 和 `slave` 放在路径中间，这一层就会成为 map 的 key。

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

我们可以将上述配置绑定到如下字段，todo 添加类型和字段。

然后解释 `connections["master"]` 和 `connections["slave"]` 会分别绑定到对应配置。

这种方式非常适合多数据源、多 Redis、多 HTTP 客户端这类按名称管理的实例配置。

## 复杂类型绑定的边界是类型转换而不是业务决策

复杂类型绑定的价值，是让业务结构自然承接配置结构。这样超时时间、连接列表、命名实例和嵌套对象都可以留在类型系统里，而不是退回字符串解析和手工拼装。

不过，绑定成功只说明配置能落到类型上，不代表这个值一定可用。配置合法性还需要在启动阶段继续兜住。
