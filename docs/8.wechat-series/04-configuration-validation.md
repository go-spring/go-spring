# Go-Spring 实战第 4 课 —— 配置校验：使用 expr 标签拦截非法配置

前面几篇我们解决了配置怎样表达、怎样绑定，以及复杂类型怎样进入 Go 结构体的问题。配置绑定完成以后，业务代码拿到的已经不是一组字符串，而是 `int`、`time.Duration`、slice、map 或者自定义类型。

但绑定成功只能说明“值可以转成目标类型”，并不等于“值在业务上可用”。端口号 `99999` 可以绑定成整数，但它不是合法端口。日志级别 `inf` 可以绑定成字符串，但运行时不会得到预期的日志行为。超时时间 `0s` 也可以绑定成 `time.Duration`，但对 HTTP 客户端来说可能意味着完全不同的语义。

这时候问题已经变了。如果说配置绑定回答的是“能不能转过去”的问题，那么配置校验回答的是“转过去以后能不能用”的问题。如果这类错误拖到运行期才暴露，排查时看到的往往是连接失败、请求堆积、日志缺失这类间接现象，而不是最初的那条配置错误信息。

所以，Go-Spring 把配置校验放在绑定链路里进行处理。在字段完成转换以后，Go-Spring 会根据字段上的校验规则继续检查配置值。如果校验失败，那么绑定过程就会返回错误，然后应用就会在启动阶段停下来。

## expr 标签

Go-Spring 基于 `expr-lang/expr` 库提供了表达式校验能力。使用方式是在字段上添加 `expr` 标签，然后在表达式里用 `$` 表示当前字段绑定后的值。

这时，表达式看到的不是原始字符串，而是已经完成类型转换的 Go 值。端口字段上的 `$` 是 `int`，超时字段上的 `$` 是 `time.Duration`，字符串字段上的 `$` 是 `string`。也就是说，校验规则可以直接围绕业务真正使用的类型来写。

下面这组字段覆盖了几类常见约束。读代码时可以重点看 `value` 和 `expr` 是怎样配合使用的。

