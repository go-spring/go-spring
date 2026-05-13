# 复杂类型绑定

## 本篇要解决的问题

真实配置很少只有字符串和整数。超时时间、时间点、枚举、列表、对象数组、连接字典都很常见。Go-Spring 的配置绑定能力围绕 Go 类型系统展开，目标是让配置结构自然映射到业务结构体，而不是在业务代码里手动解析字符串。

## 基础类型

Go-Spring 开箱支持常见基础类型：

- `bool`：支持 `true`/`false`、`1`/`0`、`t`/`f` 等写法。
- 整数类型：支持 `int`、`int8`、`int16`、`int32`、`int64` 以及对应无符号类型。
- 浮点类型：支持 `float32`、`float64`，包括科学计数法。
- `string`：按原始字符串绑定。

如果目标类型是结构体，且没有对应转换器，框架会递归绑定结构体字段。

## 内置特殊转换器

Go-Spring 内置了常用类型转换器，最典型的是：

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

配置里写 `timeout=5m`，绑定后就是 `5 * time.Minute`，业务代码不需要自己解析。

## 自定义类型转换器

当业务里有枚举、第三方库类型或特殊字符串格式时，可以注册自定义转换器：

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

转换器是全局注册的，应在程序启动前完成注册，通常放在 `init()` 中。

## Slice 和 Array

切片支持两种输入方式。

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

## Map 绑定

Map 绑定会收集指定 path 下的所有子节点。例如：

```properties
database.connections.master.host=localhost
database.connections.master.port=5432
database.connections.slave.host=replica
database.connections.slave.port=5433
```

如果目标类型是 `map[string]DatabaseConfig`，那么 `connections["master"]` 和 `connections["slave"]` 会分别绑定到对应配置。

这种方式非常适合多数据源、多 Redis、多 HTTP 客户端这类按名称管理的实例配置。

## 使用边界

本篇只讨论绑定时如何表达复杂数据结构，不讨论配置从哪里来，也不讨论不同来源之间如何合并。配置来源和优先级会在后续文章单独展开。

