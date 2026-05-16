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

// 这里需要补充一些内容，比较好。

## Slice & Array

接着看集合类型。切片仍然沿用 path 模型，只是输入方式可以有两种。

如果列表元素未来可能变复杂，可以用 YAML 多行结构表达每个元素。这个写法保留了数组元素的位置，后续对象数组也能沿同一套规则扩展。

```yaml
apps:
  - a
  - b
  - c
```

进入 `Properties` 后，它会被展开为带下标的路径。

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

如果只是短字符串列表，也可以直接使用逗号分隔，写起来更紧凑。

```properties
apps=a,b,c
```

两种方式最终都可以绑定到 `[]string{"a", "b", "c"}`。

这里仍然回到第一篇的 path 模型，即数组只是通过下标进入同一棵配置树，并没有变成一套额外规则。

## Map 更适合承接按名称管理的实例

Map 绑定会收集指定 path 下的所有子节点。下面这组 key 把 `master` 和 `slave` 放在路径中间，这一层就会成为 map 的 key。

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

如果目标类型是 `map[string]DatabaseConfig`，那么 `connections["master"]` 和 `connections["slave"]` 会分别绑定到对应配置。

这种方式非常适合多数据源、多 Redis、多 HTTP 客户端这类按名称管理的实例配置，因为实例名称放在配置路径里，实例结构又由 Go 类型系统承接，两边刚好能对上。

## 复杂类型绑定的边界是类型转换而不是业务决策

复杂类型绑定的价值，是让业务结构自然承接配置结构。这样超时时间、连接列表、命名实例和嵌套对象都可以留在类型系统里，而不是退回字符串解析和手工拼装。

不过，绑定成功只说明配置能落到类型上，不代表这个值一定可用。配置合法性还需要在启动阶段继续兜住。