```go
type ServerConfig struct {
	// 端口号必须在 1-65535 范围内
	Port int `value:"${server.port:=8080}" expr:"$ > 0 && $ < 65536"`

	// 日志级别必须是 debug/info/warn/error 中的一个
	LogLevel string `value:"${log.level:=info}" expr:"$ in ['debug', 'info', 'warn', 'error']"`

	// 用户名必须是 3-31 个字符，并且以小写字母开头
	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{2,30}$'"`

	// 超时时间必须大于等于 1s
	Timeout time.Duration `value:"${timeout:=5s}" expr:"$ >= duration(\"1s\")"`

	// 重试次数必须在 0-10 范围内
	RetryCount int `value:"${retry:=3}" expr:"$ >= 0 && $ <= 10"`
}
```

Go-Spring 允许将字段校验的规则写在字段声明旁边。这样，后续修改配置路径、默认值或字段类型时，就不需要再到业务代码深处寻找对应的判断逻辑。

表达式要求必须返回 `bool`。如果表达式结果是 `true`，则校验通过；如果结果是 `false`，或者表达式本身写错了，那么校验失败。然后，配置错误会停在启动阶段，而不是让一个带着错误参数的对象进入运行期。

## 必填字段

配置校验可能会被理解成 `required` 检查，但在 Go-Spring 里，这两件事分属不同层次。

如果字段没有默认值，同时配置来源里也没有对应 key，那么 Go-Spring 会在绑定阶段就失败。比如下面这个字段没有给默认值。

```go
type AuthConfig struct {
	Username string `value:"${auth.username}"`
}
```

当 `auth.username` 配置不存在时，问题不是用户名是否合法，而是配置值根本没有进入字段。

只有当字段已经成功绑定，才会到 `expr` 判断业务规则。比如 `auth.username` 存在但为空字符串，或者虽然有值但不符合命名规则，这时候表达式就有意义。

```go
type AuthConfig struct {
	Username string `value:"${auth.username}" expr:"$ matches '^[a-z][a-z0-9_]{2,30}$'"`
}
```

这个边界很重要。也就是说，配置的存在性由绑定负责，配置的合法性由校验负责。两层职责分开以后，配置结构会更清楚，错误也更容易定位。

## 自定义校验函数

Go-Spring 内置的表达式规则适合描述范围、枚举、正则和简单组合条件。但真实业务里经常会出现更有领域意味的规则，比如端口是否允许暴露、租户编号是否符合内部格式、某个名称是否符合团队约定。

如果这些规则直接塞进表达式，字段标签会变得很长，也不利于复用。因此，Go-Spring 提供了 `conf.RegisterValidateFunc` 函数，可以把这类规则注册成表达式里的函数。

自定义校验函数要求返回值是 `bool`。函数返回 `true` 表示校验通过，返回 `false` 表示校验失败。

```go
func init() {
	// validPort 校验端口号是否在 1-65535 范围内
	conf.RegisterValidateFunc[int]("validPort", func(port int) bool {
		return port > 0 && port < 65536
	})

	// minLength 校验字符串长度是否大于等于 3
	conf.RegisterValidateFunc[string]("minLength", func(s string) bool {
		return len(s) >= 3
	})
}
```

注册完成以后，字段上的 `expr` 标签就可以像调用内置函数一样调用这些校验函数。

```go
type ServerConfig struct {
	Port     int    `value:"${port}" expr:"validPort($)"`
	Username string `value:"${auth.username}" expr:"minLength($)"`
	APIKey   string `value:"${security.api-key}" expr:"minLength($) && $ contains 'prod-'"`
}
```

自定义校验函数的内部最好保持纯粹，不要访问网络、打开文件或依赖会变化的外部状态。因为配置绑定发生在启动链路里，校验函数一旦变重，启动过程也会跟着变慢，错误来源也会变得不稳定。

另外，`RegisterValidateFunc` 应该在 `init` 阶段完成注册。这样业务配置开始绑定时，Go-Spring 已经能在表达式环境里找到对应函数。

## 业务判断

需要注意的是，`expr` 标签解决的是字段级问题。它适合表达“当前字段的值是否可用”，比如范围、格式、枚举、最小长度和最大长度。

但有些规则不是单个字段能完整判断的。比如最小连接数不能大于最大连接数，读超时应该小于整体请求超时，某个开关打开后必须同时配置一组下游地址。这些规则需要同时看多个字段，甚至需要结合已经创建好的依赖对象。

这类校验不适合硬塞进字段标签。更合适的位置是 Bean 的初始化阶段。到了这个阶段，配置已经完成绑定和字段级校验，业务对象也就可以围绕着完整结构进行一致性检查。

```go
type PoolConfig struct {
	MinIdle int `value:"${min-idle:=1}" expr:"$ >= 0"`
	MaxOpen int `value:"${max-open:=16}" expr:"$ > 0"`
}

type Client struct {
	Config PoolConfig `value:"${pool}"`
}

func (c *Client) Init() error {
	if c.Config.MinIdle > c.Config.MaxOpen {
		return fmt.Errorf("pool.min-idle cannot be greater than pool.max-open")
	}
	return nil
}

func init() {
	gs.Provide(new(Client)).InitMethod("Init")
}
```

上面的例子中，`MinIdle` 不能小于 0，`MaxOpen` 必须大于 0，这些是可以放在 `expr` 里的简单规则。但 `MinIdle <= MaxOpen` 是两个字段之间的关系，放到初始化阶段会更加自然。

也就是说，配置校验不是为了替代业务初始化，而是先把局部、确定、字段内的错误拦下来。等对象进入初始化阶段以后，再处理跨字段、跨对象或者依赖外部资源的判断。

## 配置校验

配置校验的价值不在于少写几行 `if` 代码，而是将校验和绑定放在一起，执行也在一起。
