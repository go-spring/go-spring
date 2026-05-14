# Go-Spring 实战第 3 课：复杂配置绑定：Duration、Time、Slice、Map 怎么落到 Go 类型

有了 Go-Spring 的结构体绑定以后，我们已经可以比较自然地把配置放进 Go 类型里。但如果只把配置当字符串读，业务代码很快就会重新长出一堆手动解析逻辑：超时时间要 `time.ParseDuration`，时间点要单独解析，列表和连接字典还要自己拆分。

真实配置很少只会有字符串和整数。超时时间、时间点、枚举、列表、对象数组、连接字典都很常见。换句话说，如果绑定层只能处理简单值，业务代码就会被迫接手大量解析工作。Go-Spring 的配置绑定围绕 Go 类型系统展开，目标是让配置结构自然映射到业务结构体，而不是把解析细节散落到业务代码里。

## 基础类型

我们先看最基础的部分。Go-Spring 开箱支持常见基础类型：

- `bool`：支持 `true`/`false`、`1`/`0`、`t`/`f` 等写法。
- 整数类型：支持 `int`、`int8`、`int16`、`int32`、`int64` 以及对应无符号类型。
- 浮点类型：支持 `float32`、`float64`，包括科学计数法。
- `string`：按原始字符串绑定。

如果目标类型是结构体，且没有对应转换器，Go-Spring 会递归绑定结构体字段。这也是嵌套配置能够自然落到嵌套结构体上的原因。

## 内置特殊转换器

基础类型之外，Go-Spring 内置了常用类型转换器，最典型的是：

| 类型 | 用途 | 示例 |
|------|------|------|
| `time.Duration` | 时间时长 | `30s`、`5m`、`1h30m` |
| `time.Time` | 时间点 | `2006-01-02`、`2006-01-02 15:04:05` |

例如：

```go
type Config struct {
	Timeout time.Duration `value:"${timeout:=30s}"`
}
```

配置里写 `timeout=5m`，绑定后就是 `5 * time.Minute`。这样业务代码拿到的已经是可直接使用的类型，不需要自己解析。

## 自定义类型转换器

当业务里有枚举、第三方库类型或特殊字符串格式时，我们可以注册自定义转换器：

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

注册后可以直接用于配置绑定：

```go
type AppConfig struct {
	Status Status `value:"${app.status:=enabled}"`
}
```

转换器是全局注册的，因此应在程序启动前完成注册，通常放在 `init()` 中。这样 Go-Spring 容器启动时，绑定阶段就已经知道如何把字符串转换成目标类型。

## Slice 和 Array

接着看集合类型。这里先别急着把它理解成另一套规则，切片支持两种输入方式。

多行展开格式适合复杂元素：

```yaml
apps:
  - a
  - b
  - c
```

展开后是：

```properties
apps[0]=a
apps[1]=b
apps[2]=c
```

简单字符串列表可以使用逗号分隔：

```properties
apps=a,b,c
```

两种方式最终都可以绑定到 `[]string{"a", "b", "c"}`。

这里仍然回到第一篇的 path 模型：数组只是通过下标进入同一棵配置树，并没有变成一套额外规则。

## Map 绑定

Map 绑定会收集指定 path 下的所有子节点。例如：

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

如果目标类型是 `map[string]DatabaseConfig`，那么 `connections["master"]` 和 `connections["slave"]` 会分别绑定到对应配置。

这种方式非常适合多数据源、多 Redis、多 HTTP 客户端这类按名称管理的实例配置。也就是说，我们把实例名称放在配置路径里，把实例结构交给 Go 类型系统承接。

## 复杂类型不是额外负担

复杂类型绑定的价值，是让业务结构自然承接配置结构。这样超时时间、连接列表、命名实例和嵌套对象都可以留在类型系统里，而不是退回字符串解析和手工拼装。

类型能接住配置以后呢，下一步要补上的就是合法性：绑定成功不代表配置一定可用，我们还需要在启动阶段把错误挡住。
